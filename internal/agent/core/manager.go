package core

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// CoreInfo 描述核心类型与可用性。
type CoreInfo struct {
	Type         CoreType `json:"type"`
	Version      string   `json:"version,omitempty"`
	Installed    bool     `json:"installed"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// Manager 管理多个核心实例，并维护可用核心列表。
type Manager struct {
	cores     map[CoreType]ProxyCore
	instances map[string]*CoreInstance
	mu        sync.RWMutex
}

// NewManager 创建核心管理器。
func NewManager() *Manager {
	return &Manager{
		cores:     make(map[CoreType]ProxyCore),
		instances: make(map[string]*CoreInstance),
	}
}

// Register 注册核心适配器。
func (m *Manager) Register(core ProxyCore) {
	if core == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cores[core.Type()] = core
}

// GetCore 按类型获取核心适配器。
func (m *Manager) GetCore(coreType CoreType) (ProxyCore, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	core, ok := m.cores[coreType]
	return core, ok
}

// ListCores 返回所有已注册核心的状态信息。
func (m *Manager) ListCores(ctx context.Context) ([]*CoreInfo, error) {
	// 读取已注册核心并输出排序后的状态列表。
	m.mu.RLock()
	cores := make([]ProxyCore, 0, len(m.cores))
	for _, core := range m.cores {
		cores = append(cores, core)
	}
	m.mu.RUnlock()

	infos := make([]*CoreInfo, 0, len(cores))
	for _, core := range cores {
		info := &CoreInfo{Type: core.Type()}
		if core.IsInstalled(ctx) {
			info.Installed = true
			if version, err := core.Version(ctx); err == nil {
				info.Version = version
			}
			if caps, err := core.Capabilities(ctx); err == nil {
				info.Capabilities = append([]string(nil), caps...)
			}
		}
		infos = append(infos, info)
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Type < infos[j].Type
	})

	return infos, nil
}

// StartInstance 启动指定核心实例，并记录到内存索引。
// listenPorts 由调用方显式传入，避免解析配置推导端口。
func (m *Manager) StartInstance(ctx context.Context, coreType CoreType, instanceID, configPath string, listenPorts []int) error {
	// 启动实例后写入内存索引缓存状态。
	core, ok := m.GetCore(coreType)
	if !ok {
		return fmt.Errorf("core not registered: %s", coreType)
	}

	if err := core.Start(ctx, instanceID, configPath, listenPorts); err != nil {
		return err
	}

	inst, err := core.Status(ctx, instanceID)
	if err != nil {
		return err
	}
	if inst == nil {
		return nil
	}

	m.mu.Lock()
	m.instances[instanceID] = cloneInstance(inst)
	m.mu.Unlock()

	return nil
}

// StopInstance 停止实例，并同步更新索引。
func (m *Manager) StopInstance(ctx context.Context, instanceID string) error {
	slog.Info("StopInstance called", "instance_id", instanceID)
	// 停止实例并刷新内存索引状态。
	core, err := m.coreForInstance(instanceID)
	if err != nil {
		slog.Error("StopInstance core lookup failed", "instance_id", instanceID, "error", err)
		return err
	}

	stopErr := core.Stop(ctx, instanceID)
	if stopErr != nil {
		slog.Error("StopInstance core.Stop failed", "instance_id", instanceID, "error", stopErr)
		// Don't return yet, check status to see if it actually stopped (maybe error was transient)
	}

	// Wait briefly to allow process to terminate fully
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
	}

	inst, err := core.Status(ctx, instanceID)
	if err != nil {
		slog.Error("StopInstance core.Status failed", "instance_id", instanceID, "error", err)
		return err
	}
	slog.Info("StopInstance core.Status result", "instance_id", instanceID, "inst_is_nil", inst == nil)

	m.mu.Lock()
	if inst != nil {
		slog.Warn("StopInstance: instance still running after stop", "instance_id", instanceID)
		m.instances[instanceID] = cloneInstance(inst)
	} else {
		slog.Info("StopInstance: instance removed from index", "instance_id", instanceID)
		delete(m.instances, instanceID)
	}
	m.mu.Unlock()

	// If the instance is still considered running (inst != nil and Status is Running/Starting), we must report error.
	if inst != nil {
		if inst.Status == StatusRunning || inst.Status == StatusStarting {
			if stopErr != nil {
				return stopErr
			}
			return fmt.Errorf("instance still running after stop")
		}
	}

	// Instance is not running.
	// Even if Stop command returned error (e.g. "process not found"), we consider it success since the instance is stopped.
	return nil
}

// SwitchCore 执行核心切换，采用冷切换策略（先停后启），失败时回滚到原实例。
// listenPorts 由调用方显式传入，避免解析配置推导端口。
func (m *Manager) SwitchCore(ctx context.Context, fromInstanceID string, toCoreType CoreType, toConfigPath string, listenPorts []int) (string, error) {
	logger := slog.Default()

	// 1. 获取旧实例信息并备份
	fromCore, err := m.coreForInstance(fromInstanceID)
	if err != nil {
		return "", err
	}

	fromInst, err := fromCore.Status(ctx, fromInstanceID)
	if err != nil {
		return "", err
	}
	if fromInst == nil {
		return "", fmt.Errorf("instance not found: %s", fromInstanceID)
	}

	backup := cloneInstance(fromInst)

	// 2. 验证目标核心可用
	toCore, ok := m.GetCore(toCoreType)
	if !ok {
		return "", fmt.Errorf("target core not registered: %s", toCoreType)
	}
	if !toCore.IsInstalled(ctx) {
		return "", fmt.Errorf("target core not installed: %s", toCoreType)
	}

	// 3. 停止旧实例（冷切换关键步骤 - 先停后启避免端口竞态）
	if err := m.StopInstance(ctx, fromInstanceID); err != nil {
		return "", fmt.Errorf("failed to stop old instance: %w", err)
	}

	// 4. 等待端口释放（处理 TCP TIME_WAIT 等）
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(300 * time.Millisecond):
	}

	// 5. 启动新实例
	toInstanceID := fmt.Sprintf("%s-%d", toCoreType, time.Now().UnixNano())
	if err := m.StartInstance(ctx, toCoreType, toInstanceID, toConfigPath, listenPorts); err != nil {
		logger.Error("new instance failed to start, rolling back", "error", err)

		// 6. 回滚：重启旧实例（使用备份的端口列表）
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
		if rbErr := m.StartInstance(ctx, backup.CoreType, backup.ID, backup.ConfigPath, backup.ListenPorts); rbErr != nil {
			logger.Error("rollback failed: system in degraded state", "rollback_error", rbErr)
			return "", fmt.Errorf("switch failed and rollback failed: %v (original: %w)", rbErr, err)
		}
		logger.Info("rollback successful, old instance restored")
		return "", fmt.Errorf("switch failed, rolled back: %w", err)
	}

	return toInstanceID, nil
}

// ListInstances 返回当前跟踪的实例列表。
func (m *Manager) ListInstances() []*CoreInstance {
	// 返回按实例 ID 排序的快照列表。
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*CoreInstance, 0, len(m.instances))
	for _, inst := range m.instances {
		result = append(result, cloneInstance(inst))
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result
}

// coreForInstance 根据实例 ID 找到对应核心。
func (m *Manager) coreForInstance(instanceID string) (ProxyCore, error) {
	m.mu.RLock()
	inst := m.instances[instanceID]
	m.mu.RUnlock()
	if inst == nil {
		return nil, fmt.Errorf("instance not registered: %s", instanceID)
	}

	core, ok := m.GetCore(inst.CoreType)
	if !ok {
		return nil, fmt.Errorf("core not registered: %s", inst.CoreType)
	}

	return core, nil
}

// GetCoreAny implements proxy.coreManager interface (to break dependency cycle)
func (m *Manager) GetCoreAny(coreType string) (any, bool) {
	return m.GetCore(CoreType(coreType))
}

// StartInstanceAny implements proxy.coreManager interface (to break dependency cycle)
func (m *Manager) StartInstanceAny(ctx context.Context, coreType string, instanceID, configPath string, listenPorts []int) error {
	return m.StartInstance(ctx, CoreType(coreType), instanceID, configPath, listenPorts)
}