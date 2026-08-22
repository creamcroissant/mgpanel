package mcp

import (
	"net/http"
	"strings"
)

// KeyValidator validates MCP API keys against stored keys.
type KeyValidator interface {
	Validate(rawKey string) (bool, error)
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

			// Try config key first
			if configKey != "" && token == configKey {
				next.ServeHTTP(w, r)
				return
			}

			// Try DB-backed validator
			if validator != nil {
				valid, err := validator.Validate(token)
				if err == nil && valid {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, `{"error":"invalid API key"}`, http.StatusUnauthorized)
		})
	}
}
