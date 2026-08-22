package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

// Helper to respond with JSON
func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Warn("failed to encode response JSON", "error", err)
	}
}

// Helper to respond with translated JSON
func respondJSONTranslated(ctx context.Context, w http.ResponseWriter, status int, payload any, i18nMgr *i18n.Manager) {
	lang := requestctx.GetLanguage(ctx)

	// If payload is a map, try to translate values that look like keys?
	// For now, let's just assume the payload structure is handled by the caller or we translate specific fields.
	// A common pattern is to have a Message field.

	if m, ok := payload.(map[string]any); ok {
		if msg, ok := m["message"].(string); ok {
			m["message"] = i18nMgr.Translate(lang, msg)
		}
		if msg, ok := m["error"].(string); ok {
			// If error is a key, translate it.
			// But usually error contains dynamic info.
			// Let's assume for now we pass keys for static errors.
			m["error"] = i18nMgr.Translate(lang, msg)
		}
	}

	respondJSON(w, status, payload)
}

func respondNotImplemented(w http.ResponseWriter, namespace string, r *http.Request) {
	respondNotImplementedI18n(r.Context(), w, namespace, r.Method, r.URL.Path, nil)
}

func respondNotImplementedI18n(ctx context.Context, w http.ResponseWriter, namespace string, method string, path string, i18nMgr *i18n.Manager) {
	lang := requestctx.GetLanguage(ctx)
	slog.Warn("not implemented endpoint called", "namespace", namespace, "method", method, "path", path)
	message := "error.not_implemented"
	if i18nMgr != nil {
		message = i18nMgr.Translate(lang, message)
	}
	respondJSON(w, http.StatusNotImplemented, map[string]any{
		"message": message,
	})
}

func respondError(w http.ResponseWriter, status int, action string, err error) {
	if status >= 500 {
		slog.Error("api: respondError", "action", action, "error", err)
	} else {
		slog.Warn("api: respondError", "action", action, "error", err)
	}
	respondJSON(w, status, map[string]any{
		"error":  "internal error",
		"action": action,
	})
}

func RespondErrorI18nAction(ctx context.Context, w http.ResponseWriter, status int, action string, key string, i18nMgr *i18n.Manager, args ...interface{}) {
	if key == "" {
		key = action
	}
	if status >= 500 {
		slog.Error("handler internal error", "action", action, "key", key, "status", status)
	}
	lang := requestctx.GetLanguage(ctx)
	var msg string
	if i18nMgr != nil {
		msg = i18nMgr.Translate(lang, key, args...)
	} else {
		msg = key
	}
	resp := map[string]any{
		"error": msg,
	}
	if action != "" {
		resp["action"] = action
	}
	respondJSON(w, status, resp)
}

func respondAgentOperationBusy(ctx context.Context, w http.ResponseWriter, action string, err error, i18nMgr *i18n.Manager) bool {
	if !errors.Is(err, service.ErrAgentOperationBusy) {
		return false
	}
	blocker, ok := service.OperationBlockerFromError(err)
	if !ok || blocker == nil {
		RespondErrorI18nAction(ctx, w, http.StatusConflict, action, "error.conflict", i18nMgr)
		return true
	}
	message := "error.conflict"
	if i18nMgr != nil {
		message = i18nMgr.Translate(requestctx.GetLanguage(ctx), message)
	}
	respondJSON(w, http.StatusConflict, map[string]any{
		"error":  message,
		"action": action,
		"details": map[string]any{
			"blocker": blocker,
		},
	})
	return true
}

// New helper for i18n error responses
func RespondErrorI18n(ctx context.Context, w http.ResponseWriter, status int, key string, i18nMgr *i18n.Manager, args ...interface{}) {
	if status >= 500 {
		slog.Error("handler internal error", "key", key, "status", status)
	}
	lang := requestctx.GetLanguage(ctx)
	var msg string
	if i18nMgr != nil {
		msg = i18nMgr.Translate(lang, key, args...)
	} else {
		msg = key // Fallback if manager is missing (e.g. in tests)
	}
	respondJSON(w, status, map[string]any{
		"error": msg,
	})
}

// New helper for i18n success responses
func RespondSuccessI18n(ctx context.Context, w http.ResponseWriter, key string, i18nMgr *i18n.Manager, data any) {
	lang := requestctx.GetLanguage(ctx)
	var msg string
	if i18nMgr != nil {
		msg = i18nMgr.Translate(lang, key)
	} else {
		msg = key // Fallback
	}

	resp := map[string]any{
		"message": msg,
	}
	if data != nil {
		resp["data"] = data
	}

	respondJSON(w, http.StatusOK, resp)
}
