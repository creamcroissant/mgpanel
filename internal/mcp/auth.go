package mcp

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/mcp/tools"
)

// KeyValidator validates MCP API keys against stored keys and returns the granted scopes.
// scopes 为空表示拒绝；实现方负责校验哈希与启用状态。
type KeyValidator interface {
	Validate(rawKey string) (scopes []string, ok bool, err error)
}

// AuthMiddleware validates MCP API key from Authorization header.
// Supports both config-based key (fallback) and DB-backed key validation.
func AuthMiddleware(configKey string, validator KeyValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if configKey == "" && validator == nil {
				http.Error(w, `{"error":"MCP not configured"}`, http.StatusServiceUnavailable)
				return
			}
			auth := strings.TrimSpace(r.Header.Get("Authorization"))
			if auth == "" {
				http.Error(w, `{"error":"missing Authorization header"}`, http.StatusUnauthorized)
				return
			}
			token := auth
			if len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
				token = strings.TrimSpace(auth[7:])
			}

			// Try config key first：服务端静态密钥为完全信任，授予全部作用域
			if configKey != "" && subtle.ConstantTimeCompare([]byte(token), []byte(configKey)) == 1 {
				next.ServeHTTP(w, r.WithContext(tools.WithScopes(r.Context(), []string{tools.ScopeRead, tools.ScopeOps})))
				return
			}

			// Try DB-backed validator
			if validator != nil {
				scopes, valid, err := validator.Validate(token)
				if err == nil && valid {
					next.ServeHTTP(w, r.WithContext(tools.WithScopes(r.Context(), scopes)))
					return
				}
			}

			http.Error(w, `{"error":"invalid API key"}`, http.StatusUnauthorized)
		})
	}
}
