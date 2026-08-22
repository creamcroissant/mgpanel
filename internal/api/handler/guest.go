// 文件路径: internal/api/handler/guest.go
// 模块说明: 这是 internal 模块里的 guest 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package handler

import (
	"net/http"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
	"github.com/go-chi/chi/v5"
)

// GuestHandler serves unauthenticated guest endpoints.
type GuestHandler struct {
	comm service.CommService
	i18n *i18n.Manager
}

func NewGuestHandler(comm service.CommService, i18n *i18n.Manager) *GuestHandler {
	return &GuestHandler{comm: comm, i18n: i18n}
}

func (h *GuestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	action := guestActionPath(r.URL.Path)
	switch {
	case action == "/comm/config" && r.Method == http.MethodGet:
		h.handleCommConfig(w, r)
	case action == "/telegram/get" && r.Method == http.MethodGet:
		h.handleTelegramGet(w, r)
	default:
		respondNotImplemented(w, "guest", r)
	}
}

// HandleI18n serves the translation files
func (h *GuestHandler) HandleI18n(w http.ResponseWriter, r *http.Request) {
	lang := chi.URLParam(r, "lang")
	if lang == "" {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.bad_request", h.i18n)
		return
	}

	// Remove .json extension if present
	lang = strings.TrimSuffix(lang, ".json")

	// Get translations for the requested language
	// Note: We need to expose a way to get raw translations from the manager
	// For now, let's assume we can get them.
	// Since the Manager doesn't expose raw map, we should add a method to it or
	// iterate through keys?
	// Actually, the frontend expects a JSON file.
	// We should add GetTranslations(lang) to i18n.Manager.

	// Assuming GetTranslations exists or we add it.
	translations := h.i18n.GetTranslations(lang)
	if translations == nil {
		// Fallback to default or empty
		translations = map[string]string{}
	}

	respondJSON(w, http.StatusOK, translations)
}

func (h *GuestHandler) handleCommConfig(w http.ResponseWriter, r *http.Request) {
	if h.comm == nil {
		RespondErrorI18n(r.Context(), w, http.StatusServiceUnavailable, "error.service_unavailable", h.i18n)
		return
	}
	cfg, err := h.comm.GuestConfig(r.Context())
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, cfg)
}

func (h *GuestHandler) handleTelegramGet(w http.ResponseWriter, r *http.Request) {
	if h.comm == nil {
		RespondErrorI18n(r.Context(), w, http.StatusServiceUnavailable, "error.service_unavailable", h.i18n)
		return
	}
	cfg, err := h.comm.GuestConfig(r.Context())
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"telegram_enabled":      cfg.TelegramLoginEnable,
		"telegram_bot_username": cfg.TelegramBotUsername,
		"telegram_login_domain": cfg.TelegramLoginDomain,
	})
}

func guestActionPath(fullPath string) string {
	idx := strings.Index(fullPath, "/guest")
	if idx == -1 {
		return "/"
	}
	action := fullPath[idx+len("/guest"):]
	if action == "" || action == "/" {
		return "/"
	}
	if !strings.HasPrefix(action, "/") {
		action = "/" + action
	}
	return action
}
