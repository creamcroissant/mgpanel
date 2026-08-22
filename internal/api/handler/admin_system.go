// 文件路径: internal/api/handler/admin_system.go
// 模块说明: 这是 internal 模块里的 admin_system 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

// AdminSystemHandler 提供系统仪表盘接口。
type AdminSystemHandler struct {
	system   service.AdminSystemService
	settings service.AdminSystemSettingsService
	i18n     *i18n.Manager
}

// NewAdminSystemHandler 绑定 service 实例。
func NewAdminSystemHandler(system service.AdminSystemService) *AdminSystemHandler {
	return &AdminSystemHandler{system: system, i18n: system.I18n()}
}

// NewAdminSystemSettingsHandler 绑定 system 与 settings service。
func NewAdminSystemSettingsHandler(system service.AdminSystemService, settings service.AdminSystemSettingsService) *AdminSystemHandler {
	return &AdminSystemHandler{system: system, settings: settings, i18n: system.I18n()}
}

// ServeHTTP 按 /system 子路径分发系统状态/设置/密钥/SMTP 请求。
func (h *AdminSystemHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	action := adminSystemActionPath(r.URL.Path)
	switch {
	case action == "/getSystemStatus" && r.Method == http.MethodGet:
		h.handleGetSystemStatus(w, r)
	case action == "/getQueueStats" && r.Method == http.MethodGet:
		h.handleGetQueueStats(w, r)
	case action == "/settings" && r.Method == http.MethodGet:
		h.handleGetSettings(w, r)
	case action == "/settings" && r.Method == http.MethodPost:
		h.handleSaveSettings(w, r)
	case action == "/smtp/test" && r.Method == http.MethodPost:
		h.handleTestSMTP(w, r)
	case action == "/key" && r.Method == http.MethodGet:
		h.handleGetKey(w, r)
	case action == "/key/reveal" && r.Method == http.MethodPost:
		h.handleRevealKey(w, r)
	case action == "/key/reset" && r.Method == http.MethodPost:
		h.handleResetKey(w, r)
	default:
		respondNotImplemented(w, "admin.system", r)
	}
}

// handleGetSystemStatus 获取系统状态数据并返回。
func (h *AdminSystemHandler) handleGetSystemStatus(w http.ResponseWriter, r *http.Request) {
	if h.system == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.system.status", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.system.status", "error.unauthorized", h.i18n)
		return
	}
	status, err := h.system.SystemStatus(r.Context())
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.system.status", "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, status)
}

// Status 提供 RESTful 形式的系统状态接口。
func (h *AdminSystemHandler) Status(w http.ResponseWriter, r *http.Request) {
	h.handleGetSystemStatus(w, r)
}

// handleGetQueueStats 获取队列统计数据并返回。
func (h *AdminSystemHandler) handleGetQueueStats(w http.ResponseWriter, r *http.Request) {
	if h.system == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.system.queue", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.system.queue", "error.unauthorized", h.i18n)
		return
	}
	stats, err := h.system.QueueStats(r.Context())
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.system.queue", "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, stats)
}

// adminSystemActionPath 提取 /system 后的动作子路径。
func adminSystemActionPath(fullPath string) string {
	idx := strings.Index(fullPath, "/system")
	if idx == -1 {
		return "/"
	}
	tail := fullPath[idx+len("/system"):]
	if tail == "" || tail == "/" {
		return "/"
	}
	if !strings.HasPrefix(tail, "/") {
		tail = "/" + tail
	}
	return tail
}

// handleGetSettings 读取指定分类的系统设置。
func (h *AdminSystemHandler) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	if h.settings == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.system.settings.fetch", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.system.settings.fetch", "error.unauthorized", h.i18n)
		return
	}
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	if category == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.system.settings.fetch", "error.missing_fields", h.i18n)
		return
	}
	settings, err := h.settings.GetByCategory(r.Context(), category)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.system.settings.fetch", "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": settings})
}

type adminSettingsSaveRequest struct {
	Category string            `json:"category"`
	Settings map[string]string `json:"settings"`
}

