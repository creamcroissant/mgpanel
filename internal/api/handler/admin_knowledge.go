// 文件路径: internal/api/handler/admin_knowledge.go
// 模块说明: 这是 internal 模块里的 admin_knowledge 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
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
)

// AdminKnowledgeHandler 提供知识库管理接口。
type AdminKnowledgeHandler struct {
	knowledge service.AdminKnowledgeService
	i18n      *i18n.Manager
}

// NewAdminKnowledgeHandler 构造知识库管理处理器。
func NewAdminKnowledgeHandler(knowledge service.AdminKnowledgeService, i18nMgr *i18n.Manager) *AdminKnowledgeHandler {
	return &AdminKnowledgeHandler{knowledge: knowledge, i18n: i18nMgr}
}

// ServeHTTP 分发 /knowledge 下的管理操作。
func (h *AdminKnowledgeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.knowledge.auth", "error.unauthorized", h.i18n)
		return
	}
	action := adminKnowledgeActionPath(r.URL.Path)
	switch {
	case action == "/" && r.Method == http.MethodGet:
		// GET /knowledge - 返回知识库列表
		h.handleFetch(w, r)
	case action == "/" && r.Method == http.MethodPost:
		h.handleSave(w, r)
	case action == "/fetch" && r.Method == http.MethodGet:
		h.handleFetch(w, r)
	case action == "/getCategory" && r.Method == http.MethodGet:
		h.handleCategories(w, r)
	case action == "/save" && r.Method == http.MethodPost:
		h.handleSave(w, r)
	case action == "/show" && r.Method == http.MethodPost:
		h.handleShow(w, r)
	case action == "/drop" && r.Method == http.MethodPost:
		h.handleDrop(w, r)
	case action == "/sort" && r.Method == http.MethodPost:
		h.handleSort(w, r)
	default:
		respondNotImplemented(w, "admin.knowledge", r)
	}
}

// handleFetch 获取知识库列表或单条详情。
func (h *AdminKnowledgeHandler) handleFetch(w http.ResponseWriter, r *http.Request) {
	if h.knowledge == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.knowledge.fetch", "error.service_unavailable", h.i18n)
		return
	}
	if id, ok := parseKnowledgeIDQuery(r); ok {
		detail, err := h.knowledge.FindByID(r.Context(), id)
		if err != nil {
			status := http.StatusInternalServerError
			key := "error.internal_server_error"
			if errors.Is(err, service.ErrNotFound) {
				status = http.StatusNotFound
				key = "error.not_found"
			}
			RespondErrorI18nAction(r.Context(), w, status, "admin.knowledge.fetch", key, h.i18n)
			return
		}
		respondJSON(w, http.StatusOK, detail)
		return
	}
	list, err := h.knowledge.List(r.Context())
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.knowledge.fetch", "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, list)
}

// handleCategories 返回知识库分类列表。
func (h *AdminKnowledgeHandler) handleCategories(w http.ResponseWriter, r *http.Request) {
	if h.knowledge == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.knowledge.category", "error.service_unavailable", h.i18n)
		return
	}
	categories, err := h.knowledge.Categories(r.Context())
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.knowledge.category", "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, categories)
}

// handleSave 新建或更新知识库条目。
func (h *AdminKnowledgeHandler) handleSave(w http.ResponseWriter, r *http.Request) {
	if h.knowledge == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.knowledge.save", "error.service_unavailable", h.i18n)
		return
	}
	var payload service.AdminKnowledgeSaveInput
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.knowledge.save", "error.bad_request", h.i18n)
		return
	}
	if err := h.knowledge.Save(r.Context(), payload); err != nil {
		status := http.StatusBadRequest
		key := "error.bad_request"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(r.Context(), w, status, "admin.knowledge.save", key, h.i18n)
		return
	}
	RespondSuccessI18n(r.Context(), w, "success.updated", h.i18n, nil)
}

// handleShow 切换知识库条目的展示状态。
func (h *AdminKnowledgeHandler) handleShow(w http.ResponseWriter, r *http.Request) {
	if h.knowledge == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.knowledge.show", "error.service_unavailable", h.i18n)
		return
	}
	id, err := parseKnowledgeIDBody(r)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.knowledge.show", "error.bad_request", h.i18n)
		return
	}
	if err := h.knowledge.Toggle(r.Context(), id); err != nil {
		status := http.StatusBadRequest
		key := "error.bad_request"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(r.Context(), w, status, "admin.knowledge.show", key, h.i18n)
		return
	}
	RespondSuccessI18n(r.Context(), w, "success.updated", h.i18n, nil)
}

// handleDrop 删除知识库条目。
func (h *AdminKnowledgeHandler) handleDrop(w http.ResponseWriter, r *http.Request) {
	if h.knowledge == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.knowledge.drop", "error.service_unavailable", h.i18n)
		return
	}
	id, err := parseKnowledgeIDBody(r)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.knowledge.drop", "error.bad_request", h.i18n)
		return
	}
	if err := h.knowledge.Delete(r.Context(), id); err != nil {
		status := http.StatusBadRequest
		key := "error.bad_request"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(r.Context(), w, status, "admin.knowledge.drop", key, h.i18n)
		return
	}
	RespondSuccessI18n(r.Context(), w, "success.deleted", h.i18n, nil)
}

// handleSort 按传入 ID 列表更新排序。
func (h *AdminKnowledgeHandler) handleSort(w http.ResponseWriter, r *http.Request) {
	if h.knowledge == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.knowledge.sort", "error.service_unavailable", h.i18n)
		return
	}
	var payload struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.knowledge.sort", "error.bad_request", h.i18n)
		return
	}
	if err := h.knowledge.Sort(r.Context(), payload.IDs); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.knowledge.sort", "error.bad_request", h.i18n)
		return
	}
	RespondSuccessI18n(r.Context(), w, "success.updated", h.i18n, nil)
}

// parseKnowledgeIDQuery 解析查询参数中的 id。
func parseKnowledgeIDQuery(r *http.Request) (int64, bool) {
	queryID := strings.TrimSpace(r.URL.Query().Get("id"))
	if queryID == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(queryID, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// parseKnowledgeIDBody 解析请求体中的 id。
func parseKnowledgeIDBody(r *http.Request) (int64, error) {
	var payload struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return 0, err
	}
	if payload.ID <= 0 {
		return 0, errors.New("id is required / id 不能为空")
	}
	return payload.ID, nil
}

// adminKnowledgeActionPath 解析 /knowledge 后的子路径。
func adminKnowledgeActionPath(fullPath string) string {
	idx := strings.Index(fullPath, "/knowledge")
	if idx == -1 {
		return "/"
	}
	action := fullPath[idx+len("/knowledge"):]
	if action == "" || action == "/" {
		return "/"
	}
	if !strings.HasPrefix(action, "/") {
		action = "/" + action
	}
	return action
}
