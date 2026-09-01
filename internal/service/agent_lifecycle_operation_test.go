package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type mockAgentLifecycleOperationRepo struct {
	operations map[string]*repository.AgentLifecycleOperation

	createErr error
	listErr   error
	countErr  error
	claimErr  error
	updateErr error

	lastCreated *repository.AgentLifecycleOperation
	lastList    repository.AgentLifecycleOperationFilter
	lastCount   repository.AgentLifecycleOperationFilter
}

func newMockAgentLifecycleOperationRepo() *mockAgentLifecycleOperationRepo {
	return &mockAgentLifecycleOperationRepo{operations: make(map[string]*repository.AgentLifecycleOperation)}
}

func (m *mockAgentLifecycleOperationRepo) Create(ctx context.Context, operation *repository.AgentLifecycleOperation) error {
	if m.createErr != nil {
		return m.createErr
	}
	clone := cloneAgentLifecycleOperationForTest(operation)
	m.operations[clone.ID] = clone
	m.lastCreated = cloneAgentLifecycleOperationForTest(clone)
	return nil
}

func (m *mockAgentLifecycleOperationRepo) UpdateStatus(ctx context.Context, id, status string, resultPayload json.RawMessage, errorMessage string, claimedBy string, claimedAt, startedAt, finishedAt *int64) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	operation, ok := m.operations[id]
	if !ok {
		return repository.ErrNotFound
	}
	operation.Status = status
	operation.ResultPayload = append(json.RawMessage(nil), resultPayload...)
	operation.ErrorMessage = errorMessage
	operation.ClaimedBy = claimedBy
	operation.ClaimedAt = cloneInt64Ptr(claimedAt)
	operation.StartedAt = cloneInt64Ptr(startedAt)
	operation.FinishedAt = cloneInt64Ptr(finishedAt)
	return nil
}

func (m *mockAgentLifecycleOperationRepo) UpdateClaimedStatus(ctx context.Context, id, claimedBy, status string, resultPayload json.RawMessage, errorMessage string, startedAt, finishedAt *int64) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	operation, ok := m.operations[id]
	if !ok || operation.ClaimedBy != claimedBy || isAgentLifecycleOperationTerminalStatus(operation.Status) {
		return repository.ErrNotFound
	}
	operation.Status = status
	operation.ResultPayload = append(json.RawMessage(nil), resultPayload...)
	operation.ErrorMessage = errorMessage
	operation.StartedAt = cloneInt64Ptr(startedAt)
	operation.FinishedAt = cloneInt64Ptr(finishedAt)
	return nil
}

func (m *mockAgentLifecycleOperationRepo) FindByID(ctx context.Context, id string) (*repository.AgentLifecycleOperation, error) {
	operation, ok := m.operations[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return cloneAgentLifecycleOperationForTest(operation), nil
}

func (m *mockAgentLifecycleOperationRepo) List(ctx context.Context, filter repository.AgentLifecycleOperationFilter) ([]*repository.AgentLifecycleOperation, error) {
	m.lastList = filter
	if m.listErr != nil {
		return nil, m.listErr
	}
	items := m.filteredOperations(filter)
	return items, nil
}

func (m *mockAgentLifecycleOperationRepo) Count(ctx context.Context, filter repository.AgentLifecycleOperationFilter) (int64, error) {
	m.lastCount = filter
	if m.countErr != nil {
		return 0, m.countErr
	}
	return int64(len(m.filteredOperations(filter))), nil
}

func (m *mockAgentLifecycleOperationRepo) ClaimNext(ctx context.Context, agentHostID int64, statuses []string, operationTypes []string, claimedBy string, claimedAt int64, reclaimBefore *int64, limit int) ([]*repository.AgentLifecycleOperation, error) {
	if m.claimErr != nil {
		return nil, m.claimErr
	}
	if limit <= 0 {
		limit = 1
	}
	operationTypeAllowed := func(operation *repository.AgentLifecycleOperation) bool {
		for _, operationType := range operationTypes {
			if operation.OperationType == operationType {
				return true
			}
		}
		return false
	}
	statusAllowed := func(operation *repository.AgentLifecycleOperation) bool {
		if len(statuses) == 0 {
			return operation.Status == agentLifecycleOperationStatusPending
		}
		for _, status := range statuses {
			if operation.Status != status {
				continue
			}
			if status == agentLifecycleOperationStatusClaimed && reclaimBefore != nil && operation.ClaimedAt != nil && *operation.ClaimedAt > *reclaimBefore {
				continue
			}
			return true
		}
		return false
	}
	candidates := make([]*repository.AgentLifecycleOperation, 0, len(m.operations))
	for _, operation := range m.operations {
		if operation.AgentHostID != agentHostID || !operationTypeAllowed(operation) || !statusAllowed(operation) {
			continue
		}
		candidates = append(candidates, operation)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].CreatedAt == candidates[j].CreatedAt {
			return candidates[i].ID < candidates[j].ID
		}
		return candidates[i].CreatedAt < candidates[j].CreatedAt
	})
	if len(candidates) == 0 {
		return nil, repository.ErrNotFound
	}
	claimed := make([]*repository.AgentLifecycleOperation, 0, limit)
	for _, operation := range candidates {
		if len(claimed) >= limit {
			break
		}
		startedAt := claimedAt
		operation.Status = agentLifecycleOperationStatusClaimed
		operation.ClaimedBy = claimedBy
		operation.ClaimedAt = &claimedAt
		operation.StartedAt = &startedAt
		claimed = append(claimed, cloneAgentLifecycleOperationForTest(operation))
	}
	return claimed, nil
}

