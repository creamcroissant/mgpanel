package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/creamcroissant/xboard/internal/repository"
	"github.com/creamcroissant/xboard/internal/security"
)

var (
	ErrAgentLifecycleOperationInvalidRequest = errors.New("service: invalid agent lifecycle operation request / Agent 生命周期操作请求无效")
	ErrAgentLifecycleOperationNotConfigured  = errors.New("service: agent lifecycle operation service not configured / Agent 生命周期操作服务未配置")
	ErrAgentLifecycleOperationNotFound       = errors.New("service: agent lifecycle operation not found / Agent 生命周期操作不存在")
	ErrAgentLifecycleOperationForbidden      = errors.New("service: agent lifecycle operation forbidden / Agent 生命周期操作不属于当前节点")
)

const (
	AgentLifecycleOperationTypeAgentUpdate      = "agent_update"
	AgentLifecycleOperationTypeAgentUpdateCheck = "agent_update_check"
	AgentLifecycleOperationTypeTrafficReset     = "traffic_reset"
	AgentLifecycleOperationTypeThresholdAction  = "threshold_action"
	AgentLifecycleOperationTypeResetLinks       = "reset_links"
	AgentLifecycleOperationTypeCDNDeploySite    = "deploy_cdn_site"
	AgentLifecycleOperationTypeCDNRemoveSite    = "remove_cdn_site"
	AgentLifecycleOperationTypeSetRoutingTable  = "set_routing_table"

	agentLifecycleOperationTypeAgentUpdate      = AgentLifecycleOperationTypeAgentUpdate
	agentLifecycleOperationTypeAgentUpdateCheck = AgentLifecycleOperationTypeAgentUpdateCheck
	agentLifecycleOperationTypeTrafficReset     = AgentLifecycleOperationTypeTrafficReset
	agentLifecycleOperationTypeThresholdAction  = AgentLifecycleOperationTypeThresholdAction
	agentLifecycleOperationTypeResetLinks       = AgentLifecycleOperationTypeResetLinks

	agentLifecycleOperationTypeCDNDeploySite = AgentLifecycleOperationTypeCDNDeploySite
	agentLifecycleOperationTypeCDNRemoveSite = AgentLifecycleOperationTypeCDNRemoveSite
	agentLifecycleOperationTypeSetRoutingTable = AgentLifecycleOperationTypeSetRoutingTable

	agentLifecycleOperationStatusPending           = "pending"
	agentLifecycleOperationStatusClaimed           = "claimed"
	agentLifecycleOperationStatusInProgress        = "in_progress"
	agentLifecycleOperationStatusSuccess           = "success"
	agentLifecycleOperationStatusFailed            = "failed"
	agentLifecycleOperationStatusTimeout           = "timeout"
	agentLifecycleOperationStatusCancelled         = "cancelled"
	agentLifecycleOperationStatusUnsupportedAction = "unsupported_action"
	agentLifecycleOperationStatusQueueFull         = "queue_full"

	agentLifecycleOperationEventAccepted = "accepted"
	agentLifecycleOperationEventProgress = "progress"
	agentLifecycleOperationEventResult   = "result"

	agentLifecycleOperationSourceAdmin  = "admin"
	agentLifecycleOperationSourceSystem = "system"

	agentLifecycleOperationDefaultLimit = 100
	agentLifecycleOperationMaxLimit     = 200
	agentLifecycleOperationMaxClaim     = 20


	agentLifecycleOperationCreatedAuditKind   = "admin.agent_lifecycle_operation.created"
	agentLifecycleOperationForbiddenAuditKind = "agent.agent_lifecycle_operation.forbidden"
)