type adminSMTPTestRequest struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Encryption  string `json:"encryption"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	FromAddress string `json:"from_address"`
	ToAddress   string `json:"to_address"`
}

type adminKeyRevealRequest struct {
	Confirm bool `json:"confirm"`
}

func communicationKeyActionRequest(r *http.Request, claims requestctx.AdminClaims) service.CommunicationKeyActionRequest {
	return service.CommunicationKeyActionRequest{ActorID: claims.ID, IP: clientIP(r), UserAgent: r.UserAgent()}
}

func communicationKeyAuditRequest(r *http.Request, claims requestctx.AdminClaims, action, result, reason string) service.CommunicationKeyAuditRequest {
	return service.CommunicationKeyAuditRequest{Action: action, ActorID: claims.ID, IP: clientIP(r), UserAgent: r.UserAgent(), Result: result, Reason: reason}
}

// handleSaveSettings 保存指定分类的系统设置。
func (h *AdminSystemHandler) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	if h.settings == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.system.settings.save", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.system.settings.save", "error.unauthorized", h.i18n)
		return
	}
	var payload adminSettingsSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.system.settings.save", "error.bad_request", h.i18n)
		return
	}
	category := strings.TrimSpace(payload.Category)
	if category == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.system.settings.save", "error.missing_fields", h.i18n)
		return
	}
	if err := h.settings.SaveSettings(r.Context(), category, payload.Settings); err != nil {
		if errors.Is(err, service.ErrSystemSettingsInvalid) {
			details := map[string]any{}
			var validationErr *service.SystemSettingsValidationError
			if errors.As(err, &validationErr) {
				details["violations"] = validationErr.Violations
			}
			message := "error.bad_request"
			if h.i18n != nil {
				message = h.i18n.Translate(requestctx.GetLanguage(r.Context()), "error.bad_request")
			}
			resp := map[string]any{
				"error":  message,
				"action": "admin.system.settings.save",
			}
			if len(details) > 0 {
				resp["details"] = details
			}
			respondJSON(w, http.StatusBadRequest, resp)
			return
		}
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.system.settings.save", "error.internal_server_error", h.i18n)
		return
	}
	RespondSuccessI18n(r.Context(), w, "success.updated", h.i18n, nil)
}

// handleTestSMTP 校验 SMTP 配置并触发测试流程。
func (h *AdminSystemHandler) handleTestSMTP(w http.ResponseWriter, r *http.Request) {
	if h.settings == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.system.smtp.test", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.system.smtp.test", "error.unauthorized", h.i18n)
		return
	}
	var payload adminSMTPTestRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.system.smtp.test", "error.bad_request", h.i18n)
		return
	}
	config := service.SMTPConfig{
		Host:        payload.Host,
		Port:        payload.Port,
		Encryption:  payload.Encryption,
		Username:    payload.Username,
		Password:    payload.Password,
		FromAddress: payload.FromAddress,
		ToAddress:   payload.ToAddress,
	}
	start := time.Now()
	if err := h.settings.TestSMTP(r.Context(), config); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.system.smtp.test", "error.bad_request", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"message": "success.updated",
		"data": map[string]any{
			"encryption": strings.ToLower(strings.TrimSpace(payload.Encryption)),
			"elapsed_ms": time.Since(start).Milliseconds(),
		},
	})
}

// handleGetKey 返回掩码后的通讯密钥。
func (h *AdminSystemHandler) handleGetKey(w http.ResponseWriter, r *http.Request) {
	if h.settings == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.system.key", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.system.key", "error.unauthorized", h.i18n)
		return
	}
	result, err := h.settings.GetMaskedCommunicationKey(r.Context())
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.system.key", "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, result)
}

// handleRevealKey 校验确认后返回明文通讯密钥。
func (h *AdminSystemHandler) handleRevealKey(w http.ResponseWriter, r *http.Request) {
	if h.settings == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.system.key.reveal", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		h.settings.AuditCommunicationKeyAction(r.Context(), communicationKeyAuditRequest(r, claims, "reveal", "failure", "unauthorized"))
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.system.key.reveal", "error.unauthorized", h.i18n)
		return
	}
	var payload adminKeyRevealRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.settings.AuditCommunicationKeyAction(r.Context(), communicationKeyAuditRequest(r, claims, "reveal", "failure", "bad_request"))
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.system.key.reveal", "error.bad_request", h.i18n)
		return
	}
	if !payload.Confirm {
		h.settings.AuditCommunicationKeyAction(r.Context(), communicationKeyAuditRequest(r, claims, "reveal", "failure", "confirmation_required"))
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.system.key.reveal", "error.bad_request", h.i18n)
		return
	}
	result, err := h.settings.GetCommunicationKey(r.Context(), communicationKeyActionRequest(r, claims))
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.system.key.reveal", "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, result)
}

// handleResetKey 重置通讯密钥并返回新值。
func (h *AdminSystemHandler) handleResetKey(w http.ResponseWriter, r *http.Request) {
	if h.settings == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.system.key.reset", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		h.settings.AuditCommunicationKeyAction(r.Context(), communicationKeyAuditRequest(r, claims, "reset", "failure", "unauthorized"))
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.system.key.reset", "error.unauthorized", h.i18n)
		return
	}
	result, err := h.settings.ResetCommunicationKey(r.Context(), communicationKeyActionRequest(r, claims))
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.system.key.reset", "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, result)
}
