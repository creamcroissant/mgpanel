// 文件路径: internal/cache/store.go
// 模块说明: 这是 internal 模块里的 store 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

// Store 定义鉴权、限流与验证码流程共用的缓存接口。
// Metrics exposes cache hit/miss counters for observability.
type Metrics struct {
	Hits   atomic.Int64
	Misses atomic.Int64
}

func (m *Metrics) Snapshot() (hits, misses int64) {
	if m == nil {
		return 0, 0
	}
	return m.Hits.Load(), m.Misses.Load()
}

// StatsGetter is an optional interface that cache implementations
// may implement to expose hit/miss counters for observability.
type StatsGetter interface {
	Stats() *Metrics
}

// StoreStats returns the cache metrics if the store supports it.
func StoreStats(s Store) (hits, misses int64, ok bool) {
	if sg, ok := s.(StatsGetter); ok {
		m := sg.Stats()
		if m != nil {
			return m.Hits.Load(), m.Misses.Load(), true
		}
	}
	return 0, 0, false
}

type Store interface {
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	SetString(ctx context.Context, key, value string, ttl time.Duration) error
	SetBytes(ctx context.Context, key string, value []byte, ttl time.Duration) error
	SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error
	Get(ctx context.Context, key string) (any, bool)
	GetString(ctx context.Context, key string) (string, bool)
	GetBytes(ctx context.Context, key string) ([]byte, bool)
	GetJSON(ctx context.Context, key string, dest any) (bool, error)
	Delete(ctx context.Context, key string)
	TTL(ctx context.Context, key string) (time.Duration, bool)
	Namespace(prefix string) Store
	Keys(ctx context.Context) []string

	// Increment adds delta to the stored integer, returning the updated value.
	Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error)
}

// Options 配置内存缓存行为。
type Options struct {
	DefaultTTL      time.Duration
	CleanupInterval time.Duration
	Prefix          string
}

// NewStore 创建基于 go-cache 的缓存实现，并支持命名空间。
func NewStore(opts Options) Store {
	defaultTTL := opts.DefaultTTL
	if defaultTTL <= 0 {
		defaultTTL = 5 * time.Minute
	}
	cleanup := opts.CleanupInterval
	if cleanup <= 0 {
		cleanup = defaultTTL
	}
	backend := gocache.New(defaultTTL, cleanup)

	metrics := &Metrics{}
	return &goCacheStore{
		backend:    backend,
		defaultTTL: defaultTTL,
		prefix:     normalizePrefix(opts.Prefix),
		metrics:    metrics,
	}
}

type goCacheStore struct {
	backend    *gocache.Cache
	defaultTTL time.Duration
	prefix     string
	metrics    *Metrics
}

func (s *goCacheStore) Set(_ context.Context, key string, value any, ttl time.Duration) error {
	s.backend.Set(s.prefixed(key), value, s.normalizeTTL(ttl))
	return nil
}

func (s *goCacheStore) SetString(ctx context.Context, key, value string, ttl time.Duration) error {
	return s.Set(ctx, key, value, ttl)
}

func (s *goCacheStore) SetBytes(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if value == nil {
		return s.Set(ctx, key, []byte{}, ttl)
	}
	buf := make([]byte, len(value))
	copy(buf, value)
	return s.Set(ctx, key, buf, ttl)
}

func (s *goCacheStore) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.SetBytes(ctx, key, data, ttl)
}

func (s *goCacheStore) Get(_ context.Context, key string) (any, bool) {
	v, ok := s.backend.Get(s.prefixed(key))
	if s.metrics != nil {
		if ok {
			s.metrics.Hits.Add(1)
		} else {
			s.metrics.Misses.Add(1)
		}
	}
	return v, ok
}

func (s *goCacheStore) GetString(ctx context.Context, key string) (string, bool) {
	raw, ok := s.Get(ctx, key)
	if !ok {
		return "", false
	}
	switch v := raw.(type) {
	case string:
		return v, true
	case []byte:
		return string(v), true
	}
	return "", false
}

func (s *goCacheStore) GetBytes(ctx context.Context, key string) ([]byte, bool) {
	raw, ok := s.Get(ctx, key)
	if !ok {
		return nil, false
	}
	switch v := raw.(type) {
	case []byte:
		buf := make([]byte, len(v))
		copy(buf, v)
		return buf, true
	case string:
		return []byte(v), true
	}
	return nil, false
}

func (s *goCacheStore) GetJSON(ctx context.Context, key string, dest any) (bool, error) {
	raw, ok := s.GetBytes(ctx, key)
	if !ok {
		return false, nil
	}
	if dest == nil {
		return true, nil
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return false, err
	}
	return true, nil
}

func (s *goCacheStore) Delete(_ context.Context, key string) {
	s.backend.Delete(s.prefixed(key))
}

func (s *goCacheStore) TTL(_ context.Context, key string) (time.Duration, bool) {
	_, exp, ok := s.backend.GetWithExpiration(s.prefixed(key))
	if !ok || exp.IsZero() {
		return 0, false
	}
	ttl := time.Until(exp)
	if ttl < 0 {
		return 0, false
	}
	return ttl, true
}

func (s *goCacheStore) Namespace(prefix string) Store {
	return &goCacheStore{
		backend:    s.backend,
		defaultTTL: s.defaultTTL,
		prefix:     joinPrefixes(s.prefix, prefix),
		metrics:    s.metrics,
	}
}

func (s *goCacheStore) Keys(_ context.Context) []string {
	items := s.backend.Items()
	keys := make([]string, 0, len(items))
	prefix := s.prefix
	for key := range items {
		if prefix != "" {
			prefixValue := prefix + ":"
			if !strings.HasPrefix(key, prefixValue) {
				continue
			}
			keys = append(keys, strings.TrimPrefix(key, prefixValue))
			continue
		}
		keys = append(keys, key)
	}
	return keys
}

// Stats returns the cache metrics (hit/miss counters) for observability.
func (s *goCacheStore) Stats() *Metrics {
	return s.metrics
}

func (s *goCacheStore) Increment(_ context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return 0, nil
	}
	normalizedTTL := s.normalizeTTL(ttl)
	// Miss 时直接写入 delta 并返回，避免 Set(0)+Increment 两步之间
	// 被并发首击穿插而稀释计数（限流/登录失败锁定依赖该语义）。
	if _, ok := s.backend.Get(s.prefixed(trimmed)); !ok {
		s.backend.Set(s.prefixed(trimmed), int64(delta), normalizedTTL)
		return delta, nil
	}
	if err := s.backend.Increment(s.prefixed(trimmed), delta); err != nil {
		return 0, fmt.Errorf("cache increment failed: %w", err)
	}
	raw, ok := s.backend.Get(s.prefixed(trimmed))
	if !ok {
		return 0, nil
	}
	current, ok := raw.(int64)
	if !ok {
		return 0, fmt.Errorf("cache increment returned non-int64")
	}
	s.backend.Set(s.prefixed(trimmed), current, normalizedTTL)
	return current, nil
}

func (s *goCacheStore) prefixed(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return s.prefix
	}
	if s.prefix == "" {
		return key
	}
	return s.prefix + ":" + key
}

func (s *goCacheStore) normalizeTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return s.defaultTTL
	}
	return ttl
}

func normalizePrefix(prefix string) string {
	trimmed := strings.Trim(prefix, ": ")
	return trimmed
}

func joinPrefixes(parts ...string) string {
	var normalized []string
	for _, part := range parts {
		trimmed := normalizePrefix(part)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return strings.Join(normalized, ":")
}
