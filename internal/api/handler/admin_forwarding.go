package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
	"github.com/go-chi/chi/v5"
)

// AdminForwardingHandler 处理转发规则管理的 HTTP 请求
type AdminForwardingHandler struct {
	forwarding service.ForwardingService
	i18n       *i18n.Manager
}

// NewAdminForwardingHandler 创建转发规则处理器
func NewAdminForwardingHandler(svc service.ForwardingService, i18nMgr *i18n.Manager) *AdminForwardingHandler {
	return &AdminForwardingHandler{forwarding: svc, i18n: i18nMgr}
}

// requireAdmin 检查管理员权限，返回操作人 ID 指针
func (h *AdminForwardingHandler) requireAdmin(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.forwarding.auth", "error.unauthorized", h.i18n)
		return 0, false
	}
	// 将管理员 ID 转为 int64 (用于审计日志)
	adminID, err := strconv.ParseInt(claims.ID, 10, 64)
	if err != nil {
		adminID = 0 // 如果无法解析，使用 0
	}
	return adminID, true
}

// ListRules 处理 GET /api/v2/admin/forwarding/rules
// 查询参数: agent_host_id (必填)
func (h *AdminForwardingHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}

	agentHostIDStr := r.URL.Query().Get("agent_host_id")
	if agentHostIDStr == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.forwarding.list", "error.missing_fields", h.i18n)
		return
	}

	agentHostID, err := strconv.ParseInt(agentHostIDStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.forwarding.list", "error.bad_request", h.i18n)
		return
	}

	ctx := r.Context()
	rules, err := h.forwarding.ListByAgent(ctx, agentHostID)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.forwarding.list", "error.internal_server_error", h.i18n)
		return
	}

	// 获取当前版本
	version, _ := h.forwarding.GetVersionForAgent(ctx, agentHostID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{
			"rules":   rules,
			"version": version,
		},
	})
}

// CreateRuleRequest 创建转发规则的请求体
type CreateRuleRequest struct {
	AgentHostID   int64  `json:"agent_host_id"`
	Name          string `json:"name"`
	Protocol      string `json:"protocol"`
	ListenPort    int    `json:"listen_port"`
	TargetAddress string `json:"target_address"`
	TargetPort    int    `json:"target_port"`
	Enabled       bool   `json:"enabled"`
	Priority      int    `json:"priority"`
	Remark        string `json:"remark"`
}

// CreateRule 处理 POST /api/v2/admin/forwarding/rules
func (h *AdminForwardingHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	adminID, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}

	var req CreateRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.forwarding.create", "error.bad_request", h.i18n)
		return
	}

	ctx := r.Context()
	rule, err := h.forwarding.CreateRule(ctx, service.CreateForwardingRuleRequest{
		AgentHostID:   req.AgentHostID,
		Name:          req.Name,
		Protocol:      req.Protocol,
		ListenPort:    req.ListenPort,
		TargetAddress: req.TargetAddress,
		TargetPort:    req.TargetPort,
		Enabled:       req.Enabled,
		Priority:      req.Priority,
		Remark:        req.Remark,
		OperatorID:    &adminID,
	})
	if err != nil {
		// 根据错误类型返回不同的状态码
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrInvalidProtocol) ||
			errors.Is(err, service.ErrInvalidListenPort) ||
			errors.Is(err, service.ErrInvalidTargetPort) ||
			errors.Is(err, service.ErrInvalidTargetAddress) ||
			errors.Is(err, service.ErrRuleNameRequired) ||
			errors.Is(err, service.ErrAgentHostRequired) {
			status = http.StatusBadRequest
			key = "error.bad_request"
		} else if errors.Is(err, service.ErrPortConflict) {
			status = http.StatusConflict
			key = "error.bad_request"
		} else if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(r.Context(), w, status, "admin.forwarding.create", key, h.i18n)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"data": rule,
	})
}

// UpdateRuleRequest 更新转发规则的请求体
type UpdateRuleRequest struct {
	Name          *string `json:"name,omitempty"`
	Protocol      *string `json:"protocol,omitempty"`
	ListenPort    *int    `json:"listen_port,omitempty"`
	TargetAddress *string `json:"target_address,omitempty"`
	TargetPort    *int    `json:"target_port,omitempty"`
	Enabled       *bool   `json:"enabled,omitempty"`
	Priority      *int    `json:"priority,omitempty"`
	Remark        *string `json:"remark,omitempty"`
}

