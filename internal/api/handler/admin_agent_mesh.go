package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/creamcroissant/xboard/internal/api/requestctx"
	"github.com/creamcroissant/xboard/internal/repository"
	"github.com/creamcroissant/xboard/internal/service"
	"github.com/creamcroissant/xboard/internal/support/i18n"
	"github.com/go-chi/chi/v5"
)

type AdminAgentMeshHandler struct {
	meshService service.AgentMeshService
	i18n        *i18n.Manager
}

func NewAdminAgentMeshHandler(mesh service.AgentMeshService, i18nMgr *i18n.Manager) *AdminAgentMeshHandler {
	return &AdminAgentMeshHandler{meshService: mesh, i18n: i18nMgr}
}

type meshPeerResponse struct {
	ID           int64  `json:"id"`
	AgentHostID  int64  `json:"agent_host_id"`
	WGPublicKey  string `json:"wg_public_key"`
	WGIP         string `json:"wg_ip"`
	WGListenPort int    `json:"wg_listen_port"`
	NetworkID    string `json:"network_id"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	// Latency probe data
	LatencyMs    float64 `json:"latency_ms,omitempty"`
	PacketLoss   float64 `json:"packet_loss,omitempty"`
	TotalProbes  int     `json:"total_probes,omitempty"`
	LatUpdatedAt int64   `json:"latency_updated_at,omitempty"`
	Online       bool   `json:"online"`
}

func toMeshPeerResponse(p *repository.AgentMeshPeer, latencies []service.MeshPeerLatencyView) meshPeerResponse {
	resp := meshPeerResponse{
		ID:           p.ID,
		AgentHostID:  p.AgentHostID,
		WGPublicKey:  p.WGPublicKey,
		WGIP:         p.WGIP,
		WGListenPort: p.WGListenPort,
		NetworkID:    p.NetworkID,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
	peerID := "agent-" + strconv.FormatInt(p.AgentHostID, 10)
	for _, l := range latencies {
		if l.PeerID == peerID || l.PeerID == p.WGPublicKey {
			resp.LatencyMs = l.LatencyMs
			resp.PacketLoss = l.PacketLoss
			resp.TotalProbes = l.TotalProbes
			resp.LatUpdatedAt = l.UpdatedAt
			break
		}
	}
	resp.Online = resp.LatUpdatedAt > 0 && time.Now().Unix()-resp.LatUpdatedAt < 300
	return resp
}

type meshJoinRequest struct {
	AgentHostID int64  `json:"agent_host_id"`
	NetworkID   string `json:"network_id"`
}

// Join handles POST /agent-hosts/{id}/mesh/join
func (h *AdminAgentMeshHandler) Join(w http.ResponseWriter, r *http.Request) {
	if claims := requestctx.AdminFromContext(r.Context()); claims.ID == "" {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	agentHostIDStr := chi.URLParam(r, "id")
	agentHostID, err := strconv.ParseInt(agentHostIDStr, 10, 64)
	if err != nil || agentHostID <= 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent_host_id"})
		return
	}
	var req struct {
		NetworkID string `json:"network_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.NetworkID = "default"
	}
	if req.NetworkID == "" {
		req.NetworkID = "default"
	}
	ip, pub, err := h.meshService.JoinNetwork(r.Context(), agentHostID, req.NetworkID)
	if err != nil {
		slog.Error("mesh join failed", "agent_host_id", agentHostID, "error", err)
		RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "admin.mesh.joinFailed", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"data": map[string]string{
			"wg_ip":         ip,
			"wg_public_key": pub,
		},
	})
}

// Leave handles DELETE /agent-hosts/{id}/mesh/leave
func (h *AdminAgentMeshHandler) Leave(w http.ResponseWriter, r *http.Request) {
	if claims := requestctx.AdminFromContext(r.Context()); claims.ID == "" {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	agentHostIDStr := chi.URLParam(r, "id")
	agentHostID, err := strconv.ParseInt(agentHostIDStr, 10, 64)
	if err != nil || agentHostID <= 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent_host_id"})
		return
	}
	if err := h.meshService.LeaveNetwork(r.Context(), agentHostID); err != nil {
		slog.Error("mesh leave failed", "agent_host_id", agentHostID, "error", err)
		RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "admin.mesh.leaveFailed", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"left": true}})
}

// GetStatus handles GET /agent-hosts/{id}/mesh/status
func (h *AdminAgentMeshHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	if claims := requestctx.AdminFromContext(r.Context()); claims.ID == "" {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	agentHostIDStr := chi.URLParam(r, "id")
	agentHostID, err := strconv.ParseInt(agentHostIDStr, 10, 64)
	if err != nil || agentHostID <= 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent_host_id"})
		return
	}
	peer, err := h.meshService.GetMeshPeer(r.Context(), agentHostID)
	if err != nil {
		slog.Error("mesh get peer failed", "agent_host_id", agentHostID, "error", err)
		RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "admin.mesh.statusFailed", h.i18n)
		return
	}
	if peer == nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "not joined"})
		return
	}
	latencies, _ := h.meshService.GetPeerLatencies(r.Context(), peer.NetworkID)
	respondJSON(w, http.StatusOK, map[string]any{"data": toMeshPeerResponse(peer, latencies)})
}

// ListNetwork handles GET /mesh/network/{networkID}
func (h *AdminAgentMeshHandler) ListNetwork(w http.ResponseWriter, r *http.Request) {
	if claims := requestctx.AdminFromContext(r.Context()); claims.ID == "" {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	networkID := chi.URLParam(r, "networkID")
	if networkID == "" {
		networkID = "default"
	}
	peers, err := h.meshService.ListNetworkPeers(r.Context(), networkID)
	if err != nil {
		slog.Error("mesh list peers failed", "network_id", networkID, "error", err)
		RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "admin.mesh.statusFailed", h.i18n)
		return
	}
	latencies, _ := h.meshService.GetPeerLatencies(r.Context(), networkID)
	resp := make([]meshPeerResponse, 0, len(peers))
	for _, p := range peers {
		resp = append(resp, toMeshPeerResponse(p, latencies))
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": resp})
}
