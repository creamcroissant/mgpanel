package job

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/creamcroissant/mgpanel/internal/service"
)

type AgentTrafficResetJob struct {
	TrafficLifecycle service.AgentTrafficLifecycleService
	Logger           *slog.Logger
	Now              func() time.Time
}

func NewAgentTrafficResetJob(trafficLifecycle service.AgentTrafficLifecycleService, logger *slog.Logger) *AgentTrafficResetJob {
	if logger == nil {
		logger = slog.Default()
	}
	return &AgentTrafficResetJob{
		TrafficLifecycle: trafficLifecycle,
		Logger:           logger,
	}
}

func (j *AgentTrafficResetJob) Name() string {
	return "agent.traffic.reset"
}

func (j *AgentTrafficResetJob) Run(ctx context.Context) error {
	if j == nil || j.TrafficLifecycle == nil {
		return fmt.Errorf("agent traffic reset job dependencies not configured / Agent 流量重置任务依赖未配置")
	}
	now := time.Now
	if j.Now != nil {
		now = j.Now
	}
	result, err := j.TrafficLifecycle.RunScheduledResets(ctx, now().Unix())
	if err != nil {
		return fmt.Errorf("agent traffic reset job: %w", err)
	}
	if result == nil {
		j.Logger.Debug("agent traffic reset returned no result")
		return nil
	}
	if result.Processed > 0 || result.Failed > 0 {
		j.Logger.Info("processed agent traffic scheduled resets", "processed", result.Processed, "skipped", result.Skipped, "failed", result.Failed)
	} else {
		j.Logger.Debug("no agent traffic cycles to reset", "skipped", result.Skipped)
	}
	return nil
}
