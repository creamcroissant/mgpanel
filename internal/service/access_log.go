package service

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/creamcroissant/mgpanel/internal/async"
	"github.com/creamcroissant/mgpanel/internal/repository"
)

// AccessLogService manages access logging logic.
type AccessLogService interface {
	LogAccessRecords(ctx context.Context, agentHostID int64, records []*repository.AccessLog) error
	ListAccessLogs(ctx context.Context, filter repository.AccessLogFilter) ([]*repository.AccessLog, int64, error)
	GetStats(ctx context.Context, filter repository.AccessLogFilter) (*repository.AccessLogStats, error)
	CleanupOldLogs(ctx context.Context) (int64, error)
	IsEnabled(ctx context.Context) bool
}

type accessLogService struct {
	logs     repository.AccessLogRepository
	users    repository.UserRepository
	settings repository.SettingRepository
	queue    *async.AccessLogQueue
}

func NewAccessLogService(store repository.Store) AccessLogService {
	return &accessLogService{
		logs:     store.AccessLogs(),
		users:    store.Users(),
		settings: store.Settings(),
	}
}

// NewAccessLogServiceWithQueue 构建带异步队列的访问日志服务。
// 队列 worker 在后台批量解析 email 并落库，避免 gRPC handler 阻塞。
func NewAccessLogServiceWithQueue(store repository.Store, logger *slog.Logger) AccessLogService {
	svc := &accessLogService{
		logs:     store.AccessLogs(),
		users:    store.Users(),
		settings: store.Settings(),
	}
	svc.queue = async.NewAccessLogQueue(svc.handleBatchFlush, logger)
	return svc
}

func (s *accessLogService) LogAccessRecords(ctx context.Context, agentHostID int64, records []*repository.AccessLog) error {
	if len(records) == 0 {
		return nil
	}
	// If async queue is available, enqueue for background processing
	if s.queue != nil {
		s.queue.Enqueue(agentHostID, records)
		return nil
	}
	// Legacy synchronous path (fallback if no queue configured)
	return s.logRecordsSync(ctx, agentHostID, records)
}
// logRecordsSync 是同步写入路径（email 解析 + BatchCreate）。
func (s *accessLogService) logRecordsSync(ctx context.Context, agentHostID int64, records []*repository.AccessLog) error {
	emailToID := make(map[string]int64)
	emailCached := make(map[string]bool)

	for _, record := range records {
		record.AgentHostID = agentHostID

		if record.UserEmail == "" {
			continue
		}

		if emailCached[record.UserEmail] {
			if uid, ok := emailToID[record.UserEmail]; ok {
				resolvedID := uid
				record.UserID = &resolvedID
			}
			continue
		}

		user, err := s.users.FindByEmail(ctx, record.UserEmail)
		emailCached[record.UserEmail] = true
		if err == nil && user != nil {
			emailToID[record.UserEmail] = user.ID
			resolvedID := user.ID
			record.UserID = &resolvedID
		}
	}

	return s.logs.BatchCreate(ctx, records)
}

// Stop 优雅停止访问日志队列 worker（停机排水用）。不加入接口，避免 mock 扩散。
func (s *accessLogService) Stop() {
	if s == nil || s.queue == nil {
		return
	}
	s.queue.Stop()
}

// handleBatchFlush 是 AccessLogQueue 的批量落库回调（异步 worker 消费时调用）。
func (s *accessLogService) handleBatchFlush(ctx context.Context, batch async.AccessLogBatch) error {
	return s.logRecordsSync(ctx, batch.AgentHostID, batch.Records)
}

func (s *accessLogService) ListAccessLogs(ctx context.Context, filter repository.AccessLogFilter) ([]*repository.AccessLog, int64, error) {
	logs, err := s.logs.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	count, err := s.logs.Count(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	return logs, count, nil
}

func (s *accessLogService) GetStats(ctx context.Context, filter repository.AccessLogFilter) (*repository.AccessLogStats, error) {
	return s.logs.GetStats(ctx, filter)
}

func (s *accessLogService) CleanupOldLogs(ctx context.Context) (int64, error) {
	setting, err := s.settings.Get(ctx, "access_log.retention_days")
	days := 7 // Default
	if err == nil && setting != nil {
		if d, err := strconv.Atoi(setting.Value); err == nil && d > 0 {
			days = d
		}
	}
	return s.logs.DeleteByRetentionDays(ctx, days)
}

func (s *accessLogService) IsEnabled(ctx context.Context) bool {
	setting, err := s.settings.Get(ctx, "access_log.enabled")
	if err != nil || setting == nil {
		return false
	}
	return setting.Value == "1" || setting.Value == "true"
}

// RetentionTarget 描述一个需要定期清理的数据表。
type RetentionTarget struct {
	Table        string                                                    // 表名（日志用）
	SettingKey   string                                                    // settings 表中的配置 key
	DefaultDays  int                                                       // 默认保留天数
	DeleteFn     func(ctx context.Context, days int) (int64, error)        // 清理回调
}
