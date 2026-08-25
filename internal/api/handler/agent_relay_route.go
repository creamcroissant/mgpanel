package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/creamcroissant/mgpanel/internal/service"
)

// AgentRelayRouteHandler 下发本机中继链路路由角色（agent 侧 RelayRouteManager 消费）。
type AgentRelayRouteHandler struct {
	svc    service.AgentRelayRouteService
	logger *slog.Logger
}

func NewAgentRelayRouteHandler(svc service.AgentRelayRouteService, logger *slog.Logger) *AgentRelayRouteHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &AgentRelayRouteHandler{svc: svc, logger: logger}
}

// ServeHTTP GET /api/v1/agent/relay-routes?token=<host_token>
func (h *AgentRelayRouteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := agentTokenFromRequest(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing token"})
		return
	}
	routes, err := h.svc.RoutesForToken(ctx, token)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			return
		}
		h.logger.Error("relay-route: failed to expand assignments", slog.String("err", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": routes})
}
