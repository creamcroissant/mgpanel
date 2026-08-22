package transport

import (
	"context"
	"errors"
	"time"

	"github.com/cenkalti/backoff/v4"
)

// RetryConfig 控制 gRPC 重试策略。
type RetryConfig struct {
	Enabled         bool
	MaxRetries      int
	InitialInterval time.Duration
	MaxInterval     time.Duration
	Multiplier      float64
}

// DefaultRetryConfig 返回默认重试配置。
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		Enabled:         true,
		MaxRetries:      3,
		InitialInterval: 500 * time.Millisecond,
		MaxInterval:     5 * time.Second,
		Multiplier:      2,
	}
}

// CriticalRetryConfig 返回更激进的重试配置。
func CriticalRetryConfig() RetryConfig {
	return RetryConfig{
		Enabled:         true,
		MaxRetries:      5,
		InitialInterval: 200 * time.Millisecond,
		MaxInterval:     10 * time.Second,
		Multiplier:      2,
	}
}

// normalizeRetryConfig 填补缺省值，确保配置可用。
func normalizeRetryConfig(cfg RetryConfig) RetryConfig {
	if cfg.InitialInterval == 0 {
		cfg.InitialInterval = 500 * time.Millisecond
	}
	if cfg.MaxInterval == 0 {
		cfg.MaxInterval = 5 * time.Second
	}
	if cfg.Multiplier == 0 {
		cfg.Multiplier = 2
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	return cfg
}

// DoWithRetry 按配置执行重试，并在不可重试/超限时退出。
func DoWithRetry(ctx context.Context, cfg RetryConfig, fn func(ctx context.Context) error) error {
	if !cfg.Enabled {
		return fn(ctx)
	}
	cfg = normalizeRetryConfig(cfg)

	backoffCfg := backoff.NewExponentialBackOff()
	backoffCfg.InitialInterval = cfg.InitialInterval
	backoffCfg.MaxInterval = cfg.MaxInterval
	backoffCfg.Multiplier = cfg.Multiplier
	backoffCfg.MaxElapsedTime = 0

	attempts := 0
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := fn(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			if !IsRetryable(err) {
				return err
			}
			if attempts >= cfg.MaxRetries {
				return err
			}
			attempts++

			wait := backoffCfg.NextBackOff()
			if wait == backoff.Stop {
				return err
			}
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return ctx.Err()
			case <-timer.C:
				continue
			}
		}
		return nil
	}
}
