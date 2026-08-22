// 文件路径: internal/api/handler/user_stat.go
// 模块说明: 这是 internal 模块里的 user_stat 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package handler

import (
	"net/http"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

// UserStatHandler 提供用户流量统计相关接口。
type UserStatHandler struct {
	stats service.UserStatService
	i18n  *i18n.Manager
}

// NewUserStatHandler 构造用户统计处理器。
func NewUserStatHandler(stats service.UserStatService, i18nMgr *i18n.Manager) *UserStatHandler {
	return &UserStatHandler{stats: stats, i18n: i18nMgr}
}

// ServeHTTP 处理 /user/stat 下的子路由分发。
func (h *UserStatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	action := userStatActionPath(r.URL.Path)
	switch {
	case action == "/" && r.Method == http.MethodGet:
		// GET /user/stat - 返回用户流量统计
		h.handleGetTrafficLog(w, r)
	case action == "/getTrafficLog" && r.Method == http.MethodGet:
		h.handleGetTrafficLog(w, r)
	default:
		respondNotImplemented(w, "user.stat", r)
	}
}

// handleGetTrafficLog 返回用户流量统计列表。
func (h *UserStatHandler) handleGetTrafficLog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.stats == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "user.stat.traffic", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.UserFromContext(ctx)
	if claims.ID == "" {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "user.stat.traffic", "error.unauthorized", h.i18n)
		return
	}
	logs, err := h.stats.TrafficLogs(ctx, claims.ID)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "user.stat.traffic", "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, logs)
}

// userStatActionPath 解析 /user/stat 后的子路径。
func userStatActionPath(fullPath string) string {
	idx := strings.Index(fullPath, "/stat")
	if idx == -1 {
		return "/"
	}
	action := fullPath[idx+len("/stat"):]
	if action == "" || action == "/" {
		return "/"
	}
	if !strings.HasPrefix(action, "/") {
		action = "/" + action
	}
	return action
}
