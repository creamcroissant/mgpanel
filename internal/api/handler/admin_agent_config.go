package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"
)

// AdminAgentConfigHandler handles agent configuration endpoints.
type AdminAgentConfigHandler struct {
	agentHosts service.AgentHostService
	operations service.AgentLifecycleOperationService
	i18n       *i18n.Manager
}

// NewAdminAgentConfigHandler creates a new config handler.
func NewAdminAgentConfigHandler(agentHosts service.AgentHostService, operations service.AgentLifecycleOperationService, i18nMgr *i18n.Manager) *AdminAgentConfigHandler {
	return &AdminAgentConfigHandler{agentHosts: agentHosts, operations: operations, i18n: i18nMgr}
}

func (h *AdminAgentConfigHandler) requireAdmin(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.agents.config.auth", "error.unauthorized", h.i18n)
		return 0, false
	}
	adminID, err := strconv.ParseInt(claims.ID, 10, 64)
	if err != nil {
		adminID = 0
	}
	return adminID, true
}

var configSensitivePatterns = []struct {
	pattern *regexp.Regexp
	replace string
}{
	{regexp.MustCompile(`(?m)^(\s*password:\s*).+`), "${1}***"},
	{regexp.MustCompile(`(?m)^(\s*secret:\s*).+`), "${1}***"},
	{regexp.MustCompile(`(?m)^(\s*token:\s*).+`), "${1}***"},
	{regexp.MustCompile(`(?m)^(\s*key:\s*).+`), "${1}***"},
	{regexp.MustCompile(`(?m)^(\s*private_key:\s*).+`), "${1}***"},
	{regexp.MustCompile(`(?m)^(\s*api_key:\s*).+`), "${1}***"},
	{regexp.MustCompile(`(?m)^(\s*api_secret:\s*).+`), "${1}***"},
}

func sanitizeConfigYAML(yaml string) string {
	for _, p := range configSensitivePatterns {
		yaml = p.pattern.ReplaceAllString(yaml, p.replace)
	}
	return yaml
}

// GetConfig handles GET /agent-hosts/{id}/config
// Returns the agent's reported running config YAML (sensitive fields redacted).
func (h *AdminAgentConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.agents.config.get", "error.bad_request", h.i18n)
		return
	}
	configYAML, err := h.agentHosts.GetConfigYAML(r.Context(), id)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.agents.config.get", "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": sanitizeConfigYAML(configYAML)})
}

// agentConfigPushRequest 收敛 PUT /agent-hosts/{id}/config 的请求体。
type agentConfigPushRequest struct {
	ConfigYAML string `json:"config_yaml"`
}

// agentConfigPushPayload 是 push_config operation 下发给 agent 的载荷。
type agentConfigPushPayload struct {
	ConfigYAML string `json:"config_yaml"`
}

// UpdateConfig handles PUT /agent-hosts/{id}/config
// 校验全量 YAML（含 panel 段）后创建 push_config lifecycle operation，
// agent 通过已有 claim/report 通道拉取并应用。
func (h *AdminAgentConfigHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	const action = "admin.agents.config.update"
	adminID, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	if !h.ensureOperations(w, r, action) {
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return
	}
	var req agentConfigPushRequest
	if err := decodeOptionalJSON(r, &req); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return
	}
	if err := validateAgentConfigYAML(req.ConfigYAML); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return
	}
	body, err := json.Marshal(agentConfigPushPayload{ConfigYAML: req.ConfigYAML})
	if err != nil {
		h.respondConfigServiceError(r, w, action, err)
		return
	}
	operation, err := h.operations.Create(r.Context(), service.CreateAgentLifecycleOperationRequest{
		AgentHostID:    id,
		OperationType:  service.AgentLifecycleOperationTypePushConfig,
		RequestPayload: body,
		OperatorID:     &adminID,
		Source:         "admin",
	})
	if err != nil {
		h.respondConfigServiceError(r, w, action, err)
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]any{"data": operation})
}

