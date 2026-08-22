package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
	agentv1 "github.com/creamcroissant/mgpanel/pkg/pb/agent/v1"
)

var (
	ErrCoreOperationInvalidRequest = errors.New("service: invalid core operation request / core 任务请求无效")
	ErrCoreOperationNotConfigured  = errors.New("service: core operation service not configured / core 任务服务未配置")
	ErrCoreOperationNotFound       = errors.New("service: core operation not found / core 任务不存在")
	ErrCoreOperationForbidden      = errors.New("service: core operation forbidden / core 任务不属于当前节点")
)

const (
	coreOperationTypeCreate  = "create"
	coreOperationTypeSwitch  = "switch"
	coreOperationTypeInstall = "install"
	coreOperationTypeEnsure  = "ensure"

	coreOperationStatusPending    = "pending"
	coreOperationStatusClaimed    = "claimed"
	coreOperationStatusInProgress = "in_progress"
	coreOperationStatusCompleted  = "completed"
	coreOperationStatusFailed     = "failed"
	coreOperationStatusRolledBack = "rolled_back"
)


type CoreOperationService interface {
	Create(ctx context.Context, req CreateCoreOperationRequest) (*repository.CoreOperation, error)
	List(ctx context.Context, req ListCoreOperationsRequest) ([]*repository.CoreOperation, int64, error)
	Get(ctx context.Context, operationID string) (*repository.CoreOperation, error)
	ClaimNext(ctx context.Context, req ClaimCoreOperationRequest) (*repository.CoreOperation, error)
	ReportResult(ctx context.Context, req ReportCoreOperationResultRequest) error
}

type CreateCoreOperationRequest struct {
	AgentHostID    int64
	OperationType  string
	CoreType       string
	RequestPayload json.RawMessage
	OperatorID     *int64
}

type ListCoreOperationsRequest struct {
	AgentHostID   *int64
	OperationType string
	CoreType      string
	Statuses      []string
	StartAt       *int64
	EndAt         *int64
	Limit         int
	Offset        int
}

type ClaimCoreOperationRequest struct {
	AgentHostID int64
	ClaimedBy   string
	Statuses    []string
}

type ReportCoreOperationResultRequest struct {
	AgentHostID   int64
	OperationID   string
	Status        string
	ResultPayload json.RawMessage
	ErrorMessage  string
	FinishedAt    int64
}

type coreOperationService struct {
	operations repository.CoreOperationRepository
	guard      AgentOperationGuard
}

func NewCoreOperationService(operations repository.CoreOperationRepository, guards ...AgentOperationGuard) CoreOperationService {
	var guard AgentOperationGuard
	if len(guards) > 0 {
		guard = guards[0]
	}
	return &coreOperationService{operations: operations, guard: guard}
}

func (s *coreOperationService) Create(ctx context.Context, req CreateCoreOperationRequest) (*repository.CoreOperation, error) {
	if s == nil || s.operations == nil {
		return nil, ErrCoreOperationNotConfigured
	}
	if req.AgentHostID <= 0 || strings.TrimSpace(req.OperationType) == "" || strings.TrimSpace(req.CoreType) == "" {
		return nil, ErrCoreOperationInvalidRequest
	}
	operationType := strings.TrimSpace(req.OperationType)
	coreType := strings.TrimSpace(req.CoreType)
	if len(req.RequestPayload) == 0 {
		req.RequestPayload = json.RawMessage(`{}`)
	}
	if s.guard != nil {
		if err := s.guard.CheckIdle(ctx, AgentOperationGuardRequest{AgentHostID: req.AgentHostID, Scope: OperationLogScopeCoreOperation, OperationType: operationType, OperatorID: req.OperatorID}); err != nil {
			return nil, err
		}
	}
	id, err := generateCoreOperationID(req.AgentHostID)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	op := &repository.CoreOperation{
		ID:             id,
		AgentHostID:    req.AgentHostID,
		OperationType:  operationType,
		CoreType:       coreType,
		Status:         coreOperationStatusPending,
		RequestPayload: append(json.RawMessage(nil), req.RequestPayload...),
		ResultPayload:  json.RawMessage(`{}`),
		OperatorID:     req.OperatorID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.operations.Create(ctx, op); err != nil {
		return nil, err
	}
	return op, nil
}

func (s *coreOperationService) List(ctx context.Context, req ListCoreOperationsRequest) ([]*repository.CoreOperation, int64, error) {
	if s == nil || s.operations == nil {
		return nil, 0, ErrCoreOperationNotConfigured
	}
	filter := repository.CoreOperationFilter{Limit: req.Limit, Offset: req.Offset, Statuses: append([]string(nil), req.Statuses...)}
	if req.AgentHostID != nil {
		filter.AgentHostID = req.AgentHostID
	}
	if opType := strings.TrimSpace(req.OperationType); opType != "" {
		filter.OperationType = &opType
	}
	if coreType := strings.TrimSpace(req.CoreType); coreType != "" {
		filter.CoreType = &coreType
	}
	if req.StartAt != nil {
		filter.CreatedAfter = req.StartAt
	}
	if req.EndAt != nil {
		filter.CreatedBefore = req.EndAt
	}
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

func (s *coreOperationService) Get(ctx context.Context, operationID string) (*repository.CoreOperation, error) {
	if s == nil || s.operations == nil {
		return nil, ErrCoreOperationNotConfigured
	}
	if strings.TrimSpace(operationID) == "" {
		return nil, ErrCoreOperationInvalidRequest
	}
	op, err := s.operations.FindByID(ctx, strings.TrimSpace(operationID))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrCoreOperationNotFound
		}
		return nil, err
	}
	return op, nil
}

