package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
	"github.com/go-chi/chi/v5"
)

// AgentHostHandler handles agent host API requests.
type AgentHostHandler struct {
	service service.AgentHostService
	i18n    *i18n.Manager
}

// NewAgentHostHandler creates a new agent host handler.
func NewAgentHostHandler(svc service.AgentHostService, i18nMgr *i18n.Manager) *AgentHostHandler {
	return &AgentHostHandler{service: svc, i18n: i18nMgr}
}

type agentHostResponse struct {
	ID                    int64   `json:"id"`
	Name                  string  `json:"name"`
	Host                  string  `json:"host"`
	Status                int     `json:"status"`
	ProvisionStatus       int     `json:"provision_status"`
	TemplateID            int64   `json:"template_id,omitempty"`
	CoreVersion           string  `json:"core_version,omitempty"`
	CPUTotal              float64 `json:"cpu_total"`
	CPUUsed               float64 `json:"cpu_used"`
	MemTotal              int64   `json:"mem_total"`
	MemUsed               int64   `json:"mem_used"`
	DiskTotal             int64   `json:"disk_total"`
	DiskUsed              int64   `json:"disk_used"`
	UploadTotal           int64   `json:"upload_total"`
	DownloadTotal         int64   `json:"download_total"`
	UploadRateBps         int64   `json:"upload_rate_bps"`
	DownloadRateBps       int64   `json:"download_rate_bps"`
	RawUploadTotalBytes   int64   `json:"raw_upload_total_bytes"`
	RawDownloadTotalBytes int64   `json:"raw_download_total_bytes"`
	BootID                string  `json:"boot_id,omitempty"`
	LastRealtimeReportAt  int64   `json:"last_realtime_report_at"`
	LastRestartAt         int64   `json:"last_restart_at"`
	AgentVersion          string  `json:"agent_version,omitempty"`
	CurrentCoreType       string  `json:"current_core_type,omitempty"`
	LastHeartbeatAt       int64   `json:"last_heartbeat_at"`
	CreatedAt             int64   `json:"created_at"`
	UpdatedAt             int64   `json:"updated_at"`
}

func newAgentHostResponse(host *repository.AgentHost) agentHostResponse {
	return agentHostResponse{
		ID:                    host.ID,
		Name:                  host.Name,
		Host:                  host.Host,
		Status:                host.Status,
		ProvisionStatus:       host.ProvisionStatus,
		TemplateID:            host.TemplateID,
		CoreVersion:           host.CoreVersion,
		CPUTotal:              host.CPUTotal,
		CPUUsed:               host.CPUUsed,
		MemTotal:              host.MemTotal,
		MemUsed:               host.MemUsed,
		DiskTotal:             host.DiskTotal,
		DiskUsed:              host.DiskUsed,
		UploadTotal:           host.UploadTotal,
		DownloadTotal:         host.DownloadTotal,
		UploadRateBps:         host.UploadRateBps,
		DownloadRateBps:       host.DownloadRateBps,
		RawUploadTotalBytes:   host.RawUploadTotalBytes,
		RawDownloadTotalBytes: host.RawDownloadTotalBytes,
		BootID:                host.BootID,
		LastRealtimeReportAt:  host.LastRealtimeReportAt,
		LastRestartAt:         host.LastRestartAt,
		AgentVersion:          host.AgentVersion,
		CurrentCoreType:       host.CurrentCoreType,
		LastHeartbeatAt:       host.LastHeartbeatAt,
		CreatedAt:             host.CreatedAt,
		UpdatedAt:             host.UpdatedAt,
	}
}

// AgentHostStatusRequest represents the status payload from an agent.
type AgentHostStatusRequest struct {
	CPU float64 `json:"cpu"`
	Mem struct {
		Total int64 `json:"total"`
		Used  int64 `json:"used"`
	} `json:"mem"`
	Disk struct {
		Total int64 `json:"total"`
		Used  int64 `json:"used"`
	} `json:"disk"`
	UploadTotal   int64 `json:"upload_total"`
	DownloadTotal int64 `json:"download_total"`
}

