package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
	"github.com/go-chi/chi/v5"
)

// AdminAgentCoreHandler 处理管理端核心管理相关接口。
type AdminAgentCoreHandler struct {
	cores service.AgentCoreService
	i18n  *i18n.Manager
}

// NewAdminAgentCoreHandler 创建管理端核心接口处理器。
func NewAdminAgentCoreHandler(cores service.AgentCoreService, i18nMgr *i18n.Manager) *AdminAgentCoreHandler {
	return &AdminAgentCoreHandler{cores: cores, i18n: i18nMgr}
}

func (h *AdminAgentCoreHandler) requireAdmin(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.agent_core.auth", "error.unauthorized", h.i18n)
		return 0, false
	}
	adminID, err := strconv.ParseInt(claims.ID, 10, 64)
	if err != nil {
		adminID = 0
	}
	return adminID, true
}

// ListCores 处理 GET /api/v2/admin/agent-hosts/{id}/cores。
func (h *AdminAgentCoreHandler) ListCores(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if h.cores == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.agent_core.cores", "error.service_unavailable", h.i18n)
		return
	}
	agentHostID, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || agentHostID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.agent_core.cores", "error.bad_request", h.i18n)
		return
	}
	cores, err := h.cores.GetCores(r.Context(), agentHostID)
	if err != nil {
		h.respondServiceError(r.Context(), w, "admin.agent_core.cores", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": cores})
}

// ListInstances 处理 GET /api/v2/admin/agent-hosts/{id}/core-instances。
func (h *AdminAgentCoreHandler) ListInstances(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if h.cores == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.agent_core.instances", "error.service_unavailable", h.i18n)
		return
	}
	agentHostID, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || agentHostID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.agent_core.instances", "error.bad_request", h.i18n)
		return
	}
	instances, err := h.cores.GetInstances(r.Context(), agentHostID)
	if err != nil {
		h.respondServiceError(r.Context(), w, "admin.agent_core.instances", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": instances})
}

// CreateInstanceRequest 定义核心实例创建请求体。
type CreateInstanceRequest struct {
	CoreType         string          `json:"core_type"`
	InstanceID       string          `json:"instance_id"`
	ConfigTemplateID int64           `json:"config_template_id"`
	ConfigJSON       json.RawMessage `json:"config_json"`
}

// CreateInstance 处理 POST /api/v2/admin/agent-hosts/{id}/core-instances。
func (h *AdminAgentCoreHandler) CreateInstance(w http.ResponseWriter, r *http.Request) {
	adminID, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	if h.cores == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.agent_core.instance.create", "error.service_unavailable", h.i18n)
		return
	}
	agentHostID, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || agentHostID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.agent_core.instance.create", "error.bad_request", h.i18n)
		return
	}
	var req CreateInstanceRequest
	if err := decodeJSON(r, &req); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.agent_core.instance.create", "error.bad_request", h.i18n)
		return
	}
	operation, err := h.cores.CreateInstance(r.Context(), service.CreateInstanceRequest{AgentHostID: agentHostID, CoreType: req.CoreType, InstanceID: req.InstanceID, ConfigTemplateID: req.ConfigTemplateID, ConfigJSON: req.ConfigJSON, OperatorID: &adminID})
	if err != nil {
		h.respondServiceError(r.Context(), w, "admin.agent_core.instance.create", err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"data": operation})
}

