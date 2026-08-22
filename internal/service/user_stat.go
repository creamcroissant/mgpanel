// 文件路径: internal/service/user_stat.go
// 模块说明: 这是 internal 模块里的 user_stat 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// UserStatService exposes per-user traffic log queries.
type UserStatService interface {
	TrafficLogs(ctx context.Context, userID string) ([]UserTrafficLog, error)
}

// UserTrafficLog mirrors the payload expected by V1 client traffic history.
type UserTrafficLog struct {
	RecordAt int64 `json:"record_at"`
	Upload   int64 `json:"u"`
	Download int64 `json:"d"`
	Total    int64 `json:"total"`
}

type userStatService struct {
	stats repository.StatUserRepository
	now   func() time.Time
}

// NewUserStatService wires repository-backed traffic log access.
func NewUserStatService(stats repository.StatUserRepository) UserStatService {
	return &userStatService{stats: stats, now: time.Now}
}

func (s *userStatService) TrafficLogs(ctx context.Context, userID string) ([]UserTrafficLog, error) {
	if s == nil || s.stats == nil {
		return nil, fmt.Errorf("user stat service not configured / 用户统计服务未配置")
	}
	// userID 是字符串格式（token 里提取的），这里转成 int64，能最早发现参数错误。
	uid, err := parseUserID(userID)
	if err != nil {
		return nil, err
	}
	// 只拉本月数据，startOfMonthUTC 会自动把时间对齐到月初 00:00。
	since := startOfMonthUTC(s.now())
	records, err := s.stats.ListByUserSince(ctx, uid, since, 62)
	if err != nil {
		return nil, err
	}
	logs := make([]UserTrafficLog, 0, len(records))
	for _, record := range records {
		// 每条记录都带上 upload/download，并额外算一个 total，方便前端直接显示。
		logs = append(logs, UserTrafficLog{
			RecordAt: record.RecordAt,
			Upload:   record.Upload,
			Download: record.Download,
			Total:    record.Upload + record.Download,
		})
	}
	return logs, nil
}

func startOfMonthUTC(t time.Time) int64 {
	utc := t.UTC()
	y, m, _ := utc.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC).Unix()
}
