package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

const (
	agentTrafficDefaultMaxDeltaBytes int64 = 1 << 40

	AgentTrafficSkipReasonMissingBootID  = "missing_boot_id"
	AgentTrafficSkipReasonMissingCounter = "missing_counter"
	AgentTrafficSkipReasonInvalidCounter = "invalid_counter"
	AgentTrafficSkipReasonBootChanged    = "boot_changed"
	AgentTrafficSkipReasonNegativeDelta  = "negative_delta"
	AgentTrafficSkipReasonExcessiveDelta = "excessive_delta"

	AgentTrafficLimitTypeUpload   = "upload"
	AgentTrafficLimitTypeDownload = "download"
	AgentTrafficLimitTypeSum      = "sum"

	AgentTrafficThresholdActionNotifyOnly          = "notify_only"
	AgentTrafficThresholdActionSubscriptionExclude = "subscription_exclude"
	AgentTrafficThresholdActionDisableServers      = "disable_servers"
	AgentTrafficThresholdActionResetLinks          = "reset_links"

	AgentTrafficResetModeOff           = "off"
	AgentTrafficResetModeFixedDay      = "fixed_day"
	AgentTrafficResetModeCalendarMonth = "calendar_month"
	AgentTrafficResetModeIntervalDays  = "interval_days"

	agentTrafficLimitTypeUpload   = AgentTrafficLimitTypeUpload
	agentTrafficLimitTypeDownload = AgentTrafficLimitTypeDownload
	agentTrafficLimitTypeSum      = AgentTrafficLimitTypeSum

	agentTrafficThresholdActionNotifyOnly          = AgentTrafficThresholdActionNotifyOnly
	agentTrafficThresholdActionSubscriptionExclude = AgentTrafficThresholdActionSubscriptionExclude
	agentTrafficThresholdActionDisableServers      = AgentTrafficThresholdActionDisableServers
	agentTrafficThresholdActionResetLinks          = AgentTrafficThresholdActionResetLinks

	agentTrafficResetModeOff           = AgentTrafficResetModeOff
	agentTrafficResetModeFixedDay      = AgentTrafficResetModeFixedDay
	agentTrafficResetModeCalendarMonth = AgentTrafficResetModeCalendarMonth
	agentTrafficResetModeIntervalDays  = AgentTrafficResetModeIntervalDays

	agentTrafficFilterSourcePolicy           = "agent_traffic_policy"
	agentTrafficFilterReasonThresholdReached = "threshold_reached"
	agentTrafficFilterReasonDisabledServers  = "threshold_disable_servers"
)

var (
	ErrAgentTrafficLifecycleNotConfigured  = errors.New("service: agent traffic lifecycle service not configured / Agent 流量生命周期服务未配置")
	ErrAgentTrafficLifecycleInvalidRequest = errors.New("service: invalid agent traffic lifecycle request / Agent 流量生命周期请求无效")
)

type AgentTrafficLifecycleService interface {
	ApplyReport(ctx context.Context, agentHostID int64, report AgentTrafficReport) (*AgentTrafficApplyResult, error)
	GetPolicyStatus(ctx context.Context, agentHostID int64) (*AgentTrafficPolicyStatus, error)
	UpsertPolicy(ctx context.Context, req UpsertAgentTrafficPolicyRequest) (*AgentTrafficPolicyStatus, error)
	ResetCycle(ctx context.Context, agentHostID int64, source string) (*AgentTrafficResetResult, error)
	RunScheduledResets(ctx context.Context, now int64) (*AgentTrafficScheduledResetResult, error)
}

type AgentTrafficLifecycleOptions struct {
	MaxDeltaBytes       int64
	Policies            repository.AgentTrafficPolicyRepository
	AgentHosts          repository.AgentHostRepository
	Servers             repository.ServerRepository
	SubscriptionReasons repository.SubscriptionFilterReasonRepository
	LifecycleOperations AgentLifecycleOperationService
	Logger              *slog.Logger
	Now                 func() time.Time
}

type AgentTrafficReport struct {
	BootID                string
	RawUploadTotalBytes   *int64
	RawDownloadTotalBytes *int64
	ReportedAt            int64
}

type AgentTrafficApplyResult struct {
	AgentHostID             int64  `json:"agent_host_id"`
	BootID                  string `json:"boot_id"`
	ReportedAt              int64  `json:"reported_at"`
	Accepted                bool   `json:"accepted"`
	BaselineInitialized     bool   `json:"baseline_initialized"`
	BaselineReset           bool   `json:"baseline_reset"`
	BootChanged             bool   `json:"boot_changed"`
	Skipped                 bool   `json:"skipped"`
	SkipReason              string `json:"skip_reason,omitempty"`
	UploadDeltaBytes        int64  `json:"upload_delta_bytes"`
	DownloadDeltaBytes      int64  `json:"download_delta_bytes"`
	CycleUploadBytes        int64  `json:"cycle_upload_bytes"`
	CycleDownloadBytes      int64  `json:"cycle_download_bytes"`
	LastRawUploadBytes      int64  `json:"last_raw_upload_bytes"`
	LastRawDownloadBytes    int64  `json:"last_raw_download_bytes"`
	ThresholdReached        bool   `json:"threshold_reached"`
	ThresholdUsageBytes     int64  `json:"threshold_usage_bytes"`
	ThresholdLimitBytes     int64  `json:"threshold_limit_bytes"`
	ThresholdAction         string `json:"threshold_action,omitempty"`
	ThresholdActionExecuted bool   `json:"threshold_action_executed"`
	ThresholdActionFailed   bool   `json:"threshold_action_failed"`
	ThresholdActionError    string `json:"threshold_action_error,omitempty"`
}

