package job

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/creamcroissant/mgpanel/internal/service"
)

// AccessLogCleanupJob handles cleanup of old access logs.
type AccessLogCleanupJob struct {
	AccessLogService service.AccessLogService
	Logger           *slog.Logger
}

// NewAccessLogCleanupJob creates a new AccessLogCleanupJob.
func NewAccessLogCleanupJob(accessLogService service.AccessLogService, logger *slog.Logger) *AccessLogCleanupJob {
	if logger == nil {
		logger = slog.Default()
	}
	return &AccessLogCleanupJob{
		AccessLogService: accessLogService,
		Logger:           logger,
	}
}

// Name implements Runnable interface.
func (j *AccessLogCleanupJob) Name() string {
	return "access_log.cleanup"
}

// Run implements Runnable interface.
func (j *AccessLogCleanupJob) Run(ctx context.Context) error {
	if j == nil || j.AccessLogService == nil {
		return fmt.Errorf("access log cleanup job dependencies not configured / 访问日志清理任务依赖未配置")
	}

	if !j.AccessLogService.IsEnabled(ctx) {
		return nil
	}

	deleted, err := j.AccessLogService.CleanupOldLogs(ctx)
	if err != nil {
		return fmt.Errorf("access log cleanup job: %w", err)
	}

	if deleted > 0 {
		j.Logger.Info("cleaned up old access logs", "deleted_rows", deleted)
	}

	return nil
}
