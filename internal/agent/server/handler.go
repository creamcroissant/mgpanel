// Package server provides HTTP API handlers for the Agent.
package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/creamcroissant/mgpanel/internal/agent/protocol"
)

// Handler handles HTTP requests for the Agent API.
type Handler struct {
	protoMgr  *protocol.Manager
	authToken string
}

// NewHandler creates a new Handler.
func NewHandler(protoMgr *protocol.Manager, authToken string) *Handler {
	return &Handler{
		protoMgr:  protoMgr,
		authToken: authToken,
	}
}

// jsonResponse writes a JSON response.
func (h *Handler) jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			slog.Error("Failed to encode JSON response", "error", err)
		}
	}
}

// errorResponse writes an error response.
func (h *Handler) errorResponse(w http.ResponseWriter, status int, message string) {
	h.jsonResponse(w, status, map[string]string{"error": message})
}

// AuthMiddleware validates the authorization token.
func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.authToken == "" {
			next.ServeHTTP(w, r)
			return
		}

		token := r.Header.Get("Authorization")
		if token == "" {
			token = r.Header.Get("X-Auth-Token")
		}
		if token != h.authToken && token != "Bearer "+h.authToken {
			h.errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ListConfigs returns all protocol configuration files.
// GET /api/v1/protocols
func (h *Handler) ListConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := h.protoMgr.ListConfigs()
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.jsonResponse(w, http.StatusOK, map[string]any{
		"configs": configs,
	})
}

// ListInbounds returns only inbound configuration files.
// GET /api/v1/protocols/inbounds
func (h *Handler) ListInbounds(w http.ResponseWriter, r *http.Request) {
	configs, err := h.protoMgr.ListInbounds()
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.jsonResponse(w, http.StatusOK, map[string]any{
		"inbounds": configs,
	})
}

// GetConfig returns a specific configuration file.
// GET /api/v1/protocols/{filename}
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	if filename == "" {
		h.errorResponse(w, http.StatusBadRequest, "filename is required")
		return
	}

	content, err := h.protoMgr.ReadConfig(filename)
	if err != nil {
		h.errorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

// ApplyConfigRequest is the request body for applying a configuration.
type ApplyConfigRequest struct {
	Filename string `json:"filename"`
	CoreType string `json:"core_type"`
	Content  string `json:"content"`
}

// ApplyConfig writes a configuration file and reloads the service.
// POST /api/v1/protocols
func (h *Handler) ApplyConfig(w http.ResponseWriter, r *http.Request) {
	var req ApplyConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Filename == "" {
		h.errorResponse(w, http.StatusBadRequest, "filename is required")
		return
	}
	if req.Content == "" {
		h.errorResponse(w, http.StatusBadRequest, "content is required")
		return
	}

	if err := h.protoMgr.ApplyConfigWithCore(r.Context(), req.CoreType, req.Filename, []byte(req.Content)); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "configuration applied successfully",
	})
}

// ApplyTemplateRequest is the request body for applying a template.
type ApplyTemplateRequest struct {
	Filename string         `json:"filename"`
	Template string         `json:"template"`
	Vars     map[string]any `json:"vars"`
}

// ApplyTemplate creates a config from template and reloads the service.
// POST /api/v1/protocols/template
func (h *Handler) ApplyTemplate(w http.ResponseWriter, r *http.Request) {
	var req ApplyTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Filename == "" {
		h.errorResponse(w, http.StatusBadRequest, "filename is required")
		return
	}
	if req.Template == "" {
		h.errorResponse(w, http.StatusBadRequest, "template is required")
		return
	}

	if err := h.protoMgr.ApplyFromTemplate(r.Context(), req.Filename, req.Template, req.Vars); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "template applied successfully",
	})
}

// DeleteConfig removes a configuration file and reloads the service.
// DELETE /api/v1/protocols/{filename}
func (h *Handler) DeleteConfig(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	if filename == "" {
		h.errorResponse(w, http.StatusBadRequest, "filename is required")
		return
	}

	if err := h.protoMgr.RemoveConfig(r.Context(), filename); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "configuration removed successfully",
	})
}

// ServiceStatus returns the status of the sing-box service.
// GET /api/v1/service/status
func (h *Handler) ServiceStatus(w http.ResponseWriter, r *http.Request) {
	running, err := h.protoMgr.ServiceStatus(r.Context())
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]any{
		"running": running,
	})
}

// ReloadService reloads the sing-box service.
// POST /api/v1/service/reload
func (h *Handler) ReloadService(w http.ResponseWriter, r *http.Request) {
	if err := h.protoMgr.ReloadService(r.Context()); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "service reloaded successfully",
	})
}

// HealthCheck returns the health status of the agent.
// GET /health
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.jsonResponse(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}