type AgentTrafficResetResult struct {
	AgentHostID          int64  `json:"agent_host_id"`
	Source               string `json:"source"`
	ResetAt              int64  `json:"reset_at"`
	CycleKey             string `json:"cycle_key"`
	StateReset           bool   `json:"state_reset"`
	ThresholdCleared     bool   `json:"threshold_cleared"`
	RestoredServers      int    `json:"restored_servers"`
	ClearedFilterReasons bool   `json:"cleared_filter_reasons"`
}

type AgentTrafficScheduledResetResult struct {
	Processed int                        `json:"processed"`
	Skipped   int                        `json:"skipped"`
	Failed    int                        `json:"failed"`
	Results   []*AgentTrafficResetResult `json:"results"`
}

type UpsertAgentTrafficPolicyRequest struct {
	AgentHostID      int64  `json:"agent_host_id"`
	Enabled          bool   `json:"enabled"`
	LimitBytes       int64  `json:"limit_bytes"`
	LimitType        string `json:"limit_type"`
	ThresholdPercent int    `json:"threshold_percent"`
	ThresholdAction  string `json:"threshold_action"`
	ResetMode        string `json:"reset_mode"`
	ResetDay         int    `json:"reset_day"`
	IntervalDays     int    `json:"interval_days"`
	AnchorAt         int64  `json:"anchor_at"`
}

type AgentTrafficPolicyStatus struct {
	AgentHostID          int64                          `json:"agent_host_id"`
	Policy               *repository.AgentTrafficPolicy `json:"policy"`
	State                *repository.AgentTrafficState  `json:"state,omitempty"`
	UsageBytes           int64                          `json:"usage_bytes"`
	ThresholdBytes       int64                          `json:"threshold_bytes"`
	ThresholdReached     bool                           `json:"threshold_reached"`
	NextResetAt          int64                          `json:"next_reset_at,omitempty"`
	NextResetCycleKey    string                         `json:"next_reset_cycle_key,omitempty"`
	CycleUploadBytes     int64                          `json:"cycle_upload_bytes"`
	CycleDownloadBytes   int64                          `json:"cycle_download_bytes"`
	CycleTotalBytes      int64                          `json:"cycle_total_bytes"`
	LastRawUploadBytes   int64                          `json:"last_raw_upload_bytes,omitempty"`
	LastRawDownloadBytes int64                          `json:"last_raw_download_bytes,omitempty"`
}

type agentTrafficLifecycleService struct {
	states              repository.AgentTrafficStateRepository
	policies            repository.AgentTrafficPolicyRepository
	agentHosts          repository.AgentHostRepository
	servers             repository.ServerRepository
	subscriptionReasons repository.SubscriptionFilterReasonRepository
	lifecycleOperations AgentLifecycleOperationService
	logs                OperationLogService
	maxDeltaBytes       int64
	logger              *slog.Logger
	now                 func() time.Time
}

type agentTrafficResetDue struct {
	resetAt  int64
	cycleKey string
}

