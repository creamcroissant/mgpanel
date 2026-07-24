package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"errors"
	"net/http"
	"strconv"

	"github.com/creamcroissant/xboard/internal/api/requestctx"
	"github.com/creamcroissant/xboard/internal/service"
	"github.com/creamcroissant/xboard/internal/support/i18n"
	"github.com/go-chi/chi/v5"
)

// AdminConfigCenterSpecHandler exposes admin endpoints for inbound spec lifecycle.
type AdminConfigCenterSpecHandler struct {
	specs service.InboundSpecService
	i18n  *i18n.Manager
}

// NewAdminConfigCenterSpecHandler creates a config-center spec handler.
func NewAdminConfigCenterSpecHandler(specs service.InboundSpecService, i18nMgr *i18n.Manager) *AdminConfigCenterSpecHandler {
	return &AdminConfigCenterSpecHandler{specs: specs, i18n: i18nMgr}
}

type upsertInboundSpecRequest struct {
	AgentHostID  *int64          `json:"agent_host_id,omitempty"` // nil = 模板 spec
	CoreType     string          `json:"core_type"`
	Tag          string          `json:"tag"`
	Enabled      *bool           `json:"enabled,omitempty"`
	SemanticSpec json.RawMessage `json:"semantic_spec"`
	CoreSpecific json.RawMessage `json:"core_specific"`
	ChangeNote   string          `json:"change_note,omitempty"`
}

type bindSpecRequest struct {
	AgentHostID int64 `json:"agent_host_id"`
}

type importInboundSpecRequest struct {
	AgentHostID       *int64  `json:"agent_host_id,omitempty"`
	CoreType          string  `json:"core_type"`
	Source            *string `json:"source,omitempty"`
	Filename          *string `json:"filename,omitempty"`
	Tag               *string `json:"tag,omitempty"`
	Enabled           *bool   `json:"enabled,omitempty"`
	ChangeNote        string  `json:"change_note,omitempty"`
	OverwriteExisting bool    `json:"overwrite_existing"`
}

func (h *AdminConfigCenterSpecHandler) requireAdmin(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.config_center.spec.auth", "error.unauthorized", h.i18n)
		return 0, false
	}
	adminID, err := parseInt64(claims.ID)
	if err != nil {
		adminID = 0
	}
	return adminID, true
}

func (h *AdminConfigCenterSpecHandler) ensureService(w http.ResponseWriter, r *http.Request, action string) bool {
	if h.specs != nil {
		return true
	}
	RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, action, "error.service_unavailable", h.i18n)
	return false
}

// ListSpecs handles GET /api/v2/{securePath}/config-center/specs.
func (h *AdminConfigCenterSpecHandler) ListSpecs(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if !h.ensureService(w, r, "admin.config_center.spec.list") {
		return
	}

	query := r.URL.Query()
	filter := service.ListInboundSpecFilter{
		Limit:  clampQueryInt(query.Get("limit"), 20),
		Offset: clampNonNegativeQueryInt(query.Get("offset"), 0),
	}

	if raw := query.Get("agent_host_id"); raw != "" {
		agentHostID, err := parseInt64(raw)
		if err != nil || agentHostID <= 0 {
			RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.config_center.spec.list", "error.bad_request", h.i18n)
			return
		}
		filter.AgentHostID = &agentHostID
	}
	if raw := query.Get("core_type"); raw != "" {
		coreType := raw
		filter.CoreType = &coreType
	}
	if raw := query.Get("tag"); raw != "" {
		tag := raw
		filter.Tag = &tag
	}
	if raw := query.Get("enabled"); raw != "" {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.config_center.spec.list", "error.bad_request", h.i18n)
			return
		}
		filter.Enabled = &enabled
	}
	if raw := query.Get("is_template"); raw != "" {
		isTemplate, err := strconv.ParseBool(raw)
		if err != nil {
			RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.config_center.spec.list", "error.bad_request", h.i18n)
			return
		}
		filter.IsTemplate = &isTemplate
	}

	items, total, err := h.specs.ListSpecs(r.Context(), filter)
	if err != nil {
		h.respondSpecError(r.Context(), w, "admin.config_center.spec.list", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data":  items,
		"total": total,
	})
}

