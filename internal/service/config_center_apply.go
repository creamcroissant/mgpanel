package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

var (
	ErrApplyOrchestratorInvalidRequest   = errors.New("service: invalid apply orchestrator request / 发布编排请求无效")
	ErrApplyOrchestratorNotConfigured    = errors.New("service: apply orchestrator not configured / 发布编排服务未配置")
	ErrApplyOrchestratorNotFound         = errors.New("service: apply run not found / 发布记录不存在")
	ErrApplyOrchestratorPermissionDenied = errors.New("service: apply run permission denied / 发布记录无权限")
	ErrApplyOrchestratorInvalidState     = errors.New("service: invalid apply run state / 发布状态无效")
	ErrApplyOrchestratorNoArtifacts      = errors.New("service: no desired artifacts available / 当前无可下发产物")
	ErrApplyOrchestratorNoPayload        = errors.New("service: desired artifacts contain no valid payload / 产物不含有效内容")
)

const (
	applyRunStatusPending    = "pending"
	applyRunStatusApplying   = "applying"
	applyRunStatusSuccess    = "success"
	applyRunStatusFailed     = "failed"
	applyRunStatusRolledBack = "rolled_back"

	// applyRunClaimTimeout 定义了 apply run 从 pending 到被 agent 领取的超时阈值。
	// agent 领取后标记为 applying，超过此时间未完成则视为超时。
)

// ApplyOrchestratorService orchestrates apply run lifecycle and batch payload assembly.
type ApplyOrchestratorService interface {
	CreateApplyRun(ctx context.Context, req CreateApplyRunRequest) (*repository.ApplyRun, error)
	PrepareApplyRun(ctx context.Context, req PrepareApplyRunRequest) (*repository.ApplyRun, error)
	ListApplyRuns(ctx context.Context, req ListApplyRunsRequest) (*ListApplyRunsResult, error)
	GetApplyRunDetail(ctx context.Context, req GetApplyRunDetailRequest) (*GetApplyRunDetailResult, error)
	GetApplyBatch(ctx context.Context, req GetApplyBatchRequest) (*GetApplyBatchResult, error)
	ReportApplyResult(ctx context.Context, req ReportApplyResultRequest) error
	CancelApplyRun(ctx context.Context, req CancelApplyRunRequest) error
	CleanupExpiredApplyRuns(ctx context.Context, maxAge time.Duration) (int, error)
}

// PrepareApplyRunRequest validates target revision artifacts and creates/reuses one apply run.
// CreateApplyRunRequest defines one apply run creation.
type CreateApplyRunRequest struct {
	AgentHostID      int64
	CoreType         string
	TargetRevision   int64
	PreviousRevision int64
	OperatorID       int64
}

// PrepareApplyRunRequest validates target revision artifacts and creates/reuses one apply run.
type PrepareApplyRunRequest = CreateApplyRunRequest

// ListApplyRunsRequest defines apply run list query from control plane.
type ListApplyRunsRequest struct {
	AgentHostID *int64
	CoreType    string
	Status      string
	Limit       int
	Offset      int
}

// ListApplyRunsResult returns paged apply runs.
type ListApplyRunsResult struct {
	Items []*repository.ApplyRun
	Total int64
}

// GetApplyRunDetailRequest queries one apply run with aggregated diagnostics.
type GetApplyRunDetailRequest struct {
	RunID       string
	IncludeText bool
	TextTag     string
	TextFile    string
}

// ApplyDiagnosticIssue describes one diagnostic degradation or unavailable branch.
type ApplyDiagnosticIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// GetApplyRunDetailResult returns one apply run with aggregated diagnostics.
type GetApplyRunDetailResult struct {
	Run          *repository.ApplyRun   `json:"run"`
	SemanticDiff *SemanticDiffResult    `json:"semantic_diff,omitempty"`
	TextDiff     *TextDiffResult        `json:"text_diff,omitempty"`
	Issues       []ApplyDiagnosticIssue `json:"issues,omitempty"`
}