func NewAgentTrafficLifecycleService(states repository.AgentTrafficStateRepository, logs OperationLogService, opts AgentTrafficLifecycleOptions) AgentTrafficLifecycleService {
	maxDeltaBytes := opts.MaxDeltaBytes
	if maxDeltaBytes <= 0 {
		maxDeltaBytes = agentTrafficDefaultMaxDeltaBytes
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &agentTrafficLifecycleService{
		states:              states,
		policies:            opts.Policies,
		agentHosts:          opts.AgentHosts,
		servers:             opts.Servers,
		subscriptionReasons: opts.SubscriptionReasons,
		lifecycleOperations: opts.LifecycleOperations,
		logs:                logs,
		maxDeltaBytes:       maxDeltaBytes,
		logger:              logger,
		now:                 now,
	}
}

func AgentTrafficReportFromMetrics(metrics AgentHostMetricsReport) AgentTrafficReport {
	return AgentTrafficReport{
		BootID:                metrics.BootID,
		RawUploadTotalBytes:   metrics.RawUploadTotalBytes,
		RawDownloadTotalBytes: metrics.RawDownloadTotalBytes,
		ReportedAt:            metrics.ReportedAt,
	}
}

func (s *agentTrafficLifecycleService) ApplyReport(ctx context.Context, agentHostID int64, report AgentTrafficReport) (*AgentTrafficApplyResult, error) {
	if s == nil || s.states == nil {
		return nil, ErrAgentTrafficLifecycleNotConfigured
	}
	if agentHostID <= 0 {
		return nil, ErrAgentTrafficLifecycleInvalidRequest
	}
	reportedAt := report.ReportedAt
	if reportedAt == 0 {
		reportedAt = s.now().Unix()
	}
	bootID := strings.TrimSpace(report.BootID)
	result := &AgentTrafficApplyResult{AgentHostID: agentHostID, BootID: bootID, ReportedAt: reportedAt}
	if bootID == "" {
		result.Skipped = true
		result.SkipReason = AgentTrafficSkipReasonMissingBootID
		return result, nil
	}
	if report.RawUploadTotalBytes == nil || report.RawDownloadTotalBytes == nil {
		result.Skipped = true
		result.SkipReason = AgentTrafficSkipReasonMissingCounter
		return result, nil
	}
	rawUpload := *report.RawUploadTotalBytes
	rawDownload := *report.RawDownloadTotalBytes
	result.LastRawUploadBytes = rawUpload
	result.LastRawDownloadBytes = rawDownload
	if rawUpload < 0 || rawDownload < 0 {
		result.Skipped = true
		result.SkipReason = AgentTrafficSkipReasonInvalidCounter
		s.appendTrafficLifecycleLog(ctx, result, "invalid_counter", OperationLogLevelWarn, "agent traffic counters are invalid")
		return result, nil
	}

	state, err := s.states.FindByAgentHostID(ctx, agentHostID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if state == nil || !state.CounterSeen || strings.TrimSpace(state.BootID) == "" {
		cycleUpload, cycleDownload := int64(0), int64(0)
		if state != nil {
			cycleUpload = state.CycleUploadBytes
			cycleDownload = state.CycleDownloadBytes
		}
		updated, err := s.upsertTrafficState(ctx, agentHostID, bootID, rawUpload, rawDownload, cycleUpload, cycleDownload, reportedAt)
		if err != nil {
			return nil, err
		}
		result.BaselineInitialized = true
		result.CycleUploadBytes = updated.CycleUploadBytes
		result.CycleDownloadBytes = updated.CycleDownloadBytes
		return result, nil
	}

	state.BootID = strings.TrimSpace(state.BootID)
	result.CycleUploadBytes = state.CycleUploadBytes
	result.CycleDownloadBytes = state.CycleDownloadBytes
	if state.BootID != bootID {
		updated, err := s.upsertTrafficState(ctx, agentHostID, bootID, rawUpload, rawDownload, state.CycleUploadBytes, state.CycleDownloadBytes, reportedAt)
		if err != nil {
			return nil, err
		}
		result.BootChanged = true
		result.BaselineReset = true
		result.Skipped = true
		result.SkipReason = AgentTrafficSkipReasonBootChanged
		result.CycleUploadBytes = updated.CycleUploadBytes
		result.CycleDownloadBytes = updated.CycleDownloadBytes
		s.appendTrafficLifecycleLog(ctx, result, "boot_changed", OperationLogLevelInfo, "agent traffic baseline reset after boot change")
		return result, nil
	}

	uploadDelta := rawUpload - state.LastRawUploadBytes
	downloadDelta := rawDownload - state.LastRawDownloadBytes
	if uploadDelta < 0 || downloadDelta < 0 {
		updated, err := s.upsertTrafficState(ctx, agentHostID, bootID, rawUpload, rawDownload, state.CycleUploadBytes, state.CycleDownloadBytes, reportedAt)
		if err != nil {
			return nil, err
		}
		result.BaselineReset = true
		result.Skipped = true
		result.SkipReason = AgentTrafficSkipReasonNegativeDelta
		result.CycleUploadBytes = updated.CycleUploadBytes
		result.CycleDownloadBytes = updated.CycleDownloadBytes
		s.appendTrafficLifecycleLog(ctx, result, "negative_delta", OperationLogLevelWarn, "agent traffic counter delta is negative")
		return result, nil
	}
	if uploadDelta > s.maxDeltaBytes || downloadDelta > s.maxDeltaBytes {
		updated, err := s.upsertTrafficState(ctx, agentHostID, bootID, rawUpload, rawDownload, state.CycleUploadBytes, state.CycleDownloadBytes, reportedAt)
		if err != nil {
			return nil, err
		}
		result.BaselineReset = true
		result.Skipped = true
		result.SkipReason = AgentTrafficSkipReasonExcessiveDelta
		result.UploadDeltaBytes = uploadDelta
		result.DownloadDeltaBytes = downloadDelta
		result.CycleUploadBytes = updated.CycleUploadBytes
		result.CycleDownloadBytes = updated.CycleDownloadBytes
		s.appendTrafficLifecycleLog(ctx, result, "excessive_delta", OperationLogLevelWarn, "agent traffic counter delta exceeds limit")
		return result, nil
	}

	cycleUpload := state.CycleUploadBytes + uploadDelta
	cycleDownload := state.CycleDownloadBytes + downloadDelta
	updated, err := s.upsertTrafficState(ctx, agentHostID, bootID, rawUpload, rawDownload, cycleUpload, cycleDownload, reportedAt)
	if err != nil {
		return nil, err
	}
	result.Accepted = true
	result.UploadDeltaBytes = uploadDelta
	result.DownloadDeltaBytes = downloadDelta
	result.CycleUploadBytes = updated.CycleUploadBytes
	result.CycleDownloadBytes = updated.CycleDownloadBytes
	if err := s.evaluateTrafficThreshold(ctx, result, updated); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *agentTrafficLifecycleService) GetPolicyStatus(ctx context.Context, agentHostID int64) (*AgentTrafficPolicyStatus, error) {
	if s == nil || s.states == nil || s.policies == nil {
		return nil, ErrAgentTrafficLifecycleNotConfigured
	}
	if agentHostID <= 0 {
		return nil, ErrAgentTrafficLifecycleInvalidRequest
	}
	policy, err := s.findTrafficPolicy(ctx, agentHostID)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		policy = defaultAgentTrafficPolicy(agentHostID)
	}
	state, err := s.states.FindByAgentHostID(ctx, agentHostID)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			return nil, err
		}
		state = nil
	}
	return s.buildPolicyStatus(ctx, policy, state, s.now().Unix())
}

func (s *agentTrafficLifecycleService) UpsertPolicy(ctx context.Context, req UpsertAgentTrafficPolicyRequest) (*AgentTrafficPolicyStatus, error) {
	if s == nil || s.states == nil || s.policies == nil {
		return nil, ErrAgentTrafficLifecycleNotConfigured
	}
	if req.AgentHostID <= 0 {
		return nil, ErrAgentTrafficLifecycleInvalidRequest
	}
	policy := defaultAgentTrafficPolicy(req.AgentHostID)
	if existing, err := s.findTrafficPolicy(ctx, req.AgentHostID); err != nil {
		return nil, err
	} else if existing != nil {
		policy = existing
	}
	policy.Enabled = req.Enabled
	policy.LimitBytes = req.LimitBytes
	policy.LimitType = normalizeAgentTrafficLimitType(req.LimitType)
	policy.ThresholdPercent = normalizeAgentTrafficThresholdPercent(req.ThresholdPercent)
	policy.ThresholdAction = normalizeAgentTrafficThresholdAction(req.ThresholdAction)
	policy.ResetMode = normalizeAgentTrafficResetMode(req.ResetMode)
	policy.ResetDay = normalizeAgentTrafficResetDay(req.ResetDay)
	policy.IntervalDays = req.IntervalDays
	policy.AnchorAt = req.AnchorAt
	policy.UpdatedAt = s.now().Unix()
	updated, err := s.policies.Upsert(ctx, policy)
	if err != nil {
		return nil, err
	}
	state, err := s.states.FindByAgentHostID(ctx, req.AgentHostID)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			return nil, err
		}
		state = nil
	}
	return s.buildPolicyStatus(ctx, updated, state, policy.UpdatedAt)
}

