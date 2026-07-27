package handler

import (
	"net/http"
	"regexp"
	"strconv"

	"github.com/creamcroissant/xboard/internal/api/requestctx"
	"github.com/creamcroissant/xboard/internal/service"
	"github.com/creamcroissant/xboard/internal/support/i18n"
	"github.com/go-chi/chi/v5"
)

// AdminAgentConfigHandler handles agent configuration endpoints.
type AdminAgentConfigHandler struct {
	agentHosts service.AgentHostService
	i18n       *i18n.Manager
}

// NewAdminAgentConfigHandler creates a new config handler.
func NewAdminAgentConfigHandler(agentHosts service.AgentHostService, i18nMgr *i18n.Manager) *AdminAgentConfigHandler {
	return &AdminAgentConfigHandler{agentHosts: agentHosts, i18n: i18nMgr}
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

// ReportConfig handles POST /agent-hosts/{id}/report-config
// Triggers the agent to re-read and report its config.yml.
// Currently acknowledges the request; actual gRPC command dispatch
// is pending agent communication infrastructure.
func (h *AdminAgentConfigHandler) ReportConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"triggered": true}})
}