// GetApplyBatchRequest queries one host/core apply batch by current revision.
type GetApplyBatchRequest struct {
	AgentHostID     int64
	CoreType        string
	CurrentRevision int64
	RunOperatorID   int64
}

// ApplyBatchArtifact is one deployable file payload.
type ApplyBatchArtifact struct {
	Filename    string
	SourceTag   string
	Content     []byte
	ContentHash string
}

// GetApplyBatchResult is the orchestration result returned to transport layer.
type GetApplyBatchResult struct {
	NotModified      bool
	RunID            string
	CoreType         string
	TargetRevision   int64
	PreviousRevision int64
	Artifacts        []ApplyBatchArtifact
}

// CancelApplyRunRequest carries cancel request for an apply run.
type CancelApplyRunRequest struct {
	AgentHostID int64
	RunID       string
	OperatorID  int64
}

// ReportApplyResultRequest carries one apply callback event.
type ReportApplyResultRequest struct {
	AgentHostID      int64
	RunID            string
	CoreType         string
	TargetRevision   int64
	Success          bool
	Status           string
	ErrorMessage     string
	RollbackRevision int64
	FinishedAt       int64
}

type applyOrchestratorService struct {
	artifacts   repository.DesiredArtifactRepository
	applyRuns   repository.ApplyRunRepository
	diagnostics DriftAndDiffService
	guard       AgentOperationGuard
	compiler    ArtifactCompilerService
}

// NewApplyOrchestratorService creates ApplyOrchestratorService.
func NewApplyOrchestratorService(
	artifacts repository.DesiredArtifactRepository,
	applyRuns repository.ApplyRunRepository,
	diagnostics ...DriftAndDiffService,
) ApplyOrchestratorService {
	var diff DriftAndDiffService
	if len(diagnostics) > 0 {
		diff = diagnostics[0]
	}
	return newApplyOrchestratorService(artifacts, applyRuns, diff, nil, nil)
}

func NewApplyOrchestratorServiceWithGuard(
	artifacts repository.DesiredArtifactRepository,
	applyRuns repository.ApplyRunRepository,
	diagnostics DriftAndDiffService,
	guard AgentOperationGuard,
	compiler ArtifactCompilerService,
) ApplyOrchestratorService {
	return newApplyOrchestratorService(artifacts, applyRuns, diagnostics, guard, compiler)
}

func newApplyOrchestratorService(
	artifacts repository.DesiredArtifactRepository,
	applyRuns repository.ApplyRunRepository,
	diagnostics DriftAndDiffService,
	guard AgentOperationGuard,
	compiler ArtifactCompilerService,
) ApplyOrchestratorService {
	return &applyOrchestratorService{
		artifacts:   artifacts,
		applyRuns:   applyRuns,
		diagnostics: diagnostics,
		guard:       guard,
		compiler:    compiler,
	}
}

