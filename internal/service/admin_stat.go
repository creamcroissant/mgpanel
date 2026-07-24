// 文件路径: internal/service/admin_stat.go
// 模块说明: 这是 internal 模块里的 admin_stat 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/creamcroissant/xboard/internal/repository"
)

const (
	secondsPerDay           int64 = 24 * 60 * 60
	defaultOrderRangeDays   int64 = 30
	maxOrderRangeDays       int64 = 365
	defaultTrafficRangeDays int64 = 7
)

// AdminStatService exposes analytics for admin dashboards.
type AdminStatService interface {
	GetUserStats(ctx context.Context, input AdminStatUserInput) ([]AdminStatUserView, error)
	GetDashboardStats(ctx context.Context) (*AdminDashboardStats, error)
	GetTrafficRank(ctx context.Context, input AdminStatTrafficInput) (*AdminStatTrafficResult, error)
}

// AdminStatUserInput controls stat_users queries.
type AdminStatUserInput struct {
	UserID     int64
	RecordAt   int64
	RecordType int // 0: hourly, 1: daily
	Limit      int
}

// AdminStatUserView mirrors the payload expected by React Admin.
type AdminStatUserView struct {
	UserID     int64  `json:"user_id"`
	Email      string `json:"email"`
	Upload     int64  `json:"u"`
	Download   int64  `json:"d"`
	Total      int64  `json:"total"`
	RecordAt   int64  `json:"record_at"`
	RecordType int    `json:"record_type"`
}

// AdminDashboardStats aggregates KPI tiles for the admin home.
type AdminDashboardStats struct {
	DayIncomeGrowth        float64            `json:"dayIncomeGrowth"`
	CurrentMonthNewUsers   int64              `json:"currentMonthNewUsers"`
	UserGrowth             float64            `json:"userGrowth"`
	TotalUsers             int64              `json:"totalUsers"`
	ActiveUsers            int64              `json:"activeUsers"`
	MonthTraffic           AdminTrafficTotals `json:"monthTraffic"`
	TodayTraffic           AdminTrafficTotals `json:"todayTraffic"`
}

// AdminTrafficTotals summarizes upload/download usage.
type AdminTrafficTotals struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
	Total    int64 `json:"total"`
}

type AdminStatTrafficInput struct {
	Type      string
	StartTime int64
	EndTime   int64
	Limit     int
}

// AdminStatTrafficResult mirrors SPA expectation.
type AdminStatTrafficResult struct {
	Timestamp string                  `json:"timestamp"`
	Data      []AdminStatTrafficEntry `json:"data"`
}

// AdminStatTrafficEntry ranks a single subject (user focused for now).
type AdminStatTrafficEntry struct {
	UserID        int64   `json:"user_id"`
	Email         string  `json:"email"`
	Upload        int64   `json:"u"`
	Download      int64   `json:"d"`
	Total         int64   `json:"total"`
	PreviousTotal int64   `json:"previous_total"`
	Change        float64 `json:"change"`
	Timestamp     string  `json:"timestamp"`
}

type adminStatService struct {
	stats repository.StatUserRepository
	users repository.UserRepository
	now   func() time.Time
}

// NewAdminStatService wires repositories for admin statistics endpoints.
// Order-dependent metrics are disabled because the Order/Coupon tables were removed.
func NewAdminStatService(stats repository.StatUserRepository, users repository.UserRepository) AdminStatService {
	return &adminStatService{stats: stats, users: users, now: func() time.Time { return time.Now().UTC() }}
}

