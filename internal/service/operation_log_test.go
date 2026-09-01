package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type operationLogRepoStub struct {
	mu      sync.Mutex
	entries []*repository.OperationLogEntry
}

func (r *operationLogRepoStub) Append(ctx context.Context, entry *repository.OperationLogEntry) (*repository.OperationLogEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry.SourceEventID != "" {
		for _, existing := range r.entries {
			if existing.Scope == entry.Scope && existing.TargetID == entry.TargetID && existing.SourceEventID == entry.SourceEventID {
				return cloneRepositoryOperationLog(existing), nil
			}
		}
	}
	stored := cloneRepositoryOperationLog(entry)
	stored.ID = int64(len(r.entries) + 1)
	r.entries = append(r.entries, stored)
	return cloneRepositoryOperationLog(stored), nil
}

func (r *operationLogRepoStub) List(ctx context.Context, filter repository.OperationLogFilter) ([]*repository.OperationLogEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	matched := r.filterLocked(filter)
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= len(matched) {
		return []*repository.OperationLogEntry{}, nil
	}
	limit := filter.Limit
	if limit <= 0 || offset+limit > len(matched) {
		limit = len(matched) - offset
	}
	out := make([]*repository.OperationLogEntry, 0, limit)
	for _, entry := range matched[offset : offset+limit] {
		out = append(out, cloneRepositoryOperationLog(entry))
	}
	return out, nil
}

func (r *operationLogRepoStub) DeleteOlderThan(ctx context.Context, days int) (int64, error) {
	return 0, nil
}

func (r *operationLogRepoStub) Count(ctx context.Context, filter repository.OperationLogFilter) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return int64(len(r.filterLocked(filter))), nil
}

