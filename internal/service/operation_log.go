package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

const (
	OperationLogScopeCoreOperation   = "core_operation"
	OperationLogScopeApplyRun        = "apply_run"
	OperationLogScopeAgentOperation  = "agent_operation"
	OperationLogScopeAgentTraffic    = "agent_traffic"
	OperationLogScopeTrafficReset    = "traffic_reset"
	OperationLogScopeThresholdAction = "threshold_action"

	OperationLogLevelDebug = "debug"
	OperationLogLevelInfo  = "info"
	OperationLogLevelWarn  = "warn"
	OperationLogLevelError = "error"

	operationLogDefaultLimit = 100
	operationLogMaxLimit     = 200
	operationLogMaxPayload   = 64 * 1024
)

var (
	ErrOperationLogInvalidRequest = errors.New("service: invalid operation log request / 操作日志请求无效")
	ErrOperationLogNotConfigured  = errors.New("service: operation log service not configured / 操作日志服务未配置")

	sensitiveAssignmentPattern = regexp.MustCompile(`(?i)(\b(?:access[_-]?token|refresh[_-]?token|communication[_-]?key|private[_-]?key|api[_-]?key|password|passwd|secret|token|authorization|key)\b\s*[:=]\s*)(["']?)[^"',\s;&]+(["']?)`)
	authorizationBearerPattern = regexp.MustCompile(`(?i)(authorization\s+(?:bearer|basic)\s+)[^\s,;]+`)
)

type OperationLogService interface {
	Append(ctx context.Context, req AppendOperationLogRequest) (*repository.OperationLogEntry, error)
	List(ctx context.Context, req ListOperationLogsRequest) (*ListOperationLogsResult, error)
	Subscribe(ctx context.Context, req SubscribeOperationLogsRequest) (*OperationLogSubscription, error)
}

type AppendOperationLogRequest struct {
	Scope         string
	TargetID      string
	AgentHostID   int64
	Sequence      int64
	Phase         string
	Level         string
	Message       string
	Payload       json.RawMessage
	SourceEventID string
	ReportedAt    int64
}

type ListOperationLogsRequest struct {
	Scope       string
	TargetID    string
	AgentHostID *int64
	AfterID     *int64
	Level       string
	Limit       int
	Offset      int
}

type ListOperationLogsResult struct {
	Items []*repository.OperationLogEntry
	Total int64
}

type SubscribeOperationLogsRequest struct {
	Scope    string
	TargetID string
	AfterID  int64
}

type OperationLogSubscription struct {
	Events <-chan *repository.OperationLogEntry
	close  func()
}

func (s *OperationLogSubscription) Close() {
	if s != nil && s.close != nil {
		s.close()
	}
}

type operationLogService struct {
	logs   repository.OperationLogRepository
	broker *operationLogBroker
	logger *slog.Logger
}

func NewOperationLogService(logs repository.OperationLogRepository, logger *slog.Logger) OperationLogService {
	if logger == nil {
		logger = slog.Default()
	}
	return &operationLogService{
		logs:   logs,
		broker: newOperationLogBroker(),
		logger: logger,
	}
}

func (s *operationLogService) Append(ctx context.Context, req AppendOperationLogRequest) (*repository.OperationLogEntry, error) {
	if s == nil || s.logs == nil {
		return nil, ErrOperationLogNotConfigured
	}
	scope, err := normalizeOperationLogScope(req.Scope)
	if err != nil {
		return nil, err
	}
	targetID := strings.TrimSpace(req.TargetID)
	phase := strings.TrimSpace(req.Phase)
	level, err := normalizeOperationLogLevel(req.Level)
	if err != nil {
		return nil, err
	}
	if targetID == "" || req.AgentHostID <= 0 || phase == "" {
		return nil, ErrOperationLogInvalidRequest
	}

	payload, err := sanitizeOperationLogPayload(req.Payload)
	if err != nil {
		return nil, err
	}
	reportedAt := req.ReportedAt
	if reportedAt == 0 {
		reportedAt = time.Now().Unix()
	}

	entry, err := s.logs.Append(ctx, &repository.OperationLogEntry{
		Scope:         scope,
		TargetID:      targetID,
		AgentHostID:   req.AgentHostID,
		Sequence:      req.Sequence,
		Phase:         phase,
		Level:         level,
		Message:       sanitizeSensitiveText(strings.TrimSpace(req.Message)),
		Payload:       payload,
		SourceEventID: strings.TrimSpace(req.SourceEventID),
		ReportedAt:    reportedAt,
		CreatedAt:     time.Now().Unix(),
	})
	if err != nil {
		return nil, err
	}
	if s.broker != nil {
		s.broker.publish(entry)
	}
	return entry, nil
}

