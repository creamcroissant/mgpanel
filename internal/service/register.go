// 文件路径: internal/service/register.go
// 模块说明: 这是 internal 模块里的 register 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/cache"
	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/support/hash"
)

// RegistrationInput 表示用户注册所需的请求数据。
type RegistrationInput struct {
	Email      string
	Username   string
	Password   string
		EmailCode  string
	IP         string
}

// RegistrationService 负责用户注册的校验与持久化。
type RegistrationService interface {
	Register(ctx context.Context, input RegistrationInput) (*repository.User, error)
}

type registrationService struct {
	users    repository.UserRepository
		settings repository.SettingRepository
	hasher   hash.Hasher
	verify   VerificationService
	limits   cache.Store
}

const (
	registerIPLimitDefaultCount  = 3
	registerIPLimitDefaultExpire = 60 * time.Minute
)

// NewRegistrationService 组装仓储驱动的注册流程。
func NewRegistrationService(users repository.UserRepository, settings repository.SettingRepository, hasher hash.Hasher, verify VerificationService, store cache.Store) RegistrationService {
	var limits cache.Store
	if store != nil {
		limits = store.Namespace("auth:register")
	}
	return &registrationService{
		users:    users,
		settings: settings,
		hasher:   hasher,
		verify:   verify,
		limits:   limits,
	}
}

