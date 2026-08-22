// 文件路径: internal/security/ratelimiter.go
// 模块说明: 这是 internal 模块里的 ratelimiter 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package security

import (
	"context"
	"fmt"
	"time"

	"github.com/creamcroissant/mgpanel/internal/cache"
)

// RateLimiter 控制重复行为（如 OTP 请求或登录尝试）。
type RateLimiter struct {
	store cache.Store
}

// RateResult 描述 Allow 调用的结果。
type RateResult struct {
	Allowed   bool
	Remaining int
	ResetAt   time.Time
}

// rateBucket 在缓存中保存请求计数。
type rateBucket struct {
	Count     int   `json:"count"`
	ExpiresAt int64 `json:"expires_at"`
}

// NewRateLimiter 使用缓存存储构建限流器。
func NewRateLimiter(store cache.Store) (*RateLimiter, error) {
	if store == nil {
		return nil, fmt.Errorf("rate limiter requires cache store / 限流器需要缓存存储")
	}
	return &RateLimiter{store: store.Namespace("rate")}, nil
}

// Allow 判断指定 key 是否可以在当前限额内继续执行。
func (l *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (RateResult, error) {
	if l == nil {
		return RateResult{}, fmt.Errorf("rate limiter not initialized / 限流器未初始化")
	}
	if limit <= 0 {
		return RateResult{}, fmt.Errorf("limit must be positive / limit 必须为正数")
	}
	if window <= 0 {
		window = time.Minute
	}

	now := time.Now().UTC()
	ttl := window
	if remain, ok := l.store.TTL(ctx, key); ok && remain > 0 {
		ttl = remain
	}

	current, err := l.store.Increment(ctx, key, 1, ttl)
	if err != nil {
		return RateResult{}, fmt.Errorf("increment rate limit counter failed: %v / 限流计数自增失败: %w", err, err)
	}

	remaining := limit - int(current)
	if remaining < 0 {
		remaining = 0
	}

	allowed := current <= int64(limit)
	resetAt := now.Add(ttl)
	return RateResult{
		Allowed:   allowed,
		Remaining: remaining,
		ResetAt:   resetAt,
	}, nil
}

// Reset 清除指定 key 的计数。
func (l *RateLimiter) Reset(ctx context.Context, key string) {
	if l == nil {
		return
	}
	l.store.Delete(ctx, key)
}
