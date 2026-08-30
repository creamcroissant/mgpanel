package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
	"github.com/go-chi/chi/v5"
)

type AdminAgentLifecycleHandler struct {
	operations      service.AgentLifecycleOperationService
	versions        service.BinaryVersionService
	agentHostService service.AgentHostService
	i18n            *i18n.Manager
}

func NewAdminAgentLifecycleHandler(operations service.AgentLifecycleOperationService, versions service.BinaryVersionService, agentHostService service.AgentHostService, i18nMgr *i18n.Manager) *AdminAgentLifecycleHandler {
	return &AdminAgentLifecycleHandler{operations: operations, versions: versions, agentHostService: agentHostService, i18n: i18nMgr}
}

type agentLifecycleUpdateRequest struct {
	TargetVersion    string `json:"target_version,omitempty"`
	ReleaseTag       string `json:"release_tag,omitempty"`
	ReleaseRepo      string `json:"release_repo,omitempty"`
	ReleaseBaseURL   string `json:"release_base_url,omitempty"`
	AssetName        string `json:"asset_name,omitempty"`
	AssetURL         string `json:"asset_url,omitempty"`
	ChecksumURL      string `json:"checksum_url,omitempty"`
	SHA256           string `json:"sha256,omitempty"`
	JitterMinSeconds int64  `json:"jitter_min_seconds,omitempty"`
	JitterMaxSeconds int64  `json:"jitter_max_seconds,omitempty"`
}

type agentTrafficResetRequest struct {
	Reason string `json:"reason,omitempty"`
	Source string `json:"source,omitempty"`
}

func (h *AdminAgentLifecycleHandler) requireAdmin(w http.ResponseWriter, r *http.Request, action string) (int64, bool) {
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, action, "error.unauthorized", h.i18n)
		return 0, false
	}
	adminID, err := strconv.ParseInt(claims.ID, 10, 64)
	if err != nil {
		adminID = 0
	}
	return adminID, true
}

func (h *AdminAgentLifecycleHandler) ensureService(w http.ResponseWriter, r *http.Request, action string) bool {
	if h.operations != nil {
		return true
	}
	RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, action, "error.service_unavailable", h.i18n)
	return false
}

