package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/creamcroissant/xboard/internal/api/requestctx"
	"github.com/creamcroissant/xboard/internal/service"
	"github.com/go-chi/chi/v5"
)

// AdminMCPKeyHandler handles MCP API key management.
type AdminMCPKeyHandler struct {
	svc service.MCPApiKeyService
}

func NewAdminMCPKeyHandler(svc service.MCPApiKeyService) *AdminMCPKeyHandler {
	return &AdminMCPKeyHandler{svc: svc}
}

// Create generates a new MCP API key.
func (h *AdminMCPKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}
	claims := requestctx.AdminFromContext(r.Context())
	userID, _ := strconv.ParseInt(claims.ID, 10, 64)
	result, err := h.svc.Create(r.Context(), req.Name, userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, result)
}

// List returns all MCP API keys.
func (h *AdminMCPKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	keys, err := h.svc.List(r.Context())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if keys == nil {
		keys = []*service.MCPApiKeyResult{}
	}
	respondJSON(w, http.StatusOK, keys)
}

// Revoke disables an MCP API key.
func (h *AdminMCPKeyHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid id"})
		return
	}
	if err := h.svc.Revoke(r.Context(), id); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"success": true})
}

// Delete removes an MCP API key.
func (h *AdminMCPKeyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid id"})
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"success": true})
}
