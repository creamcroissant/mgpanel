// 文件路径: internal/job/cache_stats.go
// 模块说明: 定时输出缓存命中率指标，用于可观测性监控。
package job

import (
	"context"
	"log/slog"

	"github.com/creamcroissant/mgpanel/internal/cache"
)

// CacheStatsJob 周期性地将缓存 HIT/MISS 计数输出到结构化日志，
// 便于观察 repository 层缓存是否有效、以及是否发生频繁的缓存漂移。
type CacheStatsJob struct {
	store  cache.Store
	logger *slog.Logger
}

// NewCacheStatsJob 构建缓存统计任务。
func NewCacheStatsJob(store cache.Store, logger *slog.Logger) *CacheStatsJob {
	return &CacheStatsJob{store: store, logger: logger}
}

// Name 返回任务名。
func (j *CacheStatsJob) Name() string { return "cache_stats" }

// Run 读取缓存命中/未命中计数并输出结构化日志。
func (j *CacheStatsJob) Run(ctx context.Context) error {
	hits, misses, ok := cache.StoreStats(j.store)
	if !ok {
		// 缓存实现不支持统计（例如无缓存注入时的 nil store），跳过。
		return nil
	}
	total := hits + misses
	if j.logger == nil || total == 0 {
		return nil
	}
	hitRate := float64(hits) / float64(total)
	if j.logger.Enabled(ctx, slog.LevelInfo) {
		j.logger.Info("cache stats",
			"hits", hits,
			"misses", misses,
			"total", total,
			"hit_rate", hitRate,
		)
	}
	return nil
}