// 文件路径: internal/job/retention_cleanup.go
// 模块说明: 通用数据保留清理任务——按 settings 表配置的保留天数清理所有膨胀表。

package job

import (
	"context"
	"log/slog"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/service"
)

// RetentionCleanupJob 按配置的保留天数清理有膨胀风险的数据表。
// 各表保留天数可在 /admin/system 页面配置（settings 表 retention 分类）。
type RetentionCleanupJob struct {
	settings repository.SettingRepository
	logs     []service.RetentionTarget // 需要清理的表
	logger   *slog.Logger
}

// NewRetentionCleanupJob 构建通用保留清理任务。
func NewRetentionCleanupJob(settings repository.SettingRepository, targets []service.RetentionTarget, logger *slog.Logger) *RetentionCleanupJob {
	return &RetentionCleanupJob{settings: settings, logs: targets, logger: logger}
}

// Name 返回任务名。
func (j *RetentionCleanupJob) Name() string { return "retention.cleanup" }

// Run 遍历所有膨胀表，读取其保留天数配置并清理过期数据。
func (j *RetentionCleanupJob) Run(ctx context.Context) error {
	if j.settings == nil || j.logger == nil {
		return nil
	}
	for _, target := range j.logs {
		// 读取保留天数，默认使用 target.DefaultDays
		days := target.DefaultDays
		if val, err := j.settings.Get(ctx, target.SettingKey); err == nil {
			if n, perr := parsePositiveInt(val.Value); perr == nil && n > 0 {
				days = n
			}
		}
		if days <= 0 {
			continue
		}
		deleted, err := target.DeleteFn(ctx, days)
		if err != nil {
			j.logger.Error("retention cleanup failed",
				"table", target.Table, "days", days, "error", err)
			continue
		}
		if deleted > 0 {
			j.logger.Info("retention cleanup",
				"table", target.Table, "days", days, "deleted", deleted)
		}
	}
	return nil
}

// parsePositiveInt 解析非负整数配置值。
func parsePositiveInt(s string) (int, error) {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, errInvalidInt
		}
		n = n*10 + int(ch-'0')
	}
	return n, nil
}

var errInvalidInt = &invalidIntError{}

type invalidIntError struct{}

func (*invalidIntError) Error() string { return "invalid integer" }