// Create handles POST /api/v2/{securePath}/config-center/specs.
func (h *AdminConfigCenterSpecHandler) Create(w http.ResponseWriter, r *http.Request) {
	h.upsert(w, r, 0, "admin.config_center.spec.create")
}

// Update handles PUT /api/v2/{securePath}/config-center/specs/{id}.
func (h *AdminConfigCenterSpecHandler) Update(w http.ResponseWriter, r *http.Request) {
	specID, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || specID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.config_center.spec.update", "error.bad_request", h.i18n)
		return
	}
	h.upsert(w, r, specID, "admin.config_center.spec.update")
}

func (h *AdminConfigCenterSpecHandler) upsert(w http.ResponseWriter, r *http.Request, specID int64, action string) {
	adminID, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	if !h.ensureService(w, r, action) {
		return
	}

	var payload upsertInboundSpecRequest
	if err := decodeJSON(r, &payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return
	}


	id, revision, err := h.specs.UpsertSpec(r.Context(), service.UpsertInboundSpecRequest{
		SpecID:       specID,
		AgentHostID:  payload.AgentHostID,
		CoreType:     payload.CoreType,
		Tag:          payload.Tag,
		Enabled:      payload.Enabled,
		SemanticSpec: payload.SemanticSpec,
		CoreSpecific: payload.CoreSpecific,
		OperatorID:   adminID,
		ChangeNote:   payload.ChangeNote,
	})
	if err != nil {
		h.respondSpecError(r.Context(), w, action, err)
		return
	}

	status := http.StatusOK
	if specID == 0 {
		status = http.StatusCreated
	}
	respondJSON(w, status, map[string]any{
		"data": map[string]any{
			"spec_id":          id,
			"desired_revision": revision,
		},
	})
}

// GetHistory handles GET /api/v2/{securePath}/config-center/specs/{id}/history.
func (h *AdminConfigCenterSpecHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if !h.ensureService(w, r, "admin.config_center.spec.history") {
		return
	}

	specID, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || specID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.config_center.spec.history", "error.bad_request", h.i18n)
		return
	}
	limit := clampQueryInt(r.URL.Query().Get("limit"), 20)
	offset := clampNonNegativeQueryInt(r.URL.Query().Get("offset"), 0)

	items, err := h.specs.GetSpecHistory(r.Context(), specID, limit, offset)
	if err != nil {
		h.respondSpecError(r.Context(), w, "admin.config_center.spec.history", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data":  items,
		"total": len(items),
	})
}

// ImportFromApplied handles POST /api/v2/{securePath}/config-center/specs/import-from-applied.
func (h *AdminConfigCenterSpecHandler) ImportFromApplied(w http.ResponseWriter, r *http.Request) {
	adminID, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	if !h.ensureService(w, r, "admin.config_center.spec.import") {
		return
	}

	var payload importInboundSpecRequest
	if err := decodeJSON(r, &payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.config_center.spec.import", "error.bad_request", h.i18n)
		return
	}

	createdCount, err := h.specs.ImportFromApplied(r.Context(), service.ImportInboundSpecRequest{
		AgentHostID:       payload.AgentHostID,
		CoreType:          payload.CoreType,
		Source:            payload.Source,
		Filename:          payload.Filename,
		Tag:               payload.Tag,
		Enabled:           payload.Enabled,
		OperatorID:        adminID,
		ChangeNote:        payload.ChangeNote,
		OverwriteExisting: payload.OverwriteExisting,
	})
	if err != nil {
		h.respondSpecError(r.Context(), w, "admin.config_center.spec.import", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"created_count": createdCount,
		},
	})
}

// Delete handles DELETE /api/v2/{securePath}/config-center/specs/{id}.
func (h *AdminConfigCenterSpecHandler) Delete(w http.ResponseWriter, r *http.Request) {
	adminID, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	if !h.ensureService(w, r, "admin.config_center.spec.delete") {
		return
	}

	specID, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || specID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.config_center.spec.delete", "error.bad_request", h.i18n)
		return
	}

	if err := h.specs.DeleteSpec(r.Context(), specID); err != nil {
		h.respondSpecError(r.Context(), w, "admin.config_center.spec.delete", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"deleted": true, "operator_id": adminID}})
}

