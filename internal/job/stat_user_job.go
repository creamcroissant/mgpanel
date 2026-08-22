// 文件路径: internal/job/stat_user_job.go
// 模块说明: 定时任务，将累加器中的流量增量刷新到 stat_users 表
package job

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// RecordType 定义统计粒度。
const (
	RecordTypeHourly  = 0
	RecordTypeDaily   = 1
	RecordTypeMonthly = 2
)

// StatUserJob 将累加的用户流量写入 stat_users。
type StatUserJob struct {
	Accumulator *StatUserAccumulator
	Repo        repository.StatUserRepository
	Logger      *slog.Logger

	recordType int // 0: hourly, 1: daily, 2: monthly
	now        func() time.Time
}

// NewStatUserJob 组装统计任务（默认按日）。
func NewStatUserJob(acc *StatUserAccumulator, repo repository.StatUserRepository, logger *slog.Logger) *StatUserJob {
	if logger == nil {
		logger = slog.Default()
	}
	return &StatUserJob{
		Accumulator: acc,
		Repo:        repo,
		Logger:      logger,
		recordType:  RecordTypeDaily,
		now:         time.Now,
	}
}

// NewStatUserJobWithType 创建指定粒度的统计任务。
func NewStatUserJobWithType(acc *StatUserAccumulator, repo repository.StatUserRepository, logger *slog.Logger, recordType int) *StatUserJob {
	if logger == nil {
		logger = slog.Default()
	}
	return &StatUserJob{
		Accumulator: acc,
		Repo:        repo,
		Logger:      logger,
		recordType:  recordType,
		now:         time.Now,
	}
}

// Name 返回任务标识。
func (j *StatUserJob) Name() string {
	switch j.recordType {
	case RecordTypeHourly:
		return "stat.user.hourly"
	case RecordTypeMonthly:
		return "stat.user.monthly"
	default:
		return "stat.user.daily"
	}
}

// Run 刷新统计数据到持久化存储。
func (j *StatUserJob) Run(ctx context.Context) error {
	if j == nil || j.Repo == nil || j.Accumulator == nil {
		return fmt.Errorf("stat user job dependencies not configured / 用户流量统计任务依赖未配置")
	}
	pending := j.Accumulator.Flush()
	if len(pending) == 0 {
		return nil
	}
	pendingCount := len(pending)

	nowTime := j.now()
	nowUnix := nowTime.Unix()

	// 计算统计时间点
	var recordAt int64
	switch j.recordType {
	case RecordTypeHourly:
		recordAt = hourStart(nowTime)
	case RecordTypeMonthly:
		recordAt = monthStart(nowTime)
	default:
		recordAt = dayStart(nowTime)
	}

	records := make([]repository.StatUserRecord, 0, len(pending))
	skipped := 0
	for key, delta := range pending {
		if delta.Upload == 0 && delta.Download == 0 {
			skipped++

			continue
		}
		records = append(records, repository.StatUserRecord{
			UserID:      key.UserID,
			AgentHostID: key.AgentHostID,
			ServerRate:  1,
			RecordAt:    recordAt,
			RecordType:  j.recordType,
			Upload:      delta.Upload,
			Download:    delta.Download,
			CreatedAt:   nowUnix,
			UpdatedAt:   nowUnix,
		})

	}

	if len(records) == 0 {
		if j.Logger != nil {
			j.Logger.Debug("flushed stat users",
				"count", pendingCount,
				"accepted", 0,
				"skipped", skipped,
				"record_type", j.recordType,
			)
		}
		return nil
	}

	if err := j.Repo.UpsertBatch(ctx, records); err != nil {
		j.Accumulator.Merge(pending)
		return fmt.Errorf("stat user job: upsert batch: %w", err)
	}

	if j.Logger != nil {
		j.Logger.Debug("flushed stat users",
			"count", pendingCount,
			"accepted", len(records),
			"skipped", skipped,
			"record_type", j.recordType,
		)
	}
	return nil
}

// hourStart 返回按小时对齐的 UTC 时间戳。
func hourStart(t time.Time) int64 {
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), utc.Hour(), 0, 0, 0, time.UTC).Unix()
}

// dayStart 返回按天对齐的 UTC 时间戳。
func dayStart(t time.Time) int64 {
	utc := t.UTC()
	y, m, d := utc.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix()
}

// monthStart 返回按月对齐的 UTC 时间戳。
func monthStart(t time.Time) int64 {
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC).Unix()
}
