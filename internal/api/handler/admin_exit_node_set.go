package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/go-chi/chi/v5"
)

// ExitNodeSetHandler 管理出口节点集合与路由策略。
type ExitNodeSetHandler struct {
	exitSvc    service.ExitNodeSetService
	routingSvc service.RoutingPolicyService
}

// NewExitNodeSetHandler 创建 ExitNodeSetHandler。
func NewExitNodeSetHandler(exitSvc service.ExitNodeSetService, routingSvc service.RoutingPolicyService) *ExitNodeSetHandler {
	return &ExitNodeSetHandler{exitSvc: exitSvc, routingSvc: routingSvc}
}

// --- 出口集合 CRUD ---

func (h *ExitNodeSetHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	details, err := h.exitSvc.List(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": details})
}

func (h *ExitNodeSetHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := parseID(r)
	if id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id"})
		return
	}
	detail, err := h.exitSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": detail})
}

func (h *ExitNodeSetHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req service.ExitNodeSetCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	set, err := h.exitSvc.Create(ctx, req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": set})
}

func (h *ExitNodeSetHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := parseID(r)
	if id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id"})
		return
	}
	var req service.ExitNodeSetUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	req.ID = id
	set, err := h.exitSvc.Update(ctx, req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": set})
}

func (h *ExitNodeSetHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := parseID(r)
	if id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id"})
		return
	}
	if err := h.exitSvc.Delete(ctx, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- 出口集合成员管理 ---

func (h *ExitNodeSetHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setID := parseID(r)
	if setID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing set id"})
		return
	}
	var req service.ExitNodeSetMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	req.SetID = setID
	if err := h.exitSvc.AddMember(ctx, req); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *ExitNodeSetHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setID := parseID(r)
	agentHostID := parseIntParam(r, "agent_host_id")
	if setID <= 0 || agentHostID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing set_id or agent_host_id"})
		return
	}
	if err := h.exitSvc.RemoveMember(ctx, setID, agentHostID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- 路由策略 CRUD ---

func (h *ExitNodeSetHandler) ListPolicies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	coreType := r.URL.Query().Get("core_type")
	policies, err := h.routingSvc.List(ctx, coreType)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": policies})
}

func (h *ExitNodeSetHandler) GetPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := parseID(r)
	if id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id"})
		return
	}
	p, err := h.routingSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": p})
}

func (h *ExitNodeSetHandler) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req service.RoutingPolicyUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	p, err := h.routingSvc.Create(ctx, req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": p})
}

func (h *ExitNodeSetHandler) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := parseID(r)
	if id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id"})
		return
	}
	var req service.RoutingPolicyUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	req.ID = id
	p, err := h.routingSvc.Update(ctx, req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": p})
}

func (h *ExitNodeSetHandler) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := parseID(r)
	if id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id"})
		return
	}
	if err := h.routingSvc.Delete(ctx, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func parseID(r *http.Request) int64 {
	id := chi.URLParam(r, "setID")
	if id == "" {
		id = chi.URLParam(r, "policyID")
	}
	if id == "" {
		id = chi.URLParam(r, "id")
	}
	if id == "" {
		return 0
	}
	v, _ := strconv.ParseInt(id, 10, 64)
	return v
}

func parseIntParam(r *http.Request, key string) int64 {
	if id := chi.URLParam(r, key); id != "" {
		v, _ := strconv.ParseInt(id, 10, 64)
		return v
	}
	v := r.URL.Query().Get(key)
	if v == "" {
		return 0
	}
	id, _ := strconv.ParseInt(v, 10, 64)
	return id
}