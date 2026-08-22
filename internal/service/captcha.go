// 文件路径: internal/service/captcha.go
// 模块说明: 这是 internal 模块里的 captcha 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// CaptchaService validates captcha tokens via Turnstile or reCAPTCHA providers.
type CaptchaService struct {
	settings repository.SettingRepository
	client   *http.Client
}

// NewCaptchaService creates a captcha validator backed by settings.
func NewCaptchaService(settings repository.SettingRepository, client *http.Client) *CaptchaService {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &CaptchaService{settings: settings, client: client}
}

// Verify implements VerificationService's CaptchaValidator interface.
func (s *CaptchaService) Verify(ctx context.Context, input EmailVerificationInput) error {
	if s == nil || !s.boolSetting(ctx, "captcha_enable", false) {
		return nil
	}
	captchaType := s.settingString(ctx, "captcha_type", "recaptcha")
	switch strings.ToLower(strings.TrimSpace(captchaType)) {
	case "turnstile":
		return s.verifyTurnstile(ctx, input)
	case "recaptcha-v3":
		return s.verifyRecaptchaV3(ctx, input)
	case "recaptcha":
		return s.verifyRecaptcha(ctx, input)
	default:
		return fmt.Errorf("captcha: unsupported type %s / 不支持的类型 %s", captchaType, captchaType)
	}
}

func (s *CaptchaService) verifyTurnstile(ctx context.Context, input EmailVerificationInput) error {
	if strings.TrimSpace(input.TurnstileToken) == "" {
		return ErrInvalidCaptcha
	}
	secret := s.settingString(ctx, "turnstile_secret_key", "")
	if secret == "" {
		return errors.New("captcha: turnstile secret not configured / 未配置 turnstile 密钥")
	}
	form := url.Values{}
	form.Set("secret", secret)
	form.Set("response", input.TurnstileToken)
	if ip := strings.TrimSpace(input.IP); ip != "" {
		form.Set("remoteip", ip)
	}
	var resp struct {
		Success bool `json:"success"`
	}
	if err := s.postJSON(ctx, "https://challenges.cloudflare.com/turnstile/v0/siteverify", form, &resp); err != nil {
		return err
	}
	if !resp.Success {
		return ErrInvalidCaptcha
	}
	return nil
}

func (s *CaptchaService) verifyRecaptchaV3(ctx context.Context, input EmailVerificationInput) error {
	if strings.TrimSpace(input.RecaptchaV3Token) == "" {
		return ErrInvalidCaptcha
	}
	secret := s.settingString(ctx, "recaptcha_v3_secret_key", "")
	if secret == "" {
		return errors.New("captcha: recaptcha v3 secret not configured / 未配置 recaptcha v3 密钥")
	}
	form := url.Values{}
	form.Set("secret", secret)
	form.Set("response", input.RecaptchaV3Token)
	if ip := strings.TrimSpace(input.IP); ip != "" {
		form.Set("remoteip", ip)
	}
	var resp struct {
		Success bool     `json:"success"`
		Score   float64  `json:"score"`
		Errors  []string `json:"error-codes"`
	}
	if err := s.postJSON(ctx, "https://www.google.com/recaptcha/api/siteverify", form, &resp); err != nil {
		return err
	}
	if !resp.Success {
		return ErrInvalidCaptcha
	}
	threshold := s.floatSetting(ctx, "recaptcha_v3_score_threshold", 0.5)
	if resp.Score < threshold {
		return ErrInvalidCaptcha
	}
	return nil
}

func (s *CaptchaService) verifyRecaptcha(ctx context.Context, input EmailVerificationInput) error {
	if strings.TrimSpace(input.RecaptchaToken) == "" {
		return ErrInvalidCaptcha
	}
	secret := s.settingString(ctx, "recaptcha_key", "")
	if secret == "" {
		return errors.New("captcha: recaptcha secret not configured / 未配置 recaptcha 密钥")
	}
	form := url.Values{}
	form.Set("secret", secret)
	form.Set("response", input.RecaptchaToken)
	var resp struct {
		Success bool `json:"success"`
	}
	if err := s.postJSON(ctx, "https://www.google.com/recaptcha/api/siteverify", form, &resp); err != nil {
		return err
	}
	if !resp.Success {
		return ErrInvalidCaptcha
	}
	return nil
}

func (s *CaptchaService) postJSON(ctx context.Context, endpoint string, form url.Values, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("captcha: provider returned status %d / 服务商返回状态 %d", resp.StatusCode, resp.StatusCode)
	}
	if dest == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

func (s *CaptchaService) settingString(ctx context.Context, key, def string) string {
	if s == nil || s.settings == nil {
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

func (s *CaptchaService) boolSetting(ctx context.Context, key string, def bool) bool {
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

func (s *CaptchaService) floatSetting(ctx context.Context, key string, def float64) float64 {
	raw := strings.TrimSpace(s.settingString(ctx, key, ""))
	if raw == "" {
		return def
	}
	if v, err := strconv.ParseFloat(raw, 64); err == nil {
		return v
	}
	return def
}
