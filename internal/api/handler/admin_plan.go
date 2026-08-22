// 文件路径: internal/api/handler/admin_plan.go
// 模块说明: 这是 internal 模块里的 admin_plan 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
	"github.com/go-chi/chi/v5"
)

// AdminPlanHandler exposes admin plan management endpoints.
type AdminPlanHandler struct {
	plans service.PlanService
	admin service.AdminPlanService
	i18n  *i18n.Manager
}

// NewAdminPlanHandler wires plan service into an admin handler.
func NewAdminPlanHandler(plan service.PlanService, admin service.AdminPlanService, i18nMgr *i18n.Manager) *AdminPlanHandler {
	return &AdminPlanHandler{plans: plan, admin: admin, i18n: i18nMgr}
}

func (h *AdminPlanHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	action := adminPlanActionPath(r.URL.Path)
	switch {
	case action == "/fetch" && (r.Method == http.MethodGet || r.Method == http.MethodPost):
		h.handleFetch(w, r)
	case action == "/save" && r.Method == http.MethodPost:
		h.handleSave(w, r)
	case action == "/sort" && r.Method == http.MethodPost:
		h.handleSort(w, r)
	default:
		respondNotImplemented(w, "admin.plan", r)
	}
}

func (h *AdminPlanHandler) handleFetch(w http.ResponseWriter, r *http.Request) {
	if h.plans == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.plan.fetch", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.plan.fetch", "error.unauthorized", h.i18n)
		return
	}
	plans, err := h.plans.AdminPlans(r.Context())
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.plan.fetch", "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"data":  plans,
		"count": len(plans),
		"total": len(plans),
	})
}

func (h *AdminPlanHandler) handleSave(w http.ResponseWriter, r *http.Request) {
	if h.admin == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.plan.save", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.plan.save", "error.unauthorized", h.i18n)
		return
	}
	var payload service.AdminPlanSaveInput
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.plan.save", "error.bad_request", h.i18n)
		return
	}
	if err := h.admin.Save(r.Context(), payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.plan.save", "error.bad_request", h.i18n)
		return
	}
	RespondSuccessI18n(r.Context(), w, "success.updated", h.i18n, nil)
}

func (h *AdminPlanHandler) handleSort(w http.ResponseWriter, r *http.Request) {
	if h.admin == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.plan.sort", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.plan.sort", "error.unauthorized", h.i18n)
		return
	}
	var payload service.AdminPlanSortInput
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.plan.sort", "error.bad_request", h.i18n)
		return
	}
	if err := h.admin.Sort(r.Context(), payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.plan.sort", "error.bad_request", h.i18n)
		return
	}
	RespondSuccessI18n(r.Context(), w, "success.updated", h.i18n, nil)
}

func adminPlanActionPath(fullPath string) string {
	idx := strings.Index(fullPath, "/plan")
	if idx == -1 {
		return "/"
	}
	action := fullPath[idx+len("/plan"):]
	if action == "" || action == "/" {
		return "/"
	}
	if !strings.HasPrefix(action, "/") {
		action = "/" + action
	}
	return action
}

// requireAdmin checks admin authentication
func (h *AdminPlanHandler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.plan.auth", "error.unauthorized", h.i18n)
		return false
	}
	return true
}

// List handles GET /plan (RESTful alias for /plan/fetch)
func (h *AdminPlanHandler) List(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	h.handleFetch(w, r)
}

// Get handles GET /plan/{id}
func (h *AdminPlanHandler) Get(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.plan.get", "error.bad_request", h.i18n)
		return
	}

	plans, err := h.plans.AdminPlans(r.Context())
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.plan.get", "error.internal_server_error", h.i18n)
		return
	}

	for _, plan := range plans {
		if plan.ID == id {
			respondJSON(w, http.StatusOK, map[string]any{"data": plan})
			return
		}
	}

	RespondErrorI18nAction(r.Context(), w, http.StatusNotFound, "admin.plan.get", "error.not_found", h.i18n)
}

// Create handles POST /plan
func (h *AdminPlanHandler) Create(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	var payload service.AdminPlanSaveInput
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.plan.create", "error.bad_request", h.i18n)
		return
	}

	// Force ID to 0 to create new plan
	payload.ID = 0

	if err := h.admin.Save(r.Context(), payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.plan.create", "error.bad_request", h.i18n)
		return
	}

	RespondSuccessI18n(r.Context(), w, "success.created", h.i18n, nil)
}

// Update handles PUT /plan/{id}
func (h *AdminPlanHandler) Update(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.plan.update", "error.bad_request", h.i18n)
		return
	}

	var payload service.AdminPlanSaveInput
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.plan.update", "error.bad_request", h.i18n)
		return
	}

	// Set ID from URL path
	payload.ID = id

	if err := h.admin.Save(r.Context(), payload); err != nil {
		status := http.StatusBadRequest
		key := "error.bad_request"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(r.Context(), w, status, "admin.plan.update", key, h.i18n)
		return
	}

	RespondSuccessI18n(r.Context(), w, "success.updated", h.i18n, nil)
}

// Delete handles DELETE /plan/{id}
func (h *AdminPlanHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.plan.delete", "error.bad_request", h.i18n)
		return
	}

	if err := h.admin.Delete(r.Context(), id); err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(r.Context(), w, status, "admin.plan.delete", key, h.i18n)
		return
	}

	RespondSuccessI18n(r.Context(), w, "success.deleted", h.i18n, nil)
}
