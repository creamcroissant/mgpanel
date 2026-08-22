package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/core"

	)
// NewSwitcher creates a Switcher with default dependencies where omitted.
func NewSwitcher(opts SwitcherOptions) (*Switcher, error) {
	if opts.CoreManager == nil {
		return nil, fmt.Errorf("core manager is required")
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	cfg := normalizeSwitcherConfig(opts.Config)

	s := &Switcher{
		coreMgr:    opts.CoreManager,
		outputPath: opts.OutputPath,
		config:     cfg,
		logger:     logger,
		groups:     make(map[string]*InstanceGroup),
	}

	if opts.DNATManager != nil {
		s.dnatMgr = opts.DNATManager
	} else {
		s.dnatMgr = NewDNATManager(cfg.NftBin, cfg.NftTableName, logger)
	}
	if opts.PortAllocator != nil {
		s.portAlloc = opts.PortAllocator
	} else {
		s.portAlloc = NewPortAllocator(cfg.PortRangeStart, cfg.PortRangeEnd)
	}
	if opts.Conntrack != nil {
		s.conntrack = opts.Conntrack
	} else {
		s.conntrack = NewConntrackFlusher(cfg.ConntrackBin, logger)
	}
	if opts.ConfigPatcher != nil {
		s.configPatcher = opts.ConfigPatcher
	} else {
		s.configPatcher = NewConfigPatcher(logger)
	}
	if opts.StateRebuilder != nil {
		s.stateRebuilder = opts.StateRebuilder
	} else {
		s.stateRebuilder = NewStateRebuilder(cfg.NftBin, cfg.NftTableName, logger)
	}
	if opts.HealthChecker != nil {
		s.healthChecker = opts.HealthChecker
	} else {
		s.healthChecker = NewHealthChecker(cfg.HealthTimeout, cfg.HealthInterval)
	}
	if opts.GroupLocks != nil {
		s.groupLocks = opts.GroupLocks
	} else {
		s.groupLocks = NewGroupLockManager()
	}
	if opts.OrphanCleaner != nil {
		s.orphanCleaner = opts.OrphanCleaner
	} else {
		s.orphanCleaner = NewOrphanCleaner(cfg.PIDDir, logger)
	}
	if opts.CgroupManager != nil {
		s.cgroupMgr = opts.CgroupManager
	} else {
		s.cgroupMgr = NewCgroupManager(cfg.CgroupBasePath, logger)
	}

	return s, nil
}

// Initialize prepares nftables state and performs orphan cleanup.
func (s *Switcher) Initialize(ctx context.Context) error {
	if s.dnatMgr != nil {
		if err := s.dnatMgr.EnsureInfrastructure(ctx); err != nil {
			return err
		}
	}
	if s.orphanCleaner != nil {
		if err := s.orphanCleaner.CleanupOrphans(ctx); err != nil {
			s.logger.Warn("cleanup orphans failed", "error", err)
		}
	}
	return nil
}

// Switch executes a zero-downtime switch.
func (s *Switcher) Switch(ctx context.Context, req SwitchRequest) (*SwitchResult, error) {
	if req.ToCoreType == "" {
		return nil, fmt.Errorf("to_core_type is required")
	}
	if len(req.ConfigJSON) == 0 {
		return nil, fmt.Errorf("config_json is required")
	}
	if !json.Valid(req.ConfigJSON) {
		return nil, fmt.Errorf("config_json is not valid JSON")
	}
	if len(req.ListenPorts) == 0 {
		return nil, fmt.Errorf("listen ports are required")
	}

	groupID := ComputeGroupID(req.ListenPorts)
	if groupID == "" {
		return nil, fmt.Errorf("invalid group id")
	}
	if !s.groupLocks.TryLock(groupID) {
		return nil, fmt.Errorf("switch for group %s already in progress", groupID)
	}
	defer s.groupLocks.Unlock(groupID)

	occupied, err := s.stateRebuilder.GetOccupiedInternalPorts(ctx)
	if err != nil {
		return nil, fmt.Errorf("rebuild occupied ports: %w", err)
	}

	var (
		newInstanceID string
		internalPorts []int
		portMappings  map[int]int
	)

	startFn := func(mappings map[int]int) error {
		ports := internalPortsFromMappings(req.ListenPorts, mappings)
		if hasMappingConflict(occupied, ports) {
			return fmt.Errorf("address already in use")
		}

		patched, err := s.configPatcher.Patch(req.ToCoreType, req.ConfigJSON, mappings)
		if err != nil {
			return err
		}

		path, err := s.writeConfig(groupID, patched)
		if err != nil {
			return err
		}

		instanceID := fmt.Sprintf("%s-%d", req.ToCoreType, time.Now().UnixNano())
		if err := s.coreMgr.StartInstance(ctx, core.CoreType(req.ToCoreType), instanceID, path, ports); err != nil {
			return err
		}

		newInstanceID = instanceID
		internalPorts = ports
		portMappings = mappings
		return nil
	}

	_, err = s.portAlloc.AllocateWithRetry(ctx, req.ListenPorts, startFn, s.config.MaxRetries)
	if err != nil {
		return nil, err
	}

	pidFileWritten := false
	if err := s.ensurePIDTracking(ctx, req.ToCoreType, newInstanceID, internalPorts); err != nil {
		s.cleanupNewInstance(newInstanceID, pidFileWritten)
		return nil, err
	}
	pidFileWritten = true

	if err := s.healthChecker.CheckPorts(ctx, internalPorts); err != nil {
		s.cleanupNewInstance(newInstanceID, pidFileWritten)
		return nil, err
	}

	s.nftApplyMu.Lock()
	rules, err := s.buildRules(ctx, portMappings)
	if err != nil {
		s.nftApplyMu.Unlock()
		s.cleanupNewInstance(newInstanceID, pidFileWritten)
		return nil, err
	}

	if err := s.dnatMgr.SwitchAtomic(ctx, rules); err != nil {
		s.nftApplyMu.Unlock()
		s.cleanupNewInstance(newInstanceID, pidFileWritten)
		return nil, err
	}
	s.nftApplyMu.Unlock()

	for _, port := range req.ListenPorts {
		if err := s.conntrack.FlushAllProtocols(ctx, port); err != nil {
			s.logger.Warn("conntrack flush failed", "port", port, "error", err)
		}
	}

	// Add instance as slot in port group
	s.addSlotToGroup(groupID, SlotInfo{
		InstanceID:    newInstanceID,
		CoreType:      req.ToCoreType,
		InternalPorts: clonePorts(internalPorts),
		Status:        "active",
		CreatedAt:     time.Now(),
	}, clonePorts(req.ListenPorts))

	if req.FromInstanceID != "" && req.FromInstanceID != newInstanceID {
		s.asyncCleanup(ctx, req.FromInstanceID, groupID)
	}

	return &SwitchResult{
		Success:       true,
		NewInstanceID: newInstanceID,
		PortMappings:  portMappings,
	}, nil
}

func (s *Switcher) writeConfig(prefix string, content []byte) (string, error) {
	baseDir := "."
	if strings.TrimSpace(s.outputPath) != "" {
		baseDir = filepath.Dir(s.outputPath)
	}
	if baseDir == "" {
		baseDir = "."
	}
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}

	name := "core-switch-" + sanitizeToken(prefix)
	if name == "core-switch-" {
		name = fmt.Sprintf("core-switch-%d", time.Now().UnixNano())
	}
	path := filepath.Join(baseDir, name+".json")
	if err := os.WriteFile(path, content, 0644); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}
	return path, nil
}

