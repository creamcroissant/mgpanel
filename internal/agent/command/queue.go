package command

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrInvalidOptions    = errors.New("command queue: invalid options")
	ErrInvalidTask       = errors.New("command queue: invalid task")
	ErrQueueFull         = errors.New("command queue: queue full")
	ErrQueueStopped      = errors.New("command queue: queue stopped")
	ErrUnsupportedAction = errors.New("command queue: unsupported action")
)

type Options struct {
	Capacity    int
	Workers     int
	TaskTimeout time.Duration
	Handlers    map[string]Handler
	Reporter    Reporter
	Now         func() time.Time
}

type Stats struct {
	Capacity         int32
	Queued           int32
	Inflight         int32
	Workers          int32
	Available        int32
	ActiveCommandIDs []string
	UpdatedAt        int64
}

type Queue struct {
	tasks       chan queuedTask
	slots       chan struct{}
	workers     int
	taskTimeout time.Duration
	reporter    Reporter
	now         func() time.Time

	mu       sync.RWMutex
	running  bool
	ctx      context.Context
	cancel   context.CancelFunc
	handlers map[string]Handler
	inflight map[string]Task

	stopped chan struct{}

	wg       sync.WaitGroup
	sequence atomic.Int64
}

type queuedTask struct {
	task    Task
	handler Handler
}

func NewQueue(opts Options) (*Queue, error) {
	if opts.Capacity <= 0 || opts.Workers <= 0 {
		return nil, ErrInvalidOptions
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	q := &Queue{
		tasks:       make(chan queuedTask, opts.Capacity),
		slots:       make(chan struct{}, opts.Capacity),
		workers:     opts.Workers,
		taskTimeout: opts.TaskTimeout,
		reporter:    opts.Reporter,
		now:         now,
		handlers:    make(map[string]Handler),
		inflight:    make(map[string]Task),
		stopped:     make(chan struct{}),
	}
	for operationType, handler := range opts.Handlers {
		if err := q.Register(operationType, handler); err != nil {
			return nil, err
		}
	}
	return q, nil
}

func (q *Queue) Register(operationType string, handler Handler) error {
	operationType = strings.TrimSpace(operationType)
	if operationType == "" || handler == nil {
		return ErrInvalidOptions
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.handlers[operationType] = handler
	return nil
}

func (q *Queue) Start(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	q.mu.Lock()
	if q.running {
		q.mu.Unlock()
		return
	}
	q.ctx, q.cancel = context.WithCancel(ctx)
	q.running = true
	q.stopped = make(chan struct{})
	workerCtx := q.ctx
	workers := q.workers
	q.mu.Unlock()

	for i := 0; i < workers; i++ {
		q.wg.Add(1)
		go q.worker(workerCtx)
	}
}

func (q *Queue) Stop() {
	q.mu.Lock()
	if !q.running {
		q.mu.Unlock()
		return
	}
	cancel := q.cancel
	q.running = false
	q.cancel = nil
	q.ctx = nil
	q.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	close(q.stopped)
	q.wg.Wait()
}

// Stopped returns a channel that's closed when the queue is stopped.
func (q *Queue) Stopped() <-chan struct{} {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.stopped
}

func (q *Queue) Submit(ctx context.Context, task Task) error {
	task = normalizeTask(task)
	if task.ID == "" || task.OperationType == "" {
		return ErrInvalidTask
	}
	handler := q.handlerFor(task.OperationType)
	if handler == nil {
		q.emitTerminal(task, Result{Status: StatusUnsupportedAction, Phase: "unsupported", Level: LevelWarn, Message: "unsupported agent command", ErrorMessage: fmt.Sprintf("unsupported operation type %q", task.OperationType)})
		return ErrUnsupportedAction
	}
	return q.submit(ctx, task, handler)
}

func (q *Queue) SubmitWithHandler(ctx context.Context, task Task, handler Handler) error {
	if handler == nil {
		return ErrInvalidOptions
	}
	task = normalizeTask(task)
	if task.ID == "" || task.OperationType == "" {
		return ErrInvalidTask
	}
	return q.submit(ctx, task, handler)
}

func (q *Queue) submit(ctx context.Context, task Task, handler Handler) error {
	q.mu.RLock()
	running := q.running
	q.mu.RUnlock()
	if !running {
		return ErrQueueStopped
	}
	select {
	case q.slots <- struct{}{}:
	default:
		q.emitTerminal(task, Result{Status: StatusQueueFull, Phase: "queue_full", Level: LevelWarn, Message: "agent command queue full", ErrorMessage: ErrQueueFull.Error()})
		return ErrQueueFull
	}
	q.emitEvent(Event{CommandID: task.ID, OperationType: task.OperationType, EventType: EventTypeAccepted, Status: StatusClaimed, Phase: "accepted", Level: LevelInfo, Message: "agent command accepted"})
	select {
	case q.tasks <- queuedTask{task: task, handler: handler}:
		return nil
	default:
		q.releaseSlot()
		q.emitTerminal(task, Result{Status: StatusQueueFull, Phase: "queue_full", Level: LevelWarn, Message: "agent command queue full", ErrorMessage: ErrQueueFull.Error()})
		return ErrQueueFull
	}
}

func (q *Queue) Stats() Stats {
	q.mu.RLock()
	active := make([]string, 0, len(q.inflight))
	for id := range q.inflight {
		active = append(active, id)
	}
	q.mu.RUnlock()
	sort.Strings(active)
	queued := len(q.tasks)
	inflight := len(active)
	capacity := cap(q.slots)
	available := capacity - len(q.slots)
	if available < 0 {
		available = 0
	}
	return Stats{Capacity: int32(capacity), Queued: int32(queued), Inflight: int32(inflight), Workers: int32(q.workers), Available: int32(available), ActiveCommandIDs: active, UpdatedAt: q.now().Unix()}
}

func (q *Queue) SupportedActions() []string {
	q.mu.RLock()
	actions := make([]string, 0, len(q.handlers))
	for action := range q.handlers {
		actions = append(actions, action)
	}
	q.mu.RUnlock()
	sort.Strings(actions)
	return actions
}

func (q *Queue) Report(ctx context.Context, event Event) error {
	if q.reporter == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	event = q.normalizeEvent(event)
	return q.reporter.Report(ctx, event)
}

func (q *Queue) worker(ctx context.Context) {
	defer q.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case item := <-q.tasks:
			q.run(ctx, item)
		}
	}
}