func (s *applyOrchestratorService) PrepareApplyRun(ctx context.Context, req PrepareApplyRunRequest) (*repository.ApplyRun, error) {
	if s == nil || s.artifacts == nil || s.applyRuns == nil {
		return nil, ErrApplyOrchestratorNotConfigured
	}
	if req.AgentHostID <= 0 {
		return nil, fmt.Errorf("%w (agent_host_id is required / 不能为空)", ErrApplyOrchestratorInvalidRequest)
	}
	if req.TargetRevision <= 0 {
		return nil, fmt.Errorf("%w (target_revision must be greater than 0 / 必须大于 0)", ErrApplyOrchestratorInvalidRequest)
	}

	coreType := normalizeCoreType(req.CoreType)
	if coreType == "" {
		return nil, fmt.Errorf("%w (core_type must be sing-box or xray / 必须是 sing-box 或 xray)", ErrApplyOrchestratorInvalidRequest)
	}

	filter := repository.DesiredArtifactFilter{
		AgentHostID:     req.AgentHostID,
		CoreType:        &coreType,
		DesiredRevision: &req.TargetRevision,
		Limit:           1000,
		Offset:          0,
	}
	artifacts, err := s.artifacts.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	if len(artifacts) == 0 && s.compiler != nil {
		// 跨主机/未渲染: 按需渲染该 host 自身绑定的 spec 产物
		if _, rerr := s.compiler.RenderArtifacts(ctx, RenderArtifactsRequest{
			AgentHostID:     req.AgentHostID,
			CoreType:        coreType,
			DesiredRevision: req.TargetRevision,
		}); rerr == nil {
			artifacts, err = s.artifacts.List(ctx, filter)
			if err != nil {
				return nil, err
			}
		}
	}
	if len(artifacts) == 0 {
		return nil, fmt.Errorf("%w (agent_host_id=%d core_type=%s target_revision=%d 没有可下发的配置)", ErrApplyOrchestratorNoArtifacts, req.AgentHostID, coreType, req.TargetRevision)
	}

	openRun, err := s.findReusableOpenApplyRun(ctx, req.AgentHostID, coreType, req.TargetRevision)
	if err != nil {
		return nil, err
	}
	if openRun != nil {
		if s.guard != nil {
			if err := s.guard.CheckIdle(ctx, AgentOperationGuardRequest{AgentHostID: req.AgentHostID, Scope: OperationLogScopeApplyRun, OperationType: agentOperationTypeApply, TargetID: openRun.RunID, OperatorID: applyRunOperatorIDPtr(req.OperatorID)}); err != nil {
				return nil, err
			}
		}
		return openRun, nil
	}

	return s.CreateApplyRun(ctx, CreateApplyRunRequest{
		AgentHostID:      req.AgentHostID,
		CoreType:         coreType,
		TargetRevision:   req.TargetRevision,
		PreviousRevision: req.PreviousRevision,
		OperatorID:       req.OperatorID,
	})
}

func (s *applyOrchestratorService) CreateApplyRun(ctx context.Context, req CreateApplyRunRequest) (*repository.ApplyRun, error) {
	if s == nil || s.applyRuns == nil {
		return nil, ErrApplyOrchestratorNotConfigured
	}
	if req.AgentHostID <= 0 {
		return nil, fmt.Errorf("%w (agent_host_id is required / 不能为空)", ErrApplyOrchestratorInvalidRequest)
	}
	if req.TargetRevision <= 0 {
		return nil, fmt.Errorf("%w (target_revision must be greater than 0 / 必须大于 0)", ErrApplyOrchestratorInvalidRequest)
	}

	coreType := normalizeCoreType(req.CoreType)
	if coreType == "" {
		return nil, fmt.Errorf("%w (core_type must be sing-box or xray / 必须是 sing-box 或 xray)", ErrApplyOrchestratorInvalidRequest)
	}
	if s.guard != nil {
		if err := s.guard.CheckIdle(ctx, AgentOperationGuardRequest{AgentHostID: req.AgentHostID, Scope: OperationLogScopeApplyRun, OperationType: agentOperationTypeApply, OperatorID: applyRunOperatorIDPtr(req.OperatorID)}); err != nil {
			return nil, err
		}
	}

	runID, err := generateApplyRunID(req.AgentHostID)
	if err != nil {
		return nil, err
	}

	run := &repository.ApplyRun{
		RunID:            runID,
		AgentHostID:      req.AgentHostID,
		CoreType:         coreType,
		TargetRevision:   req.TargetRevision,
		Status:           applyRunStatusPending,
		ErrorMessage:     "",
		PreviousRevision: req.PreviousRevision,
		RollbackRevision: 0,
		OperatorID:       req.OperatorID,
		StartedAt:        time.Now().Unix(),
		FinishedAt:       0,
	}
	if err := s.applyRuns.Create(ctx, run); err != nil {
		return nil, err
	}
	return run, nil
}

