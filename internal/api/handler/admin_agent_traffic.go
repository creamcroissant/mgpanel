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

type AdminAgentTrafficHandler struct {
	traffic service.AgentTrafficLifecycleService
	i18n    *i18n.Manager
}

func NewAdminAgentTrafficHandler(traffic service.AgentTrafficLifecycleService, i18nMgr *i18n.Manager) *AdminAgentTrafficHandler {
	return &AdminAgentTrafficHandler{traffic: traffic, i18n: i18nMgr}
}

type agentTrafficPolicyRequest struct {
	Enabled          bool   `json:"enabled"`
	LimitBytes       int64  `json:"limit_bytes"`
	LimitType        string `json:"limit_type"`
	ThresholdPercent int    `json:"threshold_percent"`
	ThresholdAction  string `json:"threshold_action"`
	ResetMode        string `json:"reset_mode"`
	ResetDay         int    `json:"reset_day"`
	IntervalDays     int    `json:"interval_days"`
	AnchorAt         int64  `json:"anchor_at"`
}

type agentTrafficManualResetRequest struct {
	Source string `json:"source,omitempty"`
}

func (h *AdminAgentTrafficHandler) GetPolicy(w http.ResponseWriter, r *http.Request) {
	const action = "admin.agent_traffic.policy"
	h.getStatus(w, r, action)
}

func (h *AdminAgentTrafficHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	const action = "admin.agent_traffic.status"
	h.getStatus(w, r, action)
}

func (h *AdminAgentTrafficHandler) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	const action = "admin.agent_traffic.policy_update"
	if !h.requireAdmin(w, r, action) || !h.ensureService(w, r, action) {
		return
	}
	agentHostID, ok := h.parseAgentHostID(w, r, action)
	if !ok {
		return
	}
	var payload agentTrafficPolicyRequest
	if err := decodeOptionalJSON(r, &payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return
	}
	payload = normalizeAgentTrafficPolicyRequest(payload)
	if fields := validateAgentTrafficPolicyRequest(payload); len(fields) > 0 {
		h.respondValidationErrors(r.Context(), w, action, fields)
		return
	}
	status, err := h.traffic.UpsertPolicy(r.Context(), service.UpsertAgentTrafficPolicyRequest{
		AgentHostID:      agentHostID,
		Enabled:          payload.Enabled,
		LimitBytes:       payload.LimitBytes,
		LimitType:        payload.LimitType,
		ThresholdPercent: payload.ThresholdPercent,
		ThresholdAction:  payload.ThresholdAction,
		ResetMode:        payload.ResetMode,
		ResetDay:         payload.ResetDay,
		IntervalDays:     payload.IntervalDays,
		AnchorAt:         payload.AnchorAt,
	})
	if err != nil {
		h.respondServiceError(r.Context(), w, action, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": status})
}

func (h *AdminAgentTrafficHandler) ResetCycle(w http.ResponseWriter, r *http.Request) {
	const action = "admin.agent_traffic.reset_cycle"
	if !h.requireAdmin(w, r, action) || !h.ensureService(w, r, action) {
		return
	}
	agentHostID, ok := h.parseAgentHostID(w, r, action)
	if !ok {
		return
	}
	var payload agentTrafficManualResetRequest
	if err := decodeOptionalJSON(r, &payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return
	}
	source := strings.TrimSpace(payload.Source)
	if source == "" {
		source = "admin"
	}
	result, err := h.traffic.ResetCycle(r.Context(), agentHostID, source)
	if err != nil {
		h.respondServiceError(r.Context(), w, action, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *AdminAgentTrafficHandler) getStatus(w http.ResponseWriter, r *http.Request, action string) {
	if !h.requireAdmin(w, r, action) || !h.ensureService(w, r, action) {
		return
	}
	agentHostID, ok := h.parseAgentHostID(w, r, action)
	if !ok {
		return
	}
	status, err := h.traffic.GetPolicyStatus(r.Context(), agentHostID)
	if err != nil {
		h.respondServiceError(r.Context(), w, action, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": status})
}

func (h *AdminAgentTrafficHandler) requireAdmin(w http.ResponseWriter, r *http.Request, action string) bool {
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, action, "error.unauthorized", h.i18n)
		return false
	}
	if _, err := strconv.ParseInt(claims.ID, 10, 64); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, action, "error.unauthorized", h.i18n)
		return false
	}
	return true
}

func (h *AdminAgentTrafficHandler) ensureService(w http.ResponseWriter, r *http.Request, action string) bool {
	if h.traffic != nil {
		return true
	}
	RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, action, "error.service_unavailable", h.i18n)
	return false
}

