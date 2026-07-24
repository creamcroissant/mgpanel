package core

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/creamcroissant/xboard/internal/agent/capability"
	"github.com/creamcroissant/xboard/internal/agent/initsys"
	"github.com/creamcroissant/xboard/internal/agent/protocol/parser"
)

// SingBoxCore implements ProxyCore for sing-box.
type SingBoxCore struct {
	initSys     initsys.InitSystem
	detector    *capability.Detector
	serviceName string
	configDir   string
	binaryPath  string
	logger      *slog.Logger

	mu        sync.RWMutex
	instances map[string]*CoreInstance
}

// NewSingBoxCore creates a new sing-box core adapter.
func NewSingBoxCore(initSys initsys.InitSystem, detector *capability.Detector, serviceName, configDir string) *SingBoxCore {
	if initSys == nil {
		initSys = initsys.Detect()
	}
	if detector == nil {
		detector = capability.NewDetector("", "")
	}
	if serviceName == "" {
		serviceName = "sing-box"
	}
	if configDir == "" {
		configDir = "/etc/sing-box/conf"
	}

	return &SingBoxCore{
		initSys:     initSys,
		detector:    detector,
		serviceName: serviceName,
		configDir:   configDir,
		binaryPath:  detector.SingBoxPath(),
		instances:   make(map[string]*CoreInstance),
		logger:      slog.Default(),
	}
}

func (c *SingBoxCore) Type() CoreType {
	return CoreTypeSingBox
}

func (c *SingBoxCore) Version(ctx context.Context) (string, error) {
	caps, err := c.detectCaps(ctx)
	if err != nil {
		return "", err
	}
	return caps.CoreVersion, nil
}

func (c *SingBoxCore) Capabilities(ctx context.Context) ([]string, error) {
	caps, err := c.detectCaps(ctx)
	if err != nil {
		return nil, err
	}
	return append([]string(nil), caps.Capabilities...), nil
}

func (c *SingBoxCore) IsInstalled(ctx context.Context) bool {
	caps := c.detector.Detect(ctx)
	return caps.CoreType == string(CoreTypeSingBox)
}

func (c *SingBoxCore) ValidateConfig(ctx context.Context, configPath string) error {
	if configPath == "" {
		return fmt.Errorf("config path is required")
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	if !json.Valid(content) {
		return fmt.Errorf("invalid sing-box config JSON")
	}

	registry := parser.NewRegistry()
	if _, err := registry.Parse(filepath.Base(configPath), content); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	return nil
}

func (c *SingBoxCore) Start(ctx context.Context, instanceID, configPath string, listenPorts []int) error {
	if instanceID == "" {
		return fmt.Errorf("instance id is required")
	}
	if configPath == "" {
		return fmt.Errorf("config path is required")
	}

	if err := c.ValidateConfig(ctx, configPath); err != nil {
		c.updateInstanceError(instanceID, err.Error())
		return err
	}

	hash, content, err := hashConfigFile(configPath)
	if err != nil {
		c.updateInstanceError(instanceID, err.Error())
		return err
	}

	// 端口列表由调用方显式传入，避免解析配置推导端口
	c.updateInstance(instanceID, func(inst *CoreInstance) {
		inst.Status = StatusStarting
		inst.ConfigPath = configPath
		inst.ConfigHash = hash
		inst.ListenPorts = listenPorts
		inst.Error = ""
	})

	// Ensure config file exists at expected location for init system
	targetPath := filepath.Join(c.configDir, instanceID+".json")
	// Only write if paths differ, to avoid overwriting if already in place
	// But content might be different? configPath content comes from Handler which writes it fresh.
	// So we should always write/overwrite to ensure latest config.
	if absConfig, err := filepath.Abs(configPath); err == nil {
		if absTarget, err := filepath.Abs(targetPath); err == nil && absConfig != absTarget {
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				c.updateInstanceError(instanceID, fmt.Sprintf("create config dir: %v", err))
				return err
			}
			if err := os.WriteFile(targetPath, content, 0644); err != nil {
				c.updateInstanceError(instanceID, fmt.Sprintf("write instance config: %v", err))
				return err
			}
		}
	}

	service := c.serviceNameForInstance(instanceID)
	if err := c.ensureSystemdUnit(ctx); err != nil {
		c.logger.Warn("cannot provision systemd unit", "error", err)
	}
	if err := c.initSys.Start(ctx, service); err != nil {
		c.updateInstanceError(instanceID, err.Error())
		return err
	}

	c.updateInstance(instanceID, func(inst *CoreInstance) {
		inst.Status = StatusRunning
		inst.StartedAt = time.Now().Unix()
		inst.Error = ""
	})

	return nil
}