func (s *applyOrchestratorService) ListApplyRuns(ctx context.Context, req ListApplyRunsRequest) (*ListApplyRunsResult, error) {
	if s == nil || s.applyRuns == nil {
		return nil, ErrApplyOrchestratorNotConfigured
	}

	filter := repository.ApplyRunFilter{
		Limit:  req.Limit,
		Offset: req.Offset,
	}
	if req.AgentHostID != nil {
		if *req.AgentHostID <= 0 {
			return nil, fmt.Errorf("%w (agent_host_id must be greater than 0 / 必须大于 0)", ErrApplyOrchestratorInvalidRequest)
		}
		hostID := *req.AgentHostID
		filter.AgentHostID = &hostID
	}
	if coreType := normalizeCoreType(req.CoreType); req.CoreType != "" {
		if coreType == "" {
			return nil, fmt.Errorf("%w (core_type must be sing-box or xray / 必须是 sing-box 或 xray)", ErrApplyOrchestratorInvalidRequest)
		}
		filter.CoreType = &coreType
	}
	if statusValue := strings.ToLower(strings.TrimSpace(req.Status)); statusValue != "" {
		normalizedStatus, err := normalizeApplyRunStatus(false, statusValue)
		if err != nil {
			return nil, err
		}
		filter.Status = &normalizedStatus
	}

	total, err := s.applyRuns.Count(ctx, filter)
	if err != nil {
		return nil, err
	}
	items, err := s.applyRuns.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	return &ListApplyRunsResult{Items: items, Total: total}, nil
}

func (s *applyOrchestratorService) GetApplyRunDetail(ctx context.Context, req GetApplyRunDetailRequest) (*GetApplyRunDetailResult, error) {
	if s == nil || s.applyRuns == nil {
		return nil, ErrApplyOrchestratorNotConfigured
	}
	runID := strings.TrimSpace(req.RunID)
	if runID == "" {
		return nil, fmt.Errorf("%w (run_id is required / 不能为空)", ErrApplyOrchestratorInvalidRequest)
	}

	run, err := s.applyRuns.FindByRunID(ctx, runID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrApplyOrchestratorNotFound
		}
		return nil, err
	}

	result := &GetApplyRunDetailResult{Run: run}
	if s.diagnostics == nil {
		result.Issues = append(result.Issues, ApplyDiagnosticIssue{
			Code:    "diagnostics_unavailable",
			Message: "diagnostics service is not configured",
		})
		return result, nil
	}

	semanticDiff, err := s.diagnostics.GetSemanticDiff(ctx, GetSemanticDiffRequest{
		AgentHostID:     run.AgentHostID,
		CoreType:        run.CoreType,
		DesiredRevision: run.TargetRevision,
	})
	if err != nil {
		result.Issues = append(result.Issues, ApplyDiagnosticIssue{
			Code:    "semantic_diff_unavailable",
			Message: err.Error(),
		})
	} else {
		result.SemanticDiff = semanticDiff
	}

	if req.IncludeText {
		textTag := strings.TrimSpace(req.TextTag)
		textFile := strings.TrimSpace(req.TextFile)
		if textTag == "" && textFile == "" {
			result.Issues = append(result.Issues, ApplyDiagnosticIssue{
				Code:    "text_diff_selector_required",
				Message: "filename or tag is required when include_text is true",
			})
		} else {
			textDiff, textErr := s.diagnostics.GetTextDiff(ctx, GetTextDiffRequest{
				AgentHostID:     run.AgentHostID,
				CoreType:        run.CoreType,
				DesiredRevision: run.TargetRevision,
				Filename:        textFile,
				Tag:             textTag,
			})
			if textErr != nil {
				result.Issues = append(result.Issues, ApplyDiagnosticIssue{
					Code:    "text_diff_unavailable",
					Message: textErr.Error(),
				})
			} else {
				result.TextDiff = textDiff
			}
		}
	}

	return result, nil
}

