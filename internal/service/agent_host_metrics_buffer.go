package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/creamcroissant/xboard/internal/cache"
	"github.com/creamcroissant/xboard/internal/repository"
)

const (
	agentHostMetricsBufferNamespace = "agent_host_metrics_buffer"
	agentHostMetricsBufferTTL       = 10 * time.Minute
)

type agentHostMetricsBuffer struct {
	cache  cache.Store
	repo   repository.AgentHostRepository
	logger *slog.Logger
}

type agentHostMetricsBufferEntry struct {
	AgentHostID int64                       `json:"agent_host_id"`
	Metrics     repository.AgentHostMetrics `json:"metrics"`
}

func newAgentHostMetricsBuffer(cacheStore cache.Store, repo repository.AgentHostRepository, logger *slog.Logger) *agentHostMetricsBuffer {
	if cacheStore == nil || repo == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &agentHostMetricsBuffer{
		cache:  cacheStore.Namespace(agentHostMetricsBufferNamespace),
		repo:   repo,
		logger: logger,
	}
}

func (b *agentHostMetricsBuffer) Enqueue(ctx context.Context, agentHostID int64, metrics repository.AgentHostMetrics) error {
	if b == nil || b.cache == nil {
		return fmt.Errorf("metrics buffer unavailable")
	}
	if agentHostID <= 0 {
		return fmt.Errorf("agent_host_id is required")
	}
	entry := agentHostMetricsBufferEntry{
		AgentHostID: agentHostID,
		Metrics:     metrics,
	}
	return b.cache.SetJSON(ctx, metricsBufferKey(agentHostID), entry, agentHostMetricsBufferTTL)
}

func (b *agentHostMetricsBuffer) Latest(ctx context.Context, agentHostID int64) (repository.AgentHostMetrics, bool, error) {
	if b == nil || b.cache == nil || agentHostID <= 0 {
		return repository.AgentHostMetrics{}, false, nil
	}
	entry := agentHostMetricsBufferEntry{}
	ok, err := b.cache.GetJSON(ctx, metricsBufferKey(agentHostID), &entry)
	if err != nil {
		b.cache.Delete(ctx, metricsBufferKey(agentHostID))
		return repository.AgentHostMetrics{}, false, err
	}
	if !ok || entry.AgentHostID != agentHostID {
		return repository.AgentHostMetrics{}, false, nil
	}
	return entry.Metrics, true, nil
}

func (b *agentHostMetricsBuffer) Flush(ctx context.Context) error {
	if b == nil || b.cache == nil || b.repo == nil {
		return nil
	}
	for _, key := range b.cache.Keys(ctx) {
		entry := agentHostMetricsBufferEntry{}
		ok, err := b.cache.GetJSON(ctx, key, &entry)
		if err != nil || !ok || entry.AgentHostID <= 0 {
			b.cache.Delete(ctx, key)
			continue
		}
		// Delete before UpdateMetrics to avoid TOCTOU race with concurrent Enqueue.
		b.cache.Delete(ctx, key)
		if err := b.repo.UpdateMetrics(ctx, entry.AgentHostID, entry.Metrics); err != nil {
			// Re-enqueue on failure so data is retried on next flush cycle
			_ = b.Enqueue(ctx, entry.AgentHostID, entry.Metrics)
			b.logger.Error("failed to flush agent host metrics", "agent_host_id", entry.AgentHostID, "error", err)
			continue
		}
	}
	return nil
}

func metricsBufferKey(agentHostID int64) string {
	return fmt.Sprintf("host:%d", agentHostID)
}
