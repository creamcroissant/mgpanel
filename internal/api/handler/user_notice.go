package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

// UserNoticeHandler exposes user notice endpoints.
type UserNoticeHandler struct {
	notices service.UserNoticeService
	i18n    *i18n.Manager
}

// NewUserNoticeHandler constructs a user notice handler.
func NewUserNoticeHandler(notices service.UserNoticeService, i18nMgr *i18n.Manager) *UserNoticeHandler {
	return &UserNoticeHandler{notices: notices, i18n: i18nMgr}
}

func (h *UserNoticeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	action := userNoticeActionPath(r.URL.Path)
	switch {
	case action == "/unread" && r.Method == http.MethodGet:
		h.handleUnread(w, r)
	case action == "/fetch" && r.Method == http.MethodGet:
		h.handleUnread(w, r)
	case action == "/read" && r.Method == http.MethodPost:
		h.handleRead(w, r)
	default:
		respondNotImplemented(w, "user.notice", r)
	}
}

func (h *UserNoticeHandler) handleUnread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.notices == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "user.notice.unread", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.UserFromContext(ctx)
	if claims.ID == "" {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "user.notice.unread", "error.unauthorized", h.i18n)
		return
	}
	notice, err := h.notices.GetUnreadPopupNotice(ctx, claims.ID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			respondJSON(w, http.StatusOK, map[string]any{"data": nil})
			return
		}
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "user.notice.unread", "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": notice})
}

func (h *UserNoticeHandler) handleRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.notices == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "user.notice.read", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.UserFromContext(ctx)
	if claims.ID == "" {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "user.notice.read", "error.unauthorized", h.i18n)
		return
	}
	var payload struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.ID <= 0 {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "user.notice.read", "error.bad_request", h.i18n)
		return
	}
	if err := h.notices.MarkNoticeRead(ctx, claims.ID, payload.ID); err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(ctx, w, status, "user.notice.read", key, h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func userNoticeActionPath(fullPath string) string {
	idx := strings.Index(fullPath, "/notice")
	if idx == -1 {
		return "/"
	}
	action := fullPath[idx+len("/notice"):]
	if action == "" || action == "/" {
		return "/"
	}
	if !strings.HasPrefix(action, "/") {
		action = "/" + action
	}
	return action
}
