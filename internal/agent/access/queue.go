// 文件路径: internal/agent/access/queue.go
// 模块说明: 本地有界访问日志上报队列——防止收集突发时一次性 gRPC 上报挤占带宽/资源。

package access

import (
	"context"
	"log/slog"
	"time"

	"github.com/creamcroissant/mgpanel/internal/async"
)

const (
	// accessLogUploadCapacity 本地上报队列容量（条数）。
	accessLogUploadCapacity = 10000
	// accessLogUploadBatchMax 单次 gRPC 上报的最大条数（分片上限）。
	accessLogUploadBatchMax = 500
	// accessLogUploadEvery 上报 worker 的触发间隔。
	accessLogUploadEvery = 3 * time.Second
)

// UploadFunc 是一次批量上报的回调，由 manager 注入。
type UploadFunc func(ctx context.Context, entries []AccessLogEntry) error

// UploadQueue 是有界访问日志上报队列。
// 收集器将 entries 入队，worker 限速批量上报，避免突发时单次报文过大或
// 过多并发 gRPC 挤占资源。队列满载时丢弃最旧条目（可容忍丢失）。
type UploadQueue struct {
	queue  *async.TaskQueue
	upload UploadFunc
	logger *slog.Logger
}

// NewUploadQueue 构建有界上报队列。
func NewUploadQueue(upload UploadFunc, logger *slog.Logger) *UploadQueue {
	if logger == nil {
		logger = slog.Default()
	}
	q := &UploadQueue{upload: upload, logger: logger}
	q.queue = async.NewTaskQueue(async.TaskQueueConfig{
		Capacity:   accessLogUploadCapacity,
		FlushEvery: accessLogUploadEvery,
		BatchSize:  accessLogUploadBatchMax,
		Overflow:   async.OverflowDropOldest,
		FlushFn:    q.handle,
	})
	return q
}

// Enqueue 将一批 entries 入队（按条数分片入队）。
func (q *UploadQueue) Enqueue(entries []AccessLogEntry) {
	if q == nil || q.queue == nil || len(entries) == 0 {
		return
	}
	// 单次收集可能很大，按上报上限分片入队，worker 逐片处理
	for start := 0; start < len(entries); start += accessLogUploadBatchMax {
		end := start + accessLogUploadBatchMax
		if end > len(entries) {
			end = len(entries)
		}
		chunk := make([]AccessLogEntry, end-start)
		copy(chunk, entries[start:end])
		q.queue.Enqueue(chunk)
	}
}

// Pending 返回待上报条目数（近似）。
func (q *UploadQueue) Pending() int {
	if q == nil || q.queue == nil {
		return 0
	}
	return q.queue.Pending()
}

// Stop 优雅停止上报 worker。
func (q *UploadQueue) Stop() {
	if q == nil {
		return
	}
	q.queue.Stop()
}

// handle 是 TaskQueue 批量消费回调。
func (q *UploadQueue) handle(ctx context.Context, items []any) {
	if q.upload == nil || len(items) == 0 {
		return
	}
	for _, item := range items {
		entries, ok := item.([]AccessLogEntry)
		if !ok || len(entries) == 0 {
			continue
		}
		if err := q.upload(ctx, entries); err != nil {
			q.logger.Error("failed to upload access logs", "error", err, "count", len(entries))
		}
	}
}