func (s *adminStatService) GetUserStats(ctx context.Context, input AdminStatUserInput) ([]AdminStatUserView, error) {
	if s == nil || s.stats == nil {
		return nil, fmt.Errorf("admin stat service not configured / 管理统计服务未配置")
	}
	if input.UserID > 0 {
		limit := clampStatLimit(input.Limit)
		since := input.RecordAt
		if since <= 0 {
			days := limit
			if days <= 0 {
				days = int(defaultTrafficRangeDays)
			}
			since = startOfDayUTC(s.nowOrDefault().AddDate(0, 0, -days+1))
		}
		records, err := s.stats.ListByUserSince(ctx, input.UserID, since, limit)
		if err != nil {
			return nil, err
		}
		email := ""
		if s.users != nil {
			if user, err := s.users.FindByID(ctx, input.UserID); err == nil && user != nil {
				email = strings.TrimSpace(user.Email)
			}
		}
		views := make([]AdminStatUserView, 0, len(records))
		sort.Slice(records, func(i, j int) bool { return records[i].RecordAt < records[j].RecordAt })
		for _, record := range records {
			views = append(views, AdminStatUserView{
				UserID:     record.UserID,
				Email:      email,
				Upload:     record.Upload,
				Download:   record.Download,
				Total:      record.Upload + record.Download,
				RecordAt:   record.RecordAt,
				RecordType: record.RecordType,
			})
		}
		return views, nil
	}
	recordType := input.RecordType
	limit := clampStatLimit(input.Limit)
	recordAt := input.RecordAt
	if recordAt <= 0 {
		recordAt = startOfDayUTC(s.nowOrDefault())
	}
	records, err := s.stats.ListByRecord(ctx, recordType, recordAt, nil, limit)
	if err != nil {
		return nil, err
	}
	views := make([]AdminStatUserView, 0, len(records))
	emailCache := make(map[int64]string, len(records))
	for _, record := range records {
		email := ""
		if s.users != nil {
			if cached, ok := emailCache[record.UserID]; ok {
				email = cached
			} else if user, err := s.users.FindByID(ctx, record.UserID); err == nil && user != nil {
				email = strings.TrimSpace(user.Email)
				emailCache[record.UserID] = email
			} else {
				emailCache[record.UserID] = ""
			}
		}
		views = append(views, AdminStatUserView{
			UserID:     record.UserID,
			Email:      email,
			Upload:     record.Upload,
			Download:   record.Download,
			Total:      record.Upload + record.Download,
			RecordAt:   record.RecordAt,
			RecordType: record.RecordType,
		})
	}
	return views, nil
}

func (s *adminStatService) GetDashboardStats(ctx context.Context) (*AdminDashboardStats, error) {
	if s == nil || s.stats == nil || s.users == nil {
		return nil, fmt.Errorf("admin stat service not configured / 管理统计服务未配置")
	}
	now := s.nowOrDefault()
	todayStart := startOfDayUTC(now)
	tomorrowStart := todayStart + secondsPerDay
	monthStart := startOfMonthUTC(now)
	lastMonthStart := startOfMonthUTC(now.AddDate(0, -1, 0))
	currentMonthUsers, err := s.users.CountCreatedBetween(ctx, monthStart, tomorrowStart)
	if err != nil {
		return nil, err
	}
	lastMonthUsers, err := s.users.CountCreatedBetween(ctx, lastMonthStart, monthStart)
	if err != nil {
		return nil, err
	}
	totalUsers, err := s.users.Count(ctx)
	if err != nil {
		return nil, err
	}
	activeUsers, err := s.users.CountActive(ctx, now.Unix())
	if err != nil {
		return nil, err
	}

	monthTraffic, err := s.stats.SumByRange(ctx, repository.StatUserSumFilter{RecordType: 1, StartAt: monthStart, EndAt: tomorrowStart})
	if err != nil {
		return nil, err
	}
	todayTraffic, err := s.stats.SumByRange(ctx, repository.StatUserSumFilter{RecordType: 1, StartAt: todayStart, EndAt: tomorrowStart})
	if err != nil {
		return nil, err
	}

	stats := &AdminDashboardStats{
		CurrentMonthNewUsers:   currentMonthUsers,
		UserGrowth:             percentageChange(float64(currentMonthUsers), float64(lastMonthUsers)),
		TotalUsers:             totalUsers,
		ActiveUsers:            activeUsers,
		MonthTraffic: AdminTrafficTotals{
			Upload:   monthTraffic.Upload,
			Download: monthTraffic.Download,
			Total:    monthTraffic.Upload + monthTraffic.Download,
		},
		TodayTraffic: AdminTrafficTotals{
			Upload:   todayTraffic.Upload,
			Download: todayTraffic.Download,
			Total:    todayTraffic.Upload + todayTraffic.Download,
		},
	}
	return stats, nil
}

