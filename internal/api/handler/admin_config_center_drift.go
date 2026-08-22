package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

// AdminConfigCenterDriftHandler exposes admin endpoints for applied snapshot and drift observability.
type AdminConfigCenterDriftHandler struct {
	drift service.DriftAndDiffService
	i18n  *i18n.Manager
}

// NewAdminConfigCenterDriftHandler creates a config-center drift handler.
func NewAdminConfigCenterDriftHandler(drift service.DriftAndDiffService, i18nMgr *i18n.Manager) *AdminConfigCenterDriftHandler {
	return &AdminConfigCenterDriftHandler{drift: drift, i18n: i18nMgr}
}

type configCenterDriftBaseQuery struct {
	agentHostID int64
	coreType    string
}

func (h *AdminConfigCenterDriftHandler) requireAdmin(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.config_center.drift.auth", "error.unauthorized", h.i18n)
		return 0, false
	}
	adminID, err := parseInt64(claims.ID)
	if err != nil {
		adminID = 0
	}
	return adminID, true
}

func (h *AdminConfigCenterDriftHandler) ensureService(w http.ResponseWriter, r *http.Request, action string) bool {
	if h.drift != nil {
		return true
	}
	RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, action, "error.service_unavailable", h.i18n)
	return false
}

func (h *AdminConfigCenterDriftHandler) parseBaseQuery(w http.ResponseWriter, r *http.Request, action string) (configCenterDriftBaseQuery, bool) {
	query := r.URL.Query()
	base := configCenterDriftBaseQuery{}

	agentHostIDRaw := strings.TrimSpace(query.Get("agent_host_id"))
	agentHostID, err := parseInt64(agentHostIDRaw)
	if err != nil || agentHostID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return base, false
	}
	coreType := strings.TrimSpace(query.Get("core_type"))
	if coreType == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return base, false
	}
	base.agentHostID = agentHostID
	base.coreType = coreType
	return base, true
}

// ListAppliedSnapshot handles GET /api/v2/{securePath}/config-center/snapshot.
func (h *AdminConfigCenterDriftHandler) ListAppliedSnapshot(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if !h.ensureService(w, r, "admin.config_center.drift.snapshot") {
		return
	}

	base, ok := h.parseBaseQuery(w, r, "admin.config_center.drift.snapshot")
	if !ok {
		return
	}
	query := r.URL.Query()
	result, err := h.drift.ListAppliedSnapshot(r.Context(), service.ListAppliedSnapshotRequest{
		AgentHostID: base.agentHostID,
		CoreType:    base.coreType,
		Source:      strings.TrimSpace(query.Get("source")),
		Filename:    strings.TrimSpace(query.Get("filename")),
		Tag:         strings.TrimSpace(query.Get("tag")),
		Protocol:    strings.TrimSpace(query.Get("protocol")),
		ParseStatus: strings.TrimSpace(query.Get("parse_status")),
		Limit:       clampQueryInt(query.Get("limit"), 20),
		Offset:      clampNonNegativeQueryInt(query.Get("offset"), 0),
	})
	if err != nil {
		h.respondDriftError(r.Context(), w, "admin.config_center.drift.snapshot", err)
		return
	}

	if result == nil {
		result = &service.ListAppliedSnapshotResult{}
	}
	if result.Inventories == nil {
		result.Inventories = []*repository.AgentConfigInventory{}
	}
	if result.InboundIndexes == nil {
		result.InboundIndexes = []*repository.InboundIndex{}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"inventories":     result.Inventories,
			"inbound_indexes": result.InboundIndexes,
		},
	})
}

// ListDriftStates handles GET /api/v2/{securePath}/config-center/drift.
func (h *AdminConfigCenterDriftHandler) ListDriftStates(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if !h.ensureService(w, r, "admin.config_center.drift.list") {
		return
	}

	base, ok := h.parseBaseQuery(w, r, "admin.config_center.drift.list")
	if !ok {
		return
	}
	query := r.URL.Query()
	result, err := h.drift.ListDriftStates(r.Context(), service.ListDriftStatesRequest{
		AgentHostID: base.agentHostID,
		CoreType:    base.coreType,
		Status:      strings.TrimSpace(query.Get("status")),
		DriftType:   strings.TrimSpace(query.Get("drift_type")),
		Tag:         strings.TrimSpace(query.Get("tag")),
		Filename:    strings.TrimSpace(query.Get("filename")),
		Limit:       clampQueryInt(query.Get("limit"), 20),
		Offset:      clampNonNegativeQueryInt(query.Get("offset"), 0),
	})
	if err != nil {
		h.respondDriftError(r.Context(), w, "admin.config_center.drift.list", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data":  result.Items,
		"total": result.Total,
	})
}

// ListRecoveryStates handles GET /api/v2/{securePath}/config-center/recover.
func (h *AdminConfigCenterDriftHandler) ListRecoveryStates(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if !h.ensureService(w, r, "admin.config_center.drift.recover") {
		return
	}

	base, ok := h.parseBaseQuery(w, r, "admin.config_center.drift.recover")
	if !ok {
		return
	}
	query := r.URL.Query()
	result, err := h.drift.ListDriftStates(r.Context(), service.ListDriftStatesRequest{
		AgentHostID: base.agentHostID,
		CoreType:    base.coreType,
		Status:      "recovered",
		DriftType:   strings.TrimSpace(query.Get("drift_type")),
		Tag:         strings.TrimSpace(query.Get("tag")),
		Filename:    strings.TrimSpace(query.Get("filename")),
		Limit:       clampQueryInt(query.Get("limit"), 20),
		Offset:      clampNonNegativeQueryInt(query.Get("offset"), 0),
	})
	if err != nil {
		h.respondDriftError(r.Context(), w, "admin.config_center.drift.recover", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data":  result.Items,
		"total": result.Total,
	})
}

func (h *AdminConfigCenterDriftHandler) respondDriftError(ctx context.Context, w http.ResponseWriter, action string, err error) {
	status := http.StatusInternalServerError
	key := "error.internal_server_error"

	if errors.Is(err, service.ErrDriftAndDiffNotConfigured) {
		status = http.StatusServiceUnavailable
		key = "error.service_unavailable"
	} else if errors.Is(err, service.ErrDriftAndDiffInvalidRequest) {
		status = http.StatusBadRequest
		key = "error.bad_request"
	} else if errors.Is(err, service.ErrDriftAndDiffDesiredMissing) {
		status = http.StatusNotFound
		key = "error.not_found"
	}

	RespondErrorI18nAction(ctx, w, status, action, key, h.i18n)
}
