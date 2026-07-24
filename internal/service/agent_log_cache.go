package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/creamcroissant/xboard/internal/cache"
)

const (
	agentLogCacheNamespace       = "agent_logs"
	agentLogCacheDefaultMaxLines = 50
	agentLogCacheDefaultTTL      = 30 * time.Minute
)

// AgentLogLine represents a single log line from an agent.
type AgentLogLine struct {
	Timestamp int64  `json:"ts"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Source    string `json:"source"`
}

// AgentLogCache stores recent agent logs in memory (per-agent FIFO).
// Writes to go-cache, never touches the database.
type AgentLogCache struct {
	store    cache.Store
	maxLines int
	mu       sync.Mutex
}

// NewAgentLogCache creates an AgentLogCache.
// maxLines is the max log lines per agent host (default 50).
func NewAgentLogCache(store cache.Store, maxLines int) *AgentLogCache {
	if store == nil {
		return nil
	}
	if maxLines <= 0 {
		maxLines = agentLogCacheDefaultMaxLines
	}
	return &AgentLogCache{
		store:    store.Namespace(agentLogCacheNamespace),
		maxLines: maxLines,
	}
}

func agentLogCacheKey(hostID int64) string {
	return fmt.Sprintf("host:%d", hostID)
}

// Push adds log entries for an agent host, maintaining FIFO within maxLines.
func (c *AgentLogCache) Push(ctx context.Context, hostID int64, entries []AgentLogLine) {
	if c == nil || c.store == nil || hostID <= 0 || len(entries) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	key := agentLogCacheKey(hostID)
	var existing []AgentLogLine
	if _, err := c.store.GetJSON(ctx, key, &existing); err != nil {
		existing = nil
	}
	existing = append(existing, entries...)
	if len(existing) > c.maxLines {
		existing = existing[len(existing)-c.maxLines:]
	}
	_ = c.store.SetJSON(ctx, key, existing, agentLogCacheDefaultTTL)
}

// Fetch retrieves log entries for an agent host, with optional level filter and limit.
func (c *AgentLogCache) Fetch(ctx context.Context, hostID int64, level string, limit int) ([]AgentLogLine, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("agent log cache not configured")
	}
	if hostID <= 0 {
		return nil, fmt.Errorf("invalid agent host id")
	}
	key := agentLogCacheKey(hostID)
	var entries []AgentLogLine
	ok, err := c.store.GetJSON(ctx, key, &entries)
	if err != nil {
		return nil, err
	}
	if !ok {
		return []AgentLogLine{}, nil
	}
	level = strings.TrimSpace(strings.ToLower(level))
	if level != "" {
		filtered := make([]AgentLogLine, 0, len(entries))
		for _, e := range entries {
			if strings.EqualFold(e.Level, level) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}
	if limit > 0 && limit < len(entries) {
		return entries[len(entries)-limit:], nil
	}
	return entries, nil
}

// Count returns total entries stored for a given host.
func (c *AgentLogCache) Count(ctx context.Context, hostID int64) int {
	if c == nil || c.store == nil || hostID <= 0 {
		return 0
	}
	key := agentLogCacheKey(hostID)
	var entries []AgentLogLine
	ok, err := c.store.GetJSON(ctx, key, &entries)
	if err != nil || !ok {
		return 0
	}
	return len(entries)
}