func (c *SingBoxCore) Stop(ctx context.Context, instanceID string) error {
	if instanceID == "" {
		return fmt.Errorf("instance id is required")
	}

	c.updateInstance(instanceID, func(inst *CoreInstance) {
		inst.Status = StatusStopping
		inst.Error = ""
	})

	service := c.serviceNameForInstance(instanceID)
	if err := c.initSys.Stop(ctx, service); err != nil {
		c.updateInstanceError(instanceID, err.Error())
		return err
	}

	c.updateInstance(instanceID, func(inst *CoreInstance) {
		inst.Status = StatusStopped
		inst.Error = ""
	})

	return nil
}

func (c *SingBoxCore) Restart(ctx context.Context, instanceID string) error {
	if instanceID == "" {
		return fmt.Errorf("instance id is required")
	}

	c.updateInstance(instanceID, func(inst *CoreInstance) {
		inst.Status = StatusStarting
		inst.Error = ""
	})

	service := c.serviceNameForInstance(instanceID)
	if err := c.initSys.Restart(ctx, service); err != nil {
		c.updateInstanceError(instanceID, err.Error())
		return err
	}

	c.updateInstance(instanceID, func(inst *CoreInstance) {
		inst.Status = StatusRunning
		inst.Error = ""
	})

	return nil
}

func (c *SingBoxCore) Reload(ctx context.Context, instanceID string) error {
	if instanceID == "" {
		return fmt.Errorf("instance id is required")
	}

	service := c.serviceNameForInstance(instanceID)
	if err := c.initSys.Reload(ctx, service); err != nil {
		c.updateInstanceError(instanceID, err.Error())
		return err
	}

	c.updateInstance(instanceID, func(inst *CoreInstance) {
		inst.Status = StatusRunning
		inst.Error = ""
	})

	return nil
}

func (c *SingBoxCore) Status(ctx context.Context, instanceID string) (*CoreInstance, error) {
	if instanceID == "" {
		return nil, fmt.Errorf("instance id is required")
	}

	service := c.serviceNameForInstance(instanceID)
	running, err := c.initSys.Status(ctx, service)
	if err != nil {
		c.updateInstanceError(instanceID, err.Error())
		return nil, err
	}

	c.updateInstance(instanceID, func(inst *CoreInstance) {
		if running {
			inst.Status = StatusRunning
		} else {
			inst.Status = StatusStopped
		}
	})

	c.mu.RLock()
	inst := cloneInstance(c.instances[instanceID])
	c.mu.RUnlock()

	if inst == nil {
		inst = &CoreInstance{
			ID:       instanceID,
			CoreType: CoreTypeSingBox,
			Status:   StatusStopped,
		}
	}

	return inst, nil
}

func (c *SingBoxCore) ListInstances(ctx context.Context) ([]*CoreInstance, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.instances) == 0 {
		return []*CoreInstance{}, nil
	}

	result := make([]*CoreInstance, 0, len(c.instances))
	for _, inst := range c.instances {
		result = append(result, cloneInstance(inst))
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result, nil
}

func (c *SingBoxCore) CollectTraffic(ctx context.Context, instanceID string) ([]TrafficSample, error) {
	return nil, nil
}