// validateAgentConfigYAML 校验全量配置 YAML：非空、可解析、必须含顶层 panel 段。
func validateAgentConfigYAML(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return service.ErrBadRequest
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return err
	}
	panel, ok := doc["panel"]
	if !ok || panel == nil {
		return service.ErrBadRequest
	}
	if _, ok := panel.(map[string]any); !ok {
		return service.ErrBadRequest
	}
	return nil
}

// agentBatchConfigField 是批量改配置白名单的一项：点路径 + 类型 + 展示信息。
// 只收录重启可生效、无失联风险的运行期调优字段；连接/认证/升级类字段永远不开放。
type agentBatchConfigField struct {
	Path string // YAML 点路径，如 log.max_days
	Type string // number | bool | string
}

// agentBatchConfigWhitelist 是批量改配置允许的字段白名单。
var agentBatchConfigWhitelist = []agentBatchConfigField{
	{Path: "interval.sync", Type: "number"},
	{Path: "interval.report", Type: "number"},
	{Path: "log.max_days", Type: "number"},
	{Path: "log.upload.enabled", Type: "bool"},
	{Path: "log.upload.max_lines", Type: "number"},
	{Path: "log.upload.interval_seconds", Type: "number"},
	{Path: "mesh.enabled", Type: "bool"},
	{Path: "unlock.enabled", Type: "bool"},
	{Path: "unlock.interval_hours", Type: "number"},
	{Path: "forwarding.enabled", Type: "bool"},
	{Path: "traffic.type", Type: "string"},
	{Path: "traffic.interface", Type: "string"},
}

func lookupAgentBatchField(path string) (agentBatchConfigField, bool) {
	for _, f := range agentBatchConfigWhitelist {
		if f.Path == path {
			return f, true
		}
	}
	return agentBatchConfigField{}, false
}

// agentBatchConfigRequest 收敛 PATCH /agent-hosts/batch-config 的请求体：
// 目标 agent + 按白名单点路径给的新值（未出现的字段保持各台原值不变）。
type agentBatchConfigRequest struct {
	AgentIDs []int64        `json:"agent_ids"`
	Fields   map[string]any `json:"fields"`
}

// agentBatchConfigResult 汇总批量下发的逐台结果。
type agentBatchConfigResult struct {
	Success []int64         `json:"success"`
	Failed  []agentBatchErr `json:"failed"`
}

type agentBatchErr struct {
	AgentID int64  `json:"agent_id"`
	Error   string `json:"error"`
}

// BatchUpdateConfig handles PATCH /agent-hosts/batch-config
// 按白名单字段合并：读各台上报的 config_yaml，只覆盖 fields 指定的点路径，
// 其余保持原值，再走现有 PUT 全量下发通道（push_config operation）。
func (h *AdminAgentConfigHandler) BatchUpdateConfig(w http.ResponseWriter, r *http.Request) {
	const action = "admin.agents.config.batchUpdate"
	adminID, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	if !h.ensureOperations(w, r, action) {
		return
	}
	var req agentBatchConfigRequest
	if err := decodeOptionalJSON(r, &req); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return
	}
	if len(req.AgentIDs) == 0 || len(req.Fields) == 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return
	}
	result := agentBatchConfigResult{Success: []int64{}, Failed: []agentBatchErr{}}
	for _, agentID := range req.AgentIDs {
		if agentID <= 0 {
			result.Failed = append(result.Failed, agentBatchErr{AgentID: agentID, Error: "invalid agent id"})
			continue
		}
		if err := h.applyBatchFields(r.Context(), agentID, req.Fields, adminID); err != nil {
			result.Failed = append(result.Failed, agentBatchErr{AgentID: agentID, Error: err.Error()})
			continue
		}
		result.Success = append(result.Success, agentID)
	}
	respondJSON(w, http.StatusAccepted, map[string]any{"data": result})
}

