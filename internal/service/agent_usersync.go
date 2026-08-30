package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// ResolveUsersForHost resolves the v2ray-api user-sync targets for the agent
// identified by its host token. It mirrors the relay-routes error contract:
// an unknown token (repository.ErrNotFound) is translated to the service
// ErrNotFound sentinel so the HTTP handler can map it to 401.
func ResolveUsersForHost(ctx context.Context, hostToken string, syncs UserSyncService, hosts repository.AgentHostRepository) ([]InboundUserTarget, error) {
	if syncs == nil || hosts == nil {
		return nil, fmt.Errorf("user sync service unavailable / 用户同步服务不可用")
	}
	host, err := hosts.FindByToken(ctx, hostToken)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound // service sentinel: handler → 401
		}
		return nil, fmt.Errorf("locate agent by token: %w", err)
	}
	return syncs.ComputeForHost(ctx, host.ID)
}