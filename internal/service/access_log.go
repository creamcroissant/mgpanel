package service

import (
	"context"
	"strconv"

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
}

func NewAccessLogService(store repository.Store) AccessLogService {
	return &accessLogService{
		logs:     store.AccessLogs(),
		users:    store.Users(),
		settings: store.Settings(),
	}
}

func (s *accessLogService) LogAccessRecords(ctx context.Context, agentHostID int64, records []*repository.AccessLog) error {
	if len(records) == 0 {
		return nil
	}

	// Cache email lookup results in this batch.
	// emailCached tracks whether the email has been looked up (hit/miss);
	// emailToID stores only successful lookups.
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
