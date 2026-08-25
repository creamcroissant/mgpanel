package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/creamcroissant/mgpanel/internal/service"
)

// RelayPathHandler 管理服务器中继链路（多跳流量走向）。
type RelayPathHandler struct {
	svc service.RelayPathService
}

// NewRelayPathHandler 创建 RelayPathHandler。
func NewRelayPathHandler(svc service.RelayPathService) *RelayPathHandler {
	return &RelayPathHandler{svc: svc}
}

// List GET /relay-paths?core_type=sing-box
func (h *RelayPathHandler) List(w http.ResponseWriter, r *http.Request) {
	paths, err := h.svc.List(r.Context(), r.URL.Query().Get("core_type"))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "list_relay_paths", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": paths})
}

// Get GET /relay-paths/{pathID}
func (h *RelayPathHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := parseID(r)
	if id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id"})
		return
	}
	p, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		respondError(w, http.StatusInternalServerError, "get_relay_path", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": p})
}

// Create POST /relay-paths
func (h *RelayPathHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req service.RelayPathUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	p, err := h.svc.Create(r.Context(), req)
	if err != nil {
		respondRelayPathError(w, "create_relay_path", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": p})
}

// Update PUT /relay-paths/{pathID}
func (h *RelayPathHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := parseID(r)
	if id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id"})
		return
	}
	var req service.RelayPathUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	p, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		respondRelayPathError(w, "update_relay_path", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": p})
}

// Delete DELETE /relay-paths/{pathID}
func (h *RelayPathHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := parseID(r)
	if id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id"})
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		respondError(w, http.StatusInternalServerError, "delete_relay_path", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// Validate POST /relay-paths/validate — 保存前校验（循环检测/序列连续性/agent 存在性）。
func (h *RelayPathHandler) Validate(w http.ResponseWriter, r *http.Request) {
	var req service.RelayPathUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if err := h.svc.Validate(r.Context(), req); err != nil {
		respondRelayPathError(w, "validate_relay_path", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true})
}

// respondRelayPathError 校验类错误返回 400+中文细节，其余走通用 respondError。
func respondRelayPathError(w http.ResponseWriter, action string, err error) {
	if errors.Is(err, service.ErrRelayPathValidation) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respondError(w, http.StatusInternalServerError, action, err)
}
