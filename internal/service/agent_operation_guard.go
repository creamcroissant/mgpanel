package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/creamcroissant/xboard/internal/repository"
	"github.com/creamcroissant/xboard/internal/security"
)

var (
	ErrAgentOperationBusy                = errors.New("service: agent operation busy / 节点存在进行中的重操作")
	ErrAgentOperationGuardInvalidRequest = errors.New("service: invalid agent operation guard request / 节点操作保护请求无效")
	ErrAgentOperationGuardNotConfigured  = errors.New("service: agent operation guard not configured / 节点操作保护未配置")
)

const (
	agentOperationTypeApply        = "apply"
	agentOperationBlockedAuditKind = "admin.agent_operation.blocked"
)

type AgentOperationGuard interface {
	CheckIdle(ctx context.Context, req AgentOperationGuardRequest) error
}

type AgentOperationGuardRequest struct {
	AgentHostID   int64
	Scope         string
	OperationType string
	TargetID      string
	OperatorID    *int64
}

type AgentOperationBusyError struct {
	Blocker repository.OperationBlocker
}

func (e *AgentOperationBusyError) Error() string {
	if e == nil {
		return ErrAgentOperationBusy.Error()
	}
	return fmt.Sprintf("%s: blocker scope=%s id=%s status=%s", ErrAgentOperationBusy.Error(), e.Blocker.Scope, e.Blocker.ID, e.Blocker.Status)
}

func (e *AgentOperationBusyError) Unwrap() error {
	return ErrAgentOperationBusy
}

func OperationBlockerFromError(err error) (*repository.OperationBlocker, bool) {
	var busy *AgentOperationBusyError
	if !errors.As(err, &busy) || busy == nil {
		return nil, false
	}
	blocker := busy.Blocker
	return &blocker, true
}

type agentOperationGuard struct {
	coreOperations      repository.CoreOperationRepository
	applyRuns           repository.ApplyRunRepository
	lifecycleOperations repository.AgentLifecycleOperationRepository
	audit               security.Recorder
}

func NewAgentOperationGuard(coreOperations repository.CoreOperationRepository, applyRuns repository.ApplyRunRepository, audit security.Recorder, lifecycleOperations ...repository.AgentLifecycleOperationRepository) AgentOperationGuard {
	var lifecycle repository.AgentLifecycleOperationRepository
	if len(lifecycleOperations) > 0 {
		lifecycle = lifecycleOperations[0]
	}
	return &agentOperationGuard{coreOperations: coreOperations, applyRuns: applyRuns, lifecycleOperations: lifecycle, audit: audit}
}

func (g *agentOperationGuard) CheckIdle(ctx context.Context, req AgentOperationGuardRequest) error {
	if g == nil || (g.coreOperations == nil && g.applyRuns == nil && g.lifecycleOperations == nil) {
		return ErrAgentOperationGuardNotConfigured
	}
	if req.AgentHostID <= 0 {
		return ErrAgentOperationGuardInvalidRequest
	}

	blocker, err := g.findBlocker(ctx, req)
	if err != nil {
		return err
	}
	if blocker == nil {
		return nil
	}
	g.recordBlockedAttempt(ctx, req, *blocker)
	return &AgentOperationBusyError{Blocker: *blocker}
}

func (g *agentOperationGuard) findBlocker(ctx context.Context, req AgentOperationGuardRequest) (*repository.OperationBlocker, error) {
	if g.coreOperations != nil {
		blocker, err := g.findCoreOperationBlocker(ctx, req)
		if err != nil {
			return nil, err
		}
		if blocker != nil {
			return blocker, nil
		}
	}
	if g.applyRuns != nil {
		blocker, err := g.findApplyRunBlocker(ctx, req)
		if err != nil {
			return nil, err
		}
		if blocker != nil {
			return blocker, nil
		}
	}
	if g.lifecycleOperations != nil {
		blocker, err := g.findLifecycleOperationBlocker(ctx, req)
		if err != nil {
			return nil, err
		}
		if blocker != nil {
			return blocker, nil
		}
	}
	return nil, nil
}

func (g *agentOperationGuard) findCoreOperationBlocker(ctx context.Context, req AgentOperationGuardRequest) (*repository.OperationBlocker, error) {
	filter := repository.CoreOperationFilter{
		AgentHostID: &req.AgentHostID,
		Statuses: []string{
			coreOperationStatusPending,
			coreOperationStatusClaimed,
			coreOperationStatusInProgress,
		},
		Limit:  1000,
		Offset: 0,
	}
	operations, err := g.coreOperations.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	for _, op := range operations {
		if !isActiveCoreOperationBlocker(op, req.AgentHostID, now) {
			continue
		}
		blocker := repository.OperationBlocker{
			Scope:         OperationLogScopeCoreOperation,
			ID:            op.ID,
			AgentHostID:   op.AgentHostID,
			OperationType: strings.TrimSpace(op.OperationType),
			Status:        strings.TrimSpace(op.Status),
			CreatedAt:     op.CreatedAt,
		}
		if !isSameRequestedOperation(req, blocker) {
			return &blocker, nil
		}
	}
	return nil, nil
}

