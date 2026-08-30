// 文件路径: internal/service/verification.go
// 模块说明: 这是 internal 模块里的 verification 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"math/big"
	"net/mail"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/cache"
	"github.com/creamcroissant/mgpanel/internal/notifier"
	"github.com/creamcroissant/mgpanel/internal/repository"
)

// EmailVerificationInput contains parameters for sending verification codes.
type EmailVerificationInput struct {
	Email            string
	IP               string
	UserAgent        string
	TurnstileToken   string
	RecaptchaToken   string
	RecaptchaV3Token string
}

// VerificationService handles OTP / verification code workflows.
type VerificationService interface {
	SendEmailCode(ctx context.Context, input EmailVerificationInput) error
	ValidateEmailCode(ctx context.Context, email, code string, consume bool) error
	ClearEmailCode(ctx context.Context, email string)
}

// CaptchaValidator verifies anti-robot challenges before sending codes.
type CaptchaValidator interface {
	Verify(ctx context.Context, input EmailVerificationInput) error
}

type verificationService struct {
	codes    cache.Store
	cooldown cache.Store
	notifier notifier.Service
	settings repository.SettingRepository
	users    repository.UserRepository
	captcha  CaptchaValidator
}

const (
	emailCodeTTL    = 5 * time.Minute
	emailCooldown   = time.Minute
	emailTemplate   = "verify"
	emailSubjectFmt = "%s Email verification code"
)

var defaultEmailSuffixes = []string{
	"gmail.com",
	"qq.com",
	"163.com",
	"yahoo.com",
	"sina.com",
	"126.com",
	"outlook.com",
	"yeah.net",
	"foxmail.com",
}

// NewVerificationService constructs a verification service backed by cache + notifier.
func NewVerificationService(store cache.Store, notifier notifier.Service, settings repository.SettingRepository, users repository.UserRepository, captcha CaptchaValidator) VerificationService {
	var scopedCodes, scopedCooldown cache.Store
	if store != nil {
		namespaced := store.Namespace("verification")
		scopedCodes = namespaced.Namespace("code")
		scopedCooldown = namespaced.Namespace("cooldown")
	}
	return &verificationService{
		codes:    scopedCodes,
		cooldown: scopedCooldown,
		notifier: notifier,
		settings: settings,
		users:    users,
		captcha:  captcha,
	}
}

func (s *verificationService) SendEmailCode(ctx context.Context, input EmailVerificationInput) error {
	email := normalizeEmail(input.Email)
	if email == "" {
		return ErrInvalidEmail
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return ErrInvalidEmail
	}

	if s.captcha != nil {
		if err := s.captcha.Verify(ctx, input); err != nil {
			return err
		}
	}

	if err := s.ensureWhitelist(ctx, email); err != nil {
		return err
	}

	if s.cooldown != nil {
		key := cooldownCacheKey(email)
		ttl := emailCooldown
		if remain, ok := s.cooldown.TTL(ctx, key); ok && remain > 0 {
			ttl = remain
		}
		current, err := s.cooldown.Increment(ctx, key, 1, ttl)
		if err != nil {
			return fmt.Errorf("store cooldown: %v / 保存冷却时间失败: %w", err, err)
		}
		if current > 1 {
			return fmt.Errorf("%w: retry in %s / 请在 %s 后重试", ErrCooldownActive, ttl.Round(time.Second), ttl.Round(time.Second))
		}
	}

	code, err := generateEmailCode()
	if err != nil {
		return err
	}

	appName := s.settingString(ctx, "app_name", "MGPanel")
	appURL := s.settingString(ctx, "app_url", "")
	subject := fmt.Sprintf(emailSubjectFmt, appName)

	if s.notifier != nil {
		if err := s.notifier.SendEmail(ctx, notifier.EmailRequest{
			To:       email,
			Subject:  subject,
			Template: emailTemplate,
			Variables: map[string]any{
				"name": appName,
				"code": code,
				"url":  appURL,
			},
		}); err != nil {
			return fmt.Errorf("send email verify code: %v / 发送邮件验证码失败: %w", err, err)
		}
	}

	if s.codes != nil {
		if err := s.codes.SetString(ctx, codeCacheKey(email), code, emailCodeTTL); err != nil {
			return fmt.Errorf("store verification code: %v / 保存验证码失败: %w", err, err)
		}
	}

	return nil
}

