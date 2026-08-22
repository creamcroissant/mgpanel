// 文件路径: internal/protocol/manager.go
// 模块说明: 这是 internal 模块里的 manager 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package protocol

import (
	"context"
	"fmt"
	"strings"
)

// Manager 管理可用的协议构建器，并按客户端标识匹配。
type Manager struct {
	builders       []Builder
	defaultBuilder Builder
}

// NewManager 创建注册表并可预加载构建器。
func NewManager(builders ...Builder) *Manager {
	m := &Manager{}
	for _, builder := range builders {
		m.Register(builder)
	}
	return m
}

// Register 将构建器注册到列表。
func (m *Manager) Register(builder Builder) {
	if builder == nil {
		return
	}
	m.builders = append(m.builders, builder)
	if m.defaultBuilder == nil {
		m.defaultBuilder = builder
	}
}

// Flags 返回所有构建器支持的标识集合。
func (m *Manager) Flags() []string {
	seen := make(map[string]struct{})
	var flags []string
	for _, builder := range m.builders {
		for _, flag := range builder.Flags() {
			n := strings.ToLower(strings.TrimSpace(flag))
			if n == "" {
				continue
			}
			if _, ok := seen[n]; ok {
				continue
			}
			seen[n] = struct{}{}
			flags = append(flags, n)
		}
	}
	return flags
}

// Build 选择合适的构建器并生成订阅结果。
func (m *Manager) Build(req BuildRequest) (*Result, error) {
	if len(m.builders) == 0 {
		return nil, fmt.Errorf("no protocol builders registered / 未注册任何协议构建器")
	}
	builder := m.matchBuilder(req.Flag, req.UserAgent)
	if builder == nil {
		builder = m.defaultBuilder
	}
	if req.Context == nil {
		req.Context = context.Background()
	}
	return builder.Build(req)
}

func (m *Manager) matchBuilder(flag string, userAgent string) Builder {
	combined := strings.ToLower(strings.TrimSpace(flag))
	if combined == "" {
		combined = strings.ToLower(strings.TrimSpace(userAgent))
	}
	if combined == "" {
		return nil
	}
	for i := len(m.builders) - 1; i >= 0; i-- {
		builder := m.builders[i]
		for _, candidate := range builder.Flags() {
			if candidate == "" {
				continue
			}
			if strings.Contains(combined, strings.ToLower(candidate)) {
				return builder
			}
		}
	}
	return nil
}
