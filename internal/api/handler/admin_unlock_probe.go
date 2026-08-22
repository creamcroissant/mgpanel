package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/creamcroissant/mgpanel/internal/service"
)

// UnlockProbeHandler 处理 agent 上报解锁检测结果 + 管理员查询/触发。
type UnlockProbeHandler struct {
	agentHostSvc service.AgentHostService
	unlockSvc    service.UnlockProbeService
}

// NewUnlockProbeHandler 创建 UnlockProbeHandler。
func NewUnlockProbeHandler(agentHostSvc service.AgentHostService, unlockSvc service.UnlockProbeService) *UnlockProbeHandler {
	return &UnlockProbeHandler{agentHostSvc: agentHostSvc, unlockSvc: unlockSvc}
}

// ReportUnlock 处理 POST /api/v1/agent/unlock?token=xxx
// Agent 上报每个平台的解锁检测结果。
func (h *UnlockProbeHandler) ReportUnlock(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := r.URL.Query().Get("token")
	if token == "" {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "unlock_probe.report", "error.missing_token", nil)
		return
	}

	agentHost, err := h.agentHostSvc.GetByToken(ctx, token)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "unlock_probe.report", "error.invalid_token", nil)
			return
		}
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "unlock_probe.report", "error.internal_server_error", nil)
		return
	}

	var req struct {
		Results []service.UnlockProbeResultItem `json:"results"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "unlock_probe.report", "error.bad_request", nil)
		return
	}

	if err := h.unlockSvc.BatchUpsertResults(ctx, agentHost.ID, req.Results); err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "unlock_probe.report", "error.internal_server_error", nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// ListAll 处理 GET /api/v2/admin/unlock-probe/results
func (h *UnlockProbeHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	results, err := h.unlockSvc.ListAll(ctx)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "unlock_probe.list", "error.internal_server_error", nil)
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"data": results})
}

// ListByAgentHost 处理 GET /api/v2/admin/unlock-probe/results?agent_host_id=X
func (h *UnlockProbeHandler) ListByAgentHost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	agentHostIDStr := r.URL.Query().Get("agent_host_id")
	if agentHostIDStr == "" {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "unlock_probe.list", "error.agent_host_id_required", nil)
		return
	}
	var agentHostID int64
	if _, err := fmt.Sscanf(agentHostIDStr, "%d", &agentHostID); err != nil || agentHostID <= 0 {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "unlock_probe.list", "error.agent_host_id_required", nil)
		return
	}
	results, err := h.unlockSvc.ListByAgentHost(ctx, agentHostID)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "unlock_probe.list", "error.internal_server_error", nil)
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"data": results})
}

// TriggerProbe 处理 POST /api/v2/admin/unlock-probe/trigger
func (h *UnlockProbeHandler) TriggerProbe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		AgentHostID int64    `json:"agent_host_id"`
		Services    []string `json:"services,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "unlock_probe.trigger", "error.bad_request", nil)
		return
	}
	if req.AgentHostID <= 0 {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "unlock_probe.trigger", "error.agent_host_id_required", nil)
		return
	}
	if err := h.unlockSvc.TriggerProbe(ctx, req.AgentHostID, req.Services); err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "unlock_probe.trigger", "error.internal_server_error", nil)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok","message":"probe triggered"}`))
}