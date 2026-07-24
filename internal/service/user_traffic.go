package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/creamcroissant/xboard/internal/async"
	"github.com/creamcroissant/xboard/internal/notifier"
	"github.com/creamcroissant/xboard/internal/repository"
)

// TrafficStatCollectorWithHost collects raw traffic deltas with agent host tracking.
type TrafficStatCollectorWithHost interface {
	CollectWithHost(agentHostID, userID int64, uploadDelta, downloadDelta int64)
}

// UserTrafficDelta represents a single user's traffic delta for batch processing.
type UserTrafficDelta = repository.UserTrafficDelta

// TrafficProcessResult contains the result of batch traffic processing.
type TrafficProcessResult struct {
	AcceptedCount   int32
	ExceededUserIDs []int64
}

// UserTrafficService manages user traffic statistics and periods.
type UserTrafficService interface {
	// ProcessTraffic updates user traffic usage and checks quotas (with agent host tracking)
	ProcessTraffic(ctx context.Context, agentHostID int64, userID int64, upload, download int64) error
	// ProcessTrafficBatch processes multiple user traffic deltas in batch
	ProcessTrafficBatch(ctx context.Context, agentHostID int64, traffic []UserTrafficDelta) (*TrafficProcessResult, error)
	// GetTrafficStats returns the current traffic statistics for a user
	GetTrafficStats(ctx context.Context, userID int64) (*repository.UserTrafficStats, error)

	// Server selection management
	AddServerSelection(ctx context.Context, userID, serverID int64) error
	RemoveServerSelection(ctx context.Context, userID, serverID int64) error
	GetUserServers(ctx context.Context, userID int64) ([]int64, error)
	ClearUserServers(ctx context.Context, userID int64) error

	// Traffic period management
	EnsureCurrentPeriod(ctx context.Context, userID int64) (*repository.UserTrafficPeriod, error)
	ResetExpiredPeriods(ctx context.Context) (int, error)
	GetExceededUsers(ctx context.Context) ([]int64, error)
	ResetUserExceededStatus(ctx context.Context, userID int64) error
}

type userTrafficService struct {
	trafficRepo       repository.UserTrafficRepository
	userRepo          repository.UserRepository
	plans             repository.PlanRepository
	statCollector     TrafficStatCollectorWithHost
	notificationQueue *async.NotificationQueue
	settings          repository.SettingRepository
}

// NewUserTrafficService creates a new UserTrafficService.
func NewUserTrafficService(trafficRepo repository.UserTrafficRepository, userRepo repository.UserRepository, plans repository.PlanRepository) UserTrafficService {
	return &userTrafficService{
		trafficRepo: trafficRepo,
		userRepo:    userRepo,
		plans:       plans,
	}
}

// NewUserTrafficServiceWithCollector creates a UserTrafficService with a stat collector.
func NewUserTrafficServiceWithCollector(
	trafficRepo repository.UserTrafficRepository,
	userRepo repository.UserRepository,
	plans repository.PlanRepository,
	collector TrafficStatCollectorWithHost,
	notificationQueue *async.NotificationQueue,
	settings repository.SettingRepository,
) UserTrafficService {
	return &userTrafficService{
		trafficRepo:       trafficRepo,
		userRepo:          userRepo,
		plans:             plans,
		statCollector:     collector,
		notificationQueue: notificationQueue,
		settings:          settings,
	}
}