func (s *applyOrchestratorService) GetApplyBatch(ctx context.Context, req GetApplyBatchRequest) (*GetApplyBatchResult, error) {
	if s == nil || s.artifacts == nil || s.applyRuns == nil {
		return nil, ErrApplyOrchestratorNotConfigured
	}
	if req.AgentHostID <= 0 {
		return nil, fmt.Errorf("%w (agent_host_id is required / 不能为空)", ErrApplyOrchestratorInvalidRequest)
	}
	if req.CurrentRevision < 0 {
		return nil, fmt.Errorf("%w (current_revision must be greater than or equal to 0 / 必须大于等于 0)", ErrApplyOrchestratorInvalidRequest)
	}

	coreType := normalizeCoreType(req.CoreType)
	if coreType == "" {
		return nil, fmt.Errorf("%w (core_type is required / 不能为空)", ErrApplyOrchestratorInvalidRequest)
	}

	targetRevision, err := s.artifacts.GetLatestRevision(ctx, req.AgentHostID, coreType)
	if err != nil {
		return nil, err
	}
	if targetRevision == 0 {
		return &GetApplyBatchResult{
			NotModified:      true,
			RunID:            "",
			CoreType:         coreType,
			TargetRevision:   0,
			PreviousRevision: req.CurrentRevision,
			Artifacts:        nil,
		}, nil
	}
	if targetRevision <= req.CurrentRevision {
		return &GetApplyBatchResult{
			NotModified:      true,
			RunID:            "",
			CoreType:         coreType,
			TargetRevision:   targetRevision,
			PreviousRevision: req.CurrentRevision,
			Artifacts:        nil,
		}, nil
	}

	artifacts, err := s.artifacts.List(ctx, repository.DesiredArtifactFilter{
		AgentHostID:     req.AgentHostID,
		CoreType:        &coreType,
		DesiredRevision: &targetRevision,
		Limit:           1000,
		Offset:          0,
	})
	if err != nil {
		return nil, err
	}
	if len(artifacts) == 0 {
		return nil, fmt.Errorf("%w (agent_host_id=%d core_type=%s target_revision=%d)", ErrApplyOrchestratorNoArtifacts, req.AgentHostID, coreType, targetRevision)
	}

	payload := make([]ApplyBatchArtifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact == nil {
			continue
		}
		payload = append(payload, ApplyBatchArtifact{
			Filename:    artifact.Filename,
			SourceTag:   artifact.SourceTag,
			Content:     artifact.Content,
			ContentHash: artifact.ContentHash,
		})
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("%w (agent_host_id=%d core_type=%s target_revision=%d)", ErrApplyOrchestratorNoPayload, req.AgentHostID, coreType, targetRevision)
	}

	run, err := s.PrepareApplyRun(ctx, PrepareApplyRunRequest{
		AgentHostID:      req.AgentHostID,
		CoreType:         coreType,
		TargetRevision:   targetRevision,
		PreviousRevision: req.CurrentRevision,
		OperatorID:       req.RunOperatorID,
	})
	if err != nil {
		return nil, err
	}
	previousRevision := req.CurrentRevision
	if run.PreviousRevision > 0 {
		previousRevision = run.PreviousRevision
	}

	return &GetApplyBatchResult{
		NotModified:      false,
		RunID:            run.RunID,
		CoreType:         coreType,
		TargetRevision:   targetRevision,
		PreviousRevision: previousRevision,
		Artifacts:        payload,
	}, nil
}

