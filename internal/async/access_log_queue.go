// 文件路径: internal/async/access_log_queue.go
// 模块说明: 有界访问日志队列——防止 agent 突发上报挤占 DB 资源。
// 队列基于通用 TaskQueue，批量消费回调由上层（service 层）注入，
// 实现"缓冲 + 限速批量落库"，而非 gRPC handler 同步写库。
package async

import (
	"context"
	"log/slog"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// AccessLogBatch 是一组待落库的访问日志（同一 agent 上报批次）。
type AccessLogBatch struct {
	AgentHostID int64
	Records     []*repository.AccessLog
}

// AccessLogFlushFunc 是批量落库回调，由 service 层注入。
// 返回 error 表示该批写入失败（队列记录丢弃计数）。
type AccessLogFlushFunc func(ctx context.Context, batch AccessLogBatch) error

// AccessLogQueue 是有界访问日志队列。
// 防止大量访问日志同时落库挤占 DB，通过 worker 限速批量写入。
type AccessLogQueue struct {
	queue  *TaskQueue
	flush  AccessLogFlushFunc
	logger *slog.Logger
}

const (
	accessLogQueueCapacity   = 50000 // 单批 records 按条目计，这里按批次计
	accessLogQueueBatchSize  = 500   // 单次批量落库条数上限
	accessLogQueueFlushEvery = 2 * time.Second
)

// NewAccessLogQueue 构建有界访问日志队列。
func NewAccessLogQueue(flush AccessLogFlushFunc, logger *slog.Logger) *AccessLogQueue {
	if logger == nil {
		logger = slog.Default()
	}
	q := &AccessLogQueue{flush: flush, logger: logger}
	q.queue = NewTaskQueue(TaskQueueConfig{
		Capacity:   accessLogQueueCapacity,
		FlushEvery: accessLogQueueFlushEvery,
		BatchSize:  accessLogQueueBatchSize,
		Overflow:   OverflowDropOldest,
		FlushFn:    q.handle,
	})
	return q
}

// Enqueue 将一批访问日志入队。队列满时丢弃最旧批次（访问日志可容忍丢失）。
func (q *AccessLogQueue) Enqueue(agentHostID int64, records []*repository.AccessLog) bool {
	if q == nil || q.queue == nil || len(records) == 0 {
		return false
	}
	return q.queue.Enqueue(AccessLogBatch{AgentHostID: agentHostID, Records: records})
}

// Pending 返回当前待处理批次数量。
func (q *AccessLogQueue) Pending() int {
	if q == nil || q.queue == nil {
		return 0
	}
	return q.queue.Pending()
}

// Stats 返回队列统计指标。
func (q *AccessLogQueue) Stats() (enqueued, drained, dropped, rejected int64) {
	if q == nil || q.queue == nil {
		return 0, 0, 0, 0
	}
	return q.queue.Stats()
}

// Stop 优雅停止队列 worker。
func (q *AccessLogQueue) Stop() {
	if q == nil {
		return
	}
	q.queue.Stop()
}

// handle 是 TaskQueue 的批量消费回调。
func (q *AccessLogQueue) handle(ctx context.Context, items []any) {
	if q.flush == nil || len(items) == 0 {
		return
	}
	for _, item := range items {
		batch, ok := item.(AccessLogBatch)
		if !ok || len(batch.Records) == 0 {
			continue
		}
		if err := q.flush(ctx, batch); err != nil {
			q.logger.Error("failed to persist access logs", "error", err, "agent_host_id", batch.AgentHostID, "count", len(batch.Records))
		}
	}
}