func (s *coreOperationService) ClaimNext(ctx context.Context, req ClaimCoreOperationRequest) (*repository.CoreOperation, error) {
	if s == nil || s.operations == nil {
		return nil, ErrCoreOperationNotConfigured
	}
	if req.AgentHostID <= 0 || strings.TrimSpace(req.ClaimedBy) == "" {
		return nil, ErrCoreOperationInvalidRequest
	}
	statuses := req.Statuses
	if len(statuses) == 0 {
		statuses = []string{coreOperationStatusPending, coreOperationStatusClaimed}
	}
	now := time.Now().Unix()
	reclaimBefore := time.Now().Add(-coreOperationClaimTimeout).Unix()
	op, err := s.operations.ClaimNext(ctx, req.AgentHostID, statuses, strings.TrimSpace(req.ClaimedBy), now, &reclaimBefore)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrCoreOperationNotFound
		}
		return nil, err
	}
	return op, nil
}

func (s *coreOperationService) ReportResult(ctx context.Context, req ReportCoreOperationResultRequest) error {
	if s == nil || s.operations == nil {
		return ErrCoreOperationNotConfigured
	}
	operationID := strings.TrimSpace(req.OperationID)
	statusValue := strings.TrimSpace(req.Status)
	if req.AgentHostID <= 0 || operationID == "" || statusValue == "" {
		return ErrCoreOperationInvalidRequest
	}
	if !isValidCoreOperationTerminalStatus(statusValue) {
		return ErrCoreOperationInvalidRequest
	}
	op, err := s.operations.FindByID(ctx, operationID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrCoreOperationNotFound
		}
		return err
	}
	if op.AgentHostID != req.AgentHostID {
		return ErrCoreOperationForbidden
	}
	if !canTransitionCoreOperationStatus(op.Status, statusValue) {
		return ErrCoreOperationInvalidRequest
	}
	finishedAt := req.FinishedAt
	if finishedAt == 0 {
		finishedAt = time.Now().Unix()
	}
	payload := req.ResultPayload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	if err := s.operations.UpdateStatus(ctx, operationID, statusValue, payload, strings.TrimSpace(req.ErrorMessage), op.ClaimedBy, op.ClaimedAt, op.StartedAt, &finishedAt); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrCoreOperationNotFound
		}
		return err
	}
	return nil
}

func isValidCoreOperationTerminalStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case coreOperationStatusCompleted, coreOperationStatusFailed, coreOperationStatusRolledBack:
		return true
	default:
		return false
	}
}

func canTransitionCoreOperationStatus(current, next string) bool {
	current = strings.TrimSpace(current)
	next = strings.TrimSpace(next)
	switch current {
	case coreOperationStatusPending, coreOperationStatusClaimed, coreOperationStatusInProgress:
		return isValidCoreOperationTerminalStatus(next)
	default:
		return false
	}
}

