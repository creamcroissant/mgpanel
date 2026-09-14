package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// AgentEgressRouteService 按 host_token 回答"本机应生效哪些内核分发指派"。
// 设计：docs/plans/20260915-egress-dispatch-l3.md §10.4。
type AgentEgressRouteService interface {
	RoutesForToken(ctx context.Context, token string) ([]EgressRouteAssignment, error)
}

type agentEgressRouteService struct {
	agentHosts repository.AgentHostRepository
	dispatch   EgressDispatchService
	logger     *slog.Logger
}

func NewAgentEgressRouteService(
	agentHosts repository.AgentHostRepository,
	dispatch EgressDispatchService,
	logger *slog.Logger,
) AgentEgressRouteService {
	if logger == nil {
		logger = slog.Default()
	}
	return &agentEgressRouteService{agentHosts: agentHosts, dispatch: dispatch, logger: logger}
}

// RoutesForToken 展开本机的内核分发指派。
// 副作用（成功即生效）：刷新能力时间戳（I13）并在本机参与时回收陈旧配对。
func (s *agentEgressRouteService) RoutesForToken(ctx context.Context, token string) ([]EgressRouteAssignment, error) {
	if s.dispatch == nil {
		return []EgressRouteAssignment{}, nil
	}
	host, err := s.agentHosts.FindByToken(ctx, token)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound // 哨兵：handler 按 service.ErrNotFound 映射 401
		}
		return nil, fmt.Errorf("locate agent by token: %w", err)
	}
	out, err := s.dispatch.AssignmentsForHost(ctx, host.ID)
	if err != nil {
		return nil, fmt.Errorf("expand egress assignments: %w", err)
	}
	// 能力门控刷新放在成功展开之后：能走到这里就证明本机是支持 egress 协议的 agent。
	if err := s.dispatch.MarkCapabilitySynced(ctx, host.ID); err != nil {
		s.logger.Warn("egress-route: mark capability failed",
			slog.Int64("agent_host_id", host.ID), slog.String("err", err.Error()))
	}
	if err := s.dispatch.ReconcilePairs(ctx); err != nil {
		s.logger.Warn("egress-route: reconcile pairs failed",
			slog.Int64("agent_host_id", host.ID), slog.String("err", err.Error()))
	}
	if out == nil {
		out = []EgressRouteAssignment{}
	}
	return out, nil
}