func (h *AdminAgentLifecycleHandler) ListOperations(w http.ResponseWriter, r *http.Request) {
	const action = "admin.agent_lifecycle.operations"
	if _, ok := h.requireAdmin(w, r, action); !ok {
		return
	}
	if !h.ensureService(w, r, action) {
		return
	}
	agentHostID, ok := h.parseAgentHostID(w, r, action)
	if !ok {
		return
	}
	query := r.URL.Query()
	filter := service.ListAgentLifecycleOperationsRequest{
		AgentHostID:   &agentHostID,
		OperationType: strings.TrimSpace(query.Get("operation_type")),
		Status:        strings.TrimSpace(query.Get("status")),
		Source:        strings.TrimSpace(query.Get("source")),
		Limit:         clampQueryInt(query.Get("limit"), 20),
		Offset:        clampNonNegativeQueryInt(query.Get("offset"), 0),
	}
	if statuses := splitCommaQuery(query.Get("statuses")); len(statuses) > 0 {
		filter.Statuses = statuses
	}
	if raw := strings.TrimSpace(query.Get("start_at")); raw != "" {
		startAt, err := parseInt64(raw)
		if err != nil {
			RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
			return
		}
		filter.StartAt = &startAt
	}
	if raw := strings.TrimSpace(query.Get("end_at")); raw != "" {
		endAt, err := parseInt64(raw)
		if err != nil {
			RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
			return
		}
		filter.EndAt = &endAt
	}
	items, total, err := h.operations.List(r.Context(), filter)
	if err != nil {
		h.respondServiceError(r.Context(), w, action, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": items, "total": total})
}

func (h *AdminAgentLifecycleHandler) CreateUpdateCheck(w http.ResponseWriter, r *http.Request) {
	const action = "admin.agent_lifecycle.update_check"
	h.createUpdateOperation(w, r, action, service.AgentLifecycleOperationTypeAgentUpdateCheck, false)
}

func (h *AdminAgentLifecycleHandler) CreateUpdate(w http.ResponseWriter, r *http.Request) {
	const action = "admin.agent_lifecycle.update"
	h.createUpdateOperation(w, r, action, service.AgentLifecycleOperationTypeAgentUpdate, true)
}

func (h *AdminAgentLifecycleHandler) CreateTrafficReset(w http.ResponseWriter, r *http.Request) {
	const action = "admin.agent_lifecycle.traffic_reset"
	adminID, ok := h.requireAdmin(w, r, action)
	if !ok {
		return
	}
	if !h.ensureService(w, r, action) {
		return
	}
	agentHostID, ok := h.parseAgentHostID(w, r, action)
	if !ok {
		return
	}
	var payload agentTrafficResetRequest
	if err := decodeOptionalJSON(r, &payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return
	}
	payload.Reason = strings.TrimSpace(payload.Reason)
	payload.Source = strings.TrimSpace(payload.Source)
	body, err := json.Marshal(payload)
	if err != nil {
		h.respondServiceError(r.Context(), w, action, err)
		return
	}
	operation, err := h.operations.Create(r.Context(), service.CreateAgentLifecycleOperationRequest{AgentHostID: agentHostID, OperationType: service.AgentLifecycleOperationTypeTrafficReset, RequestPayload: body, OperatorID: &adminID, Source: "admin"})
	if err != nil {
		h.respondServiceError(r.Context(), w, action, err)
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]any{"data": operation})
}

// CreateGeoRefresh pushes a single geo_refresh lifecycle operation for one agent host.
// The agent's syncAgentCommands loop pulls it and runs the reporter immediately.
func (h *AdminAgentLifecycleHandler) CreateGeoRefresh(w http.ResponseWriter, r *http.Request) {
	const action = "admin.agent_lifecycle.geo_refresh"
	adminID, ok := h.requireAdmin(w, r, action)
	if !ok {
		return
	}
	if !h.ensureService(w, r, action) {
		return
	}
	agentHostID, ok := h.parseAgentHostID(w, r, action)
	if !ok {
		return
	}
	operation, err := h.operations.Create(r.Context(), service.CreateAgentLifecycleOperationRequest{
		AgentHostID:   agentHostID,
		OperationType: service.AgentLifecycleOperationTypeGeoRefresh,
		OperatorID:    &adminID,
		Source:        "admin",
	})
	if err != nil {
		h.respondServiceError(r.Context(), w, action, err)
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]any{"data": operation})
}

// CreateGeoRefreshAll iterates online agents and enqueues a geo_refresh operation for each.
// Returns 202 with aggregate counts; per-agent failures are counted as skipped.
func (h *AdminAgentLifecycleHandler) CreateGeoRefreshAll(w http.ResponseWriter, r *http.Request) {
	const action = "admin.agent_lifecycle.geo_refresh_all"
	adminID, ok := h.requireAdmin(w, r, action)
	if !ok {
		return
	}
	if !h.ensureService(w, r, action) {
		return
	}
	if h.agentHostService == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, action, "error.service_unavailable", h.i18n)
		return
	}
	hosts, err := h.agentHostService.List(r.Context())
	if err != nil {
		h.respondServiceError(r.Context(), w, action, err)
		return
	}
	pushed, skipped := 0, 0
	for _, host := range hosts {
		if host == nil || host.Status != 1 {
			skipped++
			continue
		}
		_, cerr := h.operations.Create(r.Context(), service.CreateAgentLifecycleOperationRequest{
			AgentHostID:   host.ID,
			OperationType: service.AgentLifecycleOperationTypeGeoRefresh,
			OperatorID:    &adminID,
			Source:        "admin",
		})
		if cerr != nil {
			skipped++
			continue
		}
		pushed++
	}
	respondJSON(w, http.StatusAccepted, map[string]any{
		"data": map[string]any{
			"pushed":  pushed,
			"skipped": skipped,
			"total":   len(hosts),
		},
	})
}