func (r *operationLogRepoStub) filterLocked(filter repository.OperationLogFilter) []*repository.OperationLogEntry {
	out := make([]*repository.OperationLogEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		if filter.Scope != nil && entry.Scope != *filter.Scope {
			continue
		}
		if filter.TargetID != nil && entry.TargetID != *filter.TargetID {
			continue
		}
		if filter.AgentHostID != nil && entry.AgentHostID != *filter.AgentHostID {
			continue
		}
		if filter.AfterID != nil && entry.ID <= *filter.AfterID {
			continue
		}
		if filter.Level != nil && entry.Level != *filter.Level {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func cloneRepositoryOperationLog(entry *repository.OperationLogEntry) *repository.OperationLogEntry {
	if entry == nil {
		return nil
	}
	clone := *entry
	clone.Payload = append(json.RawMessage(nil), entry.Payload...)
	return &clone
}

func TestOperationLogServiceAppendSanitizesAndPreservesDuplicatePhases(t *testing.T) {
	repo := &operationLogRepoStub{}
	svc := NewOperationLogService(repo, nil)
	ctx := context.Background()

	first, err := svc.Append(ctx, AppendOperationLogRequest{
		Scope:         OperationLogScopeCoreOperation,
		TargetID:      "op-1",
		AgentHostID:   7,
		Sequence:      1,
		Phase:         "download",
		Level:         OperationLogLevelInfo,
		Message:       `download token=abc Authorization Bearer secret-token`,
		Payload:       json.RawMessage(`{"token":"abc","nested":{"password":"pw"},"note":"api_key=raw"}`),
		SourceEventID: "event-1",
		ReportedAt:    100,
	})
	if err != nil {
		t.Fatalf("append first log: %v", err)
	}
	if strings.Contains(first.Message, "abc") || strings.Contains(first.Message, "secret-token") {
		t.Fatalf("expected message to be sanitized, got %q", first.Message)
	}
	var payload map[string]any
	if err := json.Unmarshal(first.Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["token"] != "[REDACTED]" {
		t.Fatalf("expected token to be redacted, got %#v", payload["token"])
	}
	nested := payload["nested"].(map[string]any)
	if nested["password"] != "[REDACTED]" {
		t.Fatalf("expected password to be redacted, got %#v", nested["password"])
	}
	if strings.Contains(payload["note"].(string), "raw") {
		t.Fatalf("expected sensitive text in payload string to be redacted, got %#v", payload["note"])
	}

	duplicateSource, err := svc.Append(ctx, AppendOperationLogRequest{
		Scope:         OperationLogScopeCoreOperation,
		TargetID:      "op-1",
		AgentHostID:   7,
		Sequence:      99,
		Phase:         "changed",
		Level:         OperationLogLevelError,
		Message:       "should not overwrite",
		SourceEventID: "event-1",
	})
	if err != nil {
		t.Fatalf("append duplicate source event: %v", err)
	}
	if duplicateSource.ID != first.ID || duplicateSource.Phase != first.Phase || duplicateSource.Message != first.Message {
		t.Fatalf("expected duplicate source event to return original entry, got %+v", duplicateSource)
	}

	second, err := svc.Append(ctx, AppendOperationLogRequest{
		Scope:       OperationLogScopeCoreOperation,
		TargetID:    "op-1",
		AgentHostID: 7,
		Sequence:    1,
		Phase:       "download",
		Level:       OperationLogLevelWarn,
		Message:     "same phase retained",
	})
	if err != nil {
		t.Fatalf("append duplicate phase: %v", err)
	}
	result, err := svc.List(ctx, ListOperationLogsRequest{Scope: OperationLogScopeCoreOperation, TargetID: "op-1", Limit: 10})
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if result.Total != 2 || len(result.Items) != 2 {
		t.Fatalf("expected two persisted logs, got total=%d len=%d", result.Total, len(result.Items))
	}
	if result.Items[0].ID != first.ID || result.Items[1].ID != second.ID {
		t.Fatalf("expected logs ordered by id, got %d then %d", result.Items[0].ID, result.Items[1].ID)
	}
}

func TestOperationLogServiceSubscribePublishesMatchingTargetOnly(t *testing.T) {
	svc := NewOperationLogService(&operationLogRepoStub{}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	subscription, err := svc.Subscribe(ctx, SubscribeOperationLogsRequest{Scope: OperationLogScopeApplyRun, TargetID: "run-1"})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer subscription.Close()

	if _, err := svc.Append(ctx, AppendOperationLogRequest{Scope: OperationLogScopeApplyRun, TargetID: "run-2", AgentHostID: 8, Phase: "start"}); err != nil {
		t.Fatalf("append other target: %v", err)
	}
	matching, err := svc.Append(ctx, AppendOperationLogRequest{Scope: OperationLogScopeApplyRun, TargetID: "run-1", AgentHostID: 8, Phase: "start"})
	if err != nil {
		t.Fatalf("append matching target: %v", err)
	}

	select {
	case event := <-subscription.Events:
		if event.ID != matching.ID || event.TargetID != "run-1" {
			t.Fatalf("unexpected event: %+v", event)
		}
	default:
		t.Fatalf("expected matching event")
	}
}

func TestOperationLogServiceRejectsInvalidRequests(t *testing.T) {
	svc := NewOperationLogService(&operationLogRepoStub{}, nil)
	ctx := context.Background()

	_, err := svc.Append(ctx, AppendOperationLogRequest{Scope: "bad", TargetID: "op-1", AgentHostID: 1, Phase: "start"})
	if !errors.Is(err, ErrOperationLogInvalidRequest) {
		t.Fatalf("expected invalid scope error, got %v", err)
	}
	_, err = svc.Append(ctx, AppendOperationLogRequest{Scope: OperationLogScopeCoreOperation, TargetID: "op-1", AgentHostID: 1, Phase: "start", Level: "fatal"})
	if !errors.Is(err, ErrOperationLogInvalidRequest) {
		t.Fatalf("expected invalid level error, got %v", err)
	}
	_, err = svc.Append(ctx, AppendOperationLogRequest{Scope: OperationLogScopeCoreOperation, TargetID: "op-1", AgentHostID: 1, Phase: "start", Payload: json.RawMessage(`{"bad"`)})
	if !errors.Is(err, ErrOperationLogInvalidRequest) {
		t.Fatalf("expected invalid payload error, got %v", err)
	}
}
