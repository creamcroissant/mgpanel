package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/creamcroissant/xboard/internal/agent/command"
	agentv1 "github.com/creamcroissant/xboard/pkg/pb/agent/v1"
)

const (
	agentCommandQueueCapacity       = 8
	agentCommandQueueWorkers        = 1
	agentCommandTaskTimeout         = 30 * time.Minute
	agentCommandPollLimit           = 4
	agentCommandWorkerID            = "agent-command-queue"
	agentCommandActionCoreOperation = "internal_core_operation"
	agentCommandActionConfigApply   = "internal_config_apply"
)

type agentCommandClient interface {
	GetAgentCommands(ctx context.Context, req *agentv1.GetAgentCommandsRequest) (*agentv1.GetAgentCommandsResponse, error)
	ReportAgentCommand(ctx context.Context, report *agentv1.ReportAgentCommandRequest) (*agentv1.ReportAgentCommandResponse, error)
}

type agentCommandReporter struct {
	client   agentCommandClient
	queue    *command.Queue
	workerID string
}

func newAgentCommandQueue(client agentCommandClient) (*command.Queue, error) {
	reporter := &agentCommandReporter{client: client, workerID: agentCommandWorkerID}
	queue, err := command.NewQueue(command.Options{
		Capacity:    agentCommandQueueCapacity,
		Workers:     agentCommandQueueWorkers,
		TaskTimeout: agentCommandTaskTimeout,
		Reporter:    reporter,
	})
	if err != nil {
		return nil, err
	}
	reporter.queue = queue
	return queue, nil
}

func (a *Agent) syncAgentCommands(ctx context.Context) {
	if a == nil || a.agentCommands == nil || a.commandQueue == nil {
		return
	}
	actions := a.commandQueue.SupportedActions()
	if len(actions) == 0 {
		slog.Debug("skip agent command sync: no supported actions")
		return
	}
	stats := a.commandQueue.Stats()
	if stats.Available <= 0 {
		slog.Debug("skip agent command sync: command queue full", "queued", stats.Queued, "inflight", stats.Inflight)
		return
	}
	limit := stats.Available
	if limit > agentCommandPollLimit {
		limit = agentCommandPollLimit
	}
	resp, err := a.agentCommands.GetAgentCommands(ctx, &agentv1.GetAgentCommandsRequest{
		SupportedActions: actions,
		QueueStats:       agentCommandQueueStatsToProto(stats),
		Limit:            limit,
		WorkerId:         agentCommandWorkerID,
		RequestedAt:      time.Now().Unix(),
	})
	if err != nil {
		slog.Error("Failed to fetch agent commands", "error", err)
		return
	}
	if resp != nil && !resp.GetSuccess() {
		slog.Warn("panel rejected agent command fetch", "message", resp.GetMessage())
		return
	}
	for _, item := range resp.GetCommands() {
		task := agentCommandTaskFromProto(item)
		if strings.TrimSpace(task.ID) == "" {
			continue
		}
		if err := a.commandQueue.Submit(ctx, task); err != nil {
			slog.Warn("agent command rejected by local queue", "command_id", task.ID, "operation_type", task.OperationType, "error", err)
		}
	}
}

func (r *agentCommandReporter) Report(ctx context.Context, event command.Event) error {
	if isInternalAgentCommandAction(event.OperationType) {
		return nil
	}
	if r == nil || r.client == nil {
		return nil
	}
	resp, err := r.client.ReportAgentCommand(ctx, &agentv1.ReportAgentCommandRequest{
		Events:     []*agentv1.AgentCommandEvent{agentCommandEventToProto(event)},
		QueueStats: r.queueStats(),
		WorkerId:   r.workerID,
	})
	if err != nil {
		return err
	}
	if resp != nil && !resp.GetSuccess() {
		return fmt.Errorf("agent command report rejected: %s", resp.GetMessage())
	}
	return nil
}

func (r *agentCommandReporter) queueStats() *agentv1.AgentCommandQueueStats {
	if r == nil || r.queue == nil {
		return nil
	}
	return agentCommandQueueStatsToProto(r.queue.Stats())
}

func isInternalAgentCommandAction(operationType string) bool {
	switch strings.TrimSpace(operationType) {
	case agentCommandActionCoreOperation, agentCommandActionConfigApply:
		return true
	case agentCommandActionSetRoutingTable:
		return false
	default:
		return false
	}
}

func (a *Agent) commandQueueStatsProto() *agentv1.AgentCommandQueueStats {
	if a == nil || a.commandQueue == nil {
		return nil
	}
	return agentCommandQueueStatsToProto(a.commandQueue.Stats())
}

func agentCommandTaskFromProto(item *agentv1.AgentCommand) command.Task {
	if item == nil {
		return command.Task{}
	}
	task := command.Task{
		ID:             item.GetId(),
		OperationType:  item.GetOperationType(),
		RequestPayload: append([]byte(nil), item.GetRequestPayload()...),
		Source:         item.GetSource(),
		CorrelationID:  item.GetCorrelationId(),
		CreatedAt:      item.GetCreatedAt(),
		UpdatedAt:      item.GetUpdatedAt(),
	}
	if timeout := item.GetTimeoutSeconds(); timeout > 0 {
		task.Timeout = time.Duration(timeout) * time.Second
	}
	return task
}

func agentCommandEventToProto(event command.Event) *agentv1.AgentCommandEvent {
	return &agentv1.AgentCommandEvent{
		CommandId:    event.CommandID,
		EventType:    event.EventType,
		Status:       event.Status,
		Phase:        event.Phase,
		Level:        event.Level,
		Message:      event.Message,
		PayloadJson:  append([]byte(nil), event.Payload...),
		ErrorMessage: event.ErrorMessage,
		OccurredAt:   event.OccurredAt,
		Sequence:     event.Sequence,
		Terminal:     event.Terminal,
	}
}

func agentCommandQueueStatsToProto(stats command.Stats) *agentv1.AgentCommandQueueStats {
	return &agentv1.AgentCommandQueueStats{
		Capacity:         stats.Capacity,
		Queued:           stats.Queued,
		Inflight:         stats.Inflight,
		Workers:          stats.Workers,
		Available:        stats.Available,
		ActiveCommandIds: append([]string(nil), stats.ActiveCommandIDs...),
		UpdatedAt:        stats.UpdatedAt,
	}
}
