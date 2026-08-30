// 文件路径: internal/api/requestctx/user.go
// 模块说明: 这是 internal 模块里的 user 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package requestctx

import (
	"context"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// UserClaims stores minimal auth info derived from middleware/user guard.
type UserClaims struct {
	ID    string
	Email string
}

// AdminClaims captures admin guard metadata.
type AdminClaims struct {
	ID    string
	Email string
}

// ServerClaims captures node-related information for server guard.
type ServerClaims struct {
	ID     string
	Type   string
	Server *repository.Server
}

type contextKey string

const (
	userContextKey   contextKey = "mgpanel-user"
	adminContextKey  contextKey = "mgpanel-admin"
	serverContextKey contextKey = "mgpanel-server"
)

// I18nKey 用于在 context 中存储语言标识的 key 类型。
type I18nKey struct{}

// WithLanguage 将语言标识附加到 context 中供下游使用。
func WithLanguage(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, I18nKey{}, lang)
}

// GetLanguage 从 context 中获取语言标识，若未设置则返回默认值 "en-US"。
func GetLanguage(ctx context.Context) string {
	if ctx == nil {
		return "en-US"
	}
	if lang, ok := ctx.Value(I18nKey{}).(string); ok {
		return lang
	}
	return "en-US"
}

// WithUserClaims attaches user data to the context for downstream handlers.
func WithUserClaims(ctx context.Context, claims UserClaims) context.Context {
	return context.WithValue(ctx, userContextKey, claims)
}

// UserFromContext fetches user claims, returning zero value if missing.
func UserFromContext(ctx context.Context) UserClaims {
	if ctx == nil {
		return UserClaims{}
	}
	claims, _ := ctx.Value(userContextKey).(UserClaims)
	return claims
}

// WithAdminClaims attaches admin data to context.
func WithAdminClaims(ctx context.Context, claims AdminClaims) context.Context {
	return context.WithValue(ctx, adminContextKey, claims)
}

// AdminFromContext fetches admin claims or zero value.
func AdminFromContext(ctx context.Context) AdminClaims {
	if ctx == nil {
		return AdminClaims{}
	}
	claims, _ := ctx.Value(adminContextKey).(AdminClaims)
	return claims
}

// WithServerClaims attaches server data to context.
func WithServerClaims(ctx context.Context, claims ServerClaims) context.Context {
	return context.WithValue(ctx, serverContextKey, claims)
}

// ServerFromContext fetches node claims or zero value.
func ServerFromContext(ctx context.Context) ServerClaims {
	if ctx == nil {
		return ServerClaims{}
	}
	claims, _ := ctx.Value(serverContextKey).(ServerClaims)
	return claims
}