func (s *agentTrafficLifecycleService) ResetCycle(ctx context.Context, agentHostID int64, source string) (*AgentTrafficResetResult, error) {
	if s == nil || s.states == nil {
		return nil, ErrAgentTrafficLifecycleNotConfigured
	}
	if agentHostID <= 0 {
		return nil, ErrAgentTrafficLifecycleInvalidRequest
	}
	resetAt := s.now().Unix()
	policy, err := s.findTrafficPolicy(ctx, agentHostID)
	if err != nil {
		return nil, err
	}
	return s.resetTrafficCycle(ctx, policy, agentHostID, resetAt, fmt.Sprintf("manual:%d", resetAt), source)
}

func (s *agentTrafficLifecycleService) RunScheduledResets(ctx context.Context, now int64) (*AgentTrafficScheduledResetResult, error) {
	if s == nil || s.states == nil || s.policies == nil {
		return nil, ErrAgentTrafficLifecycleNotConfigured
	}
	if now == 0 {
		now = s.now().Unix()
	}
	result := &AgentTrafficScheduledResetResult{}
	enabled := true
	limit := 100
	offset := 0
	var errs []error
	for {
		policies, err := s.policies.List(ctx, repository.AgentTrafficPolicyFilter{Enabled: &enabled, Limit: limit, Offset: offset})
		if err != nil {
			return result, err
		}
		if len(policies) == 0 {
			break
		}
		for _, policy := range policies {
			if policy == nil {
				result.Skipped++
				continue
			}
			due, err := s.scheduledResetDue(ctx, policy, now)
			if err != nil {
				result.Failed++
				errs = append(errs, err)
				continue
			}
			if due == nil {
				result.Skipped++
				continue
			}
			reset, err := s.resetTrafficCycle(ctx, policy, policy.AgentHostID, due.resetAt, due.cycleKey, "scheduled")
			if err != nil {
				result.Failed++
				errs = append(errs, err)
				continue
			}
			result.Processed++
			result.Results = append(result.Results, reset)
		}
		if len(policies) < limit {
			break
		}
		offset += len(policies)
	}
	return result, errors.Join(errs...)
}

func (s *agentTrafficLifecycleService) buildPolicyStatus(ctx context.Context, policy *repository.AgentTrafficPolicy, state *repository.AgentTrafficState, now int64) (*AgentTrafficPolicyStatus, error) {
	if policy == nil {
		return nil, ErrAgentTrafficLifecycleInvalidRequest
	}
	status := &AgentTrafficPolicyStatus{AgentHostID: policy.AgentHostID, Policy: policy, State: state, ThresholdReached: policy.ThresholdReached}
	if state != nil {
		status.CycleUploadBytes = state.CycleUploadBytes
		status.CycleDownloadBytes = state.CycleDownloadBytes
		status.CycleTotalBytes = state.CycleUploadBytes + state.CycleDownloadBytes
		status.LastRawUploadBytes = state.LastRawUploadBytes
		status.LastRawDownloadBytes = state.LastRawDownloadBytes
	}
	status.UsageBytes = agentTrafficPolicyUsage(policy, state)
	status.ThresholdBytes = agentTrafficThresholdBytes(policy)
	nextResetAt, nextResetCycleKey, err := s.nextReset(ctx, policy, now)
	if err != nil {
		return nil, err
	}
	status.NextResetAt = nextResetAt
	status.NextResetCycleKey = nextResetCycleKey
	return status, nil
}

func (s *agentTrafficLifecycleService) nextReset(ctx context.Context, policy *repository.AgentTrafficPolicy, nowUnix int64) (int64, string, error) {
	if policy == nil || !policy.Enabled || policy.AgentHostID <= 0 {
		return 0, "", nil
	}
	now := time.Unix(nowUnix, 0).UTC()
	switch normalizeAgentTrafficResetMode(policy.ResetMode) {
	case agentTrafficResetModeFixedDay:
		return s.nextFixedDayReset(ctx, policy, now)
	case agentTrafficResetModeCalendarMonth:
		return s.nextCalendarMonthReset(ctx, policy, now)
	case agentTrafficResetModeIntervalDays:
		return s.nextIntervalDaysReset(ctx, policy, now)
	default:
		return 0, "", nil
	}
}

func (s *agentTrafficLifecycleService) nextFixedDayReset(ctx context.Context, policy *repository.AgentTrafficPolicy, now time.Time) (int64, string, error) {
	candidate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 24; i++ {
		year, month, _ := candidate.Date()
		day := policy.ResetDay
		if day <= 0 {
			day = 1
		}
		if lastDay := lastDayOfMonth(year, month); day > lastDay {
			day = lastDay
		}
		dueAt := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
		cycleKey := fmt.Sprintf("fixed_day:%04d-%02d-%02d", year, month, day)
		if strings.TrimSpace(policy.LastResetCycleKey) != cycleKey && !s.skipInitialResetCycle(ctx, policy, dueAt) {
			return dueAt.Unix(), cycleKey, nil
		}
		candidate = candidate.AddDate(0, 1, 0)
	}
	return 0, "", nil
}

