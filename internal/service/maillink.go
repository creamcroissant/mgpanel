// 文件路径: internal/service/maillink.go
// 模块说明: 这是 internal 模块里的 maillink 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/creamcroissant/xboard/internal/cache"
	"github.com/creamcroissant/xboard/internal/notifier"
	"github.com/creamcroissant/xboard/internal/repository"
	"github.com/google/uuid"
)

// MailLinkInput describes a mail-link login request payload.
type MailLinkInput struct {
	Email    string
	Redirect string
}

// MailLinkService orchestrates login-with-mail-link tokens and quick login URLs.
type MailLinkService interface {
	SendLoginLink(ctx context.Context, input MailLinkInput) (string, error)
	ConsumeToken(ctx context.Context, token string) (int64, error)
	GenerateQuickLoginURL(ctx context.Context, userID int64, redirect string) (string, error)
	BuildLoginRedirect(ctx context.Context, verifyToken, redirect string) (string, error)
}

type mailLinkService struct {
	users    repository.UserRepository
	settings repository.SettingRepository
	notifier notifier.Service
	tokens   cache.Store
	cooldown cache.Store
}

type mailLinkToken struct {
	UserID int64 `json:"user_id"`
}

const (
	mailLinkTokenTTL      = 5 * time.Minute
	quickLoginTokenTTL    = time.Minute
	mailLinkSendCooldown  = time.Minute
	mailLinkTemplateName  = "login"
	mailLinkSubjectFormat = "Login to %s"
)

// NewMailLinkService builds a mail link workflow backed by cache + repositories.
func NewMailLinkService(users repository.UserRepository, settings repository.SettingRepository, notifier notifier.Service, store cache.Store) MailLinkService {
	var tokens, cooldown cache.Store
	if store != nil {
		scoped := store.Namespace("mail_link")
		tokens = scoped.Namespace("token")
		cooldown = scoped.Namespace("cooldown")
	}
	return &mailLinkService{
		users:    users,
		settings: settings,
		notifier: notifier,
		tokens:   tokens,
		cooldown: cooldown,
	}
}

func (s *mailLinkService) SendLoginLink(ctx context.Context, input MailLinkInput) (string, error) {
	if s == nil || s.users == nil {
		return "", fmt.Errorf("mail link service not configured / 邮件链接服务未配置")
	}
	if !s.mailLinkEnabled(ctx) {
		return "", ErrFeatureDisabled
	}
	email := normalizeEmail(input.Email)
	if email == "" {
		return "", ErrInvalidEmail
	}
	if ttl := s.cooldownTTL(ctx, email); ttl > 0 {
		return "", fmt.Errorf("%w: retry after %s / 请在 %s 后重试", ErrCooldownActive, ttl.Round(time.Second), ttl.Round(time.Second))
	}
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	if user.Status != 1 || user.Banned {
		return "", ErrAccountDisabled
	}
	code := uuid.NewString()
	if err := s.storeToken(ctx, code, user.ID, mailLinkTokenTTL); err != nil {
		return "", err
	}
	link := s.composeLink(ctx, code, input.Redirect, false)
	if s.notifier != nil {
		appName := s.settingString(ctx, "app_name", "XBoard")
		if err := s.notifier.SendEmail(ctx, notifier.EmailRequest{
			To:       user.Email,
			Subject:  fmt.Sprintf(mailLinkSubjectFormat, appName),
			Template: mailLinkTemplateName,
			Variables: map[string]any{
				"name": appName,
				"link": link,
				"url":  s.settingString(ctx, "app_url", ""),
			},
		}); err != nil {
			return "", err
		}
	}
	s.bumpCooldown(ctx, email)
	return link, nil
}