type AgentLifecycleOperationService interface {
	Create(ctx context.Context, req CreateAgentLifecycleOperationRequest) (*repository.AgentLifecycleOperation, error)
	List(ctx context.Context, req ListAgentLifecycleOperationsRequest) ([]*repository.AgentLifecycleOperation, int64, error)
	Get(ctx context.Context, operationID string) (*repository.AgentLifecycleOperation, error)
	ClaimNext(ctx context.Context, req ClaimAgentLifecycleOperationRequest) ([]*repository.AgentLifecycleOperation, error)
	Report(ctx context.Context, req ReportAgentLifecycleOperationRequest) error
}

type CreateAgentLifecycleOperationRequest struct {
	AgentHostID    int64
	OperationType  string
	RequestPayload json.RawMessage
	OperatorID     *int64
	Source         string
}

type ListAgentLifecycleOperationsRequest struct {
	AgentHostID   *int64
	OperationType string
	Status        string
	Statuses      []string
	Source        string
	StartAt       *int64
	EndAt         *int64
	Limit         int
	Offset        int
}

type AgentCommandQueueStats struct {
	Capacity         int32
	Queued           int32
	Inflight         int32
	Workers          int32
	Available        int32
	ActiveCommandIDs []string
	UpdatedAt        int64
}

type ClaimAgentLifecycleOperationRequest struct {
	AgentHostID      int64
	ClaimedBy        string
	Statuses         []string
	SupportedActions []string
	QueueStats       *AgentCommandQueueStats
	Limit            int
	RequestedAt      int64
}

type ReportAgentLifecycleOperationRequest struct {
	AgentHostID   int64
	OperationID   string
	ClaimedBy     string
	QueueStats    *AgentCommandQueueStats
	EventType     string
	Status        string
	Phase         string
	Level         string
	Message       string
	Payload       json.RawMessage
	ErrorMessage  string
	OccurredAt    int64
	SourceEventID string
	Sequence      int64
	Terminal      bool
}

type agentLifecycleOperationService struct {
	operations repository.AgentLifecycleOperationRepository
	guard      AgentOperationGuard
	logs       OperationLogService
	audit      security.Recorder
}

func NewAgentLifecycleOperationService(operations repository.AgentLifecycleOperationRepository, guard AgentOperationGuard, logs OperationLogService, audit security.Recorder) AgentLifecycleOperationService {
	return &agentLifecycleOperationService{operations: operations, guard: guard, logs: logs, audit: audit}
}

