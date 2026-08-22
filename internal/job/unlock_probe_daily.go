package job

import (
	"context"
	"log/slog"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/service"
)

// UnlockProbeDailyJob 每日对所有在线 agent 下发一次解锁检测命令。
type UnlockProbeDailyJob struct {
	unlockSvc  service.UnlockProbeService
	agentHosts repository.AgentHostRepository
	logger     *slog.Logger
}

// NewUnlockProbeDailyJob 创建每日解锁检测任务。
func NewUnlockProbeDailyJob(unlockSvc service.UnlockProbeService, agentHosts repository.AgentHostRepository, logger *slog.Logger) *UnlockProbeDailyJob {
	return &UnlockProbeDailyJob{
		unlockSvc:  unlockSvc,
		agentHosts: agentHosts,
		logger:     logger,
	}
}

// Run 遍历所有 agent host，为在线（status=1）的下发 unlock_probe 命令。
func (j *UnlockProbeDailyJob) Name() string {
	return "unlock_probe_daily"
}

func (j *UnlockProbeDailyJob) Run(ctx context.Context) error {
	if j == nil || j.unlockSvc == nil || j.agentHosts == nil {
		return nil
	}
	hosts, err := j.agentHosts.ListAll(ctx)
	if err != nil {
		if j.logger != nil {
			j.logger.Warn("unlock probe daily job: list agent hosts failed", "error", err)
		}
		return err
	}
	triggered := 0
	for _, h := range hosts {
		if h == nil || h.Status != 1 {
			continue // 仅在线 agent
		}
		if err := j.unlockSvc.TriggerProbe(ctx, h.ID, nil); err != nil {
			if j.logger != nil {
				j.logger.Warn("unlock probe daily job: trigger failed", "agent_host_id", h.ID, "error", err)
			}
			continue
		}
		triggered++
	}
	if j.logger != nil {
		j.logger.Info("unlock probe daily job completed", "triggered", triggered, "total", len(hosts))
	}
	return nil
}