func (s *mailLinkService) ConsumeToken(ctx context.Context, token string) (int64, error) {
	if s == nil || s.tokens == nil {
		return 0, ErrInvalidToken
	}
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return 0, ErrInvalidToken
	}
	var record mailLinkToken
	found, err := s.tokens.GetJSON(ctx, trimmed, &record)
	if err != nil {
		return 0, err
	}
	if !found || record.UserID == 0 {
		return 0, ErrInvalidToken
	}
	s.tokens.Delete(ctx, trimmed)
	user, err := s.users.FindByID(ctx, record.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return 0, ErrInvalidToken
		}
		return 0, err
	}
	if user.Status != 1 || user.Banned {
		return 0, ErrAccountDisabled
	}
	return user.ID, nil
}

func (s *mailLinkService) GenerateQuickLoginURL(ctx context.Context, userID int64, redirect string) (string, error) {
	if s == nil || s.tokens == nil {
		return "", fmt.Errorf("mail link service not configured / 邮件链接服务未配置")
	}
	if userID <= 0 {
		return "", ErrUnauthorized
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if user.Status != 1 || user.Banned {
		return "", ErrAccountDisabled
	}
	code := uuid.NewString()
	if err := s.storeToken(ctx, code, user.ID, quickLoginTokenTTL); err != nil {
		return "", err
	}
	return s.composeLink(ctx, code, redirect, true), nil
}

func (s *mailLinkService) BuildLoginRedirect(ctx context.Context, verifyToken, redirect string) (string, error) {
	trimmed := strings.TrimSpace(verifyToken)
	if trimmed == "" {
		return "", ErrInvalidToken
	}
	return s.composeLink(ctx, trimmed, redirect, true), nil
}

func (s *mailLinkService) storeToken(ctx context.Context, code string, userID int64, ttl time.Duration) error {
	if s.tokens == nil {
		return fmt.Errorf("token store unavailable / token 存储不可用")
	}
	record := mailLinkToken{UserID: userID}
	if err := s.tokens.SetJSON(ctx, code, record, ttl); err != nil {
		return fmt.Errorf("store mail-link token: %w", err)
	}
	return nil
}

func (s *mailLinkService) composeLink(ctx context.Context, code string, redirect string, encodeRedirect bool) string {
	target := sanitizeRedirectPath(redirect)
	if target == "" {
		target = "/dashboard"
	}
	if encodeRedirect {
		target = url.QueryEscape(target)
	}
	path := fmt.Sprintf("/#/login?verify=%s&redirect=%s", url.QueryEscape(code), target)
	base := strings.TrimSpace(s.settingString(ctx, "app_url", ""))
	if base == "" {
		return path
	}
	base = strings.TrimRight(base, "/")
	return base + path
}

func (s *mailLinkService) mailLinkEnabled(ctx context.Context) bool {
	return s.boolSetting(ctx, "login_with_mail_link_enable", false)
}

func (s *mailLinkService) cooldownTTL(ctx context.Context, email string) time.Duration {
	if s.cooldown == nil {
		return 0
	}
	ttl, ok := s.cooldown.TTL(ctx, s.cooldownKey(email))
	if !ok {
		return 0
	}
	return ttl
}

func (s *mailLinkService) bumpCooldown(ctx context.Context, email string) {
	if s.cooldown == nil {
		return
	}
	if err := s.cooldown.SetString(ctx, s.cooldownKey(email), time.Now().Format(time.RFC3339), mailLinkSendCooldown); err != nil {
		slog.Warn("failed to set mail link cooldown", "email", email, "error", err)
	}
}

func (s *mailLinkService) cooldownKey(email string) string {
	return "cooldown:" + normalizeEmail(email)
}

func (s *mailLinkService) boolSetting(ctx context.Context, key string, def bool) bool {
	raw := strings.ToLower(strings.TrimSpace(s.settingString(ctx, key, "")))
	if raw == "" {
		return def
	}
	switch raw {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func (s *mailLinkService) settingString(ctx context.Context, key, def string) string {
	if s.settings == nil {
		return def
	}
	setting, err := s.settings.Get(ctx, key)
	if err != nil || setting == nil {
		return def
	}
	trimmed := strings.TrimSpace(setting.Value)
	if trimmed == "" {
		return def
	}
	return trimmed
}