func (s *agentLifecycleOperationService) Create(ctx context.Context, req CreateAgentLifecycleOperationRequest) (*repository.AgentLifecycleOperation, error) {
	if s == nil || s.operations == nil {
		return nil, ErrAgentLifecycleOperationNotConfigured
	}
	operationType, err := normalizeAgentLifecycleOperationType(req.OperationType)
	if err != nil {
		return nil, err
	}
	if req.AgentHostID <= 0 {
		return nil, ErrAgentLifecycleOperationInvalidRequest
	}
	requestPayload, err := sanitizeAgentLifecycleOperationPayload(req.RequestPayload)
	if err != nil {
		return nil, err
	}
	if s.guard != nil && isDestructiveAgentLifecycleOperationType(operationType) {
		if err := s.guard.CheckIdle(ctx, AgentOperationGuardRequest{AgentHostID: req.AgentHostID, Scope: operationLogScopeForAgentLifecycleOperationType(operationType), OperationType: operationType, OperatorID: req.OperatorID}); err != nil {
			return nil, err
		}
	}
	id, err := generateAgentLifecycleOperationID(req.AgentHostID)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	operation := &repository.AgentLifecycleOperation{
		ID:             id,
		AgentHostID:    req.AgentHostID,
		OperationType:  operationType,
		Status:         agentLifecycleOperationStatusPending,
		RequestPayload: requestPayload,
		ResultPayload:  json.RawMessage(`{}`),
		OperatorID:     cloneInt64Ptr(req.OperatorID),
		Source:         normalizeAgentLifecycleOperationSource(req.Source),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.operations.Create(ctx, operation); err != nil {
		return nil, err
	}
	if err := s.appendLifecycleOperationLog(ctx, operation, "created", OperationLogLevelInfo, "agent lifecycle operation created", requestPayload, "", 0, now); err != nil {
		return nil, err
	}
	s.recordLifecycleOperationCreated(ctx, operation)
	return operation, nil
}

func (s *agentLifecycleOperationService) List(ctx context.Context, req ListAgentLifecycleOperationsRequest) ([]*repository.AgentLifecycleOperation, int64, error) {
	if s == nil || s.operations == nil {
		return nil, 0, ErrAgentLifecycleOperationNotConfigured
	}
	filter := repository.AgentLifecycleOperationFilter{Limit: normalizeAgentLifecycleOperationLimit(req.Limit), Offset: normalizeAgentLifecycleOperationOffset(req.Offset)}
	if req.AgentHostID != nil {
		if *req.AgentHostID <= 0 {
			return nil, 0, ErrAgentLifecycleOperationInvalidRequest
		}
		agentHostID := *req.AgentHostID
		filter.AgentHostID = &agentHostID
	}
	if strings.TrimSpace(req.OperationType) != "" {
		operationType, err := normalizeAgentLifecycleOperationType(req.OperationType)
		if err != nil {
			return nil, 0, err
		}
		filter.OperationType = &operationType
	}
	if strings.TrimSpace(req.Status) != "" {
		status, err := normalizeAgentLifecycleOperationStatus(req.Status)
		if err != nil {
			return nil, 0, err
		}
		filter.Status = &status
	}
	statuses, err := normalizeAgentLifecycleOperationStatuses(req.Statuses)
	if err != nil {
		return nil, 0, err
	}
	filter.Statuses = statuses
	if strings.TrimSpace(req.Source) != "" {
		source := strings.TrimSpace(req.Source)
		filter.Source = &source
	}
	filter.CreatedAfter = req.StartAt
	filter.CreatedBefore = req.EndAt

	items, err := s.operations.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.operations.Count(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *agentLifecycleOperationService) Get(ctx context.Context, operationID string) (*repository.AgentLifecycleOperation, error) {
	if s == nil || s.operations == nil {
		return nil, ErrAgentLifecycleOperationNotConfigured
	}
	operationID = strings.TrimSpace(operationID)
	if operationID == "" {
		return nil, ErrAgentLifecycleOperationInvalidRequest
	}
	operation, err := s.operations.FindByID(ctx, operationID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrAgentLifecycleOperationNotFound
		}
		return nil, err
	}
	return operation, nil
}

func (s *agentLifecycleOperationService) ClaimNext(ctx context.Context, req ClaimAgentLifecycleOperationRequest) ([]*repository.AgentLifecycleOperation, error) {
	if s == nil || s.operations == nil {
		return nil, ErrAgentLifecycleOperationNotConfigured
	}
	claimedBy := strings.TrimSpace(req.ClaimedBy)
	if req.AgentHostID <= 0 || claimedBy == "" {
		return nil, ErrAgentLifecycleOperationInvalidRequest
	}
	statuses, err := normalizeAgentLifecycleClaimStatuses(req.Statuses)
	if err != nil {
		return nil, err
	}
	supportedActions := normalizeAgentLifecycleSupportedActions(req.SupportedActions)
	if len(supportedActions) == 0 {
		return nil, ErrAgentLifecycleOperationNotFound
	}
	limit := normalizeAgentLifecycleOperationClaimLimit(req.Limit)
	now := time.Now().Unix()
	reclaimBefore := time.Now().Add(-agentLifecycleOperationClaimTimeout).Unix()
	operations, err := s.operations.ClaimNext(ctx, req.AgentHostID, statuses, supportedActions, claimedBy, now, &reclaimBefore, limit)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrAgentLifecycleOperationNotFound
		}
		return nil, err
	}
	for _, operation := range operations {
		if err := s.appendLifecycleOperationLog(ctx, operation, "claimed", OperationLogLevelInfo, "agent lifecycle operation claimed", json.RawMessage(`{"claimed_by":`+strconv.Quote(claimedBy)+`}`), "", 0, now); err != nil {
			return nil, err
		}
	}
	return operations, nil
}

