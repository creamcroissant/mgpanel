// 文件路径: internal/api/handler/user.go
// 模块说明: 这是 internal 模块里的 user 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

// UserHandler 处理用户侧接口（V1/V2）。
type UserHandler struct {
	Service service.UserService
	i18n    *i18n.Manager
}

func NewUserHandler(userService service.UserService, i18nMgr *i18n.Manager) *UserHandler {
	return &UserHandler{Service: userService, i18n: i18nMgr}
}

func (h *UserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 根据路径动作分发用户侧请求。
	action := userActionPath(r.URL.Path)
	switch {
	case action == "/profile" && r.Method == http.MethodGet:
		h.handleProfileFetch(w, r)
	case action == "/profile" && r.Method == http.MethodPost:
		h.handleProfileUpdate(w, r)
	case action == "/info" && r.Method == http.MethodGet:
		h.handleInfo(w, r)
	case action == "/getUserInfo" && r.Method == http.MethodGet:
		h.handleInfo(w, r)
	case action == "/changePassword" && r.Method == http.MethodPost:
		h.handleChangePassword(w, r)
	case action == "/resetSecurity" && r.Method == http.MethodGet:
		h.handleResetSecurity(w, r)
	case action == "/getSubscribe" && r.Method == http.MethodGet:
		h.handleGetSubscribe(w, r)
	default:
		respondNotImplemented(w, "user", r)
	}
}

func (h *UserHandler) handleGetSubscribe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := requestctx.UserFromContext(ctx)
	if claims.ID == "" {
		RespondErrorI18n(ctx, w, http.StatusUnauthorized, "error.unauthorized", h.i18n)
		return
	}
	// 返回用户订阅链接。
	profile, err := h.Service.Profile(ctx, claims.ID)
	if err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.user_not_found"
		}
		RespondErrorI18n(ctx, w, status, key, h.i18n)
		return
	}
	subscribeURL, _ := profile["subscribe_url"].(string)
	respondJSON(w, http.StatusOK, map[string]string{"subscribe_url": subscribeURL})
}

func (h *UserHandler) handleProfileFetch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := requestctx.UserFromContext(ctx)
	if claims.ID == "" {
		RespondErrorI18n(ctx, w, http.StatusUnauthorized, "error.unauthorized", h.i18n)
		return
	}
	// 获取用户资料。
	profile, err := h.Service.Profile(ctx, claims.ID)
	if err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18n(ctx, w, status, key, h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, profile)
}

func (h *UserHandler) handleProfileUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := requestctx.UserFromContext(ctx)
	if claims.ID == "" {
		RespondErrorI18n(ctx, w, http.StatusUnauthorized, "error.unauthorized", h.i18n)
		return
	}
	// 更新用户资料字段。
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18n(ctx, w, http.StatusBadRequest, "error.bad_request", h.i18n)
		return
	}
	if err := h.Service.UpdateProfile(ctx, claims.ID, payload); err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.user_not_found"
		}
		RespondErrorI18n(ctx, w, status, key, h.i18n)
		return
	}
	RespondSuccessI18n(ctx, w, "success.updated", h.i18n, nil)
}

func (h *UserHandler) handleInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := requestctx.UserFromContext(ctx)
	if claims.ID == "" {
		RespondErrorI18n(ctx, w, http.StatusUnauthorized, "error.unauthorized", h.i18n)
		return
	}
	// 兼容旧版 /info 返回结构。
	profile, err := h.Service.Profile(ctx, claims.ID)
	if err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.user_not_found"
		}
		RespondErrorI18n(ctx, w, status, key, h.i18n)
		return
	}
	info := make(map[string]any, len(profile)+2)
	for k, v := range profile {
		info[k] = v
	}
	if _, ok := info["avatar_url"]; !ok {
		info["avatar_url"] = ""
	}
	if _, ok := info["is_staff"]; !ok {
		info["is_staff"] = false
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": info})
}

func (h *UserHandler) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := requestctx.UserFromContext(ctx)
	if claims.ID == "" {
		RespondErrorI18n(ctx, w, http.StatusUnauthorized, "error.unauthorized", h.i18n)
		return
	}

	// 修改密码，需校验旧密码。
	var payload struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18n(ctx, w, http.StatusBadRequest, "error.bad_request", h.i18n)
		return
	}

	input := service.ChangePasswordInput{
		OldPassword: payload.OldPassword,
		NewPassword: payload.NewPassword,
	}

	if err := h.Service.ChangePassword(ctx, claims.ID, input); err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.user_not_found"
		} else if errors.Is(err, service.ErrInvalidPassword) {
			status = http.StatusBadRequest
			key = "error.invalid_password"
		}
		RespondErrorI18n(ctx, w, status, key, h.i18n)
		return
	}

	RespondSuccessI18n(ctx, w, "success.password_changed", h.i18n, nil)
}

func (h *UserHandler) handleResetSecurity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := requestctx.UserFromContext(ctx)
	if claims.ID == "" {
		RespondErrorI18n(ctx, w, http.StatusUnauthorized, "error.unauthorized", h.i18n)
		return
	}

	// 重置订阅令牌并返回新 token。
	newToken, err := h.Service.ResetSecurity(ctx, claims.ID)
	if err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.user_not_found"
		}
		RespondErrorI18n(ctx, w, status, key, h.i18n)
		return
	}

	RespondSuccessI18n(ctx, w, "success.security_reset", h.i18n, map[string]any{"token": newToken})
}

func userActionPath(fullPath string) string {
	// 提取 /user 后的动作路径。
	idx := strings.Index(fullPath, "/user")
	if idx == -1 {
		return "/"
	}
	action := fullPath[idx+len("/user"):]
	if action == "" || action == "/" {
		return "/"
	}
	if !strings.HasPrefix(action, "/") {
		action = "/" + action
	}
	return action
}
