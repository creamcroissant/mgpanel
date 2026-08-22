package proxy

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/core"
)

const defaultDrainTimeout = 5 * time.Second

// SwitcherConfig controls zero-downtime switching behavior.
type SwitcherConfig struct {
	PortRangeStart int
	PortRangeEnd   int
	MaxRetries     int
	HealthTimeout  time.Duration
	HealthInterval time.Duration
	DrainTimeout   time.Duration
	NftBin         string
	ConntrackBin   string
	NftTableName   string
	PIDDir         string
	CgroupBasePath string
}

// Dependency interfaces for Switcher.
type dnatApplier interface {
	EnsureInfrastructure(ctx context.Context) error
	SwitchAtomic(ctx context.Context, rules []*DNATRule) error
	AddRules(ctx context.Context, rules []*DNATRule) error
	RemoveRules(ctx context.Context, rules []*DNATRule, handles map[string]int) error
}

type portAllocator interface {
	AllocateWithRetry(ctx context.Context, externalPorts []int, startFn func(map[int]int) error, maxRetries int) (map[int]int, error)
}

type conntrackFlusher interface {
	FlushAllProtocols(ctx context.Context, port int) error
}

type configPatcher interface {
	Patch(coreType string, configJSON []byte, mappings map[int]int) ([]byte, error)
}

type stateRebuilder interface {
	GetOccupiedInternalPorts(ctx context.Context) (map[int]bool, error)
	RebuildState(ctx context.Context) ([]PortMapping, error)
}

type healthChecker interface {
	CheckPorts(ctx context.Context, ports []int) error
}

type groupLock interface {
	TryLock(groupID string) bool
	Lock(groupID string)
	Unlock(groupID string)
	RemoveGroupLock(groupID string)
}

type orphanCleaner interface {
	CleanupOrphans(ctx context.Context) error
	MarkDraining(instanceID string) error
	RemovePIDFile(instanceID string) error
	WritePIDFile(instanceID string, pid int, coreType string, ports []int) error
}

type cgroupManager interface {
	IsSupported() bool
	KillGroup(name string) error
	AddProcess(name string, pid int) error
}

type SlotInfo struct {
	InstanceID    string      `json:"instance_id"`
	CoreType      string      `json:"core_type"`
	InternalPorts []int       `json:"internal_ports"`
	PortMappings  map[int]int `json:"port_mappings,omitempty"` // external->internal
	Status        string      `json:"status"`                  // "active" | "draining"
	CreatedAt     time.Time   `json:"created_at"`
}

// InstanceGroup tracks a port group state, supporting multiple active instances.
type InstanceGroup struct {
	ID            string     `json:"id"`
	ExternalPorts []int      `json:"external_ports"`
	Slots         []SlotInfo `json:"slots"`
}

// SwitchRequest describes a switch operation (replaces old with new).
type SwitchRequest struct {
	FromInstanceID string
	ToCoreType     string
	ConfigJSON     []byte
	ListenPorts    []int
}

// SwitchResult reports switch outcome.
type SwitchResult struct {
	Success       bool
	NewInstanceID string
	PortMappings  map[int]int
	Error         string
}

// AddInstanceRequest describes adding a new core instance to a port group.
type AddInstanceRequest struct {
	CoreType    string
	ConfigJSON  []byte
	ListenPorts []int
}

// AddInstanceResult reports the outcome of adding an instance.
type AddInstanceResult struct {
	Success      bool
	InstanceID   string
	PortMappings map[int]int
}

// RemoveInstanceRequest describes removing a core instance from a port group.
type RemoveInstanceRequest struct {
	InstanceID  string
	ListenPorts []int
}

// ReplaceInstanceRequest describes replacing one instance with another.
type ReplaceInstanceRequest struct {
	OldInstanceID string
	NewCoreType   string
	NewConfigJSON []byte
	ListenPorts   []int
}

// SwitcherOptions configures Switcher dependencies.
type SwitcherOptions struct {
	CoreManager    *core.Manager
	OutputPath     string
	Logger         *slog.Logger
	Config         SwitcherConfig
	DNATManager    dnatApplier
	PortAllocator  portAllocator
	Conntrack      conntrackFlusher
	ConfigPatcher  configPatcher
	StateRebuilder stateRebuilder
	HealthChecker  healthChecker
	GroupLocks     groupLock
	OrphanCleaner  orphanCleaner
	CgroupManager  cgroupManager
}

// Switcher coordinates zero-downtime switching.
type Switcher struct {
	coreMgr        *core.Manager
	dnatMgr        dnatApplier
	portAlloc      portAllocator
	conntrack      conntrackFlusher
	configPatcher  configPatcher
	stateRebuilder stateRebuilder
	healthChecker  healthChecker
	groupLocks     groupLock
	orphanCleaner  orphanCleaner
	cgroupMgr      cgroupManager
	outputPath     string
	config         SwitcherConfig
	logger         *slog.Logger

	groups   map[string]*InstanceGroup
	groupsMu sync.RWMutex

	nftApplyMu sync.Mutex
}
