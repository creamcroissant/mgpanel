// 文件路径: internal/grpc/handler/agent_handler_mesh_auth.go
// 模块说明: mesh 网络 ACL——JoinMesh 网络白名单（config.mesh.allowed_networks）。
package handler

import (
	"strings"
	"sync"
)

// meshNetworkACLState 包级白名单状态；由 serve.go 在配置加载后注入。
// 空集合（未配置）时仅允许内置 default 网络。
var meshNetworkACLState struct {
	mu      sync.RWMutex
	allowed map[string]struct{}
	set     bool
}

// SetAllowedMeshNetworks 配置允许加入的 mesh 网络 ID 列表。
// 传入空/nil 表示恢复默认策略（仅允许 "default"）。
func SetAllowedMeshNetworks(ids []string) {
	allowed := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}
	meshNetworkACLState.mu.Lock()
	defer meshNetworkACLState.mu.Unlock()
	meshNetworkACLState.allowed = allowed
	meshNetworkACLState.set = true
}

// isMeshNetworkAllowed 判定目标网络是否可加入。
func isMeshNetworkAllowed(networkID string) bool {
	id := strings.TrimSpace(networkID)
	if id == "" {
		id = "default"
	}
	meshNetworkACLState.mu.RLock()
	defer meshNetworkACLState.mu.RUnlock()
	if !meshNetworkACLState.set || len(meshNetworkACLState.allowed) == 0 {
		return id == "default"
	}
	_, ok := meshNetworkACLState.allowed[id]
	return ok
}