func (s *operationLogService) List(ctx context.Context, req ListOperationLogsRequest) (*ListOperationLogsResult, error) {
	if s == nil || s.logs == nil {
		return nil, ErrOperationLogNotConfigured
	}
	scope, err := normalizeOperationLogScope(req.Scope)
	if err != nil {
		return nil, err
	}
	targetID := strings.TrimSpace(req.TargetID)
	if targetID == "" {
		return nil, ErrOperationLogInvalidRequest
	}
	limit := normalizeOperationLogLimit(req.Limit)
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	filter := repository.OperationLogFilter{
		Scope:    &scope,
		TargetID: &targetID,
		AfterID:  req.AfterID,
		Limit:    limit,
		Offset:   offset,
	}
	if req.AgentHostID != nil {
		if *req.AgentHostID <= 0 {
			return nil, ErrOperationLogInvalidRequest
		}
		agentHostID := *req.AgentHostID
		filter.AgentHostID = &agentHostID
	}
	if strings.TrimSpace(req.Level) != "" {
		level, err := normalizeOperationLogLevel(req.Level)
		if err != nil {
			return nil, err
		}
		filter.Level = &level
	}

	items, err := s.logs.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	total, err := s.logs.Count(ctx, filter)
	if err != nil {
		return nil, err
	}
	return &ListOperationLogsResult{Items: items, Total: total}, nil
}

func (s *operationLogService) Subscribe(ctx context.Context, req SubscribeOperationLogsRequest) (*OperationLogSubscription, error) {
	if s == nil || s.logs == nil || s.broker == nil {
		return nil, ErrOperationLogNotConfigured
	}
	scope, err := normalizeOperationLogScope(req.Scope)
	if err != nil {
		return nil, err
	}
	targetID := strings.TrimSpace(req.TargetID)
	if targetID == "" {
		return nil, ErrOperationLogInvalidRequest
	}
	ch, closeFn := s.broker.subscribe(scope, targetID)
	go func() {
		<-ctx.Done()
		closeFn()
	}()
	return &OperationLogSubscription{Events: ch, close: closeFn}, nil
}

func normalizeOperationLogScope(scope string) (string, error) {
	switch strings.TrimSpace(scope) {
	case OperationLogScopeCoreOperation:
		return OperationLogScopeCoreOperation, nil
	case OperationLogScopeApplyRun:
		return OperationLogScopeApplyRun, nil
	case OperationLogScopeAgentOperation:
		return OperationLogScopeAgentOperation, nil
	case OperationLogScopeAgentTraffic:
		return OperationLogScopeAgentTraffic, nil
	case OperationLogScopeTrafficReset:
		return OperationLogScopeTrafficReset, nil
	case OperationLogScopeThresholdAction:
		return OperationLogScopeThresholdAction, nil
	default:
		return "", ErrOperationLogInvalidRequest
	}
}

func normalizeOperationLogLevel(level string) (string, error) {
	switch strings.TrimSpace(level) {
	case "", OperationLogLevelInfo:
		return OperationLogLevelInfo, nil
	case OperationLogLevelDebug:
		return OperationLogLevelDebug, nil
	case OperationLogLevelWarn:
		return OperationLogLevelWarn, nil
	case OperationLogLevelError:
		return OperationLogLevelError, nil
	default:
		return "", ErrOperationLogInvalidRequest
	}
}

