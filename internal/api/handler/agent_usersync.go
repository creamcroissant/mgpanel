package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/creamcroissant/mgpanel/internal/service"
)

// AgentUserSyncHandler serves the agent-facing users-for-sync endpoint:
//
//	GET /api/v1/agent/users-for-sync?token=<host_token>
//
// It returns the desired v2ray-api user targets for the requesting agent so the
// agent can diff its current runtime users and inject/remove credentials at
// runtime (zero reload). Error mapping mirrors agent_relay_route: an unknown
// host token is a 401; repository errors surface as 500.
type AgentUserSyncHandler struct {
	svc    service.AgentUserSyncService
	logger *slog.Logger
}

func NewAgentUserSyncHandler(svc service.AgentUserSyncService, logger *slog.Logger) *AgentUserSyncHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &AgentUserSyncHandler{svc: svc, logger: logger}
}

func (h *AgentUserSyncHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := agentTokenFromRequest(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing token"})
		return
	}
	targets, err := h.svc.ResolveForToken(ctx, token)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			return
		}
		h.logger.Error("users-for-sync: failed to resolve targets", slog.String("err", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if targets == nil {
		targets = []service.InboundUserTarget{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"inbounds": targets}})
}