// 文件路径: internal/api/handler/admin.go
// 模块说明: 这是 internal 模块里的 admin 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
)

// AdminHandler 负责复刻旧版管理端入口的路由分发。
type AdminHandler struct {
	Config service.ConfigService
}

func NewAdminHandler(cfgService service.ConfigService) *AdminHandler {
	return &AdminHandler{Config: cfgService}
}

func (h *AdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18n(r.Context(), w, http.StatusUnauthorized, "error.unauthorized", h.Config.I18n())
		return
	}
	// 解析实际动作路径，剥离前缀与多余斜杠。
	path := adminActionPath(r.URL.Path)
	switch {
	case path == "/config" && r.Method == http.MethodGet:
		h.handleConfigFetch(w, r)
	case strings.HasPrefix(path, "/config/") && strings.HasSuffix(path, "/fetch") && r.Method == http.MethodGet:
		h.handleConfigFetch(w, r)
	case strings.HasPrefix(path, "/config/") && strings.HasSuffix(path, "/save") && r.Method == http.MethodPost:
		h.handleConfigSave(w, r)
		// TODO: add more admin endpoints as services become available.
	default:
		respondNotImplemented(w, "admin", r)
	}
}

func (h *AdminHandler) handleConfigFetch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// 拉取全量配置并返回给管理端。
	data, err := h.Config.Fetch(ctx)
	if err != nil {
		RespondErrorI18n(ctx, w, http.StatusInternalServerError, "config.fetch", h.Config.I18n())
		return
	}
	respondJSON(w, http.StatusOK, data)
}

func (h *AdminHandler) handleConfigSave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18n(ctx, w, http.StatusBadRequest, "config.save", h.Config.I18n())
		return
	}
	// 保存配置后返回统一成功提示。
	if err := h.Config.Save(ctx, payload); err != nil {
		RespondErrorI18n(ctx, w, http.StatusInternalServerError, "config.save", h.Config.I18n())
		return
	}
	RespondSuccessI18n(ctx, w, "success.updated", h.Config.I18n(), nil)
}

func adminActionPath(fullPath string) string {
	// 过滤查询参数并规整路径片段。
	path := fullPath
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	segments := strings.Split(path, "/")
	filtered := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		filtered = append(filtered, segment)
	}
	if len(filtered) <= 3 {
		return "/"
	}
	remainder := strings.Join(filtered[3:], "/")
	if remainder == "" {
		return "/"
	}
	return "/" + remainder
}