// DeleteInstance 处理 DELETE /api/v2/admin/agent-hosts/{id}/core-instances/{instance_id}。
func (h *AdminAgentCoreHandler) DeleteInstance(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if h.cores == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.agent_core.instance.delete", "error.service_unavailable", h.i18n)
		return
	}
	agentHostID, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || agentHostID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.agent_core.instance.delete", "error.bad_request", h.i18n)
		return
	}
	instanceID := chi.URLParam(r, "instance_id")
	if instanceID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.agent_core.instance.delete", "error.missing_fields", h.i18n)
		return
	}
	if err := h.cores.DeleteInstance(r.Context(), agentHostID, instanceID); err != nil {
		h.respondServiceError(r.Context(), w, "admin.agent_core.instance.delete", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": true})
}

// SwitchCoreRequest 定义核心切换请求体。
type SwitchCoreRequest struct {
	FromInstanceID   string          `json:"from_instance_id"`
	ToCoreType       string          `json:"to_core_type"`
	ConfigTemplateID int64           `json:"config_template_id"`
	ConfigJSON       json.RawMessage `json:"config_json"`
	SwitchID         string          `json:"switch_id"`
	ListenPorts      []int           `json:"listen_ports"`
	ZeroDowntime     *bool           `json:"zero_downtime"`
}

// InstallCoreRequest 定义核心安装/升级请求体。
type InstallCoreRequest struct {
	CoreType  string `json:"core_type"`
	Action    string `json:"action"`
	Version   string `json:"version"`
	Channel   string `json:"channel"`
	Flavor    string `json:"flavor"`
	Activate  bool   `json:"activate"`
	RequestID string `json:"request_id"`
}

// SwitchCore 处理 POST /api/v2/admin/agent-hosts/{id}/core-switch。
func (h *AdminAgentCoreHandler) SwitchCore(w http.ResponseWriter, r *http.Request) {
	adminID, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	if h.cores == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.agent_core.switch", "error.service_unavailable", h.i18n)
		return
	}
	agentHostID, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || agentHostID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.agent_core.switch", "error.bad_request", h.i18n)
		return
	}
	var req SwitchCoreRequest
	if err := decodeJSON(r, &req); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.agent_core.switch", "error.bad_request", h.i18n)
		return
	}
	operation, err := h.cores.SwitchCore(r.Context(), service.SwitchCoreRequest{AgentHostID: agentHostID, FromInstanceID: req.FromInstanceID, ToCoreType: req.ToCoreType, ConfigTemplateID: req.ConfigTemplateID, ConfigJSON: req.ConfigJSON, SwitchID: req.SwitchID, ListenPorts: req.ListenPorts, ZeroDowntime: req.ZeroDowntime, OperatorID: &adminID})
	if err != nil {
		h.respondServiceError(r.Context(), w, "admin.agent_core.switch", err)
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]any{"data": operation})
}

// InstallCore 处理 POST /api/v2/admin/agent-hosts/{id}/core-install。
func (h *AdminAgentCoreHandler) InstallCore(w http.ResponseWriter, r *http.Request) {
	adminID, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	if h.cores == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.agent_core.install", "error.service_unavailable", h.i18n)
		return
	}
	agentHostID, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || agentHostID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.agent_core.install", "error.bad_request", h.i18n)
		return
	}
	var req InstallCoreRequest
	if err := decodeJSON(r, &req); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.agent_core.install", "error.bad_request", h.i18n)
		return
	}
	operation, err := h.cores.InstallCore(r.Context(), service.InstallCoreRequest{AgentHostID: agentHostID, CoreType: req.CoreType, Action: req.Action, Version: req.Version, Channel: req.Channel, Flavor: req.Flavor, Activate: req.Activate, RequestID: req.RequestID, OperatorID: &adminID})
	if err != nil {
		h.respondServiceError(r.Context(), w, "admin.agent_core.install", err)
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]any{"data": operation})
}

// ConvertConfigRequest 定义配置转换请求体。
type ConvertConfigRequest struct {
	SourceCore string          `json:"source_core"`
	TargetCore string          `json:"target_core"`
	ConfigJSON json.RawMessage `json:"config_json"`
}

