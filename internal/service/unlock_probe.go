package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// UnlockProbeService 管理流媒体解锁检测结果。
type UnlockProbeService interface {
	// UpsertResult 接收 agent 上报的单条结果并存储。
	UpsertResult(ctx context.Context, agentHostID int64, service, status, region, detail string) error
	// BatchUpsertResults 批量接收 agent 上报的检测结果。
	BatchUpsertResults(ctx context.Context, agentHostID int64, results []UnlockProbeResultItem) error
	// ListByAgentHost 查询某个 agent 的最新结果。
	ListByAgentHost(ctx context.Context, agentHostID int64) ([]*repository.UnlockProbeResult, error)
	// ListAll 查询所有结果。
	ListAll(ctx context.Context) ([]*repository.UnlockProbeResult, error)
	// TriggerProbe 向指定 agent 下发 unlock_probe 命令。
	TriggerProbe(ctx context.Context, agentHostID int64, services []string) error
}

// UnlockProbeResultItem 是 agent 上报的单个检测结果。
type UnlockProbeResultItem struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Region  string `json:"region"`
	Detail  string `json:"detail"`
}

type unlockProbeService struct {
	results      repository.UnlockProbeResultRepository
	agentHosts   repository.AgentHostRepository
	lifecycleOps AgentLifecycleOperationService
}

// NewUnlockProbeService 创建 UnlockProbeService。
func NewUnlockProbeService(
	results repository.UnlockProbeResultRepository,
	agentHosts repository.AgentHostRepository,
	lifecycleOps AgentLifecycleOperationService,
) UnlockProbeService {
	return &unlockProbeService{
		results:      results,
		agentHosts:   agentHosts,
		lifecycleOps: lifecycleOps,
	}
}

func (s *unlockProbeService) UpsertResult(ctx context.Context, agentHostID int64, service, status, region, detail string) error {
	now := time.Now().Unix()
	return s.results.Upsert(ctx, &repository.UnlockProbeResult{
		AgentHostID: agentHostID,
		Service:     service,
		Status:      status,
		Region:      region,
		Detail:      detail,
		ProbedAt:    now,
		CreatedAt:   now,
	})
}

// BatchUpsertResults 批量存储 agent 上报的检测结果。
func (s *unlockProbeService) BatchUpsertResults(ctx context.Context, agentHostID int64, items []UnlockProbeResultItem) error {
	now := time.Now().Unix()
	for _, item := range items {
		if err := s.results.Upsert(ctx, &repository.UnlockProbeResult{
			AgentHostID: agentHostID,
			Service:     item.Service,
			Status:      item.Status,
			Region:      item.Region,
			Detail:      item.Detail,
			ProbedAt:    now,
			CreatedAt:   now,
		}); err != nil {
			return fmt.Errorf("upsert result for %s: %w", item.Service, err)
		}
	}
	return nil
}

func (s *unlockProbeService) ListByAgentHost(ctx context.Context, agentHostID int64) ([]*repository.UnlockProbeResult, error) {
	return s.results.ListByAgentHost(ctx, agentHostID)
}

func (s *unlockProbeService) ListAll(ctx context.Context) ([]*repository.UnlockProbeResult, error) {
	return s.results.ListAll(ctx)
}

func (s *unlockProbeService) TriggerProbe(ctx context.Context, agentHostID int64, services []string) error {
	payload, _ := json.Marshal(map[string]any{"services": services})
	_, err := s.lifecycleOps.Create(ctx, CreateAgentLifecycleOperationRequest{
		AgentHostID:    agentHostID,
		OperationType:  AgentLifecycleOperationTypeUnlockProbe,
		RequestPayload: payload,
	})
	return err
}