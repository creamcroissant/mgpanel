package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/capability"
	"github.com/creamcroissant/mgpanel/internal/agent/initsys"
	"github.com/creamcroissant/mgpanel/internal/agent/protocol/parser"
	"github.com/creamcroissant/mgpanel/internal/agent/traffic"
)

// XrayCore implements ProxyCore for xray.
type XrayCore struct {
	initSys     initsys.InitSystem
	detector    *capability.Detector
	serviceName string
	trafficAddr string
	configDir   string

	collectorMu sync.Mutex
	collector   *traffic.XrayCollector

	mu        sync.RWMutex
	instances map[string]*CoreInstance
}

// NewXrayCore creates a new xray core adapter.
func NewXrayCore(initSys initsys.InitSystem, detector *capability.Detector, serviceName, trafficAddr, configDir string) *XrayCore {
	if initSys == nil {
		initSys = initsys.Detect()
	}
	if detector == nil {
		detector = capability.NewDetector("", "")
	}
	if serviceName == "" {
		serviceName = "xray"
	}
	if configDir == "" {
		configDir = "/usr/local/etc/xray"
	}

	return &XrayCore{
		initSys:     initSys,
		detector:    detector,
		serviceName: serviceName,
		trafficAddr: trafficAddr,
		configDir:   configDir,
		instances:   make(map[string]*CoreInstance),
	}
}

func (c *XrayCore) Type() CoreType {
	return CoreTypeXray
}

func (c *XrayCore) Version(ctx context.Context) (string, error) {
	caps, err := c.detectCaps(ctx)
	if err != nil {
		return "", err
	}
	return caps.CoreVersion, nil
}

func (c *XrayCore) Capabilities(ctx context.Context) ([]string, error) {
	caps, err := c.detectCaps(ctx)
	if err != nil {
		return nil, err
	}
	return append([]string(nil), caps.Capabilities...), nil
}

func (c *XrayCore) IsInstalled(ctx context.Context) bool {
	_, err := c.detectCaps(ctx)
	return err == nil
}

func (c *XrayCore) ValidateConfig(ctx context.Context, configPath string) error {
	if configPath == "" {
		return fmt.Errorf("config path is required")
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	if !json.Valid(content) {
		return fmt.Errorf("invalid xray config JSON")
	}

	registry := parser.NewRegistry()
	if _, err := registry.Parse(filepath.Base(configPath), content); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	return nil
}

func (c *XrayCore) Start(ctx context.Context, instanceID, configPath string, listenPorts []int) error {
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

	service := c.serviceNameForInstance(instanceID)

	// Ensure config file exists at expected location for init system
	targetPath := filepath.Join(c.configDir, service+".json")
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

func (c *XrayCore) Stop(ctx context.Context, instanceID string) error {
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

func (c *XrayCore) Restart(ctx context.Context, instanceID string) error {
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

func (c *XrayCore) Reload(ctx context.Context, instanceID string) error {
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

func (c *XrayCore) Status(ctx context.Context, instanceID string) (*CoreInstance, error) {
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
			CoreType: CoreTypeXray,
			Status:   StatusStopped,
		}
	}

	return inst, nil
}

func (c *XrayCore) ListInstances(ctx context.Context) ([]*CoreInstance, error) {
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

func (c *XrayCore) CollectTraffic(ctx context.Context, instanceID string) ([]TrafficSample, error) {
	collector, err := c.getCollector()
	if err != nil {
		return nil, err
	}

	payloads, err := collector.Collect(ctx)
	if err != nil {
		return nil, err
	}

	samples := make([]TrafficSample, 0, len(payloads))
	for _, payload := range payloads {
		if payload.Upload == 0 && payload.Download == 0 {
			continue
		}

		sample := TrafficSample{
			Upload:   payload.Upload,
			Download: payload.Download,
		}

		if payload.UserID > 0 {
			sample.UserID = payload.UserID
		}
		if payload.UID != "" {
			sample.Email = payload.UID
			if sample.UserID == 0 {
				if parsed, parseErr := strconv.ParseInt(payload.UID, 10, 64); parseErr == nil {
					sample.UserID = parsed
				}
			}
		}

		samples = append(samples, sample)
	}

	return samples, nil
}

func (c *XrayCore) detectCaps(ctx context.Context) (*capability.DetectedCapabilities, error) {
	caps, err := c.detector.DetectXray(ctx)
	if err == nil && caps.CoreType == string(CoreTypeXray) {
		return caps, nil
	}
	if err != nil {
		return nil, err
	}

	return nil, fmt.Errorf("xray not installed")
}

func (c *XrayCore) serviceNameForInstance(instanceID string) string {
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

func (c *XrayCore) updateInstance(instanceID string, update func(*CoreInstance)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	inst, ok := c.instances[instanceID]
	if !ok {
		inst = &CoreInstance{
			ID:       instanceID,
			CoreType: CoreTypeXray,
			Status:   StatusStopped,
		}
		c.instances[instanceID] = inst
	}

	update(inst)
}

func (c *XrayCore) updateInstanceError(instanceID, message string) {
	c.updateInstance(instanceID, func(inst *CoreInstance) {
		inst.Status = StatusError
		inst.Error = message
	})
}

func (c *XrayCore) getCollector() (*traffic.XrayCollector, error) {
	c.collectorMu.Lock()
	defer c.collectorMu.Unlock()

	if c.collector != nil {
		return c.collector, nil
	}

	collector, err := traffic.NewXrayCollector(c.trafficAddr)
	if err != nil {
		return nil, err
	}

	c.collector = collector
	return collector, nil
}

// parseXrayPorts 已废弃：端口应由 Panel 显式下发而非解析配置推导
// 保留此函数仅为兼容性考虑，新代码不应使用
func parseXrayPorts(content []byte) []int {
	if len(content) == 0 {
		return nil
	}

	p := parser.NewXrayParser()
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
