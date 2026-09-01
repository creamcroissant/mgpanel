package service

import (
	"context"
	"testing"
	"time"

	"github.com/creamcroissant/xboard/internal/repository"
	"github.com/creamcroissant/xboard/internal/security"
)

type captureSecurityRecorder struct {
	events []security.Event
}

func (r *captureSecurityRecorder) Record(ctx context.Context, event security.Event) {
	r.events = append(r.events, event)
}

func TestAgentOperationGuardBlocksActiveCoreOperation(t *testing.T) {
	ops := newMockCoreOperationRepo()
	ops.operations["op-active"] = &repository.CoreOperation{ID: "op-active", AgentHostID: 10, OperationType: coreOperationTypeInstall, Status: coreOperationStatusPending, CreatedAt: 100}
	audit := &captureSecurityRecorder{}
	guard := NewAgentOperationGuard(ops, &mockApplyRunRepo{}, audit)

	// queue-mode：存在进行中操作时不拒绝（排队），但记录阻塞审计。
	err := guard.CheckIdle(context.Background(), AgentOperationGuardRequest{AgentHostID: 10, Scope: OperationLogScopeApplyRun, OperationType: agentOperationTypeApply, OperatorID: int64PtrForTest(88)})
	if err != nil {
		t.Fatalf("queue-mode: expected nil (enqueue), got %v", err)
	}
	if len(audit.events) != 1 || audit.events[0].Kind != agentOperationBlockedAuditKind || audit.events[0].ActorID != "88" {
		t.Fatalf("expected one blocked audit event, got %+v", audit.events)
	}
}

func TestAgentOperationGuardBlocksActiveApplyRun(t *testing.T) {
	applyRuns := &mockApplyRunRepo{listRuns: []*repository.ApplyRun{{RunID: "run-active", AgentHostID: 20, Status: applyRunStatusApplying, StartedAt: 200, CreatedAt: time.Now().Unix()}}}
	guard := NewAgentOperationGuard(newMockCoreOperationRepo(), applyRuns, nil)

	// queue-mode：apply run 进行中不拒绝，排队放行。
	if err := guard.CheckIdle(context.Background(), AgentOperationGuardRequest{AgentHostID: 20, Scope: OperationLogScopeCoreOperation, OperationType: coreOperationTypeSwitch}); err != nil {
		t.Fatalf("queue-mode: expected nil (enqueue), got %v", err)
	}
}

func TestAgentOperationGuardIgnoresTerminalAndStaleCoreBlockers(t *testing.T) {
	staleClaimedAt := time.Now().Add(-coreOperationClaimTimeout - time.Minute).Unix()
	ops := newMockCoreOperationRepo()
	ops.operations["op-stale"] = &repository.CoreOperation{ID: "op-stale", AgentHostID: 30, OperationType: coreOperationTypeInstall, Status: coreOperationStatusClaimed, ClaimedAt: &staleClaimedAt, CreatedAt: 100}
	ops.operations["op-done"] = &repository.CoreOperation{ID: "op-done", AgentHostID: 30, OperationType: coreOperationTypeSwitch, Status: coreOperationStatusCompleted, CreatedAt: 200}
	applyRuns := &mockApplyRunRepo{listRuns: []*repository.ApplyRun{{RunID: "run-done", AgentHostID: 30, Status: applyRunStatusSuccess, StartedAt: 300}}}
	guard := NewAgentOperationGuard(ops, applyRuns, nil)

	if err := guard.CheckIdle(context.Background(), AgentOperationGuardRequest{AgentHostID: 30, Scope: OperationLogScopeApplyRun, OperationType: agentOperationTypeApply}); err != nil {
		t.Fatalf("expected idle, got %v", err)
	}
}

func TestAgentOperationGuardAllowsSameApplyRunReuseButBlocksOtherApplyRun(t *testing.T) {
	applyRuns := &mockApplyRunRepo{listRuns: []*repository.ApplyRun{
		{RunID: "run-reuse", AgentHostID: 40, Status: applyRunStatusPending, StartedAt: 100, CreatedAt: time.Now().Unix()},
	}}
	guard := NewAgentOperationGuard(newMockCoreOperationRepo(), applyRuns, nil)
	if err := guard.CheckIdle(context.Background(), AgentOperationGuardRequest{AgentHostID: 40, Scope: OperationLogScopeApplyRun, OperationType: agentOperationTypeApply, TargetID: "run-reuse"}); err != nil {
		t.Fatalf("expected same run reuse to be allowed, got %v", err)
	}

	// queue-mode：其他 apply run 进行中不拒绝，排队放行。
	applyRuns.listRuns = append(applyRuns.listRuns, &repository.ApplyRun{RunID: "run-other", AgentHostID: 40, Status: applyRunStatusApplying, StartedAt: 200, CreatedAt: time.Now().Unix()})
	if err := guard.CheckIdle(context.Background(), AgentOperationGuardRequest{AgentHostID: 40, Scope: OperationLogScopeApplyRun, OperationType: agentOperationTypeApply, TargetID: "run-reuse"}); err != nil {
		t.Fatalf("queue-mode: expected enqueue allowed, got %v", err)
	}
}

func TestCoreOperationServiceCreateUsesGuard(t *testing.T) {
	ops := newMockCoreOperationRepo()
	applyRuns := &mockApplyRunRepo{listRuns: []*repository.ApplyRun{{RunID: "run-blocking", AgentHostID: 50, Status: applyRunStatusApplying, StartedAt: 100, CreatedAt: time.Now().Unix()}}}
	svc := NewCoreOperationService(ops, NewAgentOperationGuard(ops, applyRuns, nil))

	// queue-mode：guard 不拒绝并发，操作排队串行执行（见 CheckIdle 迁移）。
	op, err := svc.Create(context.Background(), CreateCoreOperationRequest{AgentHostID: 50, OperationType: coreOperationTypeInstall, CoreType: "sing-box"})
	if err != nil {
		t.Fatalf("queue-mode: expected create to succeed, got %v", err)
	}
	if op == nil {
		t.Fatalf("expected core operation to be created")
	}
	if len(ops.operations) != 1 {
		t.Fatalf("expected 1 core operation, got %d", len(ops.operations))
	}
}

func TestApplyOrchestratorServiceCreateUsesGuard(t *testing.T) {
	ops := newMockCoreOperationRepo()
	ops.operations["op-blocking"] = &repository.CoreOperation{ID: "op-blocking", AgentHostID: 60, OperationType: coreOperationTypeSwitch, Status: coreOperationStatusPending, CreatedAt: 100}
	applyRuns := &mockApplyRunRepo{}
	svc := NewApplyOrchestratorServiceWithGuard(&mockApplyDesiredArtifactRepo{}, applyRuns, nil, NewAgentOperationGuard(ops, applyRuns, nil), nil)

	// queue-mode：guard 不拒绝并发，apply run 排队串行执行。
	run, err := svc.CreateApplyRun(context.Background(), CreateApplyRunRequest{AgentHostID: 60, CoreType: "sing-box", TargetRevision: 9})
	if err != nil {
		t.Fatalf("queue-mode: expected create to succeed, got %v", err)
	}
	if run == nil {
		t.Fatalf("expected apply run to be created")
	}
	if len(applyRuns.createdRuns) != 1 {
		t.Fatalf("expected 1 apply run, got %d", len(applyRuns.createdRuns))
	}
}

func int64PtrForTest(value int64) *int64 {
	return &value
}
