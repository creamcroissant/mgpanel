// 文件路径: internal/api/handler/passport.go
// 模块说明: 这是 internal 模块里的 passport 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/creamcroissant/xboard/internal/service"
	"github.com/creamcroissant/xboard/internal/support/i18n"
)

// PassportHandler handles auth/registration endpoints.
type PassportHandler struct {
	auth     service.AuthService
	verify   service.VerificationService
	passwd   service.PasswordService
	register service.RegistrationService
	mailLink service.MailLinkService
	comm     service.CommService
	i18n     *i18n.Manager
}

func NewPassportHandler(auth service.AuthService, verify service.VerificationService, passwd service.PasswordService, register service.RegistrationService, mailLink service.MailLinkService, comm service.CommService, i18n *i18n.Manager) *PassportHandler {
	return &PassportHandler{auth: auth, verify: verify, passwd: passwd, register: register, mailLink: mailLink, comm: comm, i18n: i18n}
}

func (h *PassportHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := passportActionPath(r.URL.Path)
	switch {
	case strings.HasPrefix(path, "/auth/login") && r.Method == http.MethodPost:
		h.handleLogin(w, r)
	case strings.HasPrefix(path, "/auth/token2Login") && r.Method == http.MethodGet:
		h.handleToken2Login(w, r)
	case strings.HasPrefix(path, "/auth/register") && r.Method == http.MethodPost:
		h.handleRegister(w, r)
	case strings.HasPrefix(path, "/auth/refresh") && r.Method == http.MethodPost:
		h.handleRefresh(w, r)
	case strings.HasPrefix(path, "/auth/logout") && r.Method == http.MethodPost:
		h.handleLogout(w, r)
	case strings.HasPrefix(path, "/auth/forget") && r.Method == http.MethodPost:
		h.handleForget(w, r)
	case strings.HasPrefix(path, "/auth/loginWithMailLink") && r.Method == http.MethodPost:
		h.handleMailLinkLogin(w, r)
	case strings.HasPrefix(path, "/auth/getQuickLoginUrl") && r.Method == http.MethodPost:
		h.handleQuickLoginURL(w, r)
	case strings.HasPrefix(path, "/comm/config") && r.Method == http.MethodGet:
		h.handleCommConfig(w, r)
	case strings.HasPrefix(path, "/comm/sendEmailVerify") && r.Method == http.MethodPost:
		h.handleSendEmailVerify(w, r)
	case strings.HasPrefix(path, "/comm/pv") && r.Method == http.MethodPost:
	default:
		respondNotImplemented(w, "passport", r)
	}
}

type loginRequest struct {
	Email      string `json:"email"`
	Username   string `json:"username"`
	Account    string `json:"account"`
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type forgetRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	EmailCode string `json:"email_code"`
}