func (s *agentTrafficLifecycleService) nextCalendarMonthReset(ctx context.Context, policy *repository.AgentTrafficPolicy, now time.Time) (int64, string, error) {
	candidate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 24; i++ {
		year, month, _ := candidate.Date()
		dueAt := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
		cycleKey := fmt.Sprintf("calendar_month:%04d-%02d", year, month)
		if strings.TrimSpace(policy.LastResetCycleKey) != cycleKey && !s.skipInitialResetCycle(ctx, policy, dueAt) {
			return dueAt.Unix(), cycleKey, nil
		}
		candidate = candidate.AddDate(0, 1, 0)
	}
	return 0, "", nil
}

func (s *agentTrafficLifecycleService) nextIntervalDaysReset(ctx context.Context, policy *repository.AgentTrafficPolicy, now time.Time) (int64, string, error) {
	if policy.IntervalDays <= 0 {
		return 0, "", nil
	}
	anchorAt, err := s.policyIntervalAnchorAt(ctx, policy)
	if err != nil || anchorAt <= 0 {
		return 0, "", err
	}
	periodSeconds := int64(policy.IntervalDays) * 24 * 60 * 60
	if periodSeconds <= 0 {
		return 0, "", nil
	}
	periodIndex := (now.Unix() - anchorAt) / periodSeconds
	if periodIndex <= 0 {
		return anchorAt + periodSeconds, fmt.Sprintf("interval_days:%d:1", anchorAt), nil
	}
	cycleKey := fmt.Sprintf("interval_days:%d:%d", anchorAt, periodIndex)
	if strings.TrimSpace(policy.LastResetCycleKey) != cycleKey {
		return anchorAt + periodIndex*periodSeconds, cycleKey, nil
	}
	nextIndex := periodIndex + 1
	return anchorAt + nextIndex*periodSeconds, fmt.Sprintf("interval_days:%d:%d", anchorAt, nextIndex), nil
}

func (s *agentTrafficLifecycleService) evaluateTrafficThreshold(ctx context.Context, result *AgentTrafficApplyResult, state *repository.AgentTrafficState) error {
	if s == nil || s.policies == nil || result == nil || state == nil {
		return nil
	}
	policy, err := s.findTrafficPolicy(ctx, result.AgentHostID)
	if err != nil {
		return err
	}
	if policy == nil || !policy.Enabled || policy.ThresholdReached || policy.LimitBytes <= 0 {
		return nil
	}
	thresholdBytes := agentTrafficThresholdBytes(policy)
	if thresholdBytes <= 0 {
		return nil
	}
	usage := agentTrafficPolicyUsage(policy, state)
	if usage < thresholdBytes {
		return nil
	}
	if err := s.policies.UpdateThresholdReached(ctx, policy.AgentHostID, true, result.ReportedAt); err != nil {
		return err
	}
	result.ThresholdReached = true
	result.ThresholdUsageBytes = usage
	result.ThresholdLimitBytes = thresholdBytes
	result.ThresholdAction = normalizeAgentTrafficThresholdAction(policy.ThresholdAction)
	s.appendTrafficOperationLog(ctx, OperationLogScopeThresholdAction, result.AgentHostID, "threshold_reached", OperationLogLevelWarn, "agent traffic threshold reached", marshalAgentTrafficPayload(map[string]any{
		"agent_host_id":    result.AgentHostID,
		"limit_type":       normalizeAgentTrafficLimitType(policy.LimitType),
		"usage_bytes":      usage,
		"threshold_bytes":  thresholdBytes,
		"threshold_action": result.ThresholdAction,
	}), result.ReportedAt)
	details, err := s.executeThresholdAction(ctx, policy, state, usage, thresholdBytes, result.ReportedAt)
	if err != nil {
		result.ThresholdActionFailed = true
		result.ThresholdActionError = err.Error()
		s.appendTrafficOperationLog(ctx, OperationLogScopeThresholdAction, result.AgentHostID, "threshold_action_failed", OperationLogLevelError, "agent traffic threshold action failed", marshalAgentTrafficPayload(map[string]any{
			"agent_host_id":    result.AgentHostID,
			"threshold_action": result.ThresholdAction,
			"error":            err.Error(),
		}), result.ReportedAt)
		return nil
	}
	result.ThresholdActionExecuted = true
	s.appendTrafficOperationLog(ctx, OperationLogScopeThresholdAction, result.AgentHostID, "threshold_action", OperationLogLevelInfo, "agent traffic threshold action executed", details, result.ReportedAt)
	return nil
}

func (s *agentTrafficLifecycleService) executeThresholdAction(ctx context.Context, policy *repository.AgentTrafficPolicy, state *repository.AgentTrafficState, usage, thresholdBytes, now int64) (json.RawMessage, error) {
	action := normalizeAgentTrafficThresholdAction(policy.ThresholdAction)
	payload := map[string]any{
		"agent_host_id":    policy.AgentHostID,
		"threshold_action": action,
		"limit_type":       normalizeAgentTrafficLimitType(policy.LimitType),
		"usage_bytes":      usage,
		"threshold_bytes":  thresholdBytes,
		"cycle_upload":     state.CycleUploadBytes,
		"cycle_download":   state.CycleDownloadBytes,
	}
	switch action {
	case agentTrafficThresholdActionNotifyOnly:
		return marshalAgentTrafficPayload(payload), nil
	case agentTrafficThresholdActionSubscriptionExclude:
		count, err := s.replaceSubscriptionExcludeReasons(ctx, policy.AgentHostID, now)
		payload["excluded_servers"] = count
		return marshalAgentTrafficPayload(payload), err
	case agentTrafficThresholdActionDisableServers:
		count, err := s.disableServersForThreshold(ctx, policy.AgentHostID, now)
		payload["disabled_servers"] = count
		return marshalAgentTrafficPayload(payload), err
	case agentTrafficThresholdActionResetLinks:
		operation, err := s.createResetLinksOperation(ctx, policy.AgentHostID, usage, thresholdBytes)
		if operation != nil {
			payload["operation_id"] = operation.ID
		}
		return marshalAgentTrafficPayload(payload), err
	default:
		return marshalAgentTrafficPayload(payload), ErrAgentTrafficLifecycleInvalidRequest
	}
}