func (s *applyOrchestratorService) ReportApplyResult(ctx context.Context, req ReportApplyResultRequest) error {
	if s == nil || s.applyRuns == nil {
		return ErrApplyOrchestratorNotConfigured
	}
	if req.AgentHostID <= 0 {
		return fmt.Errorf("%w (agent_host_id is required / 不能为空)", ErrApplyOrchestratorInvalidRequest)
	}
	runID := strings.TrimSpace(req.RunID)
	if runID == "" {
		return fmt.Errorf("%w (run_id is required / 不能为空)", ErrApplyOrchestratorInvalidRequest)
	}

	run, err := s.applyRuns.FindByRunID(ctx, runID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrApplyOrchestratorNotFound
		}
		return err
	}
	if run.AgentHostID != req.AgentHostID {
		return ErrApplyOrchestratorPermissionDenied
	}

	if reqCoreType := strings.TrimSpace(req.CoreType); reqCoreType != "" {
		reqCoreType = normalizeCoreType(reqCoreType)
		if reqCoreType == "" {
			return fmt.Errorf("%w (core_type must be sing-box or xray / 必须是 sing-box 或 xray)", ErrApplyOrchestratorInvalidRequest)
		}
		if reqCoreType != run.CoreType {
			return fmt.Errorf("%w (core_type does not match apply run)", ErrApplyOrchestratorInvalidRequest)
		}
	}
	if req.TargetRevision > 0 && req.TargetRevision != run.TargetRevision {
		return fmt.Errorf("%w (target_revision does not match apply run)", ErrApplyOrchestratorInvalidRequest)
	}

	nextStatus, err := normalizeApplyRunStatus(req.Success, req.Status)
	if err != nil {
		return err
	}
	if err := validateApplyResultStatusConsistency(req.Success, req.Status, nextStatus); err != nil {
		return err
	}
	currentStatus, err := normalizeApplyRunStatus(false, run.Status)
	if err != nil {
		return fmt.Errorf("%w (stored status is invalid: %s)", ErrApplyOrchestratorInvalidState, run.Status)
	}
	if currentStatus == nextStatus {
		return nil
	}
	if shouldIgnoreLateApplyRunStatus(currentStatus, nextStatus) {
		return nil
	}
	if !isApplyRunTransitionAllowed(currentStatus, nextStatus) {
		return fmt.Errorf("%w (from=%s to=%s)", ErrApplyOrchestratorInvalidState, currentStatus, nextStatus)
	}

	finishedAt := req.FinishedAt
	if isApplyRunTerminalStatus(nextStatus) {
		if finishedAt <= 0 {
			finishedAt = time.Now().Unix()
		}
	} else {
		finishedAt = 0
	}

	if err := s.applyRuns.UpdateStatus(ctx, run.RunID, nextStatus, req.ErrorMessage, req.RollbackRevision, finishedAt); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrApplyOrchestratorNotFound
		}
		return err
	}
	return nil
}

func normalizeApplyRunStatus(success bool, statusValue string) (string, error) {
	if trimmed := strings.ToLower(strings.TrimSpace(statusValue)); trimmed != "" {
		switch trimmed {
		case applyRunStatusPending, applyRunStatusApplying, applyRunStatusSuccess, applyRunStatusFailed, applyRunStatusRolledBack:
			return trimmed, nil
		default:
			return "", fmt.Errorf("%w (status must be pending/applying/success/failed/rolled_back)", ErrApplyOrchestratorInvalidRequest)
		}
	}
	if success {
		return applyRunStatusSuccess, nil
	}
	return applyRunStatusFailed, nil
}

func isApplyRunTransitionAllowed(currentStatus, nextStatus string) bool {
	switch currentStatus {
	case applyRunStatusPending:
		return nextStatus == applyRunStatusApplying || nextStatus == applyRunStatusSuccess || nextStatus == applyRunStatusFailed || nextStatus == applyRunStatusRolledBack
	case applyRunStatusApplying:
		return nextStatus == applyRunStatusSuccess || nextStatus == applyRunStatusFailed || nextStatus == applyRunStatusRolledBack
	case applyRunStatusFailed:
		return nextStatus == applyRunStatusRolledBack
	case applyRunStatusSuccess, applyRunStatusRolledBack:
		return false
	default:
		return false
	}
}

func shouldIgnoreLateApplyRunStatus(currentStatus, nextStatus string) bool {
	if !isApplyRunTerminalStatus(currentStatus) {
		return false
	}
	switch nextStatus {
	case applyRunStatusPending, applyRunStatusApplying:
		return true
	case applyRunStatusFailed:
		return currentStatus == applyRunStatusSuccess || currentStatus == applyRunStatusRolledBack
	case applyRunStatusRolledBack:
		return currentStatus == applyRunStatusSuccess
	default:
		return false
	}
}

func isApplyRunTerminalStatus(statusValue string) bool {
	switch statusValue {
	case applyRunStatusSuccess, applyRunStatusFailed, applyRunStatusRolledBack:
		return true
	default:
		return false
	}
}

func validateApplyResultStatusConsistency(success bool, statusValue, nextStatus string) error {
	trimmed := strings.TrimSpace(statusValue)
	if trimmed == "" {
		return nil
	}
	expected := applyRunStatusFailed
	if success {
		expected = applyRunStatusSuccess
	}
	if nextStatus == applyRunStatusApplying || nextStatus == applyRunStatusPending {
		return nil
	}
	if nextStatus != expected {
		return fmt.Errorf("%w (status and success are inconsistent)", ErrApplyOrchestratorInvalidRequest)
	}
	return nil
}

