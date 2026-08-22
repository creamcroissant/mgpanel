// 文件路径: internal/async/traffic_queue.go
// 模块说明: 这是 internal 模块里的 traffic_queue 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package async

import (
	"sync"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// UniProxyPushSample represents a raw `[user_id, traffic]` entry submitted by nodes.
// Defined here to avoid import cycle between async and service packages.
type UniProxyPushSample struct {
	UserID   int64
	Upload   int64
	Download int64
}

// TrafficBatch stores a server snapshot with associated traffic samples.
type TrafficBatch struct {
	Server  *repository.Server
	Samples []UniProxyPushSample
}

// TrafficQueue buffers push reports before background ingestion.
type TrafficQueue struct {
	mu      sync.Mutex
	batches []TrafficBatch
}

// NewTrafficQueue constructs an empty buffer for traffic samples.
func NewTrafficQueue() *TrafficQueue {
	return &TrafficQueue{batches: make([]TrafficBatch, 0)}
}

// Enqueue appends a server+sample batch for asynchronous processing.
func (q *TrafficQueue) Enqueue(server *repository.Server, samples []UniProxyPushSample) {
	if q == nil || server == nil || len(samples) == 0 {
		return
	}
	q.mu.Lock()
	q.batches = append(q.batches, TrafficBatch{Server: cloneServer(server), Samples: cloneSamples(samples)})
	q.mu.Unlock()
}

// Drain returns all pending batches and clears the queue.
func (q *TrafficQueue) Drain() []TrafficBatch {
	if q == nil {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	drained := q.batches
	q.batches = make([]TrafficBatch, 0)
	return drained
}

// Pending reports buffered batch count.
func (q *TrafficQueue) Pending() int {
	if q == nil {
		return 0
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.batches)
}

// Requeue prepends a batch for retry handling.
func (q *TrafficQueue) Requeue(batch TrafficBatch) {
	if q == nil || batch.Server == nil || len(batch.Samples) == 0 {
		return
	}
	q.mu.Lock()
	q.batches = append([]TrafficBatch{batch.clone()}, q.batches...)
	q.mu.Unlock()
}

func (b TrafficBatch) clone() TrafficBatch {
	return TrafficBatch{Server: cloneServer(b.Server), Samples: cloneSamples(b.Samples)}
}

func cloneServer(server *repository.Server) *repository.Server {
	if server == nil {
		return nil
	}
	snapshot := *server
	return &snapshot
}

func cloneSamples(samples []UniProxyPushSample) []UniProxyPushSample {
	if len(samples) == 0 {
		return nil
	}
	cloned := make([]UniProxyPushSample, len(samples))
	copy(cloned, samples)
	return cloned
}