func normalizeOperationLogLimit(limit int) int {
	if limit <= 0 {
		return operationLogDefaultLimit
	}
	if limit > operationLogMaxLimit {
		return operationLogMaxLimit
	}
	return limit
}

func sanitizeOperationLogPayload(payload json.RawMessage) (json.RawMessage, error) {
	if len(payload) == 0 {
		return json.RawMessage(`{}`), nil
	}
	if len(payload) > operationLogMaxPayload {
		return nil, fmt.Errorf("%w (payload too large / payload 过大)", ErrOperationLogInvalidRequest)
	}
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("%w (payload must be valid json / payload 必须是合法 JSON)", ErrOperationLogInvalidRequest)
	}
	sanitized, err := json.Marshal(sanitizeOperationLogValue("", value))
	if err != nil {
		return nil, err
	}
	return json.RawMessage(sanitized), nil
}

func sanitizeOperationLogValue(key string, value any) any {
	if isSensitiveOperationLogKey(key) {
		return "[REDACTED]"
	}
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for k, v := range typed {
			out[k] = sanitizeOperationLogValue(k, v)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, v := range typed {
			out[i] = sanitizeOperationLogValue("", v)
		}
		return out
	case string:
		return sanitizeSensitiveText(typed)
	default:
		return typed
	}
}

func isSensitiveOperationLogKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.TrimSpace(key)))
	if normalized == "" {
		return false
	}
	for _, marker := range []string{"token", "password", "passwd", "privatekey", "secret", "apikey", "authorization", "communicationkey", "key"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func sanitizeSensitiveText(text string) string {
	if text == "" {
		return ""
	}
	text = authorizationBearerPattern.ReplaceAllString(text, `${1}[REDACTED]`)
	return sensitiveAssignmentPattern.ReplaceAllString(text, `${1}${2}[REDACTED]${3}`)
}

type operationLogTopic struct {
	scope    string
	targetID string
}

type operationLogBroker struct {
	mu          sync.Mutex
	subscribers map[operationLogTopic]map[chan *repository.OperationLogEntry]struct{}
}

func newOperationLogBroker() *operationLogBroker {
	return &operationLogBroker{subscribers: make(map[operationLogTopic]map[chan *repository.OperationLogEntry]struct{})}
}

func (b *operationLogBroker) subscribe(scope, targetID string) (<-chan *repository.OperationLogEntry, func()) {
	ch := make(chan *repository.OperationLogEntry, 32)
	topic := operationLogTopic{scope: scope, targetID: targetID}

	b.mu.Lock()
	if b.subscribers[topic] == nil {
		b.subscribers[topic] = make(map[chan *repository.OperationLogEntry]struct{})
	}
	b.subscribers[topic][ch] = struct{}{}
	b.mu.Unlock()

	var once sync.Once
	closeFn := func() {
		once.Do(func() {
			b.mu.Lock()
			if subscribers := b.subscribers[topic]; subscribers != nil {
				delete(subscribers, ch)
				if len(subscribers) == 0 {
					delete(b.subscribers, topic)
				}
			}
			b.mu.Unlock()
			close(ch)
		})
	}
	return ch, closeFn
}

func (b *operationLogBroker) publish(entry *repository.OperationLogEntry) {
	if entry == nil {
		return
	}
	topic := operationLogTopic{scope: entry.Scope, targetID: entry.TargetID}
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subscribers[topic] {
		select {
		case ch <- cloneOperationLogEntry(entry):
		default:
		}
	}
}

func cloneOperationLogEntry(entry *repository.OperationLogEntry) *repository.OperationLogEntry {
	if entry == nil {
		return nil
	}
	clone := *entry
	clone.Payload = append(json.RawMessage(nil), entry.Payload...)
	return &clone
}