// UpdateRule 处理 PUT /api/v2/admin/forwarding/rules/{id}
func (h *AdminForwardingHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	adminID, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.forwarding.update", "error.bad_request", h.i18n)
		return
	}

	var req UpdateRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.forwarding.update", "error.bad_request", h.i18n)
		return
	}

	ctx := r.Context()
	rule, err := h.forwarding.UpdateRule(ctx, id, service.UpdateForwardingRuleRequest{
		Name:          req.Name,
		Protocol:      req.Protocol,
		ListenPort:    req.ListenPort,
		TargetAddress: req.TargetAddress,
		TargetPort:    req.TargetPort,
		Enabled:       req.Enabled,
		Priority:      req.Priority,
		Remark:        req.Remark,
		OperatorID:    &adminID,
	})
	if err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrInvalidProtocol) ||
			errors.Is(err, service.ErrInvalidListenPort) ||
			errors.Is(err, service.ErrInvalidTargetPort) ||
			errors.Is(err, service.ErrInvalidTargetAddress) {
			status = http.StatusBadRequest
			key = "error.bad_request"
		} else if errors.Is(err, service.ErrPortConflict) {
			status = http.StatusConflict
			key = "error.bad_request"
		} else if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(r.Context(), w, status, "admin.forwarding.update", key, h.i18n)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": rule,
	})
}

// DeleteRule 处理 DELETE /api/v2/admin/forwarding/rules/{id}
func (h *AdminForwardingHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	adminID, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.forwarding.delete", "error.bad_request", h.i18n)
		return
	}

	ctx := r.Context()
	if err := h.forwarding.DeleteRule(ctx, id, &adminID); err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(r.Context(), w, status, "admin.forwarding.delete", key, h.i18n)
		return
	}

	RespondSuccessI18n(r.Context(), w, "success.deleted", h.i18n, nil)
}

// ListLogs 处理 GET /api/v2/admin/forwarding/logs
// 查询参数: agent_host_id (必填), limit, offset
func (h *AdminForwardingHandler) ListLogs(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}

	agentHostIDStr := r.URL.Query().Get("agent_host_id")
	if agentHostIDStr == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.forwarding.logs", "error.missing_fields", h.i18n)
		return
	}

	agentHostID, err := strconv.ParseInt(agentHostIDStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.forwarding.logs", "error.bad_request", h.i18n)
		return
	}

	ruleIDStr := r.URL.Query().Get("rule_id")
	startAtStr := r.URL.Query().Get("start_at")
	endAtStr := r.URL.Query().Get("end_at")

	limit := clampQueryInt(r.URL.Query().Get("limit"), 20)
	offset := clampNonNegativeQueryInt(r.URL.Query().Get("offset"), 0)

	var ruleID *int64
	if ruleIDStr != "" {
		parsed, err := strconv.ParseInt(ruleIDStr, 10, 64)
		if err != nil {
			RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.forwarding.logs", "error.bad_request", h.i18n)
			return
		}
		ruleID = &parsed
	}

	var startAt *int64
	if startAtStr != "" {
		parsed, err := strconv.ParseInt(startAtStr, 10, 64)
		if err != nil {
			RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.forwarding.logs", "error.bad_request", h.i18n)
			return
		}
		startAt = &parsed
	}

	var endAt *int64
	if endAtStr != "" {
		parsed, err := strconv.ParseInt(endAtStr, 10, 64)
		if err != nil {
			RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.forwarding.logs", "error.bad_request", h.i18n)
			return
		}
		endAt = &parsed
	}

	if startAt != nil && endAt != nil && *startAt > *endAt {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.forwarding.logs", "error.bad_request", h.i18n)
		return
	}

	ctx := r.Context()
	filter := repository.ForwardingRuleLogFilter{
		AgentHostID: agentHostID,
		RuleID:      ruleID,
		StartAt:     startAt,
		EndAt:       endAt,
		Limit:       limit,
		Offset:      offset,
	}
	logs, err := h.forwarding.GetLogs(ctx, filter)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.forwarding.logs", "error.internal_server_error", h.i18n)
		return
	}

	total, _ := h.forwarding.CountLogs(ctx, filter)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{
			"logs":  logs,
			"total": total,
		},
	})
}