func (s *agentLifecycleOperationService) Report(ctx context.Context, req ReportAgentLifecycleOperationRequest) error {
	if s == nil || s.operations == nil {
		return ErrAgentLifecycleOperationNotConfigured
	}
	operationID := strings.TrimSpace(req.OperationID)
	claimedBy := strings.TrimSpace(req.ClaimedBy)
	if req.AgentHostID <= 0 || operationID == "" || claimedBy == "" {
		return ErrAgentLifecycleOperationInvalidRequest
	}
	operation, err := s.operations.FindByID(ctx, operationID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrAgentLifecycleOperationNotFound
		}
		return err
	}
	if operation.AgentHostID != req.AgentHostID || strings.TrimSpace(operation.ClaimedBy) != claimedBy {
		s.recordLifecycleOperationForbidden(ctx, req, operation)
		return ErrAgentLifecycleOperationForbidden
	}
	nextStatus, terminal, err := normalizeAgentLifecycleReportStatus(req)
	if err != nil {
		return err
	}
	resultPayload, err := sanitizeAgentLifecycleOperationPayload(req.Payload)
	if err != nil {
		return err
	}
	errorMessage := strings.TrimSpace(req.ErrorMessage)
	if isAgentLifecycleOperationTerminalStatus(operation.Status) {
		if isIdempotentTerminalAgentLifecycleOperationReport(operation, nextStatus, resultPayload, errorMessage) {
			return nil
		}
		return ErrAgentLifecycleOperationInvalidRequest
	}
	if !canTransitionAgentLifecycleOperationStatus(operation.Status, nextStatus) {
		return ErrAgentLifecycleOperationInvalidRequest
	}
	occurredAt := req.OccurredAt
	if occurredAt == 0 {
		occurredAt = time.Now().Unix()
	}
	startedAt := operation.StartedAt
	if nextStatus == agentLifecycleOperationStatusInProgress && startedAt == nil {
		startedAt = &occurredAt
	}
	finishedAt := operation.FinishedAt
	if terminal {
		finishedAt = &occurredAt
	}
	if err := s.operations.UpdateClaimedStatus(ctx, operationID, claimedBy, nextStatus, resultPayload, errorMessage, startedAt, finishedAt); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrAgentLifecycleOperationNotFound
		}
		return err
	}
	phase := normalizeAgentLifecycleReportPhase(req, nextStatus)
	level := normalizeAgentLifecycleReportLogLevel(req, nextStatus, terminal)
	message := normalizeAgentLifecycleReportMessage(req, nextStatus)
	if err := s.appendLifecycleOperationLog(ctx, operation, phase, level, message, resultPayload, strings.TrimSpace(req.SourceEventID), req.Sequence, occurredAt); err != nil {
		return err
	}
	return nil
}

func normalizeAgentLifecycleOperationType(operationType string) (string, error) {
	switch strings.TrimSpace(operationType) {
	case agentLifecycleOperationTypeAgentUpdate:
		return agentLifecycleOperationTypeAgentUpdate, nil
	case agentLifecycleOperationTypeAgentUpdateCheck:
		return agentLifecycleOperationTypeAgentUpdateCheck, nil
	case agentLifecycleOperationTypeTrafficReset:
		return agentLifecycleOperationTypeTrafficReset, nil
	case agentLifecycleOperationTypeThresholdAction:
		return agentLifecycleOperationTypeThresholdAction, nil
	case agentLifecycleOperationTypeResetLinks:
		return agentLifecycleOperationTypeResetLinks, nil
	case agentLifecycleOperationTypeCDNDeploySite:
		return agentLifecycleOperationTypeCDNDeploySite, nil
	case agentLifecycleOperationTypeCDNRemoveSite:
		return agentLifecycleOperationTypeCDNRemoveSite, nil
	case agentLifecycleOperationTypeSetRoutingTable:
		return agentLifecycleOperationTypeSetRoutingTable, nil
	default:
		return "", ErrAgentLifecycleOperationInvalidRequest
	}
}

