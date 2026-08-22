// 文件路径: internal/job/traffic_fetch.go
// 模块说明: 这是 internal 模块里的 traffic_fetch 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package job

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/creamcroissant/mgpanel/internal/async"
	"github.com/creamcroissant/mgpanel/internal/service"
)

// TrafficFetchJob 将缓冲的流量样本刷入 ServerTrafficService。
type TrafficFetchJob struct {
	Queue   *async.TrafficQueue
	Traffic service.ServerTrafficService
	Logger  *slog.Logger
}

// NewTrafficFetchJob 组装队列与流量服务用于定时任务。
func NewTrafficFetchJob(queue *async.TrafficQueue, traffic service.ServerTrafficService, logger *slog.Logger) *TrafficFetchJob {
	if logger == nil {
		logger = slog.Default()
	}
	return &TrafficFetchJob{Queue: queue, Traffic: traffic, Logger: logger}
}

// Name 返回任务标识。
func (j *TrafficFetchJob) Name() string { return "traffic.fetch" }

// Run 拉取队列批次并通过 ServerTrafficService 持久化。
func (j *TrafficFetchJob) Run(ctx context.Context) error {
	if j == nil || j.Queue == nil || j.Traffic == nil {
		return fmt.Errorf("traffic fetch job dependencies not configured / 流量采集任务依赖未配置")
	}
	batches := j.Queue.Drain()
	if len(batches) == 0 {
		return nil
	}
	for _, batch := range batches {
		if batch.Server == nil || len(batch.Samples) == 0 {
			continue
		}
		if err := j.Traffic.Apply(ctx, batch.Server, batch.Samples); err != nil {
			j.Logger.Error("traffic fetch batch failed", "server", batch.Server.ID, "error", err)
			j.Queue.Requeue(batch)
			continue
		}
	}
	j.Logger.Debug("traffic fetch batches persisted", "count", len(batches))
	return nil
}