// BindSpec handles POST /api/v2/{securePath}/config-center/specs/{id}/bind.
func (h *AdminConfigCenterSpecHandler) BindSpec(w http.ResponseWriter, r *http.Request) {
	adminID, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	if !h.ensureService(w, r, "admin.config_center.spec.bind") {
		return
	}

	specID, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || specID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.config_center.spec.bind", "error.bad_request", h.i18n)
		return
	}

	var payload bindSpecRequest
	if err := decodeJSON(r, &payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.config_center.spec.bind", "error.bad_request", h.i18n)
		return
	}

	if err := h.specs.BindSpec(r.Context(), specID, payload.AgentHostID); err != nil {
		h.respondSpecError(r.Context(), w, "admin.config_center.spec.bind", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"bound": true, "operator_id": adminID}})
}

// UnbindSpec handles DELETE /api/v2/{securePath}/config-center/specs/{id}/bind.
func (h *AdminConfigCenterSpecHandler) UnbindSpec(w http.ResponseWriter, r *http.Request) {
	adminID, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	if !h.ensureService(w, r, "admin.config_center.spec.unbind") {
		return
	}

	specID, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || specID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.config_center.spec.unbind", "error.bad_request", h.i18n)
		return
	}

	query := r.URL.Query()
	hostID, err := parseInt64(query.Get("agent_host_id"))
	if err != nil || hostID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.config_center.spec.unbind", "error.bad_request", h.i18n)
		return
	}

	if err := h.specs.UnbindSpec(r.Context(), specID, hostID); err != nil {
		h.respondSpecError(r.Context(), w, "admin.config_center.spec.unbind", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"unbound": true, "operator_id": adminID}})
}

// ListBoundHosts handles GET /api/v2/{securePath}/config-center/specs/{id}/bind.
func (h *AdminConfigCenterSpecHandler) ListBoundHosts(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if !h.ensureService(w, r, "admin.config_center.spec.bind.list") {
		return
	}

	specID, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || specID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.config_center.spec.bind.list", "error.bad_request", h.i18n)
		return
	}

	hostIDs, err := h.specs.ListBoundHosts(r.Context(), specID)
	if err != nil {
		h.respondSpecError(r.Context(), w, "admin.config_center.spec.bind.list", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"data": hostIDs})
}


func (h *AdminConfigCenterSpecHandler) respondSpecError(ctx context.Context, w http.ResponseWriter, action string, err error) {
	status := http.StatusInternalServerError
	key := "error.internal_server_error"
	details := map[string]any{}

	if errors.Is(err, service.ErrNotFound) {
		status = http.StatusNotFound
		key = "error.not_found"
	} else if errors.Is(err, service.ErrInboundSpecInvalid) || errors.Is(err, service.ErrArtifactCompileInvalidRequest) {
		status = http.StatusBadRequest
		key = "error.bad_request"
	} else if errors.Is(err, service.ErrInboundSpecTagConflict) || errors.Is(err, service.ErrInboundSpecListenConflict) {
		status = http.StatusConflict
		key = "error.bad_request"
	}

	var validationErr *service.InboundSpecValidationError
	if errors.As(err, &validationErr) {
		details["violations"] = validationErr.Violations
		slog.Warn("inbound spec validation error", "action", action, "violations", validationErr.Violations, "error", err)
	}
	var conflictErr *service.InboundSpecConflictError
	if errors.As(err, &conflictErr) {
		details["conflict"] = conflictErr
		slog.Warn("inbound spec conflict error", "action", action, "conflict", conflictErr, "error", err)
	}

	message := key
	if h.i18n != nil {
		message = h.i18n.Translate(requestctx.GetLanguage(ctx), key)
	}

	resp := map[string]any{
		"error":  message,
		"action": action,
	}
	if len(details) > 0 {
		resp["details"] = details
	}
	respondJSON(w, status, resp)
}