// ensureSystemdUnit creates a minimal systemd template unit for sing-box if one does not exist.
func (c *SingBoxCore) ensureSystemdUnit(ctx context.Context) error {
	if _, err := os.Stat("/run/systemd/system"); os.IsNotExist(err) {
		return nil // not systemd
	}
	unitPath := "/etc/systemd/system/sing-box@.service"
	if _, err := os.Stat(unitPath); err == nil {
		return nil // already exists
	}
	binPath := strings.TrimSpace(c.binaryPath)
	if binPath == "" {
		return fmt.Errorf("sing-box binary path unknown, cannot create systemd unit")
	}
	content := fmt.Sprintf(`[Unit]
Description=Sing-box instance %%i
After=network.target

[Service]
Type=simple
User=root
ExecStart=%s run -c /etc/sing-box/conf/%%i.json
Restart=on-failure
RestartSec=5s
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
`, binPath)
	if err := os.MkdirAll("/etc/systemd/system", 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(unitPath, []byte(content), 0o644); err != nil {
		return err
	}
	c.logger.Info("created systemd template unit", "path", unitPath, "binary", binPath)
	// Run daemon-reload to register the new unit
	if err := exec.CommandContext(ctx, "systemctl", "daemon-reload").Run(); err != nil {
		c.logger.Warn("systemctl daemon-reload failed", "error", err)
	}
	return nil
}

func (c *SingBoxCore) detectCaps(ctx context.Context) (*capability.DetectedCapabilities, error) {
	caps, err := c.detector.DetectSingBox(ctx)
	if err != nil {
		return nil, err
	}
	if caps.CoreType != string(CoreTypeSingBox) {
		return nil, fmt.Errorf("sing-box not installed")
	}
	return caps, nil
}

func (c *SingBoxCore) serviceNameForInstance(instanceID string) string {
	if instanceID == "" || instanceID == c.serviceName {
		return c.serviceName
	}
	if strings.Contains(c.serviceName, "{instance}") {
		return strings.ReplaceAll(c.serviceName, "{instance}", instanceID)
	}
	if strings.Contains(c.serviceName, "@") {
		return c.serviceName + instanceID
	}
	return c.serviceName + "@" + instanceID
}

func (c *SingBoxCore) updateInstance(instanceID string, update func(*CoreInstance)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	inst, ok := c.instances[instanceID]
	if !ok {
		inst = &CoreInstance{
			ID:       instanceID,
			CoreType: CoreTypeSingBox,
			Status:   StatusStopped,
		}
		c.instances[instanceID] = inst
	}

	update(inst)
}

func (c *SingBoxCore) updateInstanceError(instanceID, message string) {
	c.updateInstance(instanceID, func(inst *CoreInstance) {
		inst.Status = StatusError
		inst.Error = message
	})
}

func cloneInstance(inst *CoreInstance) *CoreInstance {
	if inst == nil {
		return nil
	}
	copyInst := *inst
	if inst.ListenPorts != nil {
		copyInst.ListenPorts = append([]int(nil), inst.ListenPorts...)
	}
	return &copyInst
}

func hashConfigFile(path string) (string, []byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("read config: %w", err)
	}
	sum := md5.Sum(content)
	return hex.EncodeToString(sum[:]), content, nil
}

// parseSingBoxPorts 已废弃：端口应由 Panel 显式下发而非解析配置推导
// 保留此函数仅为兼容性考虑，新代码不应使用
func parseSingBoxPorts(content []byte) []int {
	if len(content) == 0 {
		return nil
	}

	p := parser.NewSingBoxParser()
	details, err := p.Parse(filepath.Base("config.json"), content)
	if err != nil {
		return nil
	}

	seen := map[int]struct{}{}
	for _, detail := range details {
		if detail.Port > 0 {
			seen[detail.Port] = struct{}{}
		}
	}

	if len(seen) == 0 {
		return nil
	}

	ports := make([]int, 0, len(seen))
	for port := range seen {
		ports = append(ports, port)
	}
	sort.Ints(ports)
	return ports
}