func (s *verificationService) ValidateEmailCode(ctx context.Context, email, code string, consume bool) error {
	normalizedEmail := normalizeEmail(email)
	if normalizedEmail == "" || strings.TrimSpace(code) == "" {
		return ErrInvalidVerificationCode
	}
	if s.codes == nil {
		return ErrInvalidVerificationCode
	}
	stored, ok := s.codes.GetString(ctx, codeCacheKey(normalizedEmail))
	if !ok || stored == "" {
		return ErrInvalidVerificationCode
	}
	if subtle.ConstantTimeCompare([]byte(stored), []byte(strings.TrimSpace(code))) != 1 {
		return ErrInvalidVerificationCode
	}
	if consume {
		s.codes.Delete(ctx, codeCacheKey(normalizedEmail))
	}
	return nil
}

func (s *verificationService) ClearEmailCode(ctx context.Context, email string) {
	if s == nil || s.codes == nil {
		return
	}
	s.codes.Delete(ctx, codeCacheKey(normalizeEmail(email)))
}

func (s *verificationService) ensureWhitelist(ctx context.Context, email string) error {
	if !s.whitelistEnabled(ctx) {
		return nil
	}
	if s.isRegisteredEmail(ctx, email) {
		return nil
	}
	suffix := emailDomain(email)
	if suffix == "" {
		return ErrEmailDomainNotAllowed
	}
	allowed := s.allowedSuffixes(ctx)
	for _, candidate := range allowed {
		if strings.EqualFold(candidate, suffix) {
			return nil
		}
	}
	return ErrEmailDomainNotAllowed
}

func (s *verificationService) whitelistEnabled(ctx context.Context) bool {
	raw := s.settingString(ctx, "email_whitelist_enable", "0")
	raw = strings.TrimSpace(strings.ToLower(raw))
	return raw == "1" || raw == "true" || raw == "yes" || raw == "on"
}

func (s *verificationService) allowedSuffixes(ctx context.Context) []string {
	raw := s.settingString(ctx, "email_whitelist_suffix", "")
	if strings.TrimSpace(raw) == "" {
		return defaultEmailSuffixes
	}
	var arr []string
	if err := json.Unmarshal([]byte(raw), &arr); err == nil && len(arr) > 0 {
		return arr
	}
	parts := strings.Split(raw, ",")
	var cleaned []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	if len(cleaned) == 0 {
		return defaultEmailSuffixes
	}
	return cleaned
}

func (s *verificationService) settingString(ctx context.Context, key, def string) string {
	if s.settings == nil {
		return def
	}
	setting, err := s.settings.Get(ctx, key)
	if err != nil || setting == nil {
		return def
	}
	val := strings.TrimSpace(setting.Value)
	if val == "" {
		return def
	}
	return val
}

func (s *verificationService) isRegisteredEmail(ctx context.Context, email string) bool {
	if s.users == nil {
		return false
	}
	_, err := s.users.FindByEmail(ctx, email)
	return err == nil
}

func (s *verificationService) cooldownTTL(ctx context.Context, email string) time.Duration {
	if s.cooldown == nil {
		return 0
	}
	ttl, ok := s.cooldown.TTL(ctx, cooldownCacheKey(email))
	if !ok {
		return 0
	}
	return ttl
}

func generateEmailCode() (string, error) {
	max := big.NewInt(900000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()+100000), nil
}

func codeCacheKey(email string) string {
	return "email:" + email
}

func cooldownCacheKey(email string) string {
	return "cooldown:" + email
}

func emailDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(parts[1]))
}

func normalizeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}

func normalizeUsername(username string) string {
	return strings.TrimSpace(strings.ToLower(username))
}
