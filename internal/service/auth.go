// 文件路径: internal/service/auth.go
// 模块说明: 这是 internal 模块里的 auth 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/creamcroissant/mgpanel/internal/auth/token"
	"github.com/creamcroissant/mgpanel/internal/cache"
	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/security"
	"github.com/creamcroissant/mgpanel/internal/support/hash"
	"github.com/google/uuid"
)

// AuthService coordinates login and session issuance compatible with Passport endpoints.
type AuthService interface {
	Login(ctx context.Context, input LoginInput) (*LoginResult, error)
	Verify(ctx context.Context, rawToken string) (*Claims, error)
	Refresh(ctx context.Context, refreshToken string) (*LoginResult, error)
	Logout(ctx context.Context, refreshToken string) error
	IssueForUser(ctx context.Context, userID int64) (*LoginResult, error)
}

// LoginInput represents the payload required for user login.
type LoginInput struct {
	Identifier string
	Password   string
	IP         string
	UserAgent  string
}

// LoginResult returns issued token information and user snapshot.
type LoginResult struct {
	Token            string    `json:"token"`
	ExpiresAt        time.Time `json:"expires_at"`
	UserID           int64     `json:"user_id"`
	Email            string    `json:"email"`
	Username         string    `json:"username"`
	IsAdmin          bool      `json:"is_admin"`
	RefreshToken     string    `json:"refresh_token"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

// Claims describe authenticated user payload extracted from tokens.
type Claims struct {
	UserID  int64  `json:"user_id"`
	Email   string `json:"email"`
	IsAdmin bool   `json:"is_admin"`
}

type authService struct {
	users         repository.UserRepository
	settings      repository.SettingRepository
	loginLogs     repository.LoginLogRepository
	tokens        repository.TokenRepository
	hasher        hash.Hasher
	tokenMgr      *token.Manager
	rate          *security.RateLimiter
	audit         security.Recorder
	loginFailures cache.Store
	usedTokens    sync.Map // recently rotated refresh tokens (reuse detection)
}

const (
	loginLimit      = 100
	loginWindow     = time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour
)

// NewAuthService wires repository + infrastructure helpers.
func NewAuthService(users repository.UserRepository, settings repository.SettingRepository, loginLogs repository.LoginLogRepository, tokens repository.TokenRepository, hasher hash.Hasher, tokenMgr *token.Manager, rate *security.RateLimiter, audit security.Recorder, cacheStore cache.Store) AuthService {
	var loginFailures cache.Store
	if cacheStore != nil {
		namespace := cacheStore.Namespace("auth")
		loginFailures = namespace.Namespace("password_fail")
	}
	return &authService{
		users:         users,
		settings:      settings,
		loginLogs:     loginLogs,
		tokens:        tokens,
		hasher:        hasher,
		tokenMgr:      tokenMgr,
		rate:          rate,
		audit:         audit,
		loginFailures: loginFailures,
	}
}

func (s *authService) Login(ctx context.Context, input LoginInput) (*LoginResult, error) {
	if s == nil || s.users == nil || s.tokenMgr == nil || s.hasher == nil {
		return nil, fmt.Errorf("auth service not fully configured / 认证服务未完整配置")
	}
	identifier := strings.TrimSpace(input.Identifier)
	password := strings.TrimSpace(input.Password)
	if identifier == "" || password == "" {
		return nil, fmt.Errorf("account and password required / 账号和密码不能为空")
	}
	limitKey := strings.ToLower(identifier)
	if err := s.ensurePasswordLimit(ctx, limitKey); err != nil {
		s.recordLoginLog(ctx, nil, identifier, false, "password_limit", input)
		s.recordAudit(ctx, "auth.login.password_limit", identifier, input, map[string]any{"reason": "password_limit"})
		return nil, err
	}

	if s.rate != nil {
		res, err := s.rate.Allow(ctx, "login:"+limitKey, loginLimit, loginWindow)
		if err != nil {
			return nil, err
		}
		if !res.Allowed {
			s.recordLoginLog(ctx, nil, identifier, false, "rate_limited", input)
			s.recordAudit(ctx, "auth.login.rate_limited", identifier, input, map[string]any{"limit": loginLimit})
			return nil, ErrRateLimited
		}
	}

	user, err := s.findUserByIdentifier(ctx, identifier)
	if err != nil {
		if err == repository.ErrNotFound {
			s.recordLoginLog(ctx, nil, identifier, false, "not_found", input)
			s.recordAudit(ctx, "auth.login.failure", identifier, input, map[string]any{"reason": "not_found"})
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if err := compareUserPassword(user, password, s.hasher); err != nil {
		if errors.Is(err, hash.ErrPasswordMismatch) {
			s.bumpLoginFailure(ctx, limitKey)
			s.recordLoginLog(ctx, user, identifier, false, "password_mismatch", input)
			s.recordAudit(ctx, "auth.login.failure", identifier, input, map[string]any{"reason": "password"})
			return nil, ErrInvalidCredentials
		}
		s.recordLoginLog(ctx, user, identifier, false, "password_error", input)
		return nil, err
	}

	if user.Status != 1 || user.Banned {
		reason := map[string]any{"reason": "disabled", "status": user.Status}
		if user.Banned {
			reason["banned"] = true
		}
		s.recordLoginLog(ctx, user, identifier, false, "account_disabled", input)
		s.recordAudit(ctx, "auth.login.failure", identifier, input, reason)
		return nil, ErrAccountDisabled
	}

	result, err := s.issueTokens(ctx, user, &input)
	if err != nil {
		return nil, err
	}
	s.touchLogin(ctx, user, input)
	s.clearLoginFailure(ctx, limitKey)
	s.recordLoginLog(ctx, user, identifier, true, "success", input)
	s.recordAudit(ctx, "auth.login.success", identifier, input, map[string]any{"user_id": user.ID})
	return result, nil
}

func (s *authService) findUserByIdentifier(ctx context.Context, identifier string) (*repository.User, error) {
	email := normalizeEmail(identifier)
	if email != "" {
		user, err := s.users.FindByEmail(ctx, email)
		if err == nil || !errors.Is(err, repository.ErrNotFound) {
			return user, err
		}
	}
	username := normalizeUsername(identifier)
	if username == "" {
		return nil, repository.ErrNotFound
	}
	return s.users.FindByUsername(ctx, username)
}

func preferredIdentifier(user *repository.User) string {
	if user == nil {
		return ""
	}
	if email := strings.TrimSpace(user.Email); email != "" {
		return email
	}
	if username := strings.TrimSpace(user.Username); username != "" {
		return username
	}
	if user.ID > 0 {
		return strconv.FormatInt(user.ID, 10)
	}
	return ""
}

func (s *authService) Verify(ctx context.Context, rawToken string) (*Claims, error) {
	if s == nil || s.users == nil || s.tokenMgr == nil {
		return nil, fmt.Errorf("auth service not fully configured / 认证服务未完整配置")
	}
	tokenStr := strings.TrimSpace(rawToken)
	if tokenStr == "" {
		return nil, ErrUnauthorized
	}
	parsed, err := s.tokenMgr.Parse(tokenStr)
	if err != nil {
		return nil, ErrUnauthorized
	}
	userID, err := strconv.ParseInt(parsed.Subject, 10, 64)
	if err != nil {
		return nil, ErrUnauthorized
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, ErrUnauthorized
	}
	if user.Status != 1 || user.Banned {
		return nil, ErrAccountDisabled
	}
	return &Claims{UserID: user.ID, Email: user.Email, IsAdmin: user.IsAdmin}, nil
}

func (s *authService) ensurePasswordLimit(ctx context.Context, identifier string) error {
	if s == nil || s.loginFailures == nil || strings.TrimSpace(identifier) == "" {
		return nil
	}
	if !s.boolSetting(ctx, "password_limit_enable", true) {
		return nil
	}
	maxAttempts := s.intSetting(ctx, "password_limit_count", 5)
	if maxAttempts <= 0 {
		return nil
	}
	if s.loginFailureCount(ctx, identifier) >= maxAttempts {
		expire := s.intSetting(ctx, "password_limit_expire", 60)
		if expire <= 0 {
			expire = 60
		}
		return fmt.Errorf("%w: retry after %d minutes / 请在 %d 分钟后重试", ErrRateLimited, expire, expire)
	}
	return nil
}

func (s *authService) loginFailureCount(ctx context.Context, identifier string) int {
	if s == nil || s.loginFailures == nil {
		return 0
	}
	key := s.loginFailureKey(identifier)
	if key == "" {
		return 0
	}
	raw, ok := s.loginFailures.Get(ctx, key)
	if !ok {
		return 0
	}
	switch v := raw.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}

func (s *authService) bumpLoginFailure(ctx context.Context, identifier string) {
	if s == nil || s.loginFailures == nil {
		return
	}
	key := s.loginFailureKey(identifier)
	if key == "" {
		return
	}
	expireMinutes := s.intSetting(ctx, "password_limit_expire", 60)
	if expireMinutes <= 0 {
		expireMinutes = 60
	}
	ttl := time.Duration(expireMinutes) * time.Minute
	if _, err := s.loginFailures.Increment(ctx, key, 1, ttl); err != nil {
		return
	}
}

func (s *authService) clearLoginFailure(ctx context.Context, identifier string) {
	if s == nil || s.loginFailures == nil {
		return
	}
	key := s.loginFailureKey(identifier)
	if key == "" {
		return
	}
	s.loginFailures.Delete(ctx, key)
}

func (s *authService) loginFailureKey(identifier string) string {
	trimmed := strings.TrimSpace(strings.ToLower(identifier))
	if trimmed == "" {
		return ""
	}
	return "PASSWORD_ERROR_LIMIT_" + trimmed
}

func (s *authService) settingString(ctx context.Context, key, def string) string {
	if s == nil || s.settings == nil {
		return def
	}
	setting, err := s.settings.Get(ctx, key)
	if err != nil || setting == nil {
		return def
	}
	value := strings.TrimSpace(setting.Value)
	if value == "" {
		return def
	}
	return value
}

func (s *authService) boolSetting(ctx context.Context, key string, def bool) bool {
	val := strings.ToLower(strings.TrimSpace(s.settingString(ctx, key, "")))
	if val == "" {
		return def
	}
	switch val {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func (s *authService) intSetting(ctx context.Context, key string, def int) int {
	val := strings.TrimSpace(s.settingString(ctx, key, ""))
	if val == "" {
		return def
	}
	if n, err := strconv.Atoi(val); err == nil {
		return n
	}
	return def
}

func (s *authService) Refresh(ctx context.Context, refreshToken string) (*LoginResult, error) {
	if s == nil || s.tokens == nil {
		return nil, fmt.Errorf("refresh not supported / 不支持刷新令牌")
	}
	trimmed := strings.TrimSpace(refreshToken)
	if trimmed == "" {
		return nil, ErrInvalidRefreshToken
	}
	// Atomic reuse detection: only the first caller wins
	if _, loaded := s.usedTokens.LoadOrStore(trimmed, true); loaded {
		return nil, ErrTokenRevoked
	}
	record, err := s.tokens.FindByRefreshToken(ctx, trimmed)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, err
	}
	if record.Revoked || record.RefreshExpiresAt <= time.Now().Unix() {
		if err := s.tokens.DeleteByRefreshToken(ctx, trimmed); err != nil {
			slog.Warn("delete refresh token failed (revoked/expired)", "error", err)
		}
		return nil, ErrInvalidRefreshToken
	}
	user, err := s.users.FindByID(ctx, record.UserID)
	if err != nil {
		if err := s.tokens.DeleteByRefreshToken(ctx, trimmed); err != nil {
			slog.Warn("delete refresh token failed (user lookup)", "error", err)
		}
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, err
	}
	if user.Status != 1 || user.Banned {
		return nil, ErrAccountDisabled
	}
	if err := s.tokens.DeleteByRefreshToken(ctx, trimmed); err != nil {
		slog.Warn("delete refresh token failed (rotation cleanup)", "error", err)
	}
	identifier := preferredIdentifier(user)
	meta := &LoginInput{Identifier: identifier, IP: record.IP, UserAgent: record.UserAgent}
	result, err := s.issueTokens(ctx, user, meta)
	if err != nil {
		return nil, err
	}
	s.recordAudit(ctx, "auth.refresh.success", identifier, LoginInput{Identifier: identifier, IP: record.IP, UserAgent: record.UserAgent}, map[string]any{"user_id": user.ID})
	return result, nil
}

func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	if s == nil || s.tokens == nil {
		return nil
	}
	trimmed := strings.TrimSpace(refreshToken)
	if trimmed == "" {
		return nil
	}
	return s.tokens.DeleteByRefreshToken(ctx, trimmed)
}

func (s *authService) IssueForUser(ctx context.Context, userID int64) (*LoginResult, error) {
	if s == nil || s.users == nil {
		return nil, fmt.Errorf("auth service not fully configured / 认证服务未完整配置")
	}
	if userID <= 0 {
		return nil, ErrUnauthorized
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	if user.Status != 1 || user.Banned {
		return nil, ErrAccountDisabled
	}
	return s.issueTokens(ctx, user, nil)
}

func (s *authService) issueTokens(ctx context.Context, user *repository.User, meta *LoginInput) (*LoginResult, error) {
	subject := strconv.FormatInt(user.ID, 10)
	tokenStr, claims, err := s.tokenMgr.Issue(token.IssueInput{
		Subject:   subject,
		TokenType: "access",
		Attributes: map[string]any{
			"email":    user.Email,
			"username": user.Username,
		},
	})
	if err != nil {
		return nil, err
	}
	result := &LoginResult{
		Token:     tokenStr,
		ExpiresAt: claims.ExpiresAt.Time,
		UserID:    user.ID,
		Email:     user.Email,
		Username:  user.Username,
		IsAdmin:   user.IsAdmin,
	}
	if s.tokens != nil {
		refreshToken := uuid.NewString()
		expires := time.Now().UTC().Add(refreshTokenTTL)
		var ip, ua string
		if meta != nil {
			ip = strings.TrimSpace(meta.IP)
			ua = strings.TrimSpace(meta.UserAgent)
		}
		payload := &repository.AccessToken{
			UserID:           user.ID,
			Token:            tokenStr,
			RefreshToken:     refreshToken,
			ExpiresAt:        result.ExpiresAt.Unix(),
			RefreshExpiresAt: expires.Unix(),
			IP:               ip,
			UserAgent:        ua,
		}
		if _, err := s.tokens.Create(ctx, payload); err != nil {
			return nil, fmt.Errorf("store refresh token: %v / 刷新令牌写入失败: %w", err, err)
		}
		result.RefreshToken = refreshToken
		result.RefreshExpiresAt = expires
	}
	return result, nil
}

func (s *authService) recordLoginLog(ctx context.Context, user *repository.User, identifier string, success bool, reason string, input LoginInput) {
	if s == nil || s.loginLogs == nil {
		return
	}
	account := strings.TrimSpace(strings.ToLower(identifier))
	if account == "" && user != nil {
		account = strings.TrimSpace(strings.ToLower(preferredIdentifier(user)))
	}
	if account == "" {
		return
	}
	now := time.Now().Unix()
	entry := &repository.LoginLog{
		Email:     account,
		IP:        strings.TrimSpace(input.IP),
		UserAgent: strings.TrimSpace(input.UserAgent),
		Success:   success,
		Reason:    reason,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if user != nil && user.ID > 0 {
		entry.UserID = &user.ID
	}
	if err := s.loginLogs.Create(ctx, entry); err != nil {
		s.recordAudit(ctx, "auth.login.log_store_failed", account, input, map[string]any{"error": err.Error()})
	}
}

func (s *authService) recordAudit(ctx context.Context, kind string, identifier string, input LoginInput, metadata map[string]any) {
	if s.audit == nil {
		return
	}
	payload := map[string]any{
		"identifier": identifier,
	}
	for k, v := range metadata {
		payload[k] = v
	}
	s.audit.Record(ctx, security.Event{
		Kind:      kind,
		ActorID:   identifier,
		IP:        input.IP,
		UserAgent: input.UserAgent,
		Metadata:  payload,
	})
}

func (s *authService) touchLogin(ctx context.Context, user *repository.User, input LoginInput) {
	if s == nil || s.users == nil || user == nil {
		return
	}
	now := time.Now().Unix()
	user.LastLoginAt = now
	user.UpdatedAt = now
	if err := s.users.Save(ctx, user); err != nil {
		s.recordAudit(ctx, "auth.login.persist_failed", user.Email, input, map[string]any{"error": err.Error(), "user_id": user.ID})
	}
}