// ReportStatus handles POST /api/v1/agent/status?token=xxx
// This endpoint is called by agents to report their status.
func (h *AgentHostHandler) ReportStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := r.URL.Query().Get("token")
	if token == "" {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "agent_host.status", "error.missing_token", h.i18n)
		return
	}

	var req AgentHostStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "agent_host.status", "error.bad_request", h.i18n)
		return
	}

	// First update heartbeat to mark host as online
	if err := h.service.UpdateHeartbeat(ctx, token); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "agent_host.status", "error.invalid_token", h.i18n)
			return
		}
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "agent_host.status", "error.internal_server_error", h.i18n)
		return
	}

	// Then update metrics
	metrics := service.AgentHostMetricsReport{
		CPUUsed:       req.CPU,
		MemTotal:      req.Mem.Total,
		MemUsed:       req.Mem.Used,
		DiskTotal:     req.Disk.Total,
		DiskUsed:      req.Disk.Used,
		UploadTotal:   req.UploadTotal,
		DownloadTotal: req.DownloadTotal,
	}

	if err := h.service.UpdateMetrics(ctx, token, metrics); err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "agent_host.status", "error.internal_server_error", h.i18n)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// Heartbeat handles POST /api/v1/agent/heartbeat?token=xxx
// Simple heartbeat endpoint for agents.
func (h *AgentHostHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := r.URL.Query().Get("token")
	if token == "" {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "agent_host.heartbeat", "error.missing_token", h.i18n)
		return
	}

	if err := h.service.UpdateHeartbeat(ctx, token); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "agent_host.heartbeat", "error.invalid_token", h.i18n)
			return
		}
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "agent_host.heartbeat", "error.internal_server_error", h.i18n)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// ReportHost 处理 POST /api/v1/agent/host
// Agent 周期上报自身公网 IP，面板据此更新 agent_hosts.host（避免 pending-* 占位）。
func (h *AgentHostHandler) ReportHost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := r.URL.Query().Get("token")
	if token == "" {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "agent_host.host", "error.missing_token", h.i18n)
		return
	}

	var req struct {
		Host string `json:"host"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "agent_host.host", "error.bad_request", h.i18n)
		return
	}

	if err := h.service.UpdateHostByToken(ctx, token, req.Host); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "agent_host.host", "error.invalid_token", h.i18n)
			return
		}
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "agent_host.host", "error.internal_server_error", h.i18n)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// List handles GET /api/v1/admin/agent-hosts
// Returns all agent hosts for admin panel.
func (h *AgentHostHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hosts, err := h.service.List(ctx)
	if err != nil {
		slog.Error("agent_host.list failed", "error", err)
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "agent_host.list", "error.internal_server_error", h.i18n)
		return
	}

	response := make([]agentHostResponse, len(hosts))
	for i, host := range hosts {
		response[i] = newAgentHostResponse(host)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": response,
	})
}

// CreateAgentHostRequest represents the request to create a new agent host.
type CreateAgentHostRequest struct {
	Name string `json:"name"`
	Host string `json:"host"`
}

// Create handles POST /api/v1/admin/agent-hosts
// Creates a new agent host and returns the token.
func (h *AgentHostHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req CreateAgentHostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "agent_host.create", "error.bad_request", h.i18n)
		return
	}

	if req.Name == "" || req.Host == "" {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "agent_host.create", "error.missing_fields", h.i18n)
		return
	}

	host, err := h.service.Create(ctx, service.CreateAgentHostRequest{
		Name: req.Name,
		Host: req.Host,
	})
	if err != nil {
		if errors.Is(err, service.ErrRequiredField) {
			RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "agent_host.create", "error.missing_fields", h.i18n)
			return
		}
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "agent_host.create", "error.internal_server_error", h.i18n)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{
			"id":    host.ID,
			"name":  host.Name,
			"host":  host.Host,
			"token": host.Token, // Only returned on create
		},
	})
}

// requireAdmin checks admin authentication and returns admin ID
func (h *AgentHostHandler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	ctx := r.Context()
	claims := requestctx.AdminFromContext(ctx)
	if claims.ID == "" {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "agent_host.auth", "error.unauthorized", h.i18n)
		return false
	}
	return true
}

// Get handles GET /agent-hosts/{id}
// Returns a single agent host by ID.
func (h *AgentHostHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "agent_host.get", "error.bad_request", h.i18n)
		return
	}

	host, err := h.service.GetByID(ctx, id)
	if err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(ctx, w, status, "agent_host.get", key, h.i18n)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": newAgentHostResponse(host),
	})
}

// UpdateAgentHostRequest represents the request to update an agent host.
type UpdateAgentHostRequest struct {
	Name *string `json:"name,omitempty"`
	Host *string `json:"host,omitempty"`
}

// RegisterAgentHostRequest represents the request to auto-register an agent host.
type RegisterAgentHostRequest struct {
	CommunicationKey string `json:"communication_key"`
	Hostname         string `json:"hostname"`
	AdvertiseHost    string `json:"advertise_host,omitempty"`
}

// Register handles POST /api/v1/agent/register
// Registers a new pending host using communication key and returns per-agent token.
func (h *AgentHostHandler) Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req RegisterAgentHostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "agent_host.register", "error.bad_request", h.i18n)
		return
	}

	if strings.TrimSpace(req.CommunicationKey) == "" {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "agent_host.register", "error.missing_communication_key", h.i18n)
		return
	}
	if strings.TrimSpace(req.Hostname) == "" {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "agent_host.register", "error.missing_hostname", h.i18n)
		return
	}

	host, err := h.service.RegisterByCommunicationKey(ctx, service.RegisterAgentHostRequest{
		CommunicationKey: req.CommunicationKey,
		Hostname:         req.Hostname,
		AdvertiseHost:    req.AdvertiseHost,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCommunicationKey):
			RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "agent_host.register", "error.invalid_token", h.i18n)
			return
		default:
			RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "agent_host.register", "error.internal_server_error", h.i18n)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{
			"id":               host.ID,
			"name":             host.Name,
			"host":             host.Host,
			"host_token":       host.Token,
			"provision_status": host.ProvisionStatus,
		},
	})
}

// Update handles PUT /agent-hosts/{id}
// Updates an agent host.
func (h *AgentHostHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "agent_host.update", "error.bad_request", h.i18n)
		return
	}

	var req UpdateAgentHostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "agent_host.update", "error.bad_request", h.i18n)
		return
	}

	updateReq := service.UpdateAgentHostRequest{}
	if req.Name != nil {
		updateReq.Name = req.Name
	}
	if req.Host != nil {
		updateReq.Host = req.Host
	}

	if err := h.service.Update(ctx, id, updateReq); err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		} else if errors.Is(err, service.ErrRequiredField) {
			status = http.StatusBadRequest
			key = "error.missing_fields"
		}
		RespondErrorI18nAction(ctx, w, status, "agent_host.update", key, h.i18n)
		return
	}

	// Fetch updated host to return
	host, err := h.service.GetByID(ctx, id)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "agent_host.update", "error.internal_server_error", h.i18n)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{
			"id":   host.ID,
			"name": host.Name,
			"host": host.Host,
		},
	})
}

// Delete handles DELETE /agent-hosts/{id}
// Deletes an agent host.
func (h *AgentHostHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "agent_host.delete", "error.bad_request", h.i18n)
		return
	}

	if err := h.service.Delete(ctx, id); err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(ctx, w, status, "agent_host.delete", key, h.i18n)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": true,
	})
}

// RefreshAll handles POST /agent-hosts/refresh
// Triggers a refresh of all agent hosts status.
func (h *AgentHostHandler) RefreshAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.requireAdmin(w, r) {
		return
	}

	hosts, err := h.service.List(ctx)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "agent_host.refresh_all", "error.internal_server_error", h.i18n)
		return
	}

	results := make([]map[string]any, 0, len(hosts))
	for _, host := range hosts {
		results = append(results, map[string]any{
			"id":     host.ID,
			"name":   host.Name,
			"status": host.Status,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": results,
	})
}

// Refresh handles POST /agent-hosts/{id}/refresh
// Triggers a refresh of the agent host status.
func (h *AgentHostHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "agent_host.refresh", "error.bad_request", h.i18n)
		return
	}

	host, err := h.service.GetByID(ctx, id)
	if err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(ctx, w, status, "agent_host.refresh", key, h.i18n)
		return
	}

	// Return current status (actual refresh would require agent communication)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{
			"id":     host.ID,
			"status": host.Status,
		},
	})
}