// ConvertConfig 处理 POST /api/v2/admin/agent-hosts/{id}/core-convert。
func (h *AdminAgentCoreHandler) ConvertConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if h.cores == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.agent_core.convert", "error.service_unavailable", h.i18n)
		return
	}
	agentHostID, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || agentHostID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.agent_core.convert", "error.bad_request", h.i18n)
		return
	}
	var req ConvertConfigRequest
	if err := decodeJSON(r, &req); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.agent_core.convert", "error.bad_request", h.i18n)
		return
	}
	result, err := h.cores.ConvertConfig(r.Context(), service.ConvertRequest{SourceCore: req.SourceCore, TargetCore: req.TargetCore, ConfigJSON: req.ConfigJSON})
	if err != nil {
		h.respondServiceError(r.Context(), w, "admin.agent_core.convert", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": result})
}

// ListOperations 处理 GET /api/v2/admin/agent-hosts/{id}/core-operations。
func (h *AdminAgentCoreHandler) ListOperations(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if h.cores == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.agent_core.operations", "error.service_unavailable", h.i18n)
		return
	}
	agentHostID, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || agentHostID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.agent_core.operations", "error.bad_request", h.i18n)
		return
	}
	query := r.URL.Query()
	filter := service.ListCoreOperationsRequest{
		AgentHostID:   &agentHostID,
		OperationType: strings.TrimSpace(query.Get("operation_type")),
		CoreType:      strings.TrimSpace(query.Get("core_type")),
		Limit:         clampQueryInt(query.Get("limit"), 20),
		Offset:        clampNonNegativeQueryInt(query.Get("offset"), 0),
	}
	if status := strings.TrimSpace(query.Get("status")); status != "" {
		filter.Statuses = []string{status}
	}
	if raw := strings.TrimSpace(query.Get("start_at")); raw != "" {
		startAt, err := parseInt64(raw)
		if err != nil {
			RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.agent_core.operations", "error.bad_request", h.i18n)
			return
		}
		filter.StartAt = &startAt
	}
	if raw := strings.TrimSpace(query.Get("end_at")); raw != "" {
		endAt, err := parseInt64(raw)
		if err != nil {
			RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.agent_core.operations", "error.bad_request", h.i18n)
			return
		}
		filter.EndAt = &endAt
	}
	items, total, err := h.cores.ListOperations(r.Context(), filter)
	if err != nil {
		h.respondServiceError(r.Context(), w, "admin.agent_core.operations", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": items, "total": total})
}

// ListSwitchLogs 处理 GET /api/v2/admin/agent-hosts/{id}/core-switch-logs。
func (h *AdminAgentCoreHandler) ListSwitchLogs(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if h.cores == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.agent_core.switch_logs", "error.service_unavailable", h.i18n)
		return
	}
	agentHostID, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || agentHostID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.agent_core.switch_logs", "error.bad_request", h.i18n)
		return
	}
	query := r.URL.Query()
	var statusPtr *string
	if status := query.Get("status"); status != "" {
		statusPtr = &status
	}
	var startAtPtr *int64
	if raw := query.Get("start_at"); raw != "" {
		value, err := parseInt64(raw)
		if err != nil {
			RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.agent_core.switch_logs", "error.bad_request", h.i18n)
			return
		}
		startAtPtr = &value
	}
	var endAtPtr *int64
	if raw := query.Get("end_at"); raw != "" {
		value, err := parseInt64(raw)
		if err != nil {
			RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.agent_core.switch_logs", "error.bad_request", h.i18n)
			return
		}
		endAtPtr = &value
	}
	limit := clampQueryInt(query.Get("limit"), 50)
	offset := clampQueryInt(query.Get("offset"), 0)
	logs, total, err := h.cores.GetSwitchLogs(r.Context(), service.SwitchLogFilter{AgentHostID: agentHostID, Status: statusPtr, StartAt: startAtPtr, EndAt: endAtPtr, Limit: limit, Offset: offset})
	if err != nil {
		h.respondServiceError(r.Context(), w, "admin.agent_core.switch_logs", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": logs, "total": total})
}

func (h *AdminAgentCoreHandler) respondServiceError(ctx context.Context, w http.ResponseWriter, code string, err error) {
	if respondAgentOperationBusy(ctx, w, code, err, h.i18n) {
		return
	}

	statusCode := http.StatusInternalServerError
	key := "error.internal_server_error"
	if errors.Is(err, service.ErrNotFound) {
		statusCode = http.StatusNotFound
		key = "error.not_found"
	} else if errors.Is(err, service.ErrBadRequest) {
		statusCode = http.StatusBadRequest
		key = "error.bad_request"
	} else if errors.Is(err, service.ErrNotImplemented) {
		statusCode = http.StatusNotImplemented
		key = "error.bad_request"
	} else if errors.Is(err, service.ErrUnauthorized) {
		statusCode = http.StatusUnauthorized
		key = "error.unauthorized"
	}
	RespondErrorI18nAction(ctx, w, statusCode, code, key, h.i18n)
}
