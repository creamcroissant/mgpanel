// 文件路径: internal/service/comm.go
// 模块说明: 这是 internal 模块里的 comm 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/creamcroissant/xboard/internal/repository"
)

// CommService exposes public configuration payloads shared with guest clients.
type CommService interface {
	GuestConfig(ctx context.Context) (*GuestCommConfig, error)
}

// GuestCommConfig mirrors Laravel guest/comm/config response shape.
type GuestCommConfig struct {
	TOSURL               string  `json:"tos_url"`
	IsEmailVerify        int     `json:"is_email_verify"`
	EmailWhitelistSuffix any     `json:"email_whitelist_suffix"`
	IsCaptcha            int     `json:"is_captcha"`
	CaptchaType          string  `json:"captcha_type"`
	RecaptchaSiteKey     string  `json:"recaptcha_site_key"`
	RecaptchaV3SiteKey   string  `json:"recaptcha_v3_site_key"`
	RecaptchaV3Score     float64 `json:"recaptcha_v3_score_threshold"`
	TurnstileSiteKey     string  `json:"turnstile_site_key"`
	AppDescription       string  `json:"app_description"`
	AppURL               string  `json:"app_url"`
	Logo                 string  `json:"logo"`
	IsRecaptcha          int     `json:"is_recaptcha"`
	TelegramLoginEnable  bool    `json:"telegram_login_enable"`
	TelegramBotUsername  string  `json:"telegram_bot_username"`
	TelegramLoginDomain  string  `json:"telegram_login_domain"`
}

type commService struct {
	settings repository.SettingRepository
	plugins  repository.PluginRepository
}

// NewCommService returns a CommService backed by settings repository.
func NewCommService(settings repository.SettingRepository, plugins repository.PluginRepository) CommService {
	return &commService{
		settings: settings,
		plugins:  plugins,
	}
}

func (s *commService) GuestConfig(ctx context.Context) (*GuestCommConfig, error) {
	cfg := &GuestCommConfig{
		TOSURL:               s.settingString(ctx, "tos_url", ""),
		IsEmailVerify:        boolToInt(s.boolSetting(ctx, "email_verify", false)),
		EmailWhitelistSuffix: s.emailWhitelistSuffix(ctx),
		IsCaptcha:            boolToInt(s.boolSetting(ctx, "captcha_enable", false)),
		CaptchaType:          s.settingString(ctx, "captcha_type", "recaptcha"),
		RecaptchaSiteKey:     s.settingString(ctx, "recaptcha_site_key", ""),
		RecaptchaV3SiteKey:   s.settingString(ctx, "recaptcha_v3_site_key", ""),
		RecaptchaV3Score:     s.floatSetting(ctx, "recaptcha_v3_score_threshold", 0.5),
		TurnstileSiteKey:     s.settingString(ctx, "turnstile_site_key", ""),
		AppDescription:       s.settingString(ctx, "app_description", ""),
		AppURL:               s.settingString(ctx, "app_url", ""),
		Logo:                 s.settingString(ctx, "logo", ""),
	}
	cfg.IsRecaptcha = cfg.IsCaptcha
	s.applyTelegramLogin(ctx, cfg)
	return cfg, nil
}

func (s *commService) applyTelegramLogin(ctx context.Context, cfg *GuestCommConfig) {
	if s.plugins == nil {
		return
	}
	plugin, err := s.plugins.FindEnabledByCode(ctx, "telegram_login")
	if err != nil || plugin == nil {
		return
	}
	cfg.TelegramLoginEnable = true
	if plugin.Config == "" {
		return
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(plugin.Config), &data); err != nil {
		return
	}
	if v, ok := data["bot_username"].(string); ok {
		cfg.TelegramBotUsername = strings.TrimSpace(v)
	}
	if v, ok := data["domain"].(string); ok {
		cfg.TelegramLoginDomain = strings.TrimSpace(v)
	}
}

func (s *commService) settingString(ctx context.Context, key, def string) string {
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

func (s *commService) boolSetting(ctx context.Context, key string, def bool) bool {
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

func (s *commService) floatSetting(ctx context.Context, key string, def float64) float64 {
	raw := strings.TrimSpace(s.settingString(ctx, key, ""))
	if raw == "" {
		return def
	}
	if v, err := strconv.ParseFloat(raw, 64); err == nil {
		return v
	}
	return def
}

func (s *commService) emailWhitelistSuffix(ctx context.Context) any {
	if !s.boolSetting(ctx, "email_whitelist_enable", false) {
		return 0
	}
	raw := strings.TrimSpace(s.settingString(ctx, "email_whitelist_suffix", ""))
	if raw == "" {
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

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