func applyRunOperatorIDPtr(operatorID int64) *int64 {
	if operatorID <= 0 {
		return nil
	}
	return &operatorID
}

func findReusableOpenApplyRun(ctx context.Context, applyRuns repository.ApplyRunRepository, agentHostID int64, coreType string, targetRevision int64) (*repository.ApplyRun, error) {
	filter := repository.ApplyRunFilter{
		AgentHostID:    &agentHostID,
		CoreType:       &coreType,
		TargetRevision: &targetRevision,
		Statuses:       []string{applyRunStatusPending, applyRunStatusApplying},
		Limit:          1,
		Offset:         0,
	}
	runs, err := applyRuns.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	for _, run := range runs {
		if run != nil {
			return run, nil
		}
	}
	return nil, nil
}


func (s *applyOrchestratorService) CancelApplyRun(ctx context.Context, req CancelApplyRunRequest) error {
	if s == nil || s.applyRuns == nil {
		return ErrApplyOrchestratorNotConfigured
	}
	if req.AgentHostID <= 0 {
		return fmt.Errorf("%w (agent_host_id is required / 不能为空)", ErrApplyOrchestratorInvalidRequest)
	}
	runID := strings.TrimSpace(req.RunID)
	if runID == "" {
		return fmt.Errorf("%w (run_id is required / 不能为空)", ErrApplyOrchestratorInvalidRequest)
	}

	run, err := s.applyRuns.FindByRunID(ctx, runID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrApplyOrchestratorNotFound
		}
		return err
	}
	if run.AgentHostID != req.AgentHostID {
		return ErrApplyOrchestratorPermissionDenied
	}

	// 只有 pending 状态的 run 可以被取消
	if run.Status != applyRunStatusPending && run.Status != applyRunStatusApplying {
		return fmt.Errorf("%w (run status is %s, expected pending or applying / 发布状态为 %s，需要 pending 或 applying)", ErrApplyOrchestratorInvalidState, run.Status, run.Status)
	}

	if err := s.applyRuns.UpdateStatus(ctx, runID, applyRunStatusFailed, "cancelled by operator", 0, time.Now().Unix()); err != nil {
		return err
	}
	return nil
}

// CleanupExpiredApplyRuns expires stale non-terminal apply runs: runs stuck in
// pending/applying for longer than maxAge (agent offline, report lost) are marked
// failed with a timeout reason, keeping the audit record instead of deleting it.
func (s *applyOrchestratorService) CleanupExpiredApplyRuns(ctx context.Context, maxAge time.Duration) (int, error) {
	if s == nil || s.applyRuns == nil {
		return 0, ErrApplyOrchestratorNotConfigured
	}
	if maxAge <= 0 {
		return 0, fmt.Errorf("%w (max_age must be positive)", ErrApplyOrchestratorInvalidRequest)
	}
	deadline := time.Now().Unix() - int64(maxAge.Seconds())
	n, err := s.applyRuns.ExpireStale(ctx, deadline, fmt.Sprintf("apply run timed out: no terminal report within %s", maxAge))
	if err != nil {
		return 0, fmt.Errorf("expire stale apply runs: %w", err)
	}
	return int(n), nil
}

func (s *applyOrchestratorService) findReusableOpenApplyRun(ctx context.Context, agentHostID int64, coreType string, targetRevision int64) (*repository.ApplyRun, error) {
	if s == nil || s.applyRuns == nil {
		return nil, ErrApplyOrchestratorNotConfigured
	}
	return findReusableOpenApplyRun(ctx, s.applyRuns, agentHostID, coreType, targetRevision)
}

func generateApplyRunID(agentHostID int64) (string, error) {
	randSuffix, err := randomHex(4)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("run_%d_%d_%s", agentHostID, time.Now().UnixNano(), randSuffix), nil
}