func (s *agentTrafficLifecycleService) replaceSubscriptionExcludeReasons(ctx context.Context, agentHostID int64, now int64) (int, error) {
	if s.servers == nil || s.subscriptionReasons == nil {
		return 0, ErrAgentTrafficLifecycleNotConfigured
	}
	servers, err := s.servers.FindByAgentHostID(ctx, agentHostID)
	if err != nil {
		return 0, err
	}
	reasons := make([]*repository.SubscriptionFilterReason, 0, len(servers))
	for _, server := range servers {
		if server == nil || server.Show == 0 {
			continue
		}
		reasons = append(reasons, &repository.SubscriptionFilterReason{
			ServerID:  server.ID,
			NodeName:  server.Name,
			Reason:    agentTrafficFilterReasonThresholdReached,
			Detail:    "agent traffic threshold reached",
			CreatedAt: now,
		})
	}
	if err := s.subscriptionReasons.ReplaceForSource(ctx, agentTrafficFilterSourcePolicy, agentHostID, reasons); err != nil {
		return 0, err
	}
	return len(reasons), nil
}

func (s *agentTrafficLifecycleService) disableServersForThreshold(ctx context.Context, agentHostID int64, now int64) (int, error) {
	if s.servers == nil || s.subscriptionReasons == nil {
		return 0, ErrAgentTrafficLifecycleNotConfigured
	}
	servers, err := s.servers.FindByAgentHostID(ctx, agentHostID)
	if err != nil {
		return 0, err
	}
	reasons := make([]*repository.SubscriptionFilterReason, 0, len(servers))
	for _, server := range servers {
		if server == nil || server.Show == 0 {
			continue
		}
		reasons = append(reasons, &repository.SubscriptionFilterReason{
			ServerID:  server.ID,
			NodeName:  server.Name,
			Reason:    agentTrafficFilterReasonDisabledServers,
			Detail:    "agent traffic threshold disabled server",
			CreatedAt: now,
		})
	}
	if err := s.subscriptionReasons.ReplaceForSource(ctx, agentTrafficFilterSourcePolicy, agentHostID, reasons); err != nil {
		return 0, err
	}
	for _, reason := range reasons {
		server, err := s.servers.FindByID(ctx, reason.ServerID)
		if err != nil {
			return 0, err
		}
		server.Show = 0
		if err := s.servers.Update(ctx, server); err != nil {
			return 0, err
		}
	}
	return len(reasons), nil
}

func (s *agentTrafficLifecycleService) createResetLinksOperation(ctx context.Context, agentHostID int64, usage, thresholdBytes int64) (*repository.AgentLifecycleOperation, error) {
	if s.lifecycleOperations == nil {
		return nil, ErrAgentTrafficLifecycleNotConfigured
	}
	payload := marshalAgentTrafficPayload(map[string]any{
		"reason":          "agent_traffic_threshold",
		"usage_bytes":     usage,
		"threshold_bytes": thresholdBytes,
	})
	return s.lifecycleOperations.Create(ctx, CreateAgentLifecycleOperationRequest{
		AgentHostID:    agentHostID,
		OperationType:  agentLifecycleOperationTypeResetLinks,
		RequestPayload: payload,
		Source:         agentLifecycleOperationSourceSystem,
	})
}

func (s *agentTrafficLifecycleService) resetTrafficCycle(ctx context.Context, policy *repository.AgentTrafficPolicy, agentHostID int64, resetAt int64, cycleKey string, source string) (*AgentTrafficResetResult, error) {
	if s == nil || s.states == nil {
		return nil, ErrAgentTrafficLifecycleNotConfigured
	}
	if agentHostID <= 0 || resetAt <= 0 || strings.TrimSpace(cycleKey) == "" {
		return nil, ErrAgentTrafficLifecycleInvalidRequest
	}
	source = strings.TrimSpace(source)
	if source == "" {
		source = "manual"
	}
	result := &AgentTrafficResetResult{AgentHostID: agentHostID, Source: source, ResetAt: resetAt, CycleKey: cycleKey}
	if err := s.states.ResetCycle(ctx, agentHostID, resetAt); err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			return nil, err
		}
	} else {
		result.StateReset = true
	}
	if policy != nil {
		restored, cleared, err := s.restoreTrafficThresholdAvailability(ctx, policy)
		if err != nil {
			return nil, err
		}
		result.RestoredServers = restored
		result.ClearedFilterReasons = cleared
		if s.policies != nil {
			if err := s.policies.UpdateResetState(ctx, agentHostID, resetAt, cycleKey, resetAt); err != nil {
				return nil, err
			}
			result.ThresholdCleared = true
		}
	}
	s.appendTrafficOperationLog(ctx, OperationLogScopeTrafficReset, agentHostID, "reset_cycle", OperationLogLevelInfo, "agent traffic cycle reset", marshalAgentTrafficPayload(result), resetAt)
	return result, nil
}