type CoreSnapshotService interface {
	BuildCoreSnapshots(ctx context.Context, agentHostID int64, instances []*repository.AgentCoreInstance) ([]*repository.CoreStatusSnapshot, error)
	ReplaceInstanceSnapshot(ctx context.Context, agentHostID int64, instances []*agentv1.CoreInstance, snapshots []*repository.CoreStatusSnapshot) error
}

type coreSnapshotService struct {
	agentHosts repository.AgentHostRepository
	instances  repository.AgentCoreInstanceRepository
}

func NewCoreSnapshotService(agentHosts repository.AgentHostRepository, instances repository.AgentCoreInstanceRepository) CoreSnapshotService {
	return &coreSnapshotService{agentHosts: agentHosts, instances: instances}
}

func (s *coreSnapshotService) BuildCoreSnapshots(ctx context.Context, agentHostID int64, instances []*repository.AgentCoreInstance) ([]*repository.CoreStatusSnapshot, error) {
	if s == nil || s.agentHosts == nil {
		return nil, ErrCoreOperationNotConfigured
	}
	if _, err := s.agentHosts.FindByID(ctx, agentHostID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	coreSet := make(map[string]*repository.CoreStatusSnapshot)
	for _, instance := range instances {
		if instance == nil || strings.TrimSpace(instance.CoreType) == "" {
			continue
		}
		coreType := strings.TrimSpace(instance.CoreType)
		if instance.CoreSnapshot != nil {
			clone := *instance.CoreSnapshot
			clone.Type = coreType
			coreSet[coreType] = &clone
			continue
		}
		coreSet[coreType] = &repository.CoreStatusSnapshot{Type: coreType, Installed: true}
	}
	result := make([]*repository.CoreStatusSnapshot, 0, len(coreSet))
	for _, snapshot := range coreSet {
		result = append(result, snapshot)
	}
	return result, nil
}

func (s *coreSnapshotService) ReplaceInstanceSnapshot(ctx context.Context, agentHostID int64, instances []*agentv1.CoreInstance, snapshots []*repository.CoreStatusSnapshot) error {
	if s == nil || s.instances == nil {
		return ErrCoreOperationNotConfigured
	}
	mapped := make([]*repository.AgentCoreInstance, 0, len(instances))
	snapshotByType := make(map[string]*repository.CoreStatusSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot == nil {
			continue
		}
		clone := *snapshot
		snapshotByType[clone.Type] = &clone
	}
	for _, inst := range instances {
		if inst == nil || strings.TrimSpace(inst.GetId()) == "" {
			continue
		}
		listenPorts := make([]int, 0, len(inst.GetListenPorts()))
		for _, port := range inst.GetListenPorts() {
			listenPorts = append(listenPorts, int(port))
		}
		configHash := strings.TrimSpace(inst.GetConfigHash())
		mapped = append(mapped, &repository.AgentCoreInstance{
			AgentHostID:     agentHostID,
			InstanceID:      strings.TrimSpace(inst.GetId()),
			CoreType:        strings.TrimSpace(inst.GetCoreType()),
			Status:          strings.TrimSpace(inst.GetStatus()),
			ListenPorts:     listenPorts,
			ConfigHash:      configHash,
			StartedAt:       nullableUnix(inst.GetStartedAt()),
			LastHeartbeatAt: unixNowPtr(),
			ErrorMessage:    strings.TrimSpace(inst.GetError()),
			CoreSnapshot:    snapshotByType[strings.TrimSpace(inst.GetCoreType())],
		})
	}
	return s.instances.ReplaceSnapshot(ctx, agentHostID, mapped)
}

func generateCoreOperationID(agentHostID int64) (string, error) {
	randSuffix, err := randomHexCoreOperation(4)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("core_%d_%d_%s", agentHostID, time.Now().UnixNano(), randSuffix), nil
}

func nullableUnix(v int64) *int64 {
	if v <= 0 {
		return nil
	}
	return &v
}

func unixNowPtr() *int64 {
	now := time.Now().Unix()
	return &now
}

func randomHexCoreOperation(n int) (string, error) {
	if n <= 0 {
		return "", nil
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
