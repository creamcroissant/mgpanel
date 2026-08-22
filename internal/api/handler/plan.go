// 文件路径: internal/api/handler/plan.go
// 模块说明: 这是 internal 模块里的 plan 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

// PlanHandler serves plan catalog endpoints for guest and user scopes.
type PlanHandler struct {
	svc         service.PlanService
	prefix      string
	namespace   string
	requireAuth bool
	i18n        *i18n.Manager
}

// NewGuestPlanHandler returns a handler bound to /guest/plan routes.
func NewGuestPlanHandler(svc service.PlanService, i18nMgr *i18n.Manager) *PlanHandler {
	return &PlanHandler{
		svc:         svc,
		prefix:      "/guest/plan",
		namespace:   "guest.plan",
		requireAuth: false,
		i18n:        i18nMgr,
	}
}

// NewUserPlanHandler returns a handler bound to /user/plan routes.
func NewUserPlanHandler(svc service.PlanService, i18nMgr *i18n.Manager) *PlanHandler {
	return &PlanHandler{
		svc:         svc,
		prefix:      "/user/plan",
		namespace:   "user.plan",
		requireAuth: true,
		i18n:        i18nMgr,
	}
}

func (h *PlanHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	action := planActionPath(r.URL.Path, h.prefix)
	if action == "/fetch" && r.Method == http.MethodGet {
		if h.requireAuth {
			h.handleUserFetch(w, r)
		} else {
			h.handleGuestFetch(w, r)
		}
		return
	}
	respondNotImplemented(w, h.namespace, r)
}

func (h *PlanHandler) handleGuestFetch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.svc == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, h.namespace+".fetch", "error.service_unavailable", h.i18n)
		return
	}
	plans, err := h.svc.GuestPlans(ctx)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, h.namespace+".fetch", "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, plans)
}

func (h *PlanHandler) handleUserFetch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.svc == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, h.namespace+".fetch", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.UserFromContext(ctx)
	if claims.ID == "" {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, h.namespace+".fetch", "error.unauthorized", h.i18n)
		return
	}
	userID, err := strconv.ParseInt(claims.ID, 10, 64)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, h.namespace+".fetch", "error.bad_request", h.i18n)
		return
	}

	planIDParam := strings.TrimSpace(r.URL.Query().Get("id"))
	if planIDParam == "" {
		plans, err := h.svc.GuestPlans(ctx)
		if err != nil {
			RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, h.namespace+".fetch", "error.internal_server_error", h.i18n)
			return
		}
		respondJSON(w, http.StatusOK, plans)
		return
	}

	planID, err := strconv.ParseInt(planIDParam, 10, 64)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, h.namespace+".fetch", "error.bad_request", h.i18n)
		return
	}

	plan, err := h.svc.UserPlanDetail(ctx, userID, planID)
	if err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(ctx, w, status, h.namespace+".fetch", key, h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, plan)
}

func planActionPath(fullPath, prefix string) string {
	idx := strings.Index(fullPath, prefix)
	if idx == -1 {
		return "/"
	}
	action := fullPath[idx+len(prefix):]
	if action == "" || action == "/" {
		return "/"
	}
	if !strings.HasPrefix(action, "/") {
		action = "/" + action
	}
	return action
}