func (s *agentTrafficLifecycleService) restoreTrafficThresholdAvailability(ctx context.Context, policy *repository.AgentTrafficPolicy) (int, bool, error) {
	if policy == nil || s.subscriptionReasons == nil {
		return 0, false, nil
	}
	reasonName := agentTrafficFilterReasonDisabledServers
	reasons, err := s.subscriptionReasons.List(ctx, repository.SubscriptionFilterReasonFilter{SourceType: stringPtr(agentTrafficFilterSourcePolicy), SourceID: &policy.AgentHostID, Reason: &reasonName, Limit: 1000})
	if err != nil {
		return 0, false, err
	}
	restored := 0
	if len(reasons) > 0 && s.servers != nil {
		for _, reason := range reasons {
			if reason == nil || reason.ServerID <= 0 {
				continue
			}
			server, err := s.servers.FindByID(ctx, reason.ServerID)
			if err != nil {
				if errors.Is(err, repository.ErrNotFound) {
					continue
				}
				return restored, false, err
			}
			if server.Show == 0 {
				server.Show = 1
				if err := s.servers.Update(ctx, server); err != nil {
					return restored, false, err
				}
				restored++
			}
		}
	}
	if err := s.subscriptionReasons.DeleteBySource(ctx, agentTrafficFilterSourcePolicy, policy.AgentHostID); err != nil {
		return restored, false, err
	}
	return restored, true, nil
}

func (s *agentTrafficLifecycleService) scheduledResetDue(ctx context.Context, policy *repository.AgentTrafficPolicy, nowUnix int64) (*agentTrafficResetDue, error) {
	if policy == nil || !policy.Enabled || policy.AgentHostID <= 0 {
		return nil, nil
	}
	now := time.Unix(nowUnix, 0).UTC()
	switch normalizeAgentTrafficResetMode(policy.ResetMode) {
	case agentTrafficResetModeOff:
		return nil, nil
	case agentTrafficResetModeFixedDay:
		return s.fixedDayResetDue(ctx, policy, now)
	case agentTrafficResetModeCalendarMonth:
		return s.calendarMonthResetDue(ctx, policy, now)
	case agentTrafficResetModeIntervalDays:
		return s.intervalDaysResetDue(ctx, policy, now)
	default:
		return nil, nil
	}
}

func (s *agentTrafficLifecycleService) fixedDayResetDue(ctx context.Context, policy *repository.AgentTrafficPolicy, now time.Time) (*agentTrafficResetDue, error) {
	year, month, _ := now.Date()
	day := policy.ResetDay
	if day <= 0 {
		day = 1
	}
	lastDay := lastDayOfMonth(year, month)
	if day > lastDay {
		day = lastDay
	}
	dueAt := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	if now.Before(dueAt) || s.skipInitialResetCycle(ctx, policy, dueAt) {
		return nil, nil
	}
	cycleKey := fmt.Sprintf("fixed_day:%04d-%02d-%02d", year, month, day)
	if strings.TrimSpace(policy.LastResetCycleKey) == cycleKey {
		return nil, nil
	}
	return &agentTrafficResetDue{resetAt: now.Unix(), cycleKey: cycleKey}, nil
}

func (s *agentTrafficLifecycleService) calendarMonthResetDue(ctx context.Context, policy *repository.AgentTrafficPolicy, now time.Time) (*agentTrafficResetDue, error) {
	year, month, _ := now.Date()
	dueAt := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	if now.Before(dueAt) || s.skipInitialResetCycle(ctx, policy, dueAt) {
		return nil, nil
	}
	cycleKey := fmt.Sprintf("calendar_month:%04d-%02d", year, month)
	if strings.TrimSpace(policy.LastResetCycleKey) == cycleKey {
		return nil, nil
	}
	return &agentTrafficResetDue{resetAt: now.Unix(), cycleKey: cycleKey}, nil
}

func (s *agentTrafficLifecycleService) intervalDaysResetDue(ctx context.Context, policy *repository.AgentTrafficPolicy, now time.Time) (*agentTrafficResetDue, error) {
	if policy.IntervalDays <= 0 {
		return nil, nil
	}
	anchorAt, err := s.policyIntervalAnchorAt(ctx, policy)
	if err != nil || anchorAt <= 0 {
		return nil, err
	}
	periodSeconds := int64(policy.IntervalDays) * 24 * 60 * 60
	if periodSeconds <= 0 || now.Unix() < anchorAt+periodSeconds {
		return nil, nil
	}
	periodIndex := (now.Unix() - anchorAt) / periodSeconds
	if periodIndex <= 0 {
		return nil, nil
	}
	cycleKey := fmt.Sprintf("interval_days:%d:%d", anchorAt, periodIndex)
	if strings.TrimSpace(policy.LastResetCycleKey) == cycleKey {
		return nil, nil
	}
	return &agentTrafficResetDue{resetAt: now.Unix(), cycleKey: cycleKey}, nil
}

func (s *agentTrafficLifecycleService) skipInitialResetCycle(ctx context.Context, policy *repository.AgentTrafficPolicy, dueAt time.Time) bool {
	if policy == nil || strings.TrimSpace(policy.LastResetCycleKey) != "" {
		return false
	}
	anchorAt := policy.AnchorAt
	if anchorAt <= 0 {
		anchorAt = policy.UpdatedAt
	}
	if anchorAt <= 0 && s.agentHosts != nil {
		if host, err := s.agentHosts.FindByID(ctx, policy.AgentHostID); err == nil && host != nil {
			anchorAt = host.CreatedAt
		}
	}
	return anchorAt >= dueAt.Unix()
}

func (s *agentTrafficLifecycleService) policyIntervalAnchorAt(ctx context.Context, policy *repository.AgentTrafficPolicy) (int64, error) {
	if policy.AnchorAt > 0 {
		return policy.AnchorAt, nil
	}
	if s.agentHosts != nil {
		host, err := s.agentHosts.FindByID(ctx, policy.AgentHostID)
		if err != nil {
			if !errors.Is(err, repository.ErrNotFound) {
				return 0, err
			}
		} else if host != nil && host.CreatedAt > 0 {
			return host.CreatedAt, nil
		}
	}
	return policy.UpdatedAt, nil
}