func (s *registrationService) Register(ctx context.Context, input RegistrationInput) (*repository.User, error) {
	if s == nil || s.users == nil || s.hasher == nil {
		return nil, fmt.Errorf("registration service not fully configured / 注册服务未完整配置")
	}

	email := normalizeEmail(input.Email)
	username := normalizeUsername(input.Username)
	if email == "" && username == "" {
		return nil, ErrIdentifierRequired
	}
	if username != "" && !isValidUsername(username) {
		return nil, ErrInvalidUsername
	}
	password := strings.TrimSpace(input.Password)
	if len(password) < 8 || !hasLetterAndNumber(password) {
		return nil, ErrInvalidPassword
	}

	if s.registrationClosed(ctx) {
		return nil, ErrRegistrationClosed
	}
	if email != "" {
		if err := s.ensureWhitelist(ctx, email); err != nil {
			return nil, err
		}
		if err := s.enforceGmailPolicy(ctx, email); err != nil {
			return nil, err
		}
	} else if s.emailVerifyRequired(ctx) {
		return nil, ErrInvalidEmail
	}

	if err := s.ensureIPLimit(ctx, input.IP); err != nil {
		return nil, err
	}


	if s.emailVerifyRequired(ctx) {
		if email == "" {
			return nil, ErrInvalidEmail
		}
		if strings.TrimSpace(input.EmailCode) == "" {
			return nil, ErrInvalidVerificationCode
		}
		if s.verify == nil {
			return nil, fmt.Errorf("verification service unavailable / 验证服务不可用")
		}
		if err := s.verify.ValidateEmailCode(ctx, email, input.EmailCode, true); err != nil {
			if errors.Is(err, ErrInvalidVerificationCode) {
				s.bumpIPLimit(ctx, input.IP)
				return nil, ErrInvalidVerificationCode
			}
			return nil, err
		}
	}

	if email != "" {
		if _, err := s.users.FindByEmail(ctx, email); err == nil {
			return nil, ErrEmailExists
		} else if !errors.Is(err, repository.ErrNotFound) {
			return nil, err
		}
	}
	if username != "" {
		if _, err := s.users.FindByUsername(ctx, username); err == nil {
			return nil, ErrUsernameExists
		} else if !errors.Is(err, repository.ErrNotFound) {
			return nil, err
		}
	}

	hashed, err := s.hasher.Hash(password)
	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	user := &repository.User{
		UUID:              newUserUUID(),
		Token:             newUserToken(),
		Username:          username,
		Email:             email,
		Password:          hashed,
		PasswordAlgo:      "",
		PasswordSalt:      "",
		PlanID:            0,
		ExpiredAt:         0,
		U:                 0,
		D:                 0,
		TransferEnable:    0,
				Status:            1,
		Banned:            false,
		LastLoginAt:       0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	
	created, err := s.users.Create(ctx, user)
	if err != nil {
		// Concurrent registration may bypass pre-checks and hit DB constraint.
		errMsg := strings.ToLower(err.Error())
		if strings.Contains(errMsg, "email") && strings.Contains(errMsg, "unique") {
			return nil, ErrEmailExists
		}
		if strings.Contains(errMsg, "username") && strings.Contains(errMsg, "unique") {
			return nil, ErrUsernameExists
		}
		return nil, err
	}

	s.bumpIPLimit(ctx, input.IP)
	if s.verify != nil && email != "" {
		s.verify.ClearEmailCode(ctx, email)
	}

	return created, nil
}

func (s *registrationService) registrationClosed(ctx context.Context) bool {
	return s.boolSetting(ctx, "stop_register", false)
}

func (s *registrationService) emailVerifyRequired(ctx context.Context) bool {
	return s.boolSetting(ctx, "email_verify", false)
}



func (s *registrationService) whitelistEnabled(ctx context.Context) bool {
	return s.boolSetting(ctx, "email_whitelist_enable", false)
}

func (s *registrationService) gmailLimitEnabled(ctx context.Context) bool {
	return s.boolSetting(ctx, "email_gmail_limit_enable", false)
}

func (s *registrationService) ensureWhitelist(ctx context.Context, email string) error {
	if !s.whitelistEnabled(ctx) {
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

func (s *registrationService) enforceGmailPolicy(ctx context.Context, email string) error {
	if !s.gmailLimitEnabled(ctx) {
		return nil
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ErrInvalidEmail
	}
	local := parts[0]
	if strings.Contains(local, ".") || strings.Contains(local, "+") {
		return ErrInvalidEmail
	}
	return nil
}

func (s *registrationService) ensureIPLimit(ctx context.Context, ip string) error {
	if s.limits == nil || strings.TrimSpace(ip) == "" {
		return nil
	}
	if !s.boolSetting(ctx, "register_limit_by_ip_enable", false) {
		return nil
	}
	max := s.intSetting(ctx, "register_limit_count", registerIPLimitDefaultCount)
	if max <= 0 {
		return nil
	}
	if count := s.ipCount(ctx, ip); count >= max {
		return ErrRateLimited
	}
	return nil
}

func (s *registrationService) bumpIPLimit(ctx context.Context, ip string) {
	if s.limits == nil || strings.TrimSpace(ip) == "" {
		return
	}
	if !s.boolSetting(ctx, "register_limit_by_ip_enable", false) {
		return
	}
	ttlMinutes := s.intSetting(ctx, "register_limit_expire", 60)
	ttl := time.Duration(ttlMinutes) * time.Minute
	if ttl <= 0 {
		ttl = registerIPLimitDefaultExpire
	}
	key := s.ipKey(ip)
	if _, err := s.limits.Increment(ctx, key, 1, ttl); err != nil {
		return
	}
}

func (s *registrationService) ipCount(ctx context.Context, ip string) int {
	if s.limits == nil {
		return 0
	}
	raw, ok := s.limits.Get(ctx, s.ipKey(ip))
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

func (s *registrationService) ipKey(ip string) string {
	return "REGISTER_IP_RATE_LIMIT_" + strings.TrimSpace(ip)
}

func (s *registrationService) allowedSuffixes(ctx context.Context) []string {
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

func (s *registrationService) boolSetting(ctx context.Context, key string, def bool) bool {
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

func (s *registrationService) intSetting(ctx context.Context, key string, def int) int {
	raw := strings.TrimSpace(s.settingString(ctx, key, ""))
	if raw == "" {
		return def
	}
	if n, err := strconv.Atoi(raw); err == nil {
		return n
	}
	return def
}

func (s *registrationService) settingString(ctx context.Context, key, def string) string {
	if s.settings == nil {
		return def
	}
	item, err := s.settings.Get(ctx, key)
	if err != nil || item == nil {
		return def
	}
	if strings.TrimSpace(item.Value) == "" {
		return def
	}
	return item.Value
}
