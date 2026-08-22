// 文件路径: internal/job/traffic_period_reset.go
// 模块说明: 流量周期重置定时任务，每日检查并重置过期的流量周期
package job

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/creamcroissant/mgpanel/internal/service"
)

// TrafficPeriodResetJob handles resetting expired traffic periods.
type TrafficPeriodResetJob struct {
	TrafficService service.UserTrafficService
	Logger         *slog.Logger
}

// NewTrafficPeriodResetJob creates a new TrafficPeriodResetJob.
func NewTrafficPeriodResetJob(trafficService service.UserTrafficService, logger *slog.Logger) *TrafficPeriodResetJob {
	if logger == nil {
		logger = slog.Default()
	}
	return &TrafficPeriodResetJob{
		TrafficService: trafficService,
		Logger:         logger,
	}
}

// Name implements Runnable interface.
func (j *TrafficPeriodResetJob) Name() string {
	return "traffic.period.reset"
}

// Run implements Runnable interface.
// It checks for users with expired traffic periods and creates new ones.
func (j *TrafficPeriodResetJob) Run(ctx context.Context) error {
	if j == nil || j.TrafficService == nil {
		return fmt.Errorf("traffic period reset job dependencies not configured / 流量周期重置任务依赖未配置")
	}

	processed, err := j.TrafficService.ResetExpiredPeriods(ctx)
	if err != nil {
		return fmt.Errorf("traffic period reset job: %w", err)
	}

	if processed > 0 {
		j.Logger.Info("reset expired traffic periods", "users_processed", processed)
	} else {
		j.Logger.Debug("no expired traffic periods to reset")
	}

	return nil
}
