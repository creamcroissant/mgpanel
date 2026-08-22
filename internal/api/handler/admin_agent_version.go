package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
	"github.com/go-chi/chi/v5"
)

type AdminAgentVersionHandler struct {
	versions service.BinaryVersionService
	i18n     *i18n.Manager
}

func NewAdminAgentVersionHandler(versions service.BinaryVersionService, i18nMgr *i18n.Manager) *AdminAgentVersionHandler {
	return &AdminAgentVersionHandler{versions: versions, i18n: i18nMgr}
}

func (h *AdminAgentVersionHandler) requireAdmin(w http.ResponseWriter, r *http.Request, action string) bool {
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, action, "error.unauthorized", h.i18n)
		return false
	}
	return true
}

func (h *AdminAgentVersionHandler) ensureService(w http.ResponseWriter, r *http.Request, action string) bool {
	if h.versions != nil {
		return true
	}
	RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, action, "error.service_unavailable", h.i18n)
	return false
}

func (h *AdminAgentVersionHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	const action = "admin.agent_versions.list"
	if !h.requireAdmin(w, r, action) {
		return
	}
	if !h.ensureService(w, r, action) {
		return
	}
	agentHostID, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || agentHostID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return
	}
	states, err := h.versions.ListVersionStates(r.Context(), service.ListVersionStatesRequest{AgentHostID: agentHostID})
	if err != nil {
		h.respondServiceError(r.Context(), w, action, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": states})
}

func (h *AdminAgentVersionHandler) RefreshVersion(w http.ResponseWriter, r *http.Request) {
	const action = "admin.agent_versions.refresh"
	if !h.requireAdmin(w, r, action) {
		return
	}
	if !h.ensureService(w, r, action) {
		return
	}
	agentHostID, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || agentHostID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return
	}
	component := chi.URLParam(r, "component")
	state, err := h.versions.RefreshRemoteVersion(r.Context(), service.RefreshRemoteVersionRequest{AgentHostID: agentHostID, Component: component})
	if err != nil {
		h.respondServiceError(r.Context(), w, action, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": state})
}

func (h *AdminAgentVersionHandler) respondServiceError(ctx context.Context, w http.ResponseWriter, action string, err error) {
	status := http.StatusInternalServerError
	key := "error.internal_server_error"
	if errors.Is(err, service.ErrNotFound) {
		status = http.StatusNotFound
		key = "error.not_found"
	} else if errors.Is(err, service.ErrBadRequest) {
		status = http.StatusBadRequest
		key = "error.bad_request"
	} else if errors.Is(err, service.ErrNotImplemented) {
		status = http.StatusNotImplemented
		key = "error.service_unavailable"
	}
	RespondErrorI18nAction(ctx, w, status, action, key, h.i18n)
}
