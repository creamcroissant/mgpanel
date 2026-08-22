package async

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// SubscriptionLogQueue buffers subscription logs before background ingestion.
type SubscriptionLogQueue struct {
	mu     sync.Mutex
	logs   []*repository.SubscriptionLog
	repo   repository.SubscriptionLogRepository
	logger *slog.Logger
	ctx    context.Context
	cancel context.CancelFunc
}

const subscriptionLogWriteTimeout = 3 * time.Second

// NewSubscriptionLogQueue constructs a buffered queue for subscription logs.
func NewSubscriptionLogQueue(repo repository.SubscriptionLogRepository, logger *slog.Logger) *SubscriptionLogQueue {
	ctx, cancel := context.WithCancel(context.Background())
	q := &SubscriptionLogQueue{
		logs:   make([]*repository.SubscriptionLog, 0),
		repo:   repo,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
	go q.worker()
	return q
}

// Enqueue appends a subscription log for asynchronous processing.
func (q *SubscriptionLogQueue) Enqueue(log *repository.SubscriptionLog) {
	if q == nil || log == nil {
		return
	}
	q.mu.Lock()
	q.logs = append(q.logs, log)
	q.mu.Unlock()
}

// worker periodically flushes logs to the database.
func (q *SubscriptionLogQueue) worker() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-q.ctx.Done():
			q.flush()
			return
		case <-ticker.C:
			q.flush()
		}
	}
}

// flush writes all pending logs to the repository.
func (q *SubscriptionLogQueue) flush() {
	q.mu.Lock()
	if len(q.logs) == 0 {
		q.mu.Unlock()
		return
	}
	pending := q.logs
	q.logs = make([]*repository.SubscriptionLog, 0)
	q.mu.Unlock()

	for _, log := range pending {
		logCtx, cancel := context.WithTimeout(q.ctx, subscriptionLogWriteTimeout)
		err := q.repo.Log(logCtx, log)
		cancel()
		if err != nil {
			q.logger.Error("failed to persist subscription log", "error", err, "user_id", log.UserID)
		}
	}
}

// Stop gracefully shuts down the queue worker.
func (q *SubscriptionLogQueue) Stop() {
	if q == nil {
		return
	}
	q.cancel()
}