func normalizeAgentLifecycleOperationStatus(status string) (string, error) {
	status = strings.TrimSpace(status)
	if isValidAgentLifecycleOperationStatus(status) {
		return status, nil
	}
	return "", ErrAgentLifecycleOperationInvalidRequest
}

func normalizeAgentLifecycleOperationStatuses(statuses []string) ([]string, error) {
	if len(statuses) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(statuses))
	for _, status := range statuses {
		normalized, err := normalizeAgentLifecycleOperationStatus(status)
		if err != nil {
			return nil, err
		}
		out = append(out, normalized)
	}
	return out, nil
}

func normalizeAgentLifecycleClaimStatuses(statuses []string) ([]string, error) {
	if len(statuses) == 0 {
		return []string{agentLifecycleOperationStatusPending, agentLifecycleOperationStatusClaimed}, nil
	}
	out := make([]string, 0, len(statuses))
	for _, status := range statuses {
		switch strings.TrimSpace(status) {
		case agentLifecycleOperationStatusPending:
			out = append(out, agentLifecycleOperationStatusPending)
		case agentLifecycleOperationStatusClaimed:
			out = append(out, agentLifecycleOperationStatusClaimed)
		default:
			return nil, ErrAgentLifecycleOperationInvalidRequest
		}
	}
	return out, nil
}

func normalizeAgentLifecycleSupportedActions(actions []string) []string {
	if len(actions) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(actions))
	out := make([]string, 0, len(actions))
	for _, action := range actions {
		normalized, err := normalizeAgentLifecycleOperationType(action)
		if err != nil {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func normalizeAgentLifecycleReportStatus(req ReportAgentLifecycleOperationRequest) (string, bool, error) {
	status := strings.TrimSpace(req.Status)
	eventType := strings.TrimSpace(req.EventType)
	switch status {
	case "":
		switch eventType {
		case agentLifecycleOperationEventAccepted:
			status = agentLifecycleOperationStatusClaimed
		case agentLifecycleOperationEventProgress:
			status = agentLifecycleOperationStatusInProgress
		case agentLifecycleOperationEventResult:
			return "", false, ErrAgentLifecycleOperationInvalidRequest
		default:
			status = agentLifecycleOperationStatusInProgress
		}
	case agentLifecycleOperationEventAccepted:
		status = agentLifecycleOperationStatusClaimed
	case agentLifecycleOperationEventProgress:
		status = agentLifecycleOperationStatusInProgress
	}
	if isAgentLifecycleOperationTerminalStatus(status) {
		return status, true, nil
	}
	if req.Terminal || eventType == agentLifecycleOperationEventResult {
		return "", false, ErrAgentLifecycleOperationInvalidRequest
	}
	switch status {
	case agentLifecycleOperationStatusClaimed, agentLifecycleOperationStatusInProgress:
		return status, false, nil
	default:
		return "", false, ErrAgentLifecycleOperationInvalidRequest
	}
}

func isValidAgentLifecycleOperationStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case agentLifecycleOperationStatusPending,
		agentLifecycleOperationStatusClaimed,
		agentLifecycleOperationStatusInProgress,
		agentLifecycleOperationStatusSuccess,
		agentLifecycleOperationStatusFailed,
		agentLifecycleOperationStatusTimeout,
		agentLifecycleOperationStatusCancelled,
		agentLifecycleOperationStatusUnsupportedAction,
		agentLifecycleOperationStatusQueueFull:
		return true
	default:
		return false
	}
}

func isAgentLifecycleOperationTerminalStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case agentLifecycleOperationStatusSuccess,
		agentLifecycleOperationStatusFailed,
		agentLifecycleOperationStatusTimeout,
		agentLifecycleOperationStatusCancelled,
		agentLifecycleOperationStatusUnsupportedAction,
		agentLifecycleOperationStatusQueueFull:
		return true
	default:
		return false
	}
}

func isIdempotentTerminalAgentLifecycleOperationReport(operation *repository.AgentLifecycleOperation, nextStatus string, resultPayload json.RawMessage, errorMessage string) bool {
	if operation == nil || !isAgentLifecycleOperationTerminalStatus(operation.Status) {
		return false
	}
	if strings.TrimSpace(operation.Status) != strings.TrimSpace(nextStatus) {
		return false
	}
	if strings.TrimSpace(operation.ErrorMessage) != strings.TrimSpace(errorMessage) {
		return false
	}
	return string(operation.ResultPayload) == string(resultPayload)
}

func canTransitionAgentLifecycleOperationStatus(current, next string) bool {
	current = strings.TrimSpace(current)
	next = strings.TrimSpace(next)
	switch current {
	case agentLifecycleOperationStatusPending:
		return next == agentLifecycleOperationStatusClaimed || next == agentLifecycleOperationStatusInProgress || isAgentLifecycleOperationTerminalStatus(next)
	case agentLifecycleOperationStatusClaimed:
		return next == agentLifecycleOperationStatusClaimed || next == agentLifecycleOperationStatusInProgress || isAgentLifecycleOperationTerminalStatus(next)
	case agentLifecycleOperationStatusInProgress:
		return next == agentLifecycleOperationStatusInProgress || isAgentLifecycleOperationTerminalStatus(next)
	default:
		return false
	}
}

func isDestructiveAgentLifecycleOperationType(operationType string) bool {
	switch strings.TrimSpace(operationType) {
	case agentLifecycleOperationTypeAgentUpdate,
		agentLifecycleOperationTypeTrafficReset,
		agentLifecycleOperationTypeThresholdAction,
		agentLifecycleOperationTypeResetLinks:
		return true
	default:
		return false
	}
}

func AgentLifecycleOperationLogScope(operationType string) string {
	return operationLogScopeForAgentLifecycleOperationType(operationType)
}

func operationLogScopeForAgentLifecycleOperationType(operationType string) string {
	switch strings.TrimSpace(operationType) {
	case agentLifecycleOperationTypeTrafficReset:
		return OperationLogScopeTrafficReset
	case agentLifecycleOperationTypeThresholdAction:
		return OperationLogScopeThresholdAction
	default:
		return OperationLogScopeAgentOperation
	}
}

func normalizeAgentLifecycleOperationSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return agentLifecycleOperationSourceAdmin
	}
	return source
}

func normalizeAgentLifecycleOperationLimit(limit int) int {
	if limit <= 0 {
		return agentLifecycleOperationDefaultLimit
	}
	if limit > agentLifecycleOperationMaxLimit {
		return agentLifecycleOperationMaxLimit
	}
	return limit
}

func normalizeAgentLifecycleOperationOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func normalizeAgentLifecycleOperationClaimLimit(limit int) int {
	if limit <= 0 {
		return 1
	}
	if limit > agentLifecycleOperationMaxClaim {
		return agentLifecycleOperationMaxClaim
	}
	return limit
}

func sanitizeAgentLifecycleOperationPayload(payload json.RawMessage) (json.RawMessage, error) {
	sanitized, err := sanitizeOperationLogPayload(payload)
	if err != nil {
		return nil, fmt.Errorf("%w (%v)", ErrAgentLifecycleOperationInvalidRequest, err)
	}
	return sanitized, nil
}

func normalizeAgentLifecycleReportPhase(req ReportAgentLifecycleOperationRequest, status string) string {
	phase := strings.TrimSpace(req.Phase)
	if phase != "" {
		return phase
	}
	if eventType := strings.TrimSpace(req.EventType); eventType != "" {
		return eventType
	}
	return strings.TrimSpace(status)
}