func (g *agentOperationGuard) findApplyRunBlocker(ctx context.Context, req AgentOperationGuardRequest) (*repository.OperationBlocker, error) {
	filter := repository.ApplyRunFilter{
		AgentHostID: &req.AgentHostID,
		Statuses:    []string{applyRunStatusPending, applyRunStatusApplying},
		Limit:       1000,
		Offset:      0,
	}
	runs, err := g.applyRuns.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	for _, run := range runs {
		if !isActiveApplyRunBlocker(run, req.AgentHostID) {
			continue
		}
		blocker := repository.OperationBlocker{
			Scope:         OperationLogScopeApplyRun,
			ID:            run.RunID,
			AgentHostID:   run.AgentHostID,
			OperationType: agentOperationTypeApply,
			Status:        strings.TrimSpace(run.Status),
			CreatedAt:     run.StartedAt,
		}
		if !isSameRequestedOperation(req, blocker) {
			return &blocker, nil
		}
	}
	return nil, nil
}

func (g *agentOperationGuard) findLifecycleOperationBlocker(ctx context.Context, req AgentOperationGuardRequest) (*repository.OperationBlocker, error) {
	filter := repository.AgentLifecycleOperationFilter{
		AgentHostID: &req.AgentHostID,
		Statuses: []string{
			agentLifecycleOperationStatusPending,
			agentLifecycleOperationStatusClaimed,
			agentLifecycleOperationStatusInProgress,
		},
		Limit:  1000,
		Offset: 0,
	}
	operations, err := g.lifecycleOperations.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	for _, op := range operations {
		if !isActiveAgentLifecycleOperationBlocker(op, req.AgentHostID, now) {
			continue
		}
		blocker := repository.OperationBlocker{
			Scope:         operationLogScopeForAgentLifecycleOperationType(op.OperationType),
			ID:            op.ID,
			AgentHostID:   op.AgentHostID,
			OperationType: strings.TrimSpace(op.OperationType),
			Status:        strings.TrimSpace(op.Status),
			CreatedAt:     op.CreatedAt,
		}
		if !isSameRequestedOperation(req, blocker) {
			return &blocker, nil
		}
	}
	return nil, nil
}

func isSameRequestedOperation(req AgentOperationGuardRequest, blocker repository.OperationBlocker) bool {
	return strings.TrimSpace(req.Scope) == blocker.Scope && strings.TrimSpace(req.TargetID) != "" && strings.TrimSpace(req.TargetID) == blocker.ID
}

func isActiveCoreOperationBlocker(op *repository.CoreOperation, agentHostID int64, now int64) bool {
	if op == nil || op.AgentHostID != agentHostID {
		return false
	}
	switch strings.TrimSpace(op.Status) {
	case coreOperationStatusPending, coreOperationStatusInProgress:
		return true
	case coreOperationStatusClaimed:
		if op.ClaimedAt != nil && *op.ClaimedAt <= now-int64(coreOperationClaimTimeout/time.Second) {
			return false
		}
		return true
	default:
		return false
	}
}

func isActiveApplyRunBlocker(run *repository.ApplyRun, agentHostID int64) bool {
	if run == nil || run.AgentHostID != agentHostID {
		return false
	}
	switch strings.TrimSpace(run.Status) {
	case applyRunStatusPending, applyRunStatusApplying:
		startTime := run.CreatedAt
		if startTime == 0 {
			startTime = run.StartedAt
		}
		if startTime > 0 && startTime <= time.Now().Unix()-int64(applyRunClaimTimeout/time.Second) {
			return false
		}
		return true
	default:
		return false
	}
}
func isActiveAgentLifecycleOperationBlocker(op *repository.AgentLifecycleOperation, agentHostID int64, now int64) bool {
	if op == nil || op.AgentHostID != agentHostID || !isDestructiveAgentLifecycleOperationType(op.OperationType) {
		return false
	}
	switch strings.TrimSpace(op.Status) {
	case agentLifecycleOperationStatusPending, agentLifecycleOperationStatusInProgress:
		return true
	case agentLifecycleOperationStatusClaimed:
		if op.ClaimedAt != nil && *op.ClaimedAt <= now-int64(agentLifecycleOperationClaimTimeout/time.Second) {
			return false
		}
		return true
	default:
		return false
	}
}

func (g *agentOperationGuard) recordBlockedAttempt(ctx context.Context, req AgentOperationGuardRequest, blocker repository.OperationBlocker) {
	if g == nil || g.audit == nil {
		return
	}
	actorID := "system"
	if req.OperatorID != nil && *req.OperatorID > 0 {
		actorID = strconv.FormatInt(*req.OperatorID, 10)
	}
	g.audit.Record(ctx, security.Event{
		Kind:    agentOperationBlockedAuditKind,
		ActorID: actorID,
		Metadata: map[string]any{
			"agent_host_id":            req.AgentHostID,
			"requested_scope":          strings.TrimSpace(req.Scope),
			"requested_operation_type": strings.TrimSpace(req.OperationType),
			"requested_target_id":      strings.TrimSpace(req.TargetID),
			"blocker_scope":            blocker.Scope,
			"blocker_id":               blocker.ID,
			"blocker_operation_type":   blocker.OperationType,
			"blocker_status":           blocker.Status,
			"blocker_created_at":       blocker.CreatedAt,
		},
	})
}