// ProcessTraffic updates user traffic usage and checks quotas.
func (s *userTrafficService) ProcessTraffic(ctx context.Context, agentHostID int64, userID int64, upload, download int64) error {
	// 1. Get or create current period
	period, err := s.trafficRepo.GetCurrentPeriod(ctx, userID)
	if err != nil {
		return err
	}

	if period == nil {
		// Need to create a new period
		user, err := s.userRepo.FindByID(ctx, userID)
		if err != nil {
			return err
		}
		if user == nil {
			// User not found, ignore
			return nil
		}

		now := time.Now()
		// Calculate period based on settings
		start, end := s.calculatePeriodTimes(ctx, user, now)

		period = &repository.UserTrafficPeriod{
			UserID:      userID,
			PeriodStart: start,
			PeriodEnd:   end,
			QuotaBytes:  user.TransferEnable,
			Exceeded:    false,
		}

		if err := s.trafficRepo.CreatePeriod(ctx, period); err != nil {
			// Another goroutine may have created the period concurrently
			period, err = s.trafficRepo.GetCurrentPeriod(ctx, userID)
			if err != nil {
				return err
			}
			if period == nil {
				return fmt.Errorf("failed to create period and no existing period found: %w", err)
			}
		}
	}

	// 2. Increment traffic (atomic operation)
	if err := s.trafficRepo.IncrementPeriodTraffic(ctx, userID, upload, download); err != nil {
		return err
	}

	// 3. Re-fetch the period to get actual current traffic values after the atomic increment.
	//    This prevents a TOCTOU race where the old period values read before the atomic update
	//    would miss traffic from concurrent requests.
	updatedPeriod, err := s.trafficRepo.GetCurrentPeriod(ctx, userID)
	if err != nil {
		return err
	}
	if updatedPeriod == nil {
		// Period was removed concurrently; nothing more to do.
		return nil
	}
	period = updatedPeriod

	// 4. Check quota if not already exceeded
	if !period.Exceeded && period.QuotaBytes > 0 {
		currentUpload := period.UploadBytes
		currentDownload := period.DownloadBytes
		// 检测溢出：如果 currentUpload + currentDownload 超过 MaxInt64，直接视为超配额
		total := currentUpload + currentDownload
		if total < currentUpload || total < currentDownload {
			total = math.MaxInt64
		}
		if total >= period.QuotaBytes {
			// Mark period as exceeded
			if err := s.trafficRepo.MarkPeriodExceeded(ctx, userID, period.PeriodStart); err != nil {
				return err
			}
			// Update user status
			if err := s.userRepo.SetTrafficExceeded(ctx, userID, true); err != nil {
				return err
			}
			// Send notification
			s.sendExceededNotification(ctx, userID)
		}
	}

	// 5. Also update the legacy user.u and user.d columns for backward compatibility
	if err := s.userRepo.IncrementTraffic(ctx, userID, upload, download); err != nil {
		return err
	}

	// 6. Collect traffic delta for stat_users aggregation (with agent host tracking)
	if s.statCollector != nil {
		s.statCollector.CollectWithHost(agentHostID, userID, upload, download)
	}

	return nil
}

func (s *userTrafficService) sendExceededNotification(ctx context.Context, userID int64) {
	if s.notificationQueue == nil {
		return
	}
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil || user.TelegramID == 0 {
		return
	}

	s.notificationQueue.EnqueueTelegram(notifier.TelegramRequest{
		ChatID:    fmt.Sprintf("%d", user.TelegramID),
		Message:   "⚠️ *Traffic Exceeded Alert*\n\nYour traffic quota has been exceeded and your service has been suspended. Please renew your subscription.",
		ParseMode: "Markdown",
	})
}

// ProcessTrafficBatch processes multiple user traffic deltas in batch.
func (s *userTrafficService) ProcessTrafficBatch(ctx context.Context, agentHostID int64, traffic []UserTrafficDelta) (*TrafficProcessResult, error) {
	nowUnix := time.Now().Unix()
	accepted, exceededUserIDs, err := s.trafficRepo.ApplyTrafficBatchAtomic(ctx, traffic, nowUnix)
	if err != nil {
		return nil, err
	}

	result := &TrafficProcessResult{
		AcceptedCount: int32(len(accepted)),
	}
	if len(exceededUserIDs) > 0 {
		result.ExceededUserIDs = append(result.ExceededUserIDs, exceededUserIDs...)
		for _, userID := range exceededUserIDs {
			s.sendExceededNotification(ctx, userID)
		}
	}

	if s.statCollector != nil {
		for _, item := range accepted {
			s.statCollector.CollectWithHost(agentHostID, item.UserID, item.Upload, item.Download)
		}
	}

	return result, nil
}

// GetTrafficStats returns the current traffic statistics for a user.
func (s *userTrafficService) GetTrafficStats(ctx context.Context, userID int64) (*repository.UserTrafficStats, error) {
	return s.trafficRepo.GetUserTrafficStats(ctx, userID)
}

// AddServerSelection adds a server to the user's selection.
func (s *userTrafficService) AddServerSelection(ctx context.Context, userID, serverID int64) error {
	return s.trafficRepo.AddServerSelection(ctx, userID, serverID)
}

// RemoveServerSelection removes a server from the user's selection.
func (s *userTrafficService) RemoveServerSelection(ctx context.Context, userID, serverID int64) error {
	return s.trafficRepo.RemoveServerSelection(ctx, userID, serverID)
}

// GetUserServers returns the list of server IDs selected by the user.
func (s *userTrafficService) GetUserServers(ctx context.Context, userID int64) ([]int64, error) {
	return s.trafficRepo.GetUserServerIDs(ctx, userID)
}

// ClearUserServers clears all server selections for a user.
func (s *userTrafficService) ClearUserServers(ctx context.Context, userID int64) error {
	return s.trafficRepo.ClearUserSelections(ctx, userID)
}

