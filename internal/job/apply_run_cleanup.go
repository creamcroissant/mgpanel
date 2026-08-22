package job

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/creamcroissant/mgpanel/internal/service"
)

// ApplyRunCleanupJob 定期清理超时未领取的 apply run。
type ApplyRunCleanupJob struct {
	applyOrchestrator service.ApplyOrchestratorService
	logger            *slog.Logger
}

// NewApplyRunCleanupJob creates a new ApplyRunCleanupJob.
func NewApplyRunCleanupJob(applyOrchestrator service.ApplyOrchestratorService, logger *slog.Logger) *ApplyRunCleanupJob {
	if logger == nil {
		logger = slog.Default()
	}
	return &ApplyRunCleanupJob{
		applyOrchestrator: applyOrchestrator,
		logger:            logger.With("job", "apply_run_cleanup"),
	}
}

// Name implements Runnable interface.
func (j *ApplyRunCleanupJob) Name() string {
	return "apply_run.cleanup"
}

// Run implements Runnable interface.
func (j *ApplyRunCleanupJob) Run(ctx context.Context) error {
	if j == nil || j.applyOrchestrator == nil {
		return fmt.Errorf("apply orchestrator not configured")
	}

	count, err := j.applyOrchestrator.CleanupExpiredApplyRuns(ctx, 30*time.Minute)
	if err != nil {
		return err
	}
	if count > 0 {
		j.logger.Info("cleaned up expired apply runs", "count", count)
	}
	return nil
}