func (s *Switcher) buildRules(ctx context.Context, overrides map[int]int) ([]*DNATRule, error) {
	mappings, err := s.stateRebuilder.RebuildState(ctx)
	if err != nil {
		return nil, err
	}

	type ruleInfo struct {
		internal int
		tcp      bool
		udp      bool
	}

	byPort := make(map[int]*ruleInfo)
	for _, mapping := range mappings {
		info := byPort[mapping.ExternalPort]
		if info == nil {
			info = &ruleInfo{internal: mapping.InternalPort}
			byPort[mapping.ExternalPort] = info
		} else if mapping.InternalPort != 0 {
			info.internal = mapping.InternalPort
		}
		protocol := strings.ToLower(strings.TrimSpace(mapping.Protocol))
		switch protocol {
		case "udp":
			info.udp = true
		default:
			info.tcp = true
		}
	}

	for external, internal := range overrides {
		info := byPort[external]
		if info == nil {
			info = &ruleInfo{}
			byPort[external] = info
		}
		info.internal = internal
		if !info.tcp && !info.udp {
			info.tcp = true
			info.udp = true
		}
	}

	rules := make([]*DNATRule, 0, len(byPort))
	for external, info := range byPort {
		if external <= 0 || info == nil || info.internal <= 0 {
			continue
		}
		protocol := "tcp"
		if info.tcp && info.udp {
			protocol = "both"
		} else if info.udp {
			protocol = "udp"
		}
		rules = append(rules, &DNATRule{ExternalPort: external, InternalPort: info.internal, Protocol: protocol})
	}
	return rules, nil
}

