// 文件路径: internal/bootstrap/infra.go
// 模块说明: 这是 internal 模块里的 infra 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package bootstrap

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/creamcroissant/mgpanel/internal/auth/token"
	"github.com/creamcroissant/mgpanel/internal/cache"
	"github.com/creamcroissant/mgpanel/internal/notifier"
	"github.com/creamcroissant/mgpanel/internal/security"
	"github.com/creamcroissant/mgpanel/internal/support/hash"
)

// Infrastructure bundles shared helpers required by auth-related services.
type Infrastructure struct {
	Cache       cache.Store
	Token       *token.Manager
	Hasher      hash.Hasher
	Notifier    notifier.Service
	RateLimiter *security.RateLimiter
	Audit       security.Recorder
}

// BuildInfrastructure wires default implementations for cache/token/hash/notification helpers.
func BuildInfrastructure(cfg *Config, logger *slog.Logger) (*Infrastructure, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required / 配置不能为空")
	}

	cacheStore := cache.NewStore(cache.Options{
		Prefix:          "mgpanel",
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: time.Minute,
	})

	if cfg.Auth.SigningKey == "change-me" {
		return nil, fmt.Errorf("auth.signing_key must be changed from default value")
	}

	tokenManager, err := token.NewManager(token.Options{
		SigningKey: []byte(cfg.Auth.SigningKey),
		Issuer:     cfg.Auth.Issuer,
		Audience:   cfg.Auth.Audience,
		TTL:        cfg.Auth.TokenTTL,
		Leeway:     cfg.Auth.Leeway,
	})
	if err != nil {
		return nil, fmt.Errorf("token manager: %w", err)
	}

	hasher, err := hash.NewBcryptHasher(cfg.Auth.BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("bcrypt hasher: %w", err)
	}

	rateLimiter, err := security.NewRateLimiter(cacheStore)
	if err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	notif := notifier.NewLoggerService(logger)
	audit := security.NewLoggerRecorder(logger)

	return &Infrastructure{
		Cache:       cacheStore,
		Token:       tokenManager,
		Hasher:      hasher,
		Notifier:    notif,
		RateLimiter: rateLimiter,
		Audit:       audit,
	}, nil
}
