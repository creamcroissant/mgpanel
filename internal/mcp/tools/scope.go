package tools

import (
	"context"
	"strings"
)

// 作用域：read=只读工具；ops=写操作工具。鉴权中间件在请求上下文写入授予的作用域。
const (
	ScopeRead = "read"
	ScopeOps  = "ops"
)

type scopeCtxKey struct{}

// WithScopes 将授予的作用域写入上下文。
func WithScopes(ctx context.Context, scopes []string) context.Context {
	return context.WithValue(ctx, scopeCtxKey{}, normalizeScopes(scopes))
}

// ScopesFrom 读取上下文中的作用域。
func ScopesFrom(ctx context.Context) []string {
	if ctx == nil {
		return nil
	}
	scopes, _ := ctx.Value(scopeCtxKey{}).([]string)
	return scopes
}

// HasScope 判断上下文是否具备指定作用域（required 为空视为公开）。
func HasScope(ctx context.Context, required string) bool {
	required = strings.ToLower(strings.TrimSpace(required))
	if required == "" {
		return true
	}
	for _, sc := range ScopesFrom(ctx) {
		if sc == required {
			return true
		}
	}
	return false
}

func normalizeScopes(scopes []string) []string {
	seen := make(map[string]struct{}, len(scopes))
	out := make([]string, 0, len(scopes))
	for _, sc := range scopes {
		sc = strings.ToLower(strings.TrimSpace(sc))
		if sc != ScopeRead && sc != ScopeOps {
			continue
		}
		if _, ok := seen[sc]; ok {
			continue
		}
		seen[sc] = struct{}{}
		out = append(out, sc)
	}
	return out
}
