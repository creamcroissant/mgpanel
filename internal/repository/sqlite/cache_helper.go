// 文件路径: internal/repository/sqlite/cache_helper.go
// 模块说明: 为热点 repository 提供统一的缓存读写与失效辅助，避免各 repo 重复实现。
package sqlite

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/creamcroissant/mgpanel/internal/cache"
)

// repoCache 封装带命名空间的缓存访问，key 自动加 repo 前缀避免冲突。
type repoCache struct {
	store  cache.Store
	prefix string
}

// cached returns true and fills dest when a cache entry exists.
func (c *repoCache) cached(ctx context.Context, key string, dest any) (bool, error) {
	if c == nil || c.store == nil {
		return false, nil
	}
	return c.store.Namespace(c.prefix).GetJSON(ctx, key, dest)
}

// storeJSON caches value under key with ttl.
func (c *repoCache) storeJSON(ctx context.Context, key string, value any, ttl time.Duration) {
	if c == nil || c.store == nil {
		return
	}
	_ = c.store.Namespace(c.prefix).SetJSON(ctx, key, value, ttl)
}

// evict removes a key (or prefix) from the cache.
func (c *repoCache) evict(ctx context.Context, key string) {
	if c == nil || c.store == nil {
		return
	}
	c.store.Namespace(c.prefix).Delete(ctx, key)
}

// evictAll removes every key under the repo namespace.
func (c *repoCache) evictAll(ctx context.Context) {
	if c == nil || c.store == nil {
		return
	}
	for _, key := range c.store.Namespace(c.prefix).Keys(ctx) {
		c.store.Namespace(c.prefix).Delete(ctx, key)
	}
}

// ptrInt64 renders a *int64 as a stable string for cache keys.
func ptrInt64(v *int64) string {
	if v == nil {
		return "nil"
	}
	return fmt.Sprintf("%d", *v)
}

// cacheDebugLog logs cache events at debug level for observability.
func cacheDebugLog(event, namespace, key string) {
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("repo cache", "event", event, "namespace", namespace, "key", key)
	}
}
