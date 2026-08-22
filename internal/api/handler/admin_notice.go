// 文件路径: internal/api/handler/admin_notice.go
// 模块说明: 这是 internal 模块里的 admin_notice 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/go-chi/chi/v5"
)

// AdminNoticeHandler 负责 /notice 管理端接口。
type AdminNoticeHandler struct {
	notices service.AdminNoticeService
}

// NewAdminNoticeHandler 创建管理端公告处理器。
func NewAdminNoticeHandler(notices service.AdminNoticeService) *AdminNoticeHandler {
	return &AdminNoticeHandler{notices: notices}
}

func (h *AdminNoticeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	action := adminNoticeActionPath(r.URL.Path)
	switch {
	case action == "/fetch" && r.Method == http.MethodGet:
		h.handleFetch(w, r)
	case action == "/save" && r.Method == http.MethodPost:
		h.handleSave(w, r)
	case action == "/show" && r.Method == http.MethodPost:
		h.handleShow(w, r)
	case action == "/drop" && r.Method == http.MethodPost:
		h.handleDrop(w, r)
	case action == "/sort" && r.Method == http.MethodPost:
		h.handleSort(w, r)
	default:
		respondNotImplemented(w, "admin.notice", r)
	}
}

func (h *AdminNoticeHandler) handleFetch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.notices == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "admin.notice.fetch", "error.service_unavailable", nil)
		return
	}
	notices, err := h.notices.List(ctx)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "admin.notice.fetch", "error.internal_server_error", h.notices.I18n())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"data":  notices,
		"total": len(notices),
	})
}

func (h *AdminNoticeHandler) handleSave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.notices == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "admin.notice.save", "error.service_unavailable", nil)
		return
	}
	var payload service.AdminNoticeSaveInput
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "admin.notice.save", "error.bad_request", h.notices.I18n())
		return
	}
	if err := h.notices.Save(ctx, payload); err != nil {
		status := http.StatusBadRequest
		key := "error.bad_request"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(ctx, w, status, "admin.notice.save", key, h.notices.I18n())
		return
	}
	RespondSuccessI18n(ctx, w, "success.updated", h.notices.I18n(), nil)
}

func (h *AdminNoticeHandler) handleShow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.notices == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "admin.notice.show", "error.service_unavailable", nil)
		return
	}
	id, err := parseNoticeID(r)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "admin.notice.show", "error.bad_request", h.notices.I18n())
		return
	}
	if err := h.notices.Toggle(ctx, id); err != nil {
		status := http.StatusBadRequest
		key := "error.bad_request"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(ctx, w, status, "admin.notice.show", key, h.notices.I18n())
		return
	}
	RespondSuccessI18n(ctx, w, "success.updated", h.notices.I18n(), nil)
}

func (h *AdminNoticeHandler) handleDrop(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.notices == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "admin.notice.drop", "error.service_unavailable", nil)
		return
	}
	id, err := parseNoticeID(r)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "admin.notice.drop", "error.bad_request", h.notices.I18n())
		return
	}
	if err := h.notices.Delete(ctx, id); err != nil {
		status := http.StatusBadRequest
		key := "error.bad_request"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(ctx, w, status, "admin.notice.drop", key, h.notices.I18n())
		return
	}
	RespondSuccessI18n(ctx, w, "success.deleted", h.notices.I18n(), nil)
}

func (h *AdminNoticeHandler) handleSort(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.notices == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "admin.notice.sort", "error.service_unavailable", nil)
		return
	}
	var payload struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "admin.notice.sort", "error.bad_request", h.notices.I18n())
		return
	}
	if err := h.notices.Sort(ctx, payload.IDs); err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "admin.notice.sort", "error.bad_request", h.notices.I18n())
		return
	}
	RespondSuccessI18n(ctx, w, "success.updated", h.notices.I18n(), nil)
}

func parseNoticeID(r *http.Request) (int64, error) {
	var payload struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
		if payload.ID > 0 {
			return payload.ID, nil
		}
	}
	if queryID := r.URL.Query().Get("id"); queryID != "" {
		id, err := strconv.ParseInt(queryID, 10, 64)
		if err == nil && id > 0 {
			return id, nil
		}
	}
	return 0, errors.New("invalid id")
}

func adminNoticeActionPath(fullPath string) string {
	idx := strings.Index(fullPath, "/notice")
	if idx == -1 {
		return "/"
	}
	action := fullPath[idx+len("/notice"):]
	if action == "" || action == "/" {
		return "/"
	}
	if !strings.HasPrefix(action, "/") {
		action = "/" + action
	}
	return action
}

// requireAdmin 检查管理员认证
func (h *AdminNoticeHandler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	ctx := r.Context()
	claims := requestctx.AdminFromContext(ctx)
	if claims.ID == "" {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "admin.notice.auth", "error.unauthorized", h.notices.I18n())
		return false
	}
	return true
}

// List 处理 GET /notice（/notice/fetch 的 RESTful 别名）
func (h *AdminNoticeHandler) List(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	h.handleFetch(w, r)
}

// Get 处理 GET /notice/{id}
func (h *AdminNoticeHandler) Get(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.notice.get", h.notices.I18n())
		return
	}

	notice, err := h.notices.GetByID(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
		}
		RespondErrorI18n(r.Context(), w, status, "admin.notice.get", h.notices.I18n())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"data": notice})
}

// Create 处理 POST /notice
func (h *AdminNoticeHandler) Create(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	var payload service.AdminNoticeSaveInput
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.notice.create", h.notices.I18n())
		return
	}

	// 强制 ID 置空以创建新公告
	payload.ID = nil

	if err := h.notices.Save(r.Context(), payload); err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.notice.create", h.notices.I18n())
		return
	}

	RespondSuccessI18n(r.Context(), w, "success.created", h.notices.I18n(), nil)
}

// Update 处理 PUT /notice/{id}
func (h *AdminNoticeHandler) Update(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.notice.update", h.notices.I18n())
		return
	}

	var payload service.AdminNoticeSaveInput
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.notice.update", h.notices.I18n())
		return
	}

	// 设置来自 URL 路径的 ID
	payload.ID = &id

	if err := h.notices.Save(r.Context(), payload); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
		}
		RespondErrorI18n(r.Context(), w, status, "admin.notice.update", h.notices.I18n())
		return
	}

	RespondSuccessI18n(r.Context(), w, "success.updated", h.notices.I18n(), nil)
}

// Delete 处理 DELETE /notice/{id}
func (h *AdminNoticeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.notice.delete", h.notices.I18n())
		return
	}

	if err := h.notices.Delete(r.Context(), id); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
		}
		RespondErrorI18n(r.Context(), w, status, "admin.notice.delete", h.notices.I18n())
		return
	}

	RespondSuccessI18n(r.Context(), w, "success.deleted", h.notices.I18n(), nil)
}
