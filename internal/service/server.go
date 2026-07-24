// 文件路径: internal/service/server.go
// 模块说明: 这是 internal 模块里的 server 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/creamcroissant/xboard/internal/repository"
)

// ServerService 提供按用户权限过滤的节点查询能力。
type ServerService interface {
	ListForUser(ctx context.Context, userID string) (*ServerListResult, error)
	Heartbeat(ctx context.Context, nodeID int) error
}

// ServerListResult 表示用户节点列表的返回结果。
type ServerListResult struct {
	Nodes []ServerNode
	ETag  string
}

// ServerNode 对齐旧 PHP 栈使用的 NodeResource 数据结构。
type ServerNode struct {
	ID          int64    `json:"id"`
	Type        string   `json:"type"`
	Version     *int     `json:"version,omitempty"`
	Name        string   `json:"name"`
	Rate        string   `json:"rate"`
	Tags        []string `json:"tags"`
	IsOnline    int      `json:"is_online"`
	CacheKey    string   `json:"cache_key"`
	LastCheckAt int64    `json:"last_check_at"`
}

type serverService struct {
	users   repository.UserRepository
	servers repository.ServerRepository
	plans   repository.PlanRepository
}

// NewServerService 组装基于 repository 的依赖。
func NewServerService(users repository.UserRepository, servers repository.ServerRepository, plans repository.PlanRepository) ServerService {
	return &serverService{users: users, servers: servers, plans: plans}
}

func (s *serverService) ListForUser(ctx context.Context, userID string) (*ServerListResult, error) {
	if s == nil || s.users == nil || s.servers == nil {
		return nil, fmt.Errorf("server service not fully configured / 节点服务未完整配置")
	}
	user, err := loadServerUser(ctx, s.users, userID)
	if err != nil {
		return nil, err
	}
	if !isServerAccessAllowed(user) {
		return &ServerListResult{Nodes: []ServerNode{}, ETag: computeETag(nil)}, nil
	}

	// 获取用户套餐允许的分组列表
	groupIDs, err := s.plans.GetGroups(ctx, user.PlanID)
	if err != nil {
		return nil, fmt.Errorf("get plan groups: %w", err)
	}

	// 获取旧版分组 ID 对应的套餐
	plan, err := s.plans.FindByID(ctx, user.PlanID)
	if err == nil && plan.GroupID != nil {
		groupIDs = append(groupIDs, *plan.GroupID)
	}

	var nodes []*repository.Server
	if len(groupIDs) > 0 {
		nodes, err = s.servers.FindByGroupIDs(ctx, groupIDs)
	} else {
		// 若未找到分组则返回空列表
		nodes = []*repository.Server{}
	}

	if err != nil {
		return nil, err
	}
	views := make([]ServerNode, 0, len(nodes))
	cacheKeys := make([]string, 0, len(nodes))
	for _, node := range nodes {
		view := transformServerNode(node)
		views = append(views, view)
		cacheKeys = append(cacheKeys, view.CacheKey)
	}
	return &ServerListResult{Nodes: views, ETag: computeETag(cacheKeys)}, nil
}

func (s *serverService) Heartbeat(ctx context.Context, nodeID int) error {
	return s.servers.UpdateHeartbeat(ctx, int64(nodeID), time.Now().Unix())
}

func transformServerNode(server *repository.Server) ServerNode {
	if server == nil {
		return ServerNode{}
	}
	tags := decodeStringArray(server.Tags)
	version := extractVersion(server.Settings)
	isOnline := 0
	if server.Status > 0 {
		isOnline = 1
	}
	cacheKey := fmt.Sprintf("%s-%d-%d-%d", server.Type, server.ID, server.UpdatedAt, isOnline)
	lastCheck := server.UpdatedAt
	return ServerNode{
		ID:          server.ID,
		Type:        server.Type,
		Version:     version,
		Name:        server.Name,
		Rate:        server.Rate,
		Tags:        tags,
		IsOnline:    isOnline,
		CacheKey:    cacheKey,
		LastCheckAt: lastCheck,
	}
}

func decodeStringArray(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}
	return nil
}

func extractVersion(raw json.RawMessage) *int {
	if len(raw) == 0 {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	if val, ok := payload["version"]; ok {
		switch v := val.(type) {
		case float64:
			n := int(v)
			return &n
		case int:
			n := v
			return &n
		case int64:
			n := int(v)
			return &n
		}
	}
	return nil
}

func computeETag(keys []string) string {
	if len(keys) == 0 {
		keys = []string{}
	}
	data, err := json.Marshal(keys)
	if err != nil {
		data = []byte{}
	}
	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:])
}