func (q *Queue) run(ctx context.Context, item queuedTask) {
	q.addInflight(item.task)
	defer q.releaseSlot()
	defer q.removeInflight(item.task.ID)

	execCtx := ctx
	cancel := func() {}
	timeout := item.task.Timeout
	if timeout <= 0 {
		timeout = q.taskTimeout
	}
	if timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	resultCh := make(chan Result, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				resultCh <- Result{Status: StatusFailed, Phase: "panic", Level: LevelError, Message: "agent command panicked", ErrorMessage: fmt.Sprint(recovered)}
			}
		}()
		resultCh <- item.handler(execCtx, item.task, scopedReporter{queue: q, task: item.task})
	}()

	var result Result
	select {
	case result = <-resultCh:
		result = q.normalizeResult(result)
	case <-execCtx.Done():
		result = q.resultFromContext(execCtx)
	}
	q.emitTerminal(item.task, result)
}

func (q *Queue) handlerFor(operationType string) Handler {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.handlers[strings.TrimSpace(operationType)]
}

func (q *Queue) addInflight(task Task) {
	q.mu.Lock()
	q.inflight[task.ID] = task
	q.mu.Unlock()
}

func (q *Queue) removeInflight(commandID string) {
	q.mu.Lock()
	delete(q.inflight, commandID)
	q.mu.Unlock()
}

func (q *Queue) releaseSlot() {
	select {
	case <-q.slots:
	default:
	}
}

func (q *Queue) resultFromContext(ctx context.Context) Result {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return Result{Status: StatusTimeout, Phase: "timeout", Level: LevelError, Message: "agent command timed out", ErrorMessage: context.DeadlineExceeded.Error()}
	}
	return Result{Status: StatusCancelled, Phase: "cancelled", Level: LevelWarn, Message: "agent command cancelled", ErrorMessage: context.Canceled.Error()}
}

func (q *Queue) normalizeResult(result Result) Result {
	result.Status = strings.TrimSpace(result.Status)
	if result.Status == "" {
		if strings.TrimSpace(result.ErrorMessage) != "" {
			result.Status = StatusFailed
		} else {
			result.Status = StatusSuccess
		}
	}
	if !result.Terminal {
		result.Terminal = true
	}
	if strings.TrimSpace(result.Phase) == "" {
		result.Phase = result.Status
	}
	if strings.TrimSpace(result.Level) == "" {
		switch result.Status {
		case StatusSuccess:
			result.Level = LevelInfo
		case StatusTimeout, StatusFailed:
			result.Level = LevelError
		default:
			result.Level = LevelWarn
		}
	}
	if strings.TrimSpace(result.Message) == "" {
		result.Message = "agent command finished"
	}
	return result
}

func (q *Queue) emitTerminal(task Task, result Result) {
	result = q.normalizeResult(result)
	q.emitEvent(Event{CommandID: task.ID, OperationType: task.OperationType, EventType: EventTypeResult, Status: result.Status, Phase: result.Phase, Level: result.Level, Message: result.Message, Payload: cloneBytes(result.Payload), ErrorMessage: result.ErrorMessage, Terminal: true})
}

func (q *Queue) emitEvent(event Event) {
	_ = q.Report(context.Background(), event)
}

func (q *Queue) normalizeEvent(event Event) Event {
	event.CommandID = strings.TrimSpace(event.CommandID)
	event.OperationType = strings.TrimSpace(event.OperationType)
	event.EventType = strings.TrimSpace(event.EventType)
	if event.EventType == "" {
		event.EventType = EventTypeProgress
	}
	event.Status = strings.TrimSpace(event.Status)
	event.Phase = strings.TrimSpace(event.Phase)
	event.Level = strings.TrimSpace(event.Level)
	if event.Level == "" {
		event.Level = LevelInfo
	}
	event.Message = strings.TrimSpace(event.Message)
	if event.OccurredAt == 0 {
		event.OccurredAt = q.now().Unix()
	}
	if event.Sequence == 0 {
		event.Sequence = q.sequence.Add(1)
	}
	event.Payload = cloneBytes(event.Payload)
	return event
}

type scopedReporter struct {
	queue *Queue
	task  Task
}

func (r scopedReporter) Report(ctx context.Context, event Event) error {
	if event.CommandID == "" {
		event.CommandID = r.task.ID
	}
	if event.OperationType == "" {
		event.OperationType = r.task.OperationType
	}
	return r.queue.Report(ctx, event)
}

func normalizeTask(task Task) Task {
	task = cloneTask(task)
	task.ID = strings.TrimSpace(task.ID)
	task.OperationType = strings.TrimSpace(task.OperationType)
	return task
}

func cloneTask(task Task) Task {
	task.RequestPayload = cloneBytes(task.RequestPayload)
	return task
}

func cloneBytes(input []byte) []byte {
	if len(input) == 0 {
		return nil
	}
	return append([]byte(nil), input...)
}
