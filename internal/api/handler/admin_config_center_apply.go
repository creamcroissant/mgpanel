package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
	"github.com/go-chi/chi/v5"
)

// AdminConfigCenterApplyHandler exposes admin endpoints for apply run operations.
type AdminConfigCenterApplyHandler struct {
	apply service.ApplyOrchestratorService
	i18n  *i18n.Manager
}

// NewAdminConfigCenterApplyHandler creates a config-center apply handler.
func NewAdminConfigCenterApplyHandler(apply service.ApplyOrchestratorService, i18nMgr *i18n.Manager) *AdminConfigCenterApplyHandler {
	return &AdminConfigCenterApplyHandler{apply: apply, i18n: i18nMgr}
}

type createApplyRunRequest struct {
	AgentHostID      int64  `json:"agent_host_id"`
	CoreType         string `json:"core_type"`
	TargetRevision   int64  `json:"target_revision"`
	PreviousRevision int64  `json:"previous_revision,omitempty"`
}

func (h *AdminConfigCenterApplyHandler) requireAdmin(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.config_center.apply.auth", "error.unauthorized", h.i18n)
		return 0, false
	}
	adminID, err := parseInt64(claims.ID)
	if err != nil {
		adminID = 0
	}
	return adminID, true
}

func (h *AdminConfigCenterApplyHandler) ensureService(w http.ResponseWriter, r *http.Request, action string) bool {
	if h.apply != nil {
		return true
	}
	RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, action, "error.service_unavailable", h.i18n)
	return false
}

// CreateApplyRun handles POST /api/v2/{securePath}/config-center/apply-runs.
func (h *AdminConfigCenterApplyHandler) CreateApplyRun(w http.ResponseWriter, r *http.Request) {
	adminID, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	if !h.ensureService(w, r, "admin.config_center.apply.create") {
		return
	}

	var payload createApplyRunRequest
	if err := decodeJSON(r, &payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.config_center.apply.create", "error.bad_request", h.i18n)
		return
	}

	run, err := h.apply.PrepareApplyRun(r.Context(), service.PrepareApplyRunRequest{
		AgentHostID:      payload.AgentHostID,
		CoreType:         payload.CoreType,
		TargetRevision:   payload.TargetRevision,
		PreviousRevision: payload.PreviousRevision,
		OperatorID:       adminID,
	})
	if err != nil {
		h.respondApplyError(r.Context(), w, "admin.config_center.apply.create", err)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]any{
		"data": run,
	})
}

// ListApplyRuns handles GET /api/v2/{securePath}/config-center/apply-runs.


// CancelApplyRun handles POST /api/v2/{securePath}/config-center/apply-runs/{run_id}/cancel.
func (h *AdminConfigCenterApplyHandler) CancelApplyRun(w http.ResponseWriter, r *http.Request) {
	adminID, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	if !h.ensureService(w, r, "admin.config_center.apply.cancel") {
		return
	}

	runID := chi.URLParam(r, "run_id")
	if runID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.config_center.apply.cancel", "error.bad_request", h.i18n)
		return
	}

	var payload struct {
		AgentHostID int64 `json:"agent_host_id"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.config_center.apply.cancel", "error.bad_request", h.i18n)
		return
	}

	if err := h.apply.CancelApplyRun(r.Context(), service.CancelApplyRunRequest{
		AgentHostID: payload.AgentHostID,
		RunID:       runID,
		OperatorID:  adminID,
	}); err != nil {
		h.respondApplyError(r.Context(), w, "admin.config_center.apply.cancel", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"cancelled": true}})
}
func (h *AdminConfigCenterApplyHandler) ListApplyRuns(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if !h.ensureService(w, r, "admin.config_center.apply.list") {
		return
	}

	query := r.URL.Query()
	filter := service.ListApplyRunsRequest{
		CoreType: strings.TrimSpace(query.Get("core_type")),
		Status:   strings.TrimSpace(query.Get("status")),
		Limit:    clampQueryInt(query.Get("limit"), 20),
		Offset:   clampNonNegativeQueryInt(query.Get("offset"), 0),
	}

	if raw := strings.TrimSpace(query.Get("agent_host_id")); raw != "" {
		agentHostID, err := parseInt64(raw)
		if err != nil {
			RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.config_center.apply.list", "error.bad_request", h.i18n)
			return
		}
		filter.AgentHostID = &agentHostID
	}

	result, err := h.apply.ListApplyRuns(r.Context(), filter)
	if err != nil {
		h.respondApplyError(r.Context(), w, "admin.config_center.apply.list", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data":  result.Items,
		"total": result.Total,
	})
}

// GetApplyRunDetail handles GET /api/v2/{securePath}/config-center/apply-runs/{run_id}.
func (h *AdminConfigCenterApplyHandler) GetApplyRunDetail(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if !h.ensureService(w, r, "admin.config_center.apply.detail") {
		return
	}

	query := r.URL.Query()
	includeText := false
	if raw := strings.TrimSpace(query.Get("include_text")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.config_center.apply.detail", "error.bad_request", h.i18n)
			return
		}
		includeText = parsed
	}

	result, err := h.apply.GetApplyRunDetail(r.Context(), service.GetApplyRunDetailRequest{
		RunID:       strings.TrimSpace(chi.URLParam(r, "run_id")),
		IncludeText: includeText,
		TextTag:     strings.TrimSpace(query.Get("text_tag")),
		TextFile:    strings.TrimSpace(query.Get("text_file")),
	})
	if err != nil {
		h.respondApplyError(r.Context(), w, "admin.config_center.apply.detail", err)
		return
	}

	if result != nil && result.SemanticDiff != nil && result.SemanticDiff.Items == nil {
		result.SemanticDiff.Items = []service.SemanticDiffItem{}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data": result,
	})
}

func (h *AdminConfigCenterApplyHandler) respondApplyError(ctx context.Context, w http.ResponseWriter, action string, err error) {
	if respondAgentOperationBusy(ctx, w, action, err, h.i18n) {
		return
	}

	status := http.StatusInternalServerError
	key := "error.internal_server_error"

	if errors.Is(err, service.ErrApplyOrchestratorNotConfigured) {
		status = http.StatusServiceUnavailable
		key = "error.service_unavailable"
	} else if errors.Is(err, service.ErrApplyOrchestratorInvalidRequest) {
		status = http.StatusBadRequest
		key = "error.bad_request"
	} else if errors.Is(err, service.ErrApplyOrchestratorNotFound) {
		status = http.StatusNotFound
		key = "error.not_found"
	} else if errors.Is(err, service.ErrApplyOrchestratorPermissionDenied) {
		status = http.StatusForbidden
		key = "error.forbidden"
	} else if errors.Is(err, service.ErrApplyOrchestratorInvalidState) {
		status = http.StatusConflict
		key = "error.bad_request"
	} else if errors.Is(err, service.ErrApplyOrchestratorNoArtifacts) {
		status = http.StatusUnprocessableEntity
		key = "error.no_artifacts_to_deliver"
	} else if errors.Is(err, service.ErrApplyOrchestratorNoPayload) {
		status = http.StatusUnprocessableEntity
		key = "error.no_payload_in_artifact"
	}

	RespondErrorI18nAction(ctx, w, status, action, key, h.i18n)
}