func (m *mockAgentLifecycleOperationRepo) DeleteOlderThan(ctx context.Context, days int) (int64, error) {
	return 0, nil
}

func (m *mockAgentLifecycleOperationRepo) filteredOperations(filter repository.AgentLifecycleOperationFilter) []*repository.AgentLifecycleOperation {
	items := make([]*repository.AgentLifecycleOperation, 0, len(m.operations))
	statusSet := make(map[string]struct{}, len(filter.Statuses))
	for _, status := range filter.Statuses {
		statusSet[status] = struct{}{}
	}
	for _, operation := range m.operations {
		if filter.AgentHostID != nil && operation.AgentHostID != *filter.AgentHostID {
			continue
		}
		if filter.OperationType != nil && operation.OperationType != *filter.OperationType {
			continue
		}
		if len(statusSet) > 0 {
			if _, ok := statusSet[operation.Status]; !ok {
				continue
			}
		} else if filter.Status != nil && operation.Status != *filter.Status {
			continue
		}
		if filter.ClaimedBy != nil && operation.ClaimedBy != *filter.ClaimedBy {
			continue
		}
		if filter.Source != nil && operation.Source != *filter.Source {
			continue
		}
		if filter.CreatedAfter != nil && operation.CreatedAt < *filter.CreatedAfter {
			continue
		}
		if filter.CreatedBefore != nil && operation.CreatedAt > *filter.CreatedBefore {
			continue
		}
		items = append(items, cloneAgentLifecycleOperationForTest(operation))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt == items[j].CreatedAt {
			return items[i].ID > items[j].ID
		}
		return items[i].CreatedAt > items[j].CreatedAt
	})
	return items
}

func cloneAgentLifecycleOperationForTest(operation *repository.AgentLifecycleOperation) *repository.AgentLifecycleOperation {
	if operation == nil {
		return nil
	}
	clone := *operation
	clone.RequestPayload = append(json.RawMessage(nil), operation.RequestPayload...)
	clone.ResultPayload = append(json.RawMessage(nil), operation.ResultPayload...)
	clone.ClaimedAt = cloneInt64Ptr(operation.ClaimedAt)
	clone.StartedAt = cloneInt64Ptr(operation.StartedAt)
	clone.FinishedAt = cloneInt64Ptr(operation.FinishedAt)
	clone.OperatorID = cloneInt64Ptr(operation.OperatorID)
	return &clone
}

