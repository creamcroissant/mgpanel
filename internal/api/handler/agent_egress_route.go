package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/creamcroissant/mgpanel/internal/service"
)

// AgentEgressRouteHandler 下发本机出口集内核分发指派（agent 侧 egressroute.Manager 消费）。
type AgentEgressRouteHandler struct {
	svc    service.AgentEgressRouteService
	logger *slog.Logger
}

func NewAgentEgressRouteHandler(svc service.AgentEgressRouteService, logger *slog.Logger) *AgentEgressRouteHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &AgentEgressRouteHandler{svc: svc, logger: logger}
}

// ServeHTTP GET /api/v1/agent/egress-routes?token=<host_token>
func (h *AgentEgressRouteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := agentTokenFromRequest(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing token"})
		return
	}
	assignments, err := h.svc.RoutesForToken(ctx, token)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			return
		}
		h.logger.Error("egress-route: failed to expand assignments", slog.String("err", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": assignments})
}