// CreateSyncUsers pushes a single sync_users lifecycle operation for one agent host.
// The agent's syncAgentCommands loop pulls it and reconciles its v2ray-api runtime
// users against the panel-computed target set (agent pulls /api/v1/agent/users-for-sync).
func (h *AdminAgentLifecycleHandler) CreateSyncUsers(w http.ResponseWriter, r *http.Request) {
	const action = "admin.agent_lifecycle.sync_users"
	adminID, ok := h.requireAdmin(w, r, action)
	if !ok {
		return
	}
	if !h.ensureService(w, r, action) {
		return
	}
	agentHostID, ok := h.parseAgentHostID(w, r, action)
	if !ok {
		return
	}
	operation, err := h.operations.Create(r.Context(), service.CreateAgentLifecycleOperationRequest{
		AgentHostID:   agentHostID,
		OperationType: service.AgentLifecycleOperationTypeSyncUsers,
		OperatorID:    &adminID,
		Source:        "admin",
	})
	if err != nil {
		h.respondServiceError(r.Context(), w, action, err)
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]any{"data": operation})
}

// CreateSyncUsersAll iterates online agents and enqueues a sync_users operation for each.
// Returns 202 with aggregate counts; per-agent failures are counted as skipped.
func (h *AdminAgentLifecycleHandler) CreateSyncUsersAll(w http.ResponseWriter, r *http.Request) {
	const action = "admin.agent_lifecycle.sync_users_all"
	adminID, ok := h.requireAdmin(w, r, action)
	if !ok {
		return
	}
	if !h.ensureService(w, r, action) {
		return
	}
	if h.agentHostService == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, action, "error.service_unavailable", h.i18n)
		return
	}
	hosts, err := h.agentHostService.List(r.Context())
	if err != nil {
		h.respondServiceError(r.Context(), w, action, err)
		return
	}
	pushed, skipped := 0, 0
	for _, host := range hosts {
		if host == nil || host.Status != 1 {
			skipped++
			continue
		}
		_, cerr := h.operations.Create(r.Context(), service.CreateAgentLifecycleOperationRequest{
			AgentHostID:   host.ID,
			OperationType: service.AgentLifecycleOperationTypeSyncUsers,
			OperatorID:    &adminID,
			Source:        "admin",
		})
		if cerr != nil {
			skipped++
			continue
		}
		pushed++
	}
	respondJSON(w, http.StatusAccepted, map[string]any{
		"data": map[string]any{
			"pushed":  pushed,
			"skipped": skipped,
			"total":   len(hosts),
		},
	})
}

func (h *AdminAgentLifecycleHandler) createUpdateOperation(w http.ResponseWriter, r *http.Request, action string, operationType string, includeJitter bool) {
	adminID, ok := h.requireAdmin(w, r, action)
	if !ok {
		return
	}
	if !h.ensureService(w, r, action) {
		return
	}
	agentHostID, ok := h.parseAgentHostID(w, r, action)
	if !ok {
		return
	}
	var payload agentLifecycleUpdateRequest
	if err := decodeOptionalJSON(r, &payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return
	}
	payload = normalizeAgentLifecycleUpdateRequest(payload)
	if !includeJitter {
		payload.JitterMinSeconds = 0
		payload.JitterMaxSeconds = 0
	}
	if err := validateAgentLifecycleUpdateRequest(payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return
	}
	if err := h.fillAgentUpdateTarget(r.Context(), agentHostID, &payload); err != nil {
		h.respondServiceError(r.Context(), w, action, err)
		return
	}
	body, err := json.Marshal(payload)
	if err != nil {
		h.respondServiceError(r.Context(), w, action, err)
		return
	}
	operation, err := h.operations.Create(r.Context(), service.CreateAgentLifecycleOperationRequest{AgentHostID: agentHostID, OperationType: operationType, RequestPayload: body, OperatorID: &adminID, Source: "admin"})
	if err != nil {
		h.respondServiceError(r.Context(), w, action, err)
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]any{"data": operation})
}

func (h *AdminAgentLifecycleHandler) parseAgentHostID(w http.ResponseWriter, r *http.Request, action string) (int64, bool) {
	agentHostID, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || agentHostID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return 0, false
	}
	return agentHostID, true
}

func (h *AdminAgentLifecycleHandler) fillAgentUpdateTarget(ctx context.Context, agentHostID int64, payload *agentLifecycleUpdateRequest) error {
	if h.versions == nil || payload == nil || (payload.TargetVersion != "" && payload.ReleaseTag != "") {
		return nil
	}
	state, err := h.versions.RefreshRemoteVersion(ctx, service.RefreshRemoteVersionRequest{AgentHostID: agentHostID, Component: service.BinaryVersionComponentAgent})
	if err != nil {
		return err
	}
	if state == nil {
		return nil
	}
	remoteVersion := strings.TrimSpace(state.RemoteVersion)
	if remoteVersion == "" {
		return nil
	}
	if payload.TargetVersion == "" {
		payload.TargetVersion = remoteVersion
	}
	if payload.ReleaseTag == "" {
		payload.ReleaseTag = remoteVersion
	}
	return nil
}

func (h *AdminAgentLifecycleHandler) respondServiceError(ctx context.Context, w http.ResponseWriter, action string, err error) {
	if respondAgentOperationBusy(ctx, w, action, err, h.i18n) {
		return
	}
	status := http.StatusInternalServerError
	key := "error.internal_server_error"
	switch {
	case errors.Is(err, service.ErrAgentLifecycleOperationNotConfigured):
		status = http.StatusServiceUnavailable
		key = "error.service_unavailable"
	case errors.Is(err, service.ErrAgentLifecycleOperationInvalidRequest), errors.Is(err, service.ErrBadRequest):
		status = http.StatusBadRequest
		key = "error.bad_request"
	case errors.Is(err, service.ErrAgentLifecycleOperationNotFound), errors.Is(err, service.ErrNotFound):
		status = http.StatusNotFound
		key = "error.not_found"
	case errors.Is(err, service.ErrAgentLifecycleOperationForbidden):
		status = http.StatusForbidden
		key = "error.forbidden"
	case errors.Is(err, service.ErrNotImplemented):
		status = http.StatusNotImplemented
		key = "error.service_unavailable"
	}
	RespondErrorI18nAction(ctx, w, status, action, key, h.i18n)
}

func normalizeAgentLifecycleUpdateRequest(req agentLifecycleUpdateRequest) agentLifecycleUpdateRequest {
	req.TargetVersion = strings.TrimSpace(req.TargetVersion)
	req.ReleaseTag = strings.TrimSpace(req.ReleaseTag)
	req.ReleaseRepo = strings.Trim(strings.TrimSpace(req.ReleaseRepo), "/")
	req.ReleaseBaseURL = strings.TrimRight(strings.TrimSpace(req.ReleaseBaseURL), "/")
	req.AssetName = strings.TrimSpace(req.AssetName)
	req.AssetURL = strings.TrimSpace(req.AssetURL)
	req.ChecksumURL = strings.TrimSpace(req.ChecksumURL)
	req.SHA256 = strings.ToLower(strings.TrimSpace(req.SHA256))
	return req
}

func validateAgentLifecycleUpdateRequest(req agentLifecycleUpdateRequest) error {
	if req.JitterMinSeconds < 0 || req.JitterMaxSeconds < 0 {
		return service.ErrAgentLifecycleOperationInvalidRequest
	}
	if req.JitterMaxSeconds > 0 && req.JitterMaxSeconds < req.JitterMinSeconds {
		return service.ErrAgentLifecycleOperationInvalidRequest
	}
	for _, raw := range []string{req.ReleaseBaseURL, req.AssetURL, req.ChecksumURL} {
		if raw == "" {
			continue
		}
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return service.ErrAgentLifecycleOperationInvalidRequest
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return service.ErrAgentLifecycleOperationInvalidRequest
		}
	}
	return nil
}

func decodeOptionalJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return nil
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, dst)
}

func splitCommaQuery(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
