package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// ShortLinkService manages short URL generation for subscription links.
type ShortLinkService interface {
	// Create generates a new short link for a user
	Create(ctx context.Context, userID int64, customCode string, expiresAt int64) (*repository.ShortLink, error)

	// Resolve finds a short link by code and returns the full redirect URL
	Resolve(ctx context.Context, code string) (*ShortLinkResolveResult, error)

	// List returns all short links for a user
	List(ctx context.Context, userID int64) ([]*repository.ShortLink, error)

	// Delete removes a short link by ID (must belong to user)
	Delete(ctx context.Context, userID int64, linkID int64) error

	// GetByID returns a short link by ID
	GetByID(ctx context.Context, id int64) (*repository.ShortLink, error)
}

// ShortLinkResolveResult contains the resolution result for a short link.
type ShortLinkResolveResult struct {
	Link       *repository.ShortLink
	RedirectTo string
	UserToken  string
	Expired    bool
}

type shortLinkService struct {
	links    repository.ShortLinkRepository
	users    repository.UserRepository
	settings repository.SettingRepository
}

// NewShortLinkService creates a new short link service.
func NewShortLinkService(links repository.ShortLinkRepository, users repository.UserRepository, settings repository.SettingRepository) ShortLinkService {
	return &shortLinkService{
		links:    links,
		users:    users,
		settings: settings,
	}
}

func (s *shortLinkService) Create(ctx context.Context, userID int64, customCode string, expiresAt int64) (*repository.ShortLink, error) {
	if s.links == nil {
		return nil, errors.New("short link repository unavailable / 短链接仓库不可用")
	}

	// Verify user exists
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}

	// Generate or use custom code
	code := strings.TrimSpace(customCode)
	if code == "" {
		code, err = s.generateUniqueCode(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		// Validate custom code
		if len(code) < 4 || len(code) > 32 {
			return nil, errors.New("code must be 4-32 characters / code 长度需为 4-32")
		}
		// Check if code already exists
		exists, err := s.links.CodeExists(ctx, code)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, errors.New("code already in use / code 已被占用")
		}
	}

	now := time.Now().Unix()
	link := &repository.ShortLink{
		Code:       code,
		UserID:     userID,
		TargetPath: "/api/v1/client/subscribe",
		ExpiresAt:  expiresAt,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := s.links.Create(ctx, link); err != nil {
		return nil, err
	}

	return link, nil
}

func (s *shortLinkService) Resolve(ctx context.Context, code string) (*ShortLinkResolveResult, error) {
	if s.links == nil {
		return nil, errors.New("short link repository unavailable / 短链接仓库不可用")
	}

	link, err := s.links.FindByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	result := &ShortLinkResolveResult{
		Link: link,
	}

	// Check expiration
	now := time.Now().Unix()
	if link.ExpiresAt > 0 && link.ExpiresAt < now {
		result.Expired = true
		return result, nil
	}

	// Get user token
	user, err := s.users.FindByID(ctx, link.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}

	result.UserToken = user.Token

	// Build redirect URL
	baseURL := s.getBaseURL(ctx)
	redirectPath := sanitizeRedirectPath(link.TargetPath)
	if redirectPath == "" {
		redirectPath = "/api/v1/client/subscribe"
	}
	redirectTo, err := buildRedirectURL(baseURL, redirectPath, user.Token)
	if err != nil {
		return nil, err
	}
	result.RedirectTo = redirectTo

	// Increment access count
	if err := s.links.IncrementAccessCount(ctx, link.ID, now); err != nil {
		slog.Warn("failed to increment short link access count", "link_id", link.ID, "error", err)
	}

	return result, nil
}

func (s *shortLinkService) List(ctx context.Context, userID int64) ([]*repository.ShortLink, error) {
	if s.links == nil {
		return nil, errors.New("short link repository unavailable / 短链接仓库不可用")
	}
	return s.links.FindByUserID(ctx, userID)
}

func (s *shortLinkService) Delete(ctx context.Context, userID int64, linkID int64) error {
	if s.links == nil {
		return errors.New("short link repository unavailable / 短链接仓库不可用")
	}

	// Verify ownership
	link, err := s.links.FindByID(ctx, linkID)
	if err != nil {
		return err
	}
	if link.UserID != userID {
		return ErrNotFound // Don't reveal that link exists but belongs to another user
	}

	return s.links.Delete(ctx, linkID)
}

func (s *shortLinkService) GetByID(ctx context.Context, id int64) (*repository.ShortLink, error) {
	if s.links == nil {
		return nil, errors.New("short link repository unavailable / 短链接仓库不可用")
	}
	return s.links.FindByID(ctx, id)
}

func (s *shortLinkService) generateUniqueCode(ctx context.Context) (string, error) {
	const maxAttempts = 10
	for i := 0; i < maxAttempts; i++ {
		code, err := generateRandomCode(8)
		if err != nil {
			return "", err
		}
		exists, err := s.links.CodeExists(ctx, code)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", errors.New("failed to generate unique code after multiple attempts / 多次尝试后仍无法生成唯一 code")
}

func (s *shortLinkService) getBaseURL(ctx context.Context) string {
	if s.settings == nil {
		return ""
	}
	if setting, err := s.settings.Get(ctx, "subscribe_url"); err == nil && setting != nil && setting.Value != "" {
		return strings.TrimRight(setting.Value, "/")
	}
	if setting, err := s.settings.Get(ctx, "app_url"); err == nil && setting != nil && setting.Value != "" {
		return strings.TrimRight(setting.Value, "/")
	}
	return ""
}

// generateRandomCode generates a URL-safe random code of specified length.
func generateRandomCode(length int) (string, error) {
	// Generate random bytes (we need more than length because base64 encoding expands)
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Encode to base64 and make URL-safe
	code := base64.RawURLEncoding.EncodeToString(bytes)

	// Truncate to desired length
	if len(code) > length {
		code = code[:length]
	}

	return code, nil
}

func buildRedirectURL(baseURL, path, token string) (string, error) {
	sanitized := sanitizeRedirectPath(path)
	if sanitized == "" {
		return "", errors.New("invalid redirect path / 无效的跳转路径")
	}
	if strings.HasPrefix(sanitized, "#/") {
		if baseURL == "" {
			return sanitized, nil
		}
		return strings.TrimRight(baseURL, "/") + sanitized, nil
	}
	target, err := url.Parse(sanitized)
	if err != nil {
		return "", err
	}
	if token != "" {
		query := target.Query()
		query.Set("token", token)
		target.RawQuery = query.Encode()
	}
	if baseURL == "" {
		return target.String(), nil
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	resolved := base.ResolveReference(target)
	return resolved.String(), nil
}
