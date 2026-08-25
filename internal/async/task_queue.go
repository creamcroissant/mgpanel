// 文件路径: internal/async/task_queue.go
// 模块说明: 通用有界任务队列，带容量限制、溢出策略、worker 池和可观测性指标。

package async

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// dropOldestMaxRetries 限制 DropOldest 腾位重试次数，保证 Enqueue 有界返回。
const dropOldestMaxRetries = 64

// OverflowPolicy 定义队列满载时的溢出策略。
type OverflowPolicy int

const (
	// OverflowDropOldest 丢弃最旧的未消费条目，为新条目腾出空间。
	OverflowDropOldest OverflowPolicy = iota
	// OverflowReject 拒绝新入队请求（Enqueue 返回 false）。
	OverflowReject
	// OverflowBlock 阻塞直到队列有空间。
	OverflowBlock
)

// TaskQueueConfig 配置通用有界任务队列。
type TaskQueueConfig struct {
	Capacity     int                                    // 缓冲容量（0 = 无缓冲，默认 1024）
	Workers      int                                    // 并发 worker 数（0 = 无 worker，仅 Drain）
	FlushEvery   time.Duration                          // worker 批量消费间隔（0 = 不自动 flush）
	BatchSize    int                                    // 单次批量消费上限（0 = 单条）
	Overflow     OverflowPolicy                         // 满载策略
	FlushFn      func(ctx context.Context, items []any) // 批量消费回调（nil = 仅 Drain）
	DrainTimeout time.Duration                          // 停机排水超时（最终 flush 的独立上下文，默认 10s）
}