// applyBatchFields 对单台 agent 做读-改-推：上报原文 + 白名单字段覆盖 → PUT 通道。
func (h *AdminAgentConfigHandler) applyBatchFields(ctx context.Context, agentID int64, fields map[string]any, adminID int64) error {
	raw, err := h.agentHosts.GetConfigYAML(ctx, agentID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(raw) == "" {
		return errors.New("agent has not reported config yet / 该 Agent 尚未上报配置")
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return err
	}
	for path, val := range fields {
		if val == nil {
			continue // 留空=不改此字段
		}
		field, ok := lookupAgentBatchField(path)
		if !ok {
			return errors.New("field not allowed: " + path + " / 字段不在白名单")
		}
		if err := setAgentYAMLPath(doc, path, coerceAgentFieldValue(field.Type, val)); err != nil {
			return err
		}
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	if err := validateAgentConfigYAML(string(out)); err != nil {
		return err
	}
	body, err := json.Marshal(agentConfigPushPayload{ConfigYAML: string(out)})
	if err != nil {
		return err
	}
	_, err = h.operations.Create(ctx, service.CreateAgentLifecycleOperationRequest{
		AgentHostID:    agentID,
		OperationType:  service.AgentLifecycleOperationTypePushConfig,
		RequestPayload: body,
		OperatorID:     &adminID,
		Source:         "admin-batch",
	})
	return err
}

// setAgentYAMLPath 按点路径在 map 中逐层建/设值（只走 map，不碰数组）。
func setAgentYAMLPath(doc map[string]any, path string, val any) error {
	parts := strings.Split(path, ".")
	cur := doc
	for _, p := range parts[:len(parts)-1] {
		next, ok := cur[p]
		if !ok || next == nil {
			m := map[string]any{}
			cur[p] = m
			cur = m
			continue
		}
		m, ok := next.(map[string]any)
		if !ok {
			return errors.New("path conflicts with non-map: " + path + " / 路径与非对象冲突")
		}
		cur = m
	}
	cur[parts[len(parts)-1]] = val
	return nil
}

// coerceAgentFieldValue 按白名单类型把 JSON 值转成 YAML 侧的 Go 值。
func coerceAgentFieldValue(typ string, val any) any {
	switch typ {
	case "number":
		switch v := val.(type) {
		case float64:
			if v == float64(int64(v)) {
				return int(v)
			}
			return v
		case string:
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				return n
			}
		}
	case "bool":
		switch v := val.(type) {
		case bool:
			return v
		case string:
			switch strings.ToLower(strings.TrimSpace(v)) {
			case "true", "1", "yes":
				return true
			case "false", "0", "no":
				return false
			}
		}
	}
	return val
}

func (h *AdminAgentConfigHandler) ensureOperations(w http.ResponseWriter, r *http.Request, action string) bool {
	if h.operations != nil {
		return true
	}
	RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, action, "error.service_unavailable", h.i18n)
	return false
}

func (h *AdminAgentConfigHandler) respondConfigServiceError(r *http.Request, w http.ResponseWriter, action string, err error) {
	if respondAgentOperationBusy(r.Context(), w, action, err, h.i18n) {
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
	}
	RespondErrorI18nAction(r.Context(), w, status, action, key, h.i18n)
}

// ReportConfig handles POST /agent-hosts/{id}/report-config
// 创建 report_config lifecycle operation，agent 通过已有 claim/report 通道拉取并重读上报。
// 响应保持 {data:{triggered:true}} 兼容，另附 operation_id 供轮询。
func (h *AdminAgentConfigHandler) ReportConfig(w http.ResponseWriter, r *http.Request) {
	const action = "admin.agents.config.report"
	adminID, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	if !h.ensureOperations(w, r, action) {
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return
	}
	operation, err := h.operations.Create(r.Context(), service.CreateAgentLifecycleOperationRequest{
		AgentHostID:   id,
		OperationType: service.AgentLifecycleOperationTypeReportConfig,
		OperatorID:    &adminID,
		Source:        "admin",
	})
	if err != nil {
		h.respondConfigServiceError(r, w, action, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"triggered": true, "operation_id": operation.ID}})
}
