package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/go-chi/chi/v5"
)

// AdminConfigCenterCoreConfigHandler exposes admin endpoints for core config items.
type AdminConfigCenterCoreConfigHandler struct {
	service service.CoreConfigItemService
}

// NewAdminConfigCenterCoreConfigHandler creates the handler.
func NewAdminConfigCenterCoreConfigHandler(svc service.CoreConfigItemService) *AdminConfigCenterCoreConfigHandler {
	return &AdminConfigCenterCoreConfigHandler{service: svc}
}

type upsertCoreConfigItemRequest struct {
	AgentHostID *int64          `json:"agent_host_id,omitempty"`
	CoreType    string          `json:"core_type"`
	ConfigType  string          `json:"config_type"`
	Tag         string          `json:"tag"`
	Enabled     *bool           `json:"enabled,omitempty"`
	ConfigData  json.RawMessage `json:"config_data"`
	ChangeNote  string          `json:"change_note,omitempty"`
}

// List handles GET /api/v2/{securePath}/config-center/core-configs.
func (h *AdminConfigCenterCoreConfigHandler) List(w http.ResponseWriter, r *http.Request) {
	if claims := requestctx.AdminFromContext(r.Context()); claims.ID == "" {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if h.service == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "service unavailable"})
		return
	}

	query := r.URL.Query()
	filter := service.ListCoreConfigItemFilter{
		Limit:  clampQueryInt(query.Get("limit"), 20),
		Offset: clampNonNegativeQueryInt(query.Get("offset"), 0),
	}

	if raw := query.Get("agent_host_id"); raw != "" {
		id, err := parseInt64(raw)
		if err != nil || id <= 0 {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "error.bad_request"})
			return
		}
		filter.AgentHostID = &id
	}
	if raw := query.Get("core_type"); raw != "" {
		filter.CoreType = &raw
	}
	if raw := query.Get("config_type"); raw != "" {
		filter.ConfigType = &raw
	}
	if raw := query.Get("tag"); raw != "" {
		filter.Tag = &raw
	}
	if raw := query.Get("enabled"); raw != "" {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "error.bad_request"})
			return
		}
		filter.Enabled = &enabled
	}
	if raw := query.Get("is_template"); raw != "" {
		isTemplate, err := strconv.ParseBool(raw)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "error.bad_request"})
			return
		}
		filter.IsTemplate = &isTemplate
	}

	items, total, err := h.service.List(r.Context(), filter)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "error.internal_server_error"})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"data":  items,
		"total": total,
	})
}

// Create handles POST /api/v2/{securePath}/config-center/core-configs.
func (h *AdminConfigCenterCoreConfigHandler) Create(w http.ResponseWriter, r *http.Request) {
	if claims := requestctx.AdminFromContext(r.Context()); claims.ID == "" {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	h.upsert(w, r, 0)
}

// Update handles PUT /api/v2/{securePath}/config-center/core-configs/{id}.
func (h *AdminConfigCenterCoreConfigHandler) Update(w http.ResponseWriter, r *http.Request) {
	if claims := requestctx.AdminFromContext(r.Context()); claims.ID == "" {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	specID, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || specID <= 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "error.bad_request"})
		return
	}
	h.upsert(w, r, specID)
}

func (h *AdminConfigCenterCoreConfigHandler) upsert(w http.ResponseWriter, r *http.Request, id int64) {
	if h.service == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "service unavailable"})
		return
	}

	var payload upsertCoreConfigItemRequest
	if err := decodeJSON(r, &payload); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "error.bad_request"})
		return
	}

	if payload.AgentHostID != nil && *payload.AgentHostID <= 0 {
		payload.AgentHostID = nil
	}

	// Extract operator ID from admin JWT claims.
	operatorID := int64(0)
	if claims := requestctx.AdminFromContext(r.Context()); claims.ID != "" {
		if id, err := strconv.ParseInt(claims.ID, 10, 64); err == nil {
			operatorID = id
		}
	}

	itemID, revision, err := h.service.Upsert(r.Context(), service.UpsertCoreConfigItemRequest{
		ID:          id,
		AgentHostID: payload.AgentHostID,
		CoreType:    payload.CoreType,
		ConfigType:  payload.ConfigType,
		Tag:         payload.Tag,
		Enabled:     payload.Enabled,
		ConfigData:  payload.ConfigData,
		OperatorID:  operatorID,
		ChangeNote:  payload.ChangeNote,
	})
	if err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		} else {
			var validationErr *service.InboundSpecValidationError
			if errors.As(err, &validationErr) {
				status = http.StatusBadRequest
				key = "error.bad_request"
			}
			var conflictErr *service.InboundSpecConflictError
			if errors.As(err, &conflictErr) {
				status = http.StatusConflict
				key = "error.bad_request"
			}
		}
		respondJSON(w, status, map[string]string{"error": key})
		return
	}

	status := http.StatusOK
	if id == 0 {
		status = http.StatusCreated
	}
	respondJSON(w, status, map[string]any{
		"data": map[string]any{
			"id":       itemID,
			"revision": revision,
		},
	})
}

// Delete handles DELETE /api/v2/{securePath}/config-center/core-configs/{id}.
func (h *AdminConfigCenterCoreConfigHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if claims := requestctx.AdminFromContext(r.Context()); claims.ID == "" {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if h.service == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "service unavailable"})
		return
	}

	specID, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || specID <= 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "error.bad_request"})
		return
	}

	if err := h.service.Delete(r.Context(), specID); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "error.not_found"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "error.internal_server_error"})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"deleted": true}})
}