// TaskQueue 是通用有界任务队列，支持 overflow 策略、worker 池和可观测性指标。
// 零值不可用，须通过 NewTaskQueue 创建。
type TaskQueue struct {
	buffer       chan any
	overflow     OverflowPolicy
	workers      int
	batchCap     int
	drainTimeout time.Duration
	flushFn      func(ctx context.Context, items []any)

	enqueued atomic.Int64
	drained  atomic.Int64
	dropped  atomic.Int64
	rejected atomic.Int64

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewTaskQueue 创建并启动通用有界任务队列。
func NewTaskQueue(cfg TaskQueueConfig) *TaskQueue {
	if cfg.Capacity <= 0 {
		cfg.Capacity = 1024
	}
	if cfg.Workers < 0 {
		cfg.Workers = 0
	}
	if cfg.BatchSize < 0 {
		cfg.BatchSize = 0
	}
	ctx, cancel := context.WithCancel(context.Background())
	if cfg.DrainTimeout <= 0 {
		cfg.DrainTimeout = 10 * time.Second
	}
	q := &TaskQueue{
		buffer:       make(chan any, cfg.Capacity),
		overflow:     cfg.Overflow,
		workers:      cfg.Workers,
		batchCap:     cfg.BatchSize,
		drainTimeout: cfg.DrainTimeout,
		flushFn:      cfg.FlushFn,
		ctx:          ctx,
		cancel:       cancel,
	}

	// 启动 worker 池
	if q.workers > 0 && q.flushFn != nil {
		for i := 0; i < q.workers; i++ {
			q.wg.Add(1)
			go q.workerLoop()
		}
	}
	// 定时 flush（仅当有 worker 但无定时 ticker 时由 worker 自行消费）
	// 如果需要定时 flush + worker，启动 ticker goroutine
	if cfg.FlushEvery > 0 && q.flushFn != nil {
		q.wg.Add(1)
		go q.tickerLoop(cfg.FlushEvery)
	}

	return q
}

// Enqueue 尝试将 item 入队。返回 true 表示成功入队。
// 当 OverflowReject 且队列满时返回 false。
func (q *TaskQueue) Enqueue(item any) bool {
	if q == nil || q.buffer == nil {
		return false
	}
	// 已停止的队列拒绝新条目（非阻塞快速路径）
	select {
	case <-q.ctx.Done():
		return false
	default:
	}
	q.enqueued.Add(1)

	switch q.overflow {
	case OverflowBlock:
		select {
		case q.buffer <- item:
			return true
		case <-q.ctx.Done():
			return false
		}
	case OverflowReject:
		select {
		case q.buffer <- item:
			return true
		default:
			q.rejected.Add(1)
			return false
		}
	case OverflowDropOldest:
		select {
		case q.buffer <- item:
			return true
		default:
			// 溢出腾位：有界重试，绝不阻塞。
			// 旧实现“<-q.buffer 裸阻塞 + 二次发送”在多生产者并发下会互相等待
			// （全部卡在接收/回填，无人发送）导致 Enqueue 死锁；且二次发送失败
			// 时条目不计入任何计数器而凭空消失。改为：非阻塞腾位+重试，
			// 兕底时丢弃的是新条目自身并计入 dropped，保证
			// enqueued ≤ drained+dropped+rejected 的可审计不变量。
			for retry := 0; retry < dropOldestMaxRetries; retry++ {
				select {
				case q.buffer <- item:
					return true
				default:
				}
				select {
				case <-q.buffer:
					q.dropped.Add(1)
				default:
					// 缓冲恰好被并发消费清空：直接重试发送即可
				}
				runtime.Gosched()
			}
			q.dropped.Add(1) // 兕底：丢弃新条目自身，计数不缺口
			return false
		}
	default:
		return false
	}
}

// Drain 返回所有已缓冲的条目并清空缓冲区（非阻塞）。
func (q *TaskQueue) Drain() []any {
	if q == nil || q.buffer == nil {
		return nil
	}
	var items []any
	for {
		select {
		case item := <-q.buffer:
			items = append(items, item)
			q.drained.Add(1)
		default:
			return items
		}
	}
}

// TryDrain 尝试最多 max 条目的批量消费（非阻塞）。
func (q *TaskQueue) TryDrain(max int) []any {
	if q == nil || q.buffer == nil || max <= 0 {
		return nil
	}
	items := make([]any, 0, max)
	for i := 0; i < max; i++ {
		select {
		case item := <-q.buffer:
			items = append(items, item)
			q.drained.Add(1)
		default:
			return items
		}
	}
	return items
}

// Pending 返回当前队列中近似等待处理的条目数。
func (q *TaskQueue) Pending() int {
	if q == nil || q.buffer == nil {
		return 0
	}
	return len(q.buffer)
}

// Stats 返回队列的累积统计指标。
func (q *TaskQueue) Stats() (enqueued, drained, dropped, rejected int64) {
	if q == nil {
		return 0, 0, 0, 0
	}
	return q.enqueued.Load(), q.drained.Load(), q.dropped.Load(), q.rejected.Load()
}

// Stop 优雅停止队列 worker，等待已拉取的条目处理完毕。
func (q *TaskQueue) Stop() {
	if q == nil {
		return
	}
	q.cancel()
	q.wg.Wait()
}

// workerLoop 持续从 buffer 消费条目。
// workerLoop 消费缓冲条目：BatchSize>0 时攒满即批处理（与 FlushEvery 先到者生效），
// 否则逐条消费。批间等待用短轮询以响应 Stop。
func (q *TaskQueue) workerLoop() {
	defer q.wg.Done()
	if q.batchCap <= 0 {
		for {
			select {
			case <-q.ctx.Done():
				return
			case item := <-q.buffer:
				q.drained.Add(1)
				q.flushOne(item)
			}
		}
	}
	const pollInterval = 20 * time.Millisecond
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	batch := make([]any, 0, q.batchCap)
	flushBatch := func() {
		if len(batch) == 0 {
			return
		}
		items := batch
		batch = make([]any, 0, q.batchCap)
		q.drained.Add(int64(len(items)))
		if q.flushFn != nil {
			q.flushFn(q.ctx, items)
		}
	}
	for {
		select {
		case <-q.ctx.Done():
			flushBatch() // 停止前排空已取出的条目（ctx 已取消，回调需自行兜底）
			return
		case item := <-q.buffer:
			batch = append(batch, item)
			if len(batch) >= q.batchCap {
				flushBatch()
			}
		case <-ticker.C:
			// 攒不满也按 poll 节奏冲刷，避免低流量下条目滞留过久
			flushBatch()
		}
	}
}

// flushOne 单条路径的统一出口。
func (q *TaskQueue) flushOne(item any) {
	if q.flushFn != nil {
		q.flushFn(q.ctx, []any{item})
	}
}

// tickerLoop 定时批量消费。
func (q *TaskQueue) tickerLoop(interval time.Duration) {
	defer q.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-q.ctx.Done():
			// 停止前消费剩余条目：使用独立的新鲜上下文排水，
			// 避免已取消的 q.ctx 阻断 FlushFn 内的 DB 写入导致最后一批数据丢失。
			drainCtx, cancel := context.WithTimeout(context.Background(), q.drainTimeout)
			defer cancel()
			q.flushAll(drainCtx)
			return
		case <-ticker.C:
			q.flushAll(q.ctx)
		}
	}
}

// flushAll 一次性消费所有缓冲条目。ctx 由调用方提供（周期路径用运行期 ctx，停机路径用新鲜上下文）。
func (q *TaskQueue) flushAll(ctx context.Context) {
	items := q.Drain()
	if len(items) == 0 {
		return
	}
	// 按 batchCap 分片
	if q.batchCap > 0 {
		for i := 0; i < len(items); i += q.batchCap {
			end := i + q.batchCap
			if end > len(items) {
				end = len(items)
			}
			q.flushFn(ctx, items[i:end])
		}
	} else {
		q.flushFn(ctx, items)
	}
}