// EnsureCurrentPeriod gets or creates the current traffic period for a user.
func (s *userTrafficService) EnsureCurrentPeriod(ctx context.Context, userID int64) (*repository.UserTrafficPeriod, error) {
	period, err := s.trafficRepo.GetCurrentPeriod(ctx, userID)
	if err != nil {
		return nil, err
	}

	if period != nil {
		return period, nil
	}

	// Need to create a new period
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}

	now := time.Now()
	start, end := s.calculatePeriodTimes(ctx, user, now)

	period = &repository.UserTrafficPeriod{
		UserID:      userID,
		PeriodStart: start,
		PeriodEnd:   end,
		QuotaBytes:  user.TransferEnable,
		Exceeded:    false,
	}

	if err := s.trafficRepo.CreatePeriod(ctx, period); err != nil {
		return nil, err
	}

	return period, nil
}

// ResetExpiredPeriods checks for users with expired periods and creates new ones.
// Returns the number of users processed.
func (s *userTrafficService) ResetExpiredPeriods(ctx context.Context) (int, error) {
	now := time.Now()
	nowUnix := now.Unix()

	// Get users with expired periods
	userIDs, err := s.trafficRepo.GetExpiredPeriodUserIDs(ctx, nowUnix)
	if err != nil {
		return 0, err
	}

	processed := 0
	var errs []error
	for _, userID := range userIDs {
		// Get user to retrieve their quota
		user, err := s.userRepo.FindByID(ctx, userID)
		if err != nil {
			errs = append(errs, fmt.Errorf("find user %d: %w", userID, err))
			continue
		}
		if user == nil {
			continue
		}

		// Check if user's plan prohibits traffic reset
		if user.PlanID > 0 && s.plans != nil {
			plan, err := s.plans.FindByID(ctx, user.PlanID)
			if err == nil && plan != nil && plan.ResetTrafficMethod != nil && *plan.ResetTrafficMethod == planResetNever {
				continue
			}
		}

		// Create new period based on settings
		start, end := s.calculatePeriodTimes(ctx, user, now)

		period := &repository.UserTrafficPeriod{
			UserID:      userID,
			PeriodStart: start,
			PeriodEnd:   end,
			QuotaBytes:  user.TransferEnable,
			Exceeded:    false,
		}

		if err := s.trafficRepo.CreatePeriod(ctx, period); err != nil {
			errs = append(errs, fmt.Errorf("create period for user %d: %w", userID, err))
			continue
		}

		// Reset user's exceeded status if they have a new quota
		if user.TrafficExceeded && user.TransferEnable > 0 {
			if err := s.userRepo.SetTrafficExceeded(ctx, userID, false); err != nil {
				errs = append(errs, fmt.Errorf("reset exceeded for user %d: %w", userID, err))
			}
		}

		processed++
	}

	if len(errs) > 0 {
		return processed, errors.Join(errs...)
	}
	return processed, nil
}

// GetExceededUsers returns the list of user IDs who have exceeded their traffic quota.
func (s *userTrafficService) GetExceededUsers(ctx context.Context) ([]int64, error) {
	return s.trafficRepo.GetExceededUserIDs(ctx)
}

// ResetUserExceededStatus resets the traffic exceeded status for a user.
func (s *userTrafficService) ResetUserExceededStatus(ctx context.Context, userID int64) error {
	return s.userRepo.SetTrafficExceeded(ctx, userID, false)
}

func (s *userTrafficService) calculatePeriodTimes(ctx context.Context, user *repository.User, now time.Time) (int64, int64) {
	// 当 settings 为 nil 时（通过 NewUserTrafficService 创建时），使用默认周期
	if s.settings == nil {
		return s.calculateDefaultPeriodTimes(now)
	}
	modeSetting, _ := s.settings.Get(ctx, "traffic_reset_mode")
	mode := ""
	if modeSetting != nil {
		mode = modeSetting.Value
	}
	targetDay := 1

	if mode == "subscription" {
		if user.CreatedAt > 0 {
			targetDay = time.Unix(user.CreatedAt, 0).Day()
		}
	} else {
		if daySetting, err := s.settings.Get(ctx, "traffic_reset_day"); err == nil && daySetting != nil && daySetting.Value != "" {
			if d, err := strconv.Atoi(daySetting.Value); err == nil {
				targetDay = d
			}
		}
	}

	// Cap at 28 to avoid month length issues
	if targetDay > 28 {
		targetDay = 28
	}
	if targetDay < 1 {
		targetDay = 1
	}

	var startYear int
	var startMonth time.Month

	if now.Day() >= targetDay {
		startYear = now.Year()
		startMonth = now.Month()
	} else {
		// Previous month
		prev := now.AddDate(0, -1, 0)
		startYear = prev.Year()
		startMonth = prev.Month()
	}

	startTime := time.Date(startYear, startMonth, targetDay, 0, 0, 0, 0, now.Location())
	endTime := startTime.AddDate(0, 1, 0)

	return startTime.Unix(), endTime.Unix()
}

func (s *userTrafficService) calculateDefaultPeriodTimes(now time.Time) (int64, int64) {
	startTime := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endTime := startTime.AddDate(0, 1, 0)
	return startTime.Unix(), endTime.Unix()
}
