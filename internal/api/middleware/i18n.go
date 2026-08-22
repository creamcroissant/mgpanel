package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
	"golang.org/x/text/language"
)

// I18n middleware detects the user's preferred language and stores it in the context
func I18n(manager *i18n.Manager) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check query param first
			lang := r.URL.Query().Get("lang")

			// Then check header
			if lang == "" {
				// Check custom header first (from frontend interceptor)
				lang = r.Header.Get("X-I18N-Lang")
			}

			if lang == "" {
				// Then check cookie
				if cookie, err := r.Cookie("i18next"); err == nil {
					lang = cookie.Value
				}
			}

			if lang == "" {
				// Finally check Accept-Language
				accept := r.Header.Get("Accept-Language")
				tags, _, err := language.ParseAcceptLanguage(accept)
				if err == nil && len(tags) > 0 {
					lang = tags[0].String()
				}
			}

			// Default fallback is handled by the manager if lang is empty or invalid
			if lang == "" {
				lang = "en-US"
			}

			// Normalize language tag (e.g. zh-cn -> zh-CN)
			// This is important because file names are case sensitive on some FS
			// and we use exact string matching for now.
			// Ideally we should use language.Matcher.
			// For simplicity let's do basic mapping if needed, but the Manager.Translate
			// also does some normalization.
			// However, for returning to frontend, we might want to be consistent.

			// Store language in context using requestctx
			ctx := requestctx.WithLanguage(r.Context(), lang)

			// Set cookie if query param was present to persist selection
			if r.URL.Query().Get("lang") != "" {
				http.SetCookie(w, &http.Cookie{
					Name:     "i18next",
					Value:    lang,
					Path:     "/",
					Expires:  time.Now().Add(365 * 24 * time.Hour),
					HttpOnly: false, // Allow JS access for i18next
				})
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetLanguage 为保持向后兼容，委托给 requestctx.GetLanguage。
// 推荐直接使用 requestctx.GetLanguage。
func GetLanguage(ctx context.Context) string {
	return requestctx.GetLanguage(ctx)
}