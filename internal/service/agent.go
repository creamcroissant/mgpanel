package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// AgentService 定义 Agent 侧的业务逻辑。
type AgentService interface {
	// GetUsersForAgent 返回需要同步到 Agent 的活跃用户列表。
	GetUsersForAgent(ctx context.Context, agentHostID int64) ([]*repository.NodeUser, error)
}

// agentService 是 AgentService 的默认实现。
type agentService struct {
	serverRepo repository.ServerRepository
	userRepo   repository.UserRepository
	deny       UserServerDenyService
}

// NewAgentService 创建 AgentService。
func NewAgentService(serverRepo repository.ServerRepository, userRepo repository.UserRepository, deny UserServerDenyService) AgentService {
	return &agentService{
		serverRepo: serverRepo,
		userRepo:   userRepo,
		deny:       deny,
	}
}

// GetUsersForAgent 为指定 Agent 汇总需要同步的活跃用户。
func (s *agentService) GetUsersForAgent(ctx context.Context, agentHostID int64) ([]*repository.NodeUser, error) {
	slog.Info("GetUsersForAgent called", "agent_host_id", agentHostID)
	// 1. 获取该 Agent 关联的全部节点
	servers, err := s.serverRepo.FindByAgentHostID(ctx, agentHostID)
	if err != nil {
		return nil, err
	}

	if len(servers) == 0 {
		slog.Info("No servers found for agent", "agent_host_id", agentHostID)
		return []*repository.NodeUser{}, nil
	}

	// 2. 提取节点所属的唯一分组 ID
	// 使用 map 去重
	groupSet := make(map[int64]struct{})
	for _, srv := range servers {
		if srv.GroupID > 0 {
			groupSet[srv.GroupID] = struct{}{}
		}
	}

	if len(groupSet) == 0 {
		slog.Info("No groups found for servers")
		return []*repository.NodeUser{}, nil
	}

	// 3. 组装分组 ID 列表，供后续查询使用
	groupIDs := make([]int64, 0, len(groupSet))
	for id := range groupSet {
		groupIDs = append(groupIDs, id)
	}
	slog.Info("Fetching users for groups", "group_ids", groupIDs)

	// 4. 获取分组下的活跃用户
	now := time.Now().Unix()
	users, err := s.userRepo.ListActiveForGroups(ctx, groupIDs, now)
	if err != nil {
		return nil, err
	}

	// 4b. 节点黑名单过滤：若某用户被该 Agent 关联的**全部**节点禁用，则剔除该用户（网络层硬阻断）。
	// 该 Agent 全部节点被禁 → 用户 denied 集合覆盖 servers 集合。
	if s != nil && s.deny != nil {
		filtered := make([]*repository.NodeUser, 0, len(users))
		for _, user := range users {
			if user == nil {
				continue
			}
			deniedIDs, dErr := s.deny.GetUserDeniedServerIDs(ctx, user.ID)
			if dErr != nil {
				continue // 查询失败时保守保留该用户，避免误删
			}
			if agentServersAllDenied(servers, deniedIDs) {
				continue // 该 Agent 全部节点对该用户禁用 → 不同步
			}
			filtered = append(filtered, user)
		}
		users = filtered
	}
	slog.Info("Found users", "count", len(users))

	return users, nil
}

// agentServersAllDenied 报告给定 agent 的全部服务器是否都被 user 的 deniedIDs 覆盖。
// 该 Agent 任一节点未被禁用 → 该用户仍需同步到该 Agent，返回 false。
func agentServersAllDenied(servers []*repository.Server, deniedIDs []int64) bool {
	if len(servers) == 0 {
		return false
	}
	denied := make(map[int64]struct{}, len(deniedIDs))
	for _, id := range deniedIDs {
		if id > 0 {
			denied[id] = struct{}{}
		}
	}
	for _, srv := range servers {
		if srv == nil {
			continue
		}
		if _, ok := denied[srv.ID]; !ok {
			return false
		}
	}
	return true
}
