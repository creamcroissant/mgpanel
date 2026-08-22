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
}

// NewAgentService 创建 AgentService。
func NewAgentService(serverRepo repository.ServerRepository, userRepo repository.UserRepository) AgentService {
	return &agentService{
		serverRepo: serverRepo,
		userRepo:   userRepo,
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
	slog.Info("Found users", "count", len(users))

	return users, nil
}
