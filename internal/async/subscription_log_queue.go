// 文件路径: internal/async/subscription_log_queue.go
// 模块说明: 订阅日志有界队列——批量落库，防止突发订阅日志挤占 DB 资源。
package async

import (
	"context"
	"log/slog"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// SubscriptionLogQueue 缓冲订阅日志并通过后台 worker 批量落库。
// 底层基于通用有界 TaskQueue，防止突发订阅日志导致内存/DB 挤占。
type SubscriptionLogQueue struct {
	queue  *TaskQueue
	repo   repository.SubscriptionLogRepository
	logger *slog.Logger
}

const (
	subscriptionLogCapacity     = 5000
	subscriptionLogBatchSize    = 200
	subscriptionLogFlushEvery   = 5 * time.Second
	subscriptionLogWriteTimeout = 3 * time.Second
)

// NewSubscriptionLogQueue 构建有界订阅日志队列。
func NewSubscriptionLogQueue(repo repository.SubscriptionLogRepository, logger *slog.Logger) *SubscriptionLogQueue {
	q := &SubscriptionLogQueue{repo: repo, logger: logger}
	q.queue = NewTaskQueue(TaskQueueConfig{
		Capacity:   subscriptionLogCapacity,
		FlushEvery: subscriptionLogFlushEvery,
		BatchSize:  subscriptionLogBatchSize,
		Overflow:   OverflowDropOldest,
		FlushFn:    q.flush,
	})
	return q
}

// Enqueue 将订阅日志入队。队列满时丢弃最旧条目（日志可容忍丢失）。
func (q *SubscriptionLogQueue) Enqueue(log *repository.SubscriptionLog) {
	if q == nil || log == nil || q.queue == nil {
		return
	}
	q.queue.Enqueue(log)
}

// flush 批量写入订阅日志到数据库。
func (q *SubscriptionLogQueue) flush(ctx context.Context, items []any) {
	if len(items) == 0 {
		return
	}
	logs := make([]*repository.SubscriptionLog, 0, len(items))
	for _, item := range items {
		if log, ok := item.(*repository.SubscriptionLog); ok {
			logs = append(logs, log)
		}
	}
	if len(logs) == 0 {
		return
	}
	for _, log := range logs {
		logCtx, cancel := context.WithTimeout(ctx, subscriptionLogWriteTimeout)
		if err := q.repo.Log(logCtx, log); err != nil {
			q.logger.Error("failed to persist subscription log", "error", err, "user_id", log.UserID)
		}
		cancel()
	}
}

// Stop 优雅停止队列 worker。
func (q *SubscriptionLogQueue) Stop() {
	if q == nil {
		return
	}
	q.queue.Stop()
}
