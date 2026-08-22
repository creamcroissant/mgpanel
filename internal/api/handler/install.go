// 文件路径: internal/api/handler/install.go
// 模块说明: 这是安装向导相关的 HTTP Handler，负责返回状态并创建首个管理员账号。
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/service"
)

// InstallHandler 暴露初始化相关的 API。
type InstallHandler struct {
	install service.InstallService
}

// NewInstallHandler 构建 InstallHandler。
func NewInstallHandler(install service.InstallService) *InstallHandler {
	return &InstallHandler{install: install}
}

// Status 返回当前是否需要初始化。
func (h *InstallHandler) Status(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.install == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "install.status", "error.service_unavailable", nil)
		return
	}
	needs, err := h.install.NeedsBootstrap(ctx)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "install.status", "error.internal_server_error", h.install.I18n())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"needs_bootstrap": needs})
}

// Create 用于创建首个管理员账号。
func (h *InstallHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.install == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "install.create", "error.service_unavailable", nil)
		return
	}
	var payload struct {
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "install.create", "error.bad_request", h.install.I18n())
		return
	}
	if strings.TrimSpace(payload.Email) == "" && strings.TrimSpace(payload.Username) == "" {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "install.create", "error.identifier_required", h.install.I18n())
		return
	}
	user, err := h.install.CreateAdmin(ctx, service.InstallInput{
		Email:    payload.Email,
		Username: payload.Username,
		Password: payload.Password,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidEmail):
			RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "install.create", "error.invalid_email", h.install.I18n())
			return
		case errors.Is(err, service.ErrInvalidUsername):
			RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "install.create", "error.invalid_username", h.install.I18n())
			return
		case errors.Is(err, service.ErrInvalidPassword):
			RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "install.create", "error.invalid_password", h.install.I18n())
			return
		case errors.Is(err, service.ErrIdentifierRequired):
			RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "install.create", "error.identifier_required", h.install.I18n())
			return
		case errors.Is(err, service.ErrEmailExists):
			RespondErrorI18nAction(ctx, w, http.StatusConflict, "install.create", "error.email_exists", h.install.I18n())
			return
		case errors.Is(err, service.ErrUsernameExists):
			RespondErrorI18nAction(ctx, w, http.StatusConflict, "install.create", "error.username_exists", h.install.I18n())
			return
		case errors.Is(err, service.ErrAlreadyInitialized):
			RespondErrorI18nAction(ctx, w, http.StatusConflict, "install.create", "error.conflict", h.install.I18n())
			return
		default:
			RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "install.create", "error.internal_server_error", h.install.I18n())
			return
		}
	}
	RespondSuccessI18n(ctx, w, "success.created", h.install.I18n(), map[string]any{
		"id":       user.ID,
		"email":    user.Email,
		"username": user.Username,
	})
}
