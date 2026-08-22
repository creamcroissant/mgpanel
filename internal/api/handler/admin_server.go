// 文件路径: internal/api/handler/admin_server.go
// 模块说明: 这是 internal 模块里的 admin_server 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package handler

import (
	"net/http"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
)

// AdminServerHandler 提供管理端节点/分组/路由相关接口。
type AdminServerHandler struct {
	servers service.AdminServerService
}

// NewAdminServerHandler 创建管理端节点接口处理器。
func NewAdminServerHandler(servers service.AdminServerService) *AdminServerHandler {
	return &AdminServerHandler{servers: servers}
}

func (h *AdminServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.servers == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.server", "error.service_unavailable", nil)
		return
	}
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18n(r.Context(), w, http.StatusUnauthorized, "error.unauthorized", h.servers.I18n())
		return
	}
	action := adminActionPath(r.URL.Path)
	// 根据 action 路径分发管理端节点相关操作。
	switch {
	case strings.HasPrefix(action, "/server/group") && strings.HasSuffix(action, "/fetch") && r.Method == http.MethodGet:
		h.handleGroupFetch(w, r)
	case strings.HasPrefix(action, "/server/route") && strings.HasSuffix(action, "/fetch") && r.Method == http.MethodGet:
		h.handleRouteFetch(w, r)
	case isAdminServerNodeFetch(action) && r.Method == http.MethodGet:
		h.handleNodeFetch(w, r)
	case strings.HasPrefix(action, "/server/manage/save") && r.Method == http.MethodPost:
		h.handleNodeSave(w, r)
	case strings.HasPrefix(action, "/server/manage/drop") && r.Method == http.MethodPost:
		h.handleNodeDrop(w, r)
	case strings.HasPrefix(action, "/server/manage/batchDrop") && r.Method == http.MethodPost:
		h.handleNodeBatchDrop(w, r)
	case strings.HasPrefix(action, "/server/manage/batchUpdate") && r.Method == http.MethodPost:
		h.handleNodeBatchUpdate(w, r)
	default:
		respondNotImplemented(w, "admin.server", r)
	}
}

func (h *AdminServerHandler) handleGroupFetch(w http.ResponseWriter, r *http.Request) {
	// 返回分组列表给管理端。
	groups, err := h.servers.Groups(r.Context())
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "admin.server.group.fetch", h.servers.I18n())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": groups, "count": len(groups)})
}

func (h *AdminServerHandler) handleRouteFetch(w http.ResponseWriter, r *http.Request) {
	// 返回路由规则列表给管理端。
	routes, err := h.servers.Routes(r.Context())
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "admin.server.route.fetch", h.servers.I18n())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": routes, "count": len(routes)})
}

func (h *AdminServerHandler) handleNodeFetch(w http.ResponseWriter, r *http.Request) {
	// 返回节点列表给管理端。
	nodes, err := h.servers.Nodes(r.Context())
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "admin.server.manage.fetch", h.servers.I18n())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": nodes, "count": len(nodes)})
}

func (h *AdminServerHandler) handleNodeSave(w http.ResponseWriter, r *http.Request) {
	// 保存单个节点的创建/更新请求。
	var input service.AdminServerNodeSaveInput
	if err := decodeJSON(r, &input); err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.server.manage.save", h.servers.I18n())
		return
	}
	if input.Name == "" {
		RespondErrorI18n(r.Context(), w, http.StatusUnprocessableEntity, "admin.server.manage.save", h.servers.I18n())
		return
	}
	if err := h.servers.SaveNode(r.Context(), input); err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "admin.server.manage.save", h.servers.I18n())
		return
	}
	RespondSuccessI18n(r.Context(), w, "success.updated", h.servers.I18n(), nil)
}

func (h *AdminServerHandler) handleNodeDrop(w http.ResponseWriter, r *http.Request) {
	// 删除单个节点。
	var input struct {
		ID int64 `json:"id"`
	}
	if err := decodeJSON(r, &input); err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.server.manage.drop", h.servers.I18n())
		return
	}
	if err := h.servers.DeleteNode(r.Context(), input.ID); err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "admin.server.manage.drop", h.servers.I18n())
		return
	}
	RespondSuccessI18n(r.Context(), w, "success.deleted", h.servers.I18n(), nil)
}

func (h *AdminServerHandler) handleNodeBatchDrop(w http.ResponseWriter, r *http.Request) {
	// 批量删除节点。
	var input struct {
		IDs []int64 `json:"ids"`
	}
	if err := decodeJSON(r, &input); err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.server.manage.batchDrop", h.servers.I18n())
		return
	}
	if len(input.IDs) == 0 {
		RespondErrorI18n(r.Context(), w, http.StatusUnprocessableEntity, "admin.server.manage.batchDrop", h.servers.I18n())
		return
	}
	for _, id := range input.IDs {
		if err := h.servers.DeleteNode(r.Context(), id); err != nil {
			RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "admin.server.manage.batchDrop", h.servers.I18n())
			return
		}
	}
	RespondSuccessI18n(r.Context(), w, "success.deleted", h.servers.I18n(), nil)
}

func (h *AdminServerHandler) handleNodeBatchUpdate(w http.ResponseWriter, r *http.Request) {
	// 批量更新节点的展示/状态字段。
	var input struct {
		IDs    []int64 `json:"ids"`
		Show   *int    `json:"show,omitempty"`
		Status *int    `json:"status,omitempty"`
	}
	if err := decodeJSON(r, &input); err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.server.manage.batchUpdate", h.servers.I18n())
		return
	}
	if len(input.IDs) == 0 {
		RespondErrorI18n(r.Context(), w, http.StatusUnprocessableEntity, "admin.server.manage.batchUpdate", h.servers.I18n())
		return
	}
	for _, id := range input.IDs {
		updateInput := service.AdminServerNodeSaveInput{ID: id}
		if input.Show != nil {
			updateInput.Show = *input.Show
		}
		if input.Status != nil {
			updateInput.Status = *input.Status
		}
		if err := h.servers.SaveNode(r.Context(), updateInput); err != nil {
			RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "admin.server.manage.batchUpdate", h.servers.I18n())
			return
		}
	}
	RespondSuccessI18n(r.Context(), w, "success.updated", h.servers.I18n(), nil)
}

func isAdminServerNodeFetch(action string) bool {
	// 兼容不同路径写法的节点列表查询。
	trimmed := strings.TrimSuffix(strings.TrimSpace(action), "/")
	if strings.HasPrefix(trimmed, "/server/manage") && strings.HasSuffix(trimmed, "/fetch") {
		return true
	}
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(lower, "/server/manage") && strings.HasSuffix(lower, "/getnodes")
}