func (h *AdminAgentTrafficHandler) parseAgentHostID(w http.ResponseWriter, r *http.Request, action string) (int64, bool) {
	agentHostID, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || agentHostID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return 0, false
	}
	return agentHostID, true
}

func (h *AdminAgentTrafficHandler) respondServiceError(ctx context.Context, w http.ResponseWriter, action string, err error) {
	if respondAgentOperationBusy(ctx, w, action, err, h.i18n) {
		return
	}
	status := http.StatusInternalServerError
	key := "error.internal_server_error"
	switch {
	case errors.Is(err, service.ErrAgentTrafficLifecycleNotConfigured):
		status = http.StatusServiceUnavailable
		key = "error.service_unavailable"
	case errors.Is(err, service.ErrAgentTrafficLifecycleInvalidRequest), errors.Is(err, service.ErrBadRequest):
		status = http.StatusBadRequest
		key = "error.bad_request"
	case errors.Is(err, service.ErrNotFound):
		status = http.StatusNotFound
		key = "error.not_found"
	case errors.Is(err, service.ErrNotImplemented):
		status = http.StatusNotImplemented
		key = "error.service_unavailable"
	}
	RespondErrorI18nAction(ctx, w, status, action, key, h.i18n)
}

func (h *AdminAgentTrafficHandler) respondValidationErrors(ctx context.Context, w http.ResponseWriter, action string, fields map[string]string) {
	message := "error.bad_request"
	if h.i18n != nil {
		message = h.i18n.Translate(requestctx.GetLanguage(ctx), message)
	}
	respondJSON(w, http.StatusBadRequest, map[string]any{
		"error":  message,
		"action": action,
		"details": map[string]any{
			"fields": fields,
		},
	})
}

func normalizeAgentTrafficPolicyRequest(req agentTrafficPolicyRequest) agentTrafficPolicyRequest {
	req.LimitType = strings.TrimSpace(req.LimitType)
	req.ThresholdAction = strings.TrimSpace(req.ThresholdAction)
	req.ResetMode = strings.TrimSpace(req.ResetMode)
	return req
}

func validateAgentTrafficPolicyRequest(req agentTrafficPolicyRequest) map[string]string {
	fields := map[string]string{}
	if req.LimitBytes < 0 || (req.Enabled && req.LimitBytes <= 0) {
		fields["limit_bytes"] = "must be greater than 0 when enabled"
	}
	if req.LimitType != "" && !service.IsAgentTrafficLimitType(req.LimitType) {
		fields["limit_type"] = "must be one of upload, download, sum"
	}
	if req.ThresholdPercent != 0 && (req.ThresholdPercent < 1 || req.ThresholdPercent > 100) {
		fields["threshold_percent"] = "must be between 1 and 100"
	}
	if req.ThresholdAction != "" && !service.IsAgentTrafficThresholdAction(req.ThresholdAction) {
		fields["threshold_action"] = "must be one of notify_only, subscription_exclude, disable_servers, reset_links"
	}
	if req.ResetMode != "" && !service.IsAgentTrafficResetMode(req.ResetMode) {
		fields["reset_mode"] = "must be one of off, fixed_day, calendar_month, interval_days"
	}
	if req.ResetDay != 0 && (req.ResetDay < 1 || req.ResetDay > 31) {
		fields["reset_day"] = "must be between 1 and 31"
	}
	if req.ResetMode == service.AgentTrafficResetModeFixedDay && (req.ResetDay < 1 || req.ResetDay > 31) {
		fields["reset_day"] = "must be between 1 and 31 for fixed_day reset"
	}
	if req.IntervalDays < 0 {
		fields["interval_days"] = "must be greater than or equal to 0"
	}
	if req.ResetMode == service.AgentTrafficResetModeIntervalDays && req.IntervalDays <= 0 {
		fields["interval_days"] = "must be greater than 0 for interval_days reset"
	}
	if req.AnchorAt < 0 {
		fields["anchor_at"] = "must be greater than or equal to 0"
	}
	return fields
}