func (s *adminStatService) GetTrafficRank(ctx context.Context, input AdminStatTrafficInput) (*AdminStatTrafficResult, error) {
	if s == nil || s.stats == nil {
		return nil, fmt.Errorf("admin stat service not configured / 管理统计服务未配置")
	}
	now := s.nowOrDefault()
	result := &AdminStatTrafficResult{Timestamp: now.UTC().Format(time.RFC3339)}
	if strings.EqualFold(strings.TrimSpace(input.Type), "node") {
		return result, nil
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 10
	} else if limit > 50 {
		limit = 50
	}
	startAt, endAt := normalizeTrafficRange(input.StartTime, input.EndTime, now)
	period := endAt - startAt
	if period <= 0 {
		period = secondsPerDay
	}
	previousStart := startAt - period
	current, err := s.stats.TopByRange(ctx, repository.StatUserTopFilter{RecordType: 1, StartAt: startAt, EndAt: endAt, Limit: limit})
	if err != nil {
		return nil, err
	}
	previous, err := s.stats.TopByRange(ctx, repository.StatUserTopFilter{RecordType: 1, StartAt: previousStart, EndAt: startAt, Limit: limit})
	if err != nil {
		return nil, err
	}
	previousMap := make(map[int64]repository.StatUserAggregate, len(previous))
	for _, agg := range previous {
		previousMap[agg.UserID] = agg
	}
	for _, agg := range current {
		email := fmt.Sprintf("user-%d", agg.UserID)
		if s.users != nil {
			if user, err := s.users.FindByID(ctx, agg.UserID); err == nil && user != nil {
				email = strings.TrimSpace(user.Email)
			}
		}
		total := agg.Upload + agg.Download
		prev := previousMap[agg.UserID]
		prevTotal := prev.Upload + prev.Download
		result.Data = append(result.Data, AdminStatTrafficEntry{
			UserID:        agg.UserID,
			Email:         email,
			Upload:        agg.Upload,
			Download:      agg.Download,
			Total:         total,
			PreviousTotal: prevTotal,
			Change:        percentageChange(float64(total), float64(prevTotal)),
			Timestamp:     result.Timestamp,
		})
	}
	return result, nil
}

func normalizeRecordType(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	switch trimmed {
	case "", "d", "day", "daily":
		return "d"
	case "m", "month", "monthly":
		return "m"
	default:
		return "d"
	}
}

func clampStatLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func startOfDayUTC(t time.Time) int64 {
	utc := t.UTC()
	y, m, d := utc.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix()
}

func (s *adminStatService) nowOrDefault() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func normalizeRange(startAt, endAt int64, now time.Time) (int64, int64) {
	end := endAt
	start := startAt
	if end <= 0 || end <= start {
		end = startOfDayUTC(now) + secondsPerDay
	}
	if start <= 0 || start >= end {
		start = end - defaultOrderRangeDays*secondsPerDay
	}
	if start < 0 {
		start = 0
	}
	maxRange := maxOrderRangeDays * secondsPerDay
	if end-start > maxRange {
		start = end - maxRange
	}
	return start, end
}

func normalizeTrafficRange(startAt, endAt int64, now time.Time) (int64, int64) {
	end := endAt
	start := startAt
	if end <= 0 || end <= start {
		end = now.UTC().Unix()
	}
	if start <= 0 || start >= end {
		start = end - defaultTrafficRangeDays*secondsPerDay
	}
	if start < 0 {
		start = 0
	}
	maxSpan := 90 * secondsPerDay
	if end-start > maxSpan {
		start = end - maxSpan
	}
	return start, end
}

func percentageChange(current, previous float64) float64 {
	if previous == 0 {
		if current == 0 {
			return 0
		}
		return 100
	}
	return roundTo(((current-previous)/previous)*100, 2)
}

func calculateAverage(total float64, count int64) float64 {
	if count <= 0 {
		return 0
	}
	return total / float64(count)
}

func calculatePercentage(part, total float64) float64 {
	if total == 0 {
		return 0
	}
	return (part / total) * 100
}

func roundTo(value float64, precision int) float64 {
	factor := math.Pow10(precision)
	return math.Round(value*factor) / factor
}

func normalizeOrderMetric(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	switch trimmed {
	case "paid_total", "paid_count", "commission_total", "commission_count":
		return trimmed
	default:
		return ""
	}
}
