// 文件路径: internal/plugin/hook/manager.go
// 模块说明: 这是 internal 模块里的 manager 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package hook

import (
	"context"
	"strings"
	"sync"
)

// FilterFunc 表示可用于转换输入 payload 的 hook。
type FilterFunc func(ctx context.Context, payload any) (any, error)

// Manager 保存 hook 注册并按注册顺序执行。
type Manager struct {
	mu      sync.RWMutex
	filters map[string][]FilterFunc
}

// NewManager 返回一个空的 hook 管理器。
func NewManager() *Manager {
	return &Manager{filters: make(map[string][]FilterFunc)}
}

// Register 将过滤函数绑定到指定 hook 名称。
func (m *Manager) Register(name string, fn FilterFunc) {
	if m == nil || fn == nil {
		return
	}
	key := normalizeName(name)
	if key == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.filters[key] = append(m.filters[key], fn)
}

// Apply 执行指定 hook 名称下的所有过滤函数。
func (m *Manager) Apply(ctx context.Context, name string, payload any) (any, error) {
	if m == nil {
		return payload, nil
	}
	key := normalizeName(name)
	if key == "" {
		return payload, nil
	}
	m.mu.RLock()
	filters := append([]FilterFunc(nil), m.filters[key]...)
	m.mu.RUnlock()
	var err error
	current := payload
	for _, filter := range filters {
		current, err = filter(ctx, current)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

// Reset 移除指定 hook 名称下的所有过滤器。
func (m *Manager) Reset(name string) {
	if m == nil {
		return
	}
	key := normalizeName(name)
	if key == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.filters, key)
}

func normalizeName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	return strings.ToLower(trimmed)
}

var (
	defaultManager = NewManager()
)

// Default 返回进程级的 hook 管理器。
func Default() *Manager {
	return defaultManager
}

// Register 将过滤器注册到默认管理器。
func Register(name string, fn FilterFunc) {
	defaultManager.Register(name, fn)
}

// Apply 通过默认管理器执行 hook。
func Apply(ctx context.Context, name string, payload any) (any, error) {
	return defaultManager.Apply(ctx, name, payload)
}

// Reset 移除默认管理器中的指定 hook。
func Reset(name string) {
	defaultManager.Reset(name)
}