type registerRequest struct {
	Email      string `json:"email"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	EmailCode  string `json:"email_code"`
}

type mailLinkRequest struct {
	Email    string `json:"email"`
	Redirect string `json:"redirect"`
}

type quickLoginRequest struct {
	AuthData string `json:"auth_data"`
	Redirect string `json:"redirect"`
}

type emailVerifyRequest struct {
	Email            string `json:"email"`
	TurnstileToken   string `json:"turnstile_token"`
	RecaptchaToken   string `json:"recaptcha_data"`
	RecaptchaV3Token string `json:"recaptcha_v3_token"`
}

func (h *PassportHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if h.auth == nil {
		RespondErrorI18n(r.Context(), w, http.StatusServiceUnavailable, "error.service_unavailable", h.i18n)
		return
	}
	var payload loginRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.bad_request", h.i18n)
		return
	}
	identifier := firstNonEmpty(strings.TrimSpace(payload.Email), strings.TrimSpace(payload.Username), strings.TrimSpace(payload.Account), strings.TrimSpace(payload.Identifier))
	if identifier == "" || strings.TrimSpace(payload.Password) == "" {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.missing_credentials", h.i18n)
		return
	}
	result, err := h.auth.Login(r.Context(), service.LoginInput{
		Identifier: identifier,
		Password:   payload.Password,
		IP:         clientIP(r),
		UserAgent:  r.UserAgent(),
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			RespondErrorI18n(r.Context(), w, http.StatusUnauthorized, "error.invalid_credentials", h.i18n)
		case errors.Is(err, service.ErrRateLimited):
			RespondErrorI18n(r.Context(), w, http.StatusTooManyRequests, "error.rate_limited", h.i18n)
		case errors.Is(err, service.ErrAccountDisabled):
			RespondErrorI18n(r.Context(), w, http.StatusForbidden, "error.account_disabled", h.i18n)
		default:
			slog.Error("login failed with unexpected error", "error", err, "identifier", identifier)
			RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "error.internal_server_error", h.i18n)
		}
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": formatAuthResponse(result)})
}

func (h *PassportHandler) handleRegister(w http.ResponseWriter, r *http.Request) {
	if h.register == nil || h.auth == nil {
		RespondErrorI18n(r.Context(), w, http.StatusServiceUnavailable, "error.service_unavailable", h.i18n)
		return
	}
	var payload registerRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.bad_request", h.i18n)
		return
	}
	email := strings.TrimSpace(payload.Email)
	username := strings.TrimSpace(payload.Username)
	if (email == "" && username == "") || strings.TrimSpace(payload.Password) == "" {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.missing_credentials", h.i18n)
		return
	}
	user, err := h.register.Register(r.Context(), service.RegistrationInput{
		Email:      payload.Email,
		Username:   payload.Username,
		Password:   payload.Password,
		EmailCode:  payload.EmailCode,
		IP:         clientIP(r),
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidEmail):
			RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.invalid_email", h.i18n)
		case errors.Is(err, service.ErrInvalidUsername):
			RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.invalid_username", h.i18n)
		case errors.Is(err, service.ErrInvalidPassword):
			RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.invalid_password", h.i18n)
		case errors.Is(err, service.ErrInvalidVerificationCode):
			RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.invalid_verification_code", h.i18n)
		case errors.Is(err, service.ErrEmailDomainNotAllowed):
			RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.email_domain_not_allowed", h.i18n)
		case errors.Is(err, service.ErrIdentifierRequired):
			RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.identifier_required", h.i18n)
		case errors.Is(err, service.ErrEmailExists):
			RespondErrorI18n(r.Context(), w, http.StatusConflict, "error.email_exists", h.i18n)
		case errors.Is(err, service.ErrUsernameExists):
			RespondErrorI18n(r.Context(), w, http.StatusConflict, "error.username_exists", h.i18n)
		case errors.Is(err, service.ErrRegistrationClosed):
			RespondErrorI18n(r.Context(), w, http.StatusForbidden, "error.registration_closed", h.i18n)
		case errors.Is(err, service.ErrRateLimited):
			RespondErrorI18n(r.Context(), w, http.StatusTooManyRequests, "error.rate_limited", h.i18n)
		default:
			RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "error.internal_server_error", h.i18n)
		}
		return
	}
	result, err := h.auth.IssueForUser(r.Context(), user.ID)
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": formatAuthResponse(result)})
}

func (h *PassportHandler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if h.auth == nil {
		RespondErrorI18n(r.Context(), w, http.StatusServiceUnavailable, "error.service_unavailable", h.i18n)
		return
	}
	var payload refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.bad_request", h.i18n)
		return
	}
	result, err := h.auth.Refresh(r.Context(), payload.RefreshToken)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRefreshToken):
			RespondErrorI18n(r.Context(), w, http.StatusUnauthorized, "error.invalid_refresh_token", h.i18n)
		case errors.Is(err, service.ErrAccountDisabled):
			RespondErrorI18n(r.Context(), w, http.StatusForbidden, "error.account_disabled", h.i18n)
		default:
			RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "error.internal_server_error", h.i18n)
		}
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": formatAuthResponse(result)})
}

func (h *PassportHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if h.auth == nil {
		RespondErrorI18n(r.Context(), w, http.StatusServiceUnavailable, "error.service_unavailable", h.i18n)
		return
	}
	var payload refreshRequest
	_ = json.NewDecoder(r.Body).Decode(&payload)
	_ = h.auth.Logout(r.Context(), payload.RefreshToken)
	RespondSuccessI18n(r.Context(), w, "success.logout", h.i18n, nil)
}

func (h *PassportHandler) handleForget(w http.ResponseWriter, r *http.Request) {
	if h.passwd == nil {
		RespondErrorI18n(r.Context(), w, http.StatusServiceUnavailable, "error.service_unavailable", h.i18n)
		return
	}
	var payload forgetRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.bad_request", h.i18n)
		return
	}
	if strings.TrimSpace(payload.Email) == "" || strings.TrimSpace(payload.Password) == "" || strings.TrimSpace(payload.EmailCode) == "" {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.missing_credentials", h.i18n)
		return
	}
	if err := h.passwd.Reset(r.Context(), service.PasswordResetInput{
		Email:     payload.Email,
		Password:  payload.Password,
		EmailCode: payload.EmailCode,
	}); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidEmail):
			RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.invalid_email", h.i18n)
		case errors.Is(err, service.ErrInvalidPassword):
			RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.invalid_password", h.i18n)
		case errors.Is(err, service.ErrInvalidVerificationCode):
			RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.invalid_verification_code", h.i18n)
		case errors.Is(err, service.ErrRateLimited):
			RespondErrorI18n(r.Context(), w, http.StatusTooManyRequests, "error.rate_limited", h.i18n)
		case errors.Is(err, service.ErrNotFound):
			RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.user_not_found", h.i18n)
		default:
			RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "error.internal_server_error", h.i18n)
		}
		return
	}
	RespondSuccessI18n(r.Context(), w, "success.password_reset", h.i18n, nil)
}

func (h *PassportHandler) handleMailLinkLogin(w http.ResponseWriter, r *http.Request) {
	if h.mailLink == nil {
		RespondErrorI18n(r.Context(), w, http.StatusServiceUnavailable, "error.service_unavailable", h.i18n)
		return
	}
	var payload mailLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.bad_request", h.i18n)
		return
	}
	if strings.TrimSpace(payload.Email) == "" {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.email_required", h.i18n)
		return
	}
	link, err := h.mailLink.SendLoginLink(r.Context(), service.MailLinkInput{
		Email:    payload.Email,
		Redirect: payload.Redirect,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrFeatureDisabled):
			RespondErrorI18n(r.Context(), w, http.StatusNotFound, "error.feature_disabled", h.i18n)
		case errors.Is(err, service.ErrInvalidEmail):
			RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.invalid_email", h.i18n)
		case errors.Is(err, service.ErrCooldownActive):
			RespondErrorI18n(r.Context(), w, http.StatusTooManyRequests, "error.cooldown_active", h.i18n)
		default:
			RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "error.internal_server_error", h.i18n)
		}
		return
	}
	resp := map[string]any{"status": "ok"}
	if link != "" {
		resp["link"] = link
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *PassportHandler) handleQuickLoginURL(w http.ResponseWriter, r *http.Request) {
	if h.mailLink == nil || h.auth == nil {
		RespondErrorI18n(r.Context(), w, http.StatusServiceUnavailable, "error.service_unavailable", h.i18n)
		return
	}
	var payload quickLoginRequest
	_ = json.NewDecoder(r.Body).Decode(&payload)
	authData := payload.AuthData
	if strings.TrimSpace(authData) == "" {
		authData = r.Header.Get("Authorization")
	}
	normalized := normalizeAuthHeader(authData)
	if normalized == "" {
		RespondErrorI18n(r.Context(), w, http.StatusUnauthorized, "error.unauthorized", h.i18n)
		return
	}
	claims, err := h.auth.Verify(r.Context(), normalized)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUnauthorized):
			RespondErrorI18n(r.Context(), w, http.StatusUnauthorized, "error.unauthorized", h.i18n)
		case errors.Is(err, service.ErrAccountDisabled):
			RespondErrorI18n(r.Context(), w, http.StatusForbidden, "error.account_disabled", h.i18n)
		default:
			RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "error.internal_server_error", h.i18n)
		}
		return
	}
	url, err := h.mailLink.GenerateQuickLoginURL(r.Context(), claims.UserID, payload.Redirect)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAccountDisabled):
			RespondErrorI18n(r.Context(), w, http.StatusForbidden, "error.account_disabled", h.i18n)
		case errors.Is(err, service.ErrUnauthorized):
			RespondErrorI18n(r.Context(), w, http.StatusUnauthorized, "error.unauthorized", h.i18n)
		default:
			RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "error.internal_server_error", h.i18n)
		}
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"url": url})
}

func (h *PassportHandler) handleToken2Login(w http.ResponseWriter, r *http.Request) {
	if h.mailLink == nil {
		RespondErrorI18n(r.Context(), w, http.StatusServiceUnavailable, "error.service_unavailable", h.i18n)
		return
	}
	query := r.URL.Query()
	if token := strings.TrimSpace(query.Get("token")); token != "" {
		link, err := h.mailLink.BuildLoginRedirect(r.Context(), token, query.Get("redirect"))
		if err != nil {
			RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.bad_request", h.i18n)
			return
		}
		http.Redirect(w, r, link, http.StatusFound)
		return
	}
	verify := strings.TrimSpace(query.Get("verify"))
	if verify == "" {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.verify_token_required", h.i18n)
		return
	}
	userID, err := h.mailLink.ConsumeToken(r.Context(), verify)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidToken):
			RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.invalid_token", h.i18n)
		case errors.Is(err, service.ErrAccountDisabled):
			RespondErrorI18n(r.Context(), w, http.StatusForbidden, "error.account_disabled", h.i18n)
		default:
			RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "error.internal_server_error", h.i18n)
		}
		return
	}
	result, err := h.auth.IssueForUser(r.Context(), userID)
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": formatAuthResponse(result)})
}

func (h *PassportHandler) handleSendEmailVerify(w http.ResponseWriter, r *http.Request) {
	if h.verify == nil {
		RespondErrorI18n(r.Context(), w, http.StatusServiceUnavailable, "error.service_unavailable", h.i18n)
		return
	}
	var payload emailVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.bad_request", h.i18n)
		return
	}
	if strings.TrimSpace(payload.Email) == "" {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.email_required", h.i18n)
		return
	}
	input := service.EmailVerificationInput{
		Email:            payload.Email,
		IP:               clientIP(r),
		UserAgent:        r.UserAgent(),
		TurnstileToken:   payload.TurnstileToken,
		RecaptchaToken:   payload.RecaptchaToken,
		RecaptchaV3Token: payload.RecaptchaV3Token,
	}
	if err := h.verify.SendEmailCode(r.Context(), input); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidEmail):
			RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.invalid_email", h.i18n)
		case errors.Is(err, service.ErrInvalidCaptcha):
			RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.invalid_captcha", h.i18n)
		case errors.Is(err, service.ErrEmailDomainNotAllowed):
			RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "error.email_domain_not_allowed", h.i18n)
		case errors.Is(err, service.ErrCooldownActive):
			RespondErrorI18n(r.Context(), w, http.StatusTooManyRequests, "error.cooldown_active", h.i18n)
		default:
			RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "error.internal_server_error", h.i18n)
		}
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *PassportHandler) handleCommConfig(w http.ResponseWriter, r *http.Request) {
	if h.comm == nil {
		RespondErrorI18n(r.Context(), w, http.StatusServiceUnavailable, "error.service_unavailable", h.i18n)
		return
	}
	cfg, err := h.comm.GuestConfig(r.Context())
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, cfg)
}

func passportActionPath(fullPath string) string {
	idx := strings.Index(fullPath, "/passport")
	if idx == -1 {
		return fullPath
	}
	sub := fullPath[idx+len("/passport"):]
	if sub == "" {
		return "/"
	}
	return sub
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func clientIP(r *http.Request) string {
	remoteIP := parseIP(r.RemoteAddr)
	if remoteIP == "" {
		return ""
	}
	if !isTrustedProxy(remoteIP) {
		return remoteIP
	}
	for _, header := range []string{"X-Forwarded-For", "X-Real-IP"} {
		if value := strings.TrimSpace(r.Header.Get(header)); value != "" {
			parts := strings.Split(value, ",")
			if ip := strings.TrimSpace(parts[0]); ip != "" {
				return ip
			}
		}
	}
	return remoteIP
}

func parseIP(addr string) string {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(trimmed); err == nil {
		return host
	}
	return trimmed
}

func isTrustedProxy(remoteIP string) bool {
	if remoteIP == "127.0.0.1" || remoteIP == "::1" {
		return true
	}
	if strings.HasPrefix(remoteIP, "10.") || strings.HasPrefix(remoteIP, "192.168.") {
		return true
	}
	if strings.HasPrefix(remoteIP, "172.") {
		parts := strings.Split(remoteIP, ".")
		if len(parts) > 1 {
			if second, err := strconv.Atoi(parts[1]); err == nil {
				if second >= 16 && second <= 31 {
					return true
				}
			}
		}
	}
	return false
}

func formatAuthResponse(result *service.LoginResult) map[string]any {
	if result == nil {
		return map[string]any{}
	}
	resp := map[string]any{
		"token":            result.Token,
		"token_type":       "Bearer",
		"token_expires_at": result.ExpiresAt.Unix(),
		"auth_data":        "Bearer " + result.Token,
		"user": map[string]any{
			"id":       result.UserID,
			"email":    result.Email,
			"username": result.Username,
			"is_admin": result.IsAdmin,
		},
	}
	if result.RefreshToken != "" {
		resp["refresh_token"] = result.RefreshToken
		resp["refresh_expires_at"] = result.RefreshExpiresAt.Unix()
	}
	return resp
}

func normalizeAuthHeader(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "bearer ") {
		return strings.TrimSpace(trimmed[7:])
	}
	return trimmed
}