func (s *agentTrafficLifecycleService) findTrafficPolicy(ctx context.Context, agentHostID int64) (*repository.AgentTrafficPolicy, error) {
	if s.policies == nil {
		return nil, nil
	}
	policy, err := s.policies.FindByAgentHostID(ctx, agentHostID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return policy, nil
}

func (s *agentTrafficLifecycleService) upsertTrafficState(ctx context.Context, agentHostID int64, bootID string, rawUpload, rawDownload, cycleUpload, cycleDownload, updatedAt int64) (*repository.AgentTrafficState, error) {
	return s.states.Upsert(ctx, &repository.AgentTrafficState{
		AgentHostID:          agentHostID,
		BootID:               bootID,
		LastRawUploadBytes:   rawUpload,
		LastRawDownloadBytes: rawDownload,
		CounterSeen:          true,
		CycleUploadBytes:     cycleUpload,
		CycleDownloadBytes:   cycleDownload,
		UpdatedAt:            updatedAt,
	})
}

func (s *agentTrafficLifecycleService) appendTrafficLifecycleLog(ctx context.Context, result *AgentTrafficApplyResult, phase, level, message string) {
	if result == nil {
		return
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return
	}
	s.appendTrafficOperationLog(ctx, OperationLogScopeAgentTraffic, result.AgentHostID, phase, level, message, payload, result.ReportedAt)
}

func (s *agentTrafficLifecycleService) appendTrafficOperationLog(ctx context.Context, scope string, agentHostID int64, phase, level, message string, payload json.RawMessage, reportedAt int64) {
	if s == nil || s.logs == nil {
		return
	}
	_, err := s.logs.Append(ctx, AppendOperationLogRequest{
		Scope:       scope,
		TargetID:    strconv.FormatInt(agentHostID, 10),
		AgentHostID: agentHostID,
		Phase:       phase,
		Level:       level,
		Message:     message,
		Payload:     payload,
		ReportedAt:  reportedAt,
	})
	if err != nil && s.logger != nil {
		s.logger.Warn("failed to append agent traffic lifecycle log", "agent_host_id", agentHostID, "scope", scope, "phase", phase, "error", err)
	}
}

func agentTrafficThresholdBytes(policy *repository.AgentTrafficPolicy) int64 {
	if policy == nil || policy.LimitBytes <= 0 {
		return 0
	}
	percent := policy.ThresholdPercent
	if percent <= 0 || percent > 100 {
		percent = 100
	}
	limit := policy.LimitBytes
	return (limit/100)*int64(percent) + ((limit%100)*int64(percent))/100
}

func agentTrafficPolicyUsage(policy *repository.AgentTrafficPolicy, state *repository.AgentTrafficState) int64 {
	if state == nil {
		return 0
	}
	switch normalizeAgentTrafficLimitType(policy.LimitType) {
	case agentTrafficLimitTypeUpload:
		return state.CycleUploadBytes
	case agentTrafficLimitTypeDownload:
		return state.CycleDownloadBytes
	default:
		return state.CycleUploadBytes + state.CycleDownloadBytes
	}
}

func IsAgentTrafficLimitType(value string) bool {
	switch value {
	case AgentTrafficLimitTypeUpload, AgentTrafficLimitTypeDownload, AgentTrafficLimitTypeSum:
		return true
	default:
		return false
	}
}

func IsAgentTrafficThresholdAction(value string) bool {
	switch value {
	case AgentTrafficThresholdActionNotifyOnly, AgentTrafficThresholdActionSubscriptionExclude, AgentTrafficThresholdActionDisableServers, AgentTrafficThresholdActionResetLinks:
		return true
	default:
		return false
	}
}

func IsAgentTrafficResetMode(value string) bool {
	switch value {
	case AgentTrafficResetModeOff, AgentTrafficResetModeFixedDay, AgentTrafficResetModeCalendarMonth, AgentTrafficResetModeIntervalDays:
		return true
	default:
		return false
	}
}

func normalizeAgentTrafficLimitType(limitType string) string {
	switch strings.TrimSpace(limitType) {
	case agentTrafficLimitTypeUpload:
		return agentTrafficLimitTypeUpload
	case agentTrafficLimitTypeDownload:
		return agentTrafficLimitTypeDownload
	default:
		return agentTrafficLimitTypeSum
	}
}

func normalizeAgentTrafficThresholdAction(action string) string {
	switch strings.TrimSpace(action) {
	case agentTrafficThresholdActionSubscriptionExclude:
		return agentTrafficThresholdActionSubscriptionExclude
	case agentTrafficThresholdActionDisableServers:
		return agentTrafficThresholdActionDisableServers
	case agentTrafficThresholdActionResetLinks:
		return agentTrafficThresholdActionResetLinks
	default:
		return agentTrafficThresholdActionNotifyOnly
	}
}

func normalizeAgentTrafficResetMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case agentTrafficResetModeFixedDay:
		return agentTrafficResetModeFixedDay
	case agentTrafficResetModeCalendarMonth:
		return agentTrafficResetModeCalendarMonth
	case agentTrafficResetModeIntervalDays:
		return agentTrafficResetModeIntervalDays
	default:
		return agentTrafficResetModeOff
	}
}

func normalizeAgentTrafficThresholdPercent(percent int) int {
	if percent <= 0 || percent > 100 {
		return 100
	}
	return percent
}

func normalizeAgentTrafficResetDay(day int) int {
	if day <= 0 {
		return 1
	}
	if day > 31 {
		return 31
	}
	return day
}

func defaultAgentTrafficPolicy(agentHostID int64) *repository.AgentTrafficPolicy {
	return &repository.AgentTrafficPolicy{
		AgentHostID:      agentHostID,
		LimitType:        agentTrafficLimitTypeSum,
		ThresholdPercent: 100,
		ThresholdAction:  agentTrafficThresholdActionNotifyOnly,
		ResetMode:        agentTrafficResetModeOff,
		ResetDay:         1,
	}
}

func lastDayOfMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func marshalAgentTrafficPayload(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return payload
}

func stringPtr(value string) *string { return &value }