func TestAgentLifecycleOperationServiceCreateClaimReportSuccess(t *testing.T) {
	repo := newMockAgentLifecycleOperationRepo()
	logRepo := &operationLogRepoStub{}
	audit := &captureSecurityRecorder{}
	svc := NewAgentLifecycleOperationService(repo, nil, NewOperationLogService(logRepo, nil), audit)
	operatorID := int64(7)

	operation, err := svc.Create(context.Background(), CreateAgentLifecycleOperationRequest{
		AgentHostID:    10,
		OperationType:  agentLifecycleOperationTypeAgentUpdate,
		RequestPayload: json.RawMessage(`{"target_version":"1.2.3","token":"secret-token"}`),
		OperatorID:     &operatorID,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if operation.Status != agentLifecycleOperationStatusPending || operation.Source != agentLifecycleOperationSourceAdmin {
		t.Fatalf("unexpected created operation: %+v", operation)
	}
	if strings.Contains(string(operation.RequestPayload), "secret-token") {
		t.Fatalf("expected request payload to be sanitized, got %s", string(operation.RequestPayload))
	}
	if len(logRepo.entries) != 1 || logRepo.entries[0].Scope != OperationLogScopeAgentOperation || logRepo.entries[0].Phase != "created" {
		t.Fatalf("expected created log, got %+v", logRepo.entries)
	}
	if len(audit.events) != 1 || audit.events[0].Kind != agentLifecycleOperationCreatedAuditKind || audit.events[0].ActorID != "7" {
		t.Fatalf("expected create audit, got %+v", audit.events)
	}

	claimed, err := svc.ClaimNext(context.Background(), ClaimAgentLifecycleOperationRequest{AgentHostID: 10, ClaimedBy: "worker-a", SupportedActions: []string{AgentLifecycleOperationTypeAgentUpdate}, Limit: 1})
	if err != nil {
		t.Fatalf("ClaimNext returned error: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != operation.ID || claimed[0].ClaimedBy != "worker-a" || claimed[0].StartedAt == nil {
		t.Fatalf("unexpected claimed operations: %+v", claimed)
	}
	if err := svc.Report(context.Background(), ReportAgentLifecycleOperationRequest{
		AgentHostID:   10,
		OperationID:   operation.ID,
		ClaimedBy:     "worker-a",
		EventType:     agentLifecycleOperationEventProgress,
		Phase:         "download",
		Payload:       json.RawMessage(`{"stage":"download"}`),
		SourceEventID: "event-progress",
		Sequence:      1,
	}); err != nil {
		t.Fatalf("Report progress returned error: %v", err)
	}
	if err := svc.Report(context.Background(), ReportAgentLifecycleOperationRequest{
		AgentHostID:   10,
		OperationID:   operation.ID,
		ClaimedBy:     "worker-a",
		EventType:     agentLifecycleOperationEventResult,
		Status:        agentLifecycleOperationStatusSuccess,
		Payload:       json.RawMessage(`{"ok":true}`),
		SourceEventID: "event-result",
		Sequence:      2,
		Terminal:      true,
	}); err != nil {
		t.Fatalf("Report result returned error: %v", err)
	}
	stored := repo.operations[operation.ID]
	if stored.Status != agentLifecycleOperationStatusSuccess || stored.FinishedAt == nil || string(stored.ResultPayload) != `{"ok":true}` {
		t.Fatalf("unexpected stored operation after report: %+v", stored)
	}
	if len(logRepo.entries) != 4 {
		t.Fatalf("expected create/claim/progress/result logs, got %d", len(logRepo.entries))
	}
}

func TestAgentLifecycleOperationServiceClaimNextRequiresSupportedActions(t *testing.T) {
	repo := newMockAgentLifecycleOperationRepo()
	repo.operations["op-update"] = &repository.AgentLifecycleOperation{ID: "op-update", AgentHostID: 10, OperationType: agentLifecycleOperationTypeAgentUpdate, Status: agentLifecycleOperationStatusPending, CreatedAt: 1}
	svc := NewAgentLifecycleOperationService(repo, nil, nil, nil)

	_, err := svc.ClaimNext(context.Background(), ClaimAgentLifecycleOperationRequest{AgentHostID: 10, ClaimedBy: "worker-a", Limit: 1})
	if !errors.Is(err, ErrAgentLifecycleOperationNotFound) {
		t.Fatalf("expected no claim without supported actions, got %v", err)
	}
	if repo.operations["op-update"].Status != agentLifecycleOperationStatusPending || repo.operations["op-update"].ClaimedBy != "" {
		t.Fatalf("unsupported empty action list must not mutate operation: %+v", repo.operations["op-update"])
	}
}

func TestAgentLifecycleOperationServiceClaimNextFiltersSupportedActions(t *testing.T) {
	repo := newMockAgentLifecycleOperationRepo()
	repo.operations["op-update"] = &repository.AgentLifecycleOperation{ID: "op-update", AgentHostID: 10, OperationType: agentLifecycleOperationTypeAgentUpdate, Status: agentLifecycleOperationStatusPending, CreatedAt: 1}
	repo.operations["op-cdn"] = &repository.AgentLifecycleOperation{ID: "op-cdn", AgentHostID: 10, OperationType: agentLifecycleOperationTypeCDNDeploySite, Status: agentLifecycleOperationStatusPending, CreatedAt: 2}
	svc := NewAgentLifecycleOperationService(repo, nil, nil, nil)

	claimed, err := svc.ClaimNext(context.Background(), ClaimAgentLifecycleOperationRequest{AgentHostID: 10, ClaimedBy: "worker-a", SupportedActions: []string{AgentLifecycleOperationTypeCDNDeploySite}, Limit: 1})
	if err != nil {
		t.Fatalf("ClaimNext returned error: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != "op-cdn" {
		t.Fatalf("expected only supported CDN operation to be claimed, got %+v", claimed)
	}
	if repo.operations["op-update"].Status != agentLifecycleOperationStatusPending || repo.operations["op-update"].ClaimedBy != "" {
		t.Fatalf("unsupported operation must stay pending, got %+v", repo.operations["op-update"])
	}
}

func TestAgentLifecycleOperationServiceReportTerminalDuplicateIsIdempotent(t *testing.T) {
	finishedAt := int64(100)
	repo := newMockAgentLifecycleOperationRepo()
	repo.operations["op-terminal"] = &repository.AgentLifecycleOperation{
		ID:            "op-terminal",
		AgentHostID:   10,
		OperationType: agentLifecycleOperationTypeAgentUpdate,
		Status:        agentLifecycleOperationStatusSuccess,
		ClaimedBy:     "worker-a",
		ResultPayload: json.RawMessage(`{"ok":true}`),
		FinishedAt:    &finishedAt,
		CreatedAt:     1,
	}
	logRepo := &operationLogRepoStub{}
	svc := NewAgentLifecycleOperationService(repo, nil, NewOperationLogService(logRepo, nil), nil)

	err := svc.Report(context.Background(), ReportAgentLifecycleOperationRequest{AgentHostID: 10, OperationID: "op-terminal", ClaimedBy: "worker-a", EventType: agentLifecycleOperationEventResult, Status: agentLifecycleOperationStatusSuccess, Payload: json.RawMessage(`{"ok":true}`), OccurredAt: 200, Terminal: true})
	if err != nil {
		t.Fatalf("duplicate terminal report should be idempotent, got %v", err)
	}
	stored := repo.operations["op-terminal"]
	if stored.FinishedAt == nil || *stored.FinishedAt != finishedAt || string(stored.ResultPayload) != `{"ok":true}` {
		t.Fatalf("duplicate terminal report must not mutate stored operation: %+v", stored)
	}
	if len(logRepo.entries) != 0 {
		t.Fatalf("duplicate terminal report must not append logs, got %+v", logRepo.entries)
	}

	err = svc.Report(context.Background(), ReportAgentLifecycleOperationRequest{AgentHostID: 10, OperationID: "op-terminal", ClaimedBy: "worker-a", EventType: agentLifecycleOperationEventResult, Status: agentLifecycleOperationStatusFailed, Payload: json.RawMessage(`{"ok":true}`), OccurredAt: 201, Terminal: true})
	if !errors.Is(err, ErrAgentLifecycleOperationInvalidRequest) {
		t.Fatalf("conflicting terminal status should be rejected, got %v", err)
	}
	err = svc.Report(context.Background(), ReportAgentLifecycleOperationRequest{AgentHostID: 10, OperationID: "op-terminal", ClaimedBy: "worker-a", EventType: agentLifecycleOperationEventResult, Status: agentLifecycleOperationStatusSuccess, Payload: json.RawMessage(`{"ok":false}`), OccurredAt: 202, Terminal: true})
	if !errors.Is(err, ErrAgentLifecycleOperationInvalidRequest) {
		t.Fatalf("conflicting terminal payload should be rejected, got %v", err)
	}
}

func TestAgentLifecycleOperationServiceTrafficResetUsesDedicatedScope(t *testing.T) {
	repo := newMockAgentLifecycleOperationRepo()
	logRepo := &operationLogRepoStub{}
	svc := NewAgentLifecycleOperationService(repo, nil, NewOperationLogService(logRepo, nil), nil)

	operation, err := svc.Create(context.Background(), CreateAgentLifecycleOperationRequest{AgentHostID: 11, OperationType: agentLifecycleOperationTypeTrafficReset})
	if err != nil {
		t.Fatalf("Create traffic reset returned error: %v", err)
	}
	if operationLogScopeForAgentLifecycleOperationType(operation.OperationType) != OperationLogScopeTrafficReset {
		t.Fatalf("unexpected scope for traffic reset")
	}
	if len(logRepo.entries) != 1 || logRepo.entries[0].Scope != OperationLogScopeTrafficReset {
		t.Fatalf("expected traffic reset log scope, got %+v", logRepo.entries)
	}
}

func TestAgentLifecycleOperationServiceInvalidTransitionAndOwnership(t *testing.T) {
	repo := newMockAgentLifecycleOperationRepo()
	repo.operations["op-1"] = &repository.AgentLifecycleOperation{ID: "op-1", AgentHostID: 1, OperationType: agentLifecycleOperationTypeAgentUpdate, Status: agentLifecycleOperationStatusSuccess, ClaimedBy: "worker-a", CreatedAt: 1}
	audit := &captureSecurityRecorder{}
	svc := NewAgentLifecycleOperationService(repo, nil, nil, audit)

	err := svc.Report(context.Background(), ReportAgentLifecycleOperationRequest{AgentHostID: 1, OperationID: "op-1", ClaimedBy: "worker-a", Status: agentLifecycleOperationStatusFailed, Terminal: true})
	if !errors.Is(err, ErrAgentLifecycleOperationInvalidRequest) {
		t.Fatalf("expected invalid transition, got %v", err)
	}
	repo.operations["op-2"] = &repository.AgentLifecycleOperation{ID: "op-2", AgentHostID: 1, OperationType: agentLifecycleOperationTypeAgentUpdate, Status: agentLifecycleOperationStatusClaimed, ClaimedBy: "worker-a", CreatedAt: 2}
	err = svc.Report(context.Background(), ReportAgentLifecycleOperationRequest{AgentHostID: 2, OperationID: "op-2", ClaimedBy: "worker-a", Status: agentLifecycleOperationStatusSuccess, Terminal: true})
	if !errors.Is(err, ErrAgentLifecycleOperationForbidden) {
		t.Fatalf("expected forbidden host mismatch, got %v", err)
	}
	err = svc.Report(context.Background(), ReportAgentLifecycleOperationRequest{AgentHostID: 1, OperationID: "op-2", ClaimedBy: "worker-b", Status: agentLifecycleOperationStatusSuccess, Terminal: true})
	if !errors.Is(err, ErrAgentLifecycleOperationForbidden) {
		t.Fatalf("expected forbidden claimer mismatch, got %v", err)
	}
	if len(audit.events) != 2 || audit.events[0].Kind != agentLifecycleOperationForbiddenAuditKind {
		t.Fatalf("expected forbidden audits, got %+v", audit.events)
	}
}

func TestAgentLifecycleOperationServiceTerminalStatuses(t *testing.T) {
	for _, statusValue := range []string{agentLifecycleOperationStatusQueueFull, agentLifecycleOperationStatusUnsupportedAction, agentLifecycleOperationStatusTimeout} {
		t.Run(statusValue, func(t *testing.T) {
			repo := newMockAgentLifecycleOperationRepo()
			repo.operations["op-"+statusValue] = &repository.AgentLifecycleOperation{ID: "op-" + statusValue, AgentHostID: 3, OperationType: agentLifecycleOperationTypeAgentUpdate, Status: agentLifecycleOperationStatusClaimed, ClaimedBy: "worker-a", CreatedAt: 1}
			logRepo := &operationLogRepoStub{}
			svc := NewAgentLifecycleOperationService(repo, nil, NewOperationLogService(logRepo, nil), nil)

			err := svc.Report(context.Background(), ReportAgentLifecycleOperationRequest{AgentHostID: 3, OperationID: "op-" + statusValue, ClaimedBy: "worker-a", EventType: agentLifecycleOperationEventResult, Status: statusValue, Payload: json.RawMessage(`{"reason":"` + statusValue + `"}`), Terminal: true})
			if err != nil {
				t.Fatalf("Report returned error: %v", err)
			}
			if repo.operations["op-"+statusValue].Status != statusValue || repo.operations["op-"+statusValue].FinishedAt == nil {
				t.Fatalf("expected terminal status %q, got %+v", statusValue, repo.operations["op-"+statusValue])
			}
			if len(logRepo.entries) != 1 || logRepo.entries[0].Level == "" {
				t.Fatalf("expected terminal operation log, got %+v", logRepo.entries)
			}
		})
	}
}

func TestAgentLifecycleOperationServiceCreateUsesBusyGuard(t *testing.T) {
	repo := newMockAgentLifecycleOperationRepo()
	ops := newMockCoreOperationRepo()
	ops.operations["core-blocking"] = &repository.CoreOperation{ID: "core-blocking", AgentHostID: 9, OperationType: coreOperationTypeInstall, Status: coreOperationStatusPending, CreatedAt: 1}
	svc := NewAgentLifecycleOperationService(repo, NewAgentOperationGuard(ops, &mockApplyRunRepo{}, nil, repo), nil, nil)

	// queue-mode：存在进行中核心操作不拒绝，生命周期操作排队放行。
	op, err := svc.Create(context.Background(), CreateAgentLifecycleOperationRequest{AgentHostID: 9, OperationType: agentLifecycleOperationTypeAgentUpdate})
	if err != nil {
		t.Fatalf("queue-mode: expected create to succeed, got %v", err)
	}
	if op == nil {
		t.Fatalf("expected lifecycle operation to be created")
	}
	if len(repo.operations) != 1 {
		t.Fatalf("expected 1 lifecycle operation, got %d", len(repo.operations))
	}

	_, err = svc.Create(context.Background(), CreateAgentLifecycleOperationRequest{AgentHostID: 9, OperationType: agentLifecycleOperationTypeAgentUpdateCheck})
	if err != nil {
		t.Fatalf("expected non-destructive update check to be allowed, got %v", err)
	}
}

func TestAgentOperationGuardBlocksActiveLifecycleOperation(t *testing.T) {
	repo := newMockAgentLifecycleOperationRepo()
	repo.operations["agent-op-active"] = &repository.AgentLifecycleOperation{ID: "agent-op-active", AgentHostID: 12, OperationType: agentLifecycleOperationTypeAgentUpdate, Status: agentLifecycleOperationStatusPending, CreatedAt: 1}
	repo.operations["agent-op-check"] = &repository.AgentLifecycleOperation{ID: "agent-op-check", AgentHostID: 12, OperationType: agentLifecycleOperationTypeAgentUpdateCheck, Status: agentLifecycleOperationStatusPending, CreatedAt: 2}
	guard := NewAgentOperationGuard(newMockCoreOperationRepo(), &mockApplyRunRepo{}, nil, repo)

	// queue-mode：进行中的生命周期操作不拒绝，排队放行。
	if err := guard.CheckIdle(context.Background(), AgentOperationGuardRequest{AgentHostID: 12, Scope: OperationLogScopeCoreOperation, OperationType: coreOperationTypeSwitch}); err != nil {
		t.Fatalf("queue-mode: expected nil (enqueue), got %v", err)
	}

	if err := guard.CheckIdle(context.Background(), AgentOperationGuardRequest{AgentHostID: 12, Scope: OperationLogScopeAgentOperation, OperationType: agentLifecycleOperationTypeAgentUpdate, TargetID: "agent-op-active"}); err != nil {
		t.Fatalf("expected same lifecycle operation to be allowed, got %v", err)
	}
}

func TestAgentOperationGuardIgnoresStaleAndNonDestructiveLifecycleBlockers(t *testing.T) {
	staleClaimedAt := time.Now().Add(-agentLifecycleOperationClaimTimeout - time.Minute).Unix()
	repo := newMockAgentLifecycleOperationRepo()
	repo.operations["agent-op-stale"] = &repository.AgentLifecycleOperation{ID: "agent-op-stale", AgentHostID: 13, OperationType: agentLifecycleOperationTypeAgentUpdate, Status: agentLifecycleOperationStatusClaimed, ClaimedAt: &staleClaimedAt, CreatedAt: 1}
	repo.operations["agent-op-check"] = &repository.AgentLifecycleOperation{ID: "agent-op-check", AgentHostID: 13, OperationType: agentLifecycleOperationTypeAgentUpdateCheck, Status: agentLifecycleOperationStatusPending, CreatedAt: 2}
	guard := NewAgentOperationGuard(newMockCoreOperationRepo(), &mockApplyRunRepo{}, nil, repo)

	if err := guard.CheckIdle(context.Background(), AgentOperationGuardRequest{AgentHostID: 13, Scope: OperationLogScopeCoreOperation, OperationType: coreOperationTypeSwitch}); err != nil {
		t.Fatalf("expected stale and non-destructive lifecycle operations to be ignored, got %v", err)
	}
}