func normalizeAgentLifecycleReportLogLevel(req ReportAgentLifecycleOperationRequest, status string, terminal bool) string {
	level := strings.TrimSpace(req.Level)
	if level != "" {
		return level
	}
	if !terminal || status == agentLifecycleOperationStatusSuccess {
		return OperationLogLevelInfo
	}
	switch status {
	case agentLifecycleOperationStatusFailed, agentLifecycleOperationStatusTimeout:
		return OperationLogLevelError
	default:
		return OperationLogLevelWarn
	}
}

func normalizeAgentLifecycleReportMessage(req ReportAgentLifecycleOperationRequest, status string) string {
	message := strings.TrimSpace(req.Message)
	if message != "" {
		return message
	}
	if errorMessage := strings.TrimSpace(req.ErrorMessage); errorMessage != "" {
		return errorMessage
	}
	return "agent lifecycle operation " + strings.TrimSpace(status)
}

func (s *agentLifecycleOperationService) appendLifecycleOperationLog(ctx context.Context, operation *repository.AgentLifecycleOperation, phase, level, message string, payload json.RawMessage, sourceEventID string, sequence int64, reportedAt int64) error {
	if s == nil || s.logs == nil || operation == nil {
		return nil
	}
	_, err := s.logs.Append(ctx, AppendOperationLogRequest{
		Scope:         operationLogScopeForAgentLifecycleOperationType(operation.OperationType),
		TargetID:      operation.ID,
		AgentHostID:   operation.AgentHostID,
		Sequence:      sequence,
		Phase:         phase,
		Level:         level,
		Message:       message,
		Payload:       payload,
		SourceEventID: sourceEventID,
		ReportedAt:    reportedAt,
	})
	return err
}

func (s *agentLifecycleOperationService) recordLifecycleOperationCreated(ctx context.Context, operation *repository.AgentLifecycleOperation) {
	if s == nil || s.audit == nil || operation == nil {
		return
	}
	s.audit.Record(ctx, security.Event{
		Kind:    agentLifecycleOperationCreatedAuditKind,
		ActorID: lifecycleOperatorActorID(operation.OperatorID),
		Metadata: map[string]any{
			"agent_host_id":  operation.AgentHostID,
			"operation_id":   operation.ID,
			"operation_type": operation.OperationType,
			"source":         operation.Source,
		},
	})
}

func (s *agentLifecycleOperationService) recordLifecycleOperationForbidden(ctx context.Context, req ReportAgentLifecycleOperationRequest, operation *repository.AgentLifecycleOperation) {
	if s == nil || s.audit == nil || operation == nil {
		return
	}
	s.audit.Record(ctx, security.Event{
		Kind:    agentLifecycleOperationForbiddenAuditKind,
		ActorID: fmt.Sprintf("agent:%d", req.AgentHostID),
		Metadata: map[string]any{
			"requested_agent_host_id": req.AgentHostID,
			"operation_agent_host_id": operation.AgentHostID,
			"operation_id":            operation.ID,
			"operation_type":          operation.OperationType,
			"claimed_by":              operation.ClaimedBy,
			"requested_claimed_by":    strings.TrimSpace(req.ClaimedBy),
		},
	})
}

func lifecycleOperatorActorID(operatorID *int64) string {
	if operatorID != nil && *operatorID > 0 {
		return strconv.FormatInt(*operatorID, 10)
	}
	return "system"
}

func generateAgentLifecycleOperationID(agentHostID int64) (string, error) {
	suffix, err := randomHexAgentLifecycleOperation(4)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("agentop_%d_%d_%s", agentHostID, time.Now().UnixNano(), suffix), nil
}

func randomHexAgentLifecycleOperation(n int) (string, error) {
	if n <= 0 {
		return "", nil
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func cloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
