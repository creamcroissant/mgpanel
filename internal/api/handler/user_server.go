// 文件路径: internal/api/handler/user_server.go
// 模块说明: 这是 internal 模块里的 user_server 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

// UserServerHandler exposes user node listing endpoints.
type UserServerHandler struct {
	Servers         service.ServerService
	ServerSelection service.UserServerSelectionService
	i18n            *i18n.Manager
}

func NewUserServerHandler(serverService service.ServerService, selectionService service.UserServerSelectionService, i18nMgr *i18n.Manager) *UserServerHandler {
	return &UserServerHandler{Servers: serverService, ServerSelection: selectionService, i18n: i18nMgr}
}

func (h *UserServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	action := userServerActionPath(r.URL.Path)
	switch {
	case action == "/fetch" && r.Method == http.MethodGet:
		h.handleFetch(w, r)
	case action == "/save" && r.Method == http.MethodPost:
		h.handleSaveSelection(w, r)
	case action == "/selection" && r.Method == http.MethodGet:
		h.handleGetSelection(w, r)
	default:
		respondNotImplemented(w, "user.server", r)
	}
}

func (h *UserServerHandler) handleSaveSelection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.ServerSelection == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "user.server.save", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.UserFromContext(ctx)
	if claims.ID == "" {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "user.server.save", "error.unauthorized", h.i18n)
		return
	}

	var req struct {
		ServerIDs []int64 `json:"server_ids"`
	}
	if err := decodeJSON(r, &req); err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "user.server.save", "error.bad_request", h.i18n)
		return
	}

	userID, err := parseInt64(claims.ID)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "user.server.save", "error.bad_request", h.i18n)
		return
	}
	if err := h.ServerSelection.UpdateSelection(ctx, userID, req.ServerIDs); err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "user.server.save", "error.internal_server_error", h.i18n)
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *UserServerHandler) handleGetSelection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.ServerSelection == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "user.server.selection", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.UserFromContext(ctx)
	if claims.ID == "" {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "user.server.selection", "error.unauthorized", h.i18n)
		return
	}

	userID, err := parseInt64(claims.ID)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "user.server.selection", "error.bad_request", h.i18n)
		return
	}
	ids, err := h.ServerSelection.GetSelection(ctx, userID)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "user.server.selection", "error.internal_server_error", h.i18n)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"data": ids})
}

func (h *UserServerHandler) handleFetch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.Servers == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "user.server.fetch", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.UserFromContext(ctx)
	if claims.ID == "" {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "user.server.fetch", "error.unauthorized", h.i18n)
		return
	}
	result, err := h.Servers.ListForUser(ctx, claims.ID)
	if err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(ctx, w, status, "user.server.fetch", key, h.i18n)
		return
	}
	if result == nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "user.server.fetch", "error.internal_server_error", h.i18n)
		return
	}
	etag := formatETag(result.ETag)
	if etag != "" {
		w.Header().Set("ETag", etag)
	}
	if requestETag := r.Header.Get("If-None-Match"); etag != "" && strings.Contains(requestETag, result.ETag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": result.Nodes})
}

func userServerActionPath(fullPath string) string {
	idx := strings.Index(fullPath, "/user/server")
	if idx == -1 {
		return "/"
	}
	action := fullPath[idx+len("/user/server"):]
	if action == "" || action == "/" {
		return "/"
	}
	if !strings.HasPrefix(action, "/") {
		action = "/" + action
	}
	return action
}
