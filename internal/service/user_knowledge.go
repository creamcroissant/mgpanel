// 文件路径: internal/service/user_knowledge.go
// 模块说明: 这是 internal 模块里的 user_knowledge 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// UserKnowledgeService exposes read-only operations for the knowledge base consumed by end users.
type UserKnowledgeService interface {
	Detail(ctx context.Context, userID string, id int64) (*UserKnowledgeArticle, error)
	List(ctx context.Context, userID string, input UserKnowledgeListInput) (map[string][]UserKnowledgeArticle, error)
	Categories(ctx context.Context, language string) ([]string, error)
}

// UserKnowledgeListInput encapsulates optional filters for listing.
type UserKnowledgeListInput struct {
	Language string
	Keyword  string
}

// UserKnowledgeArticle models the payload returned to users.
type UserKnowledgeArticle struct {
	ID        int64   `json:"id"`
	Category  string  `json:"category"`
	Title     string  `json:"title"`
	Body      *string `json:"body,omitempty"`
	UpdatedAt int64   `json:"updated_at"`
}

type userKnowledgeService struct {
	knowledge repository.KnowledgeRepository
	users     repository.UserRepository
	settings  repository.SettingRepository
	now       func() time.Time
}

// NewUserKnowledgeService constructs a user-facing knowledge service.
func NewUserKnowledgeService(knowledge repository.KnowledgeRepository, users repository.UserRepository, settings repository.SettingRepository) UserKnowledgeService {
	return &userKnowledgeService{knowledge: knowledge, users: users, settings: settings, now: time.Now}
}

func (s *userKnowledgeService) Detail(ctx context.Context, userID string, id int64) (*UserKnowledgeArticle, error) {
	if s == nil || s.knowledge == nil || s.users == nil {
		return nil, fmt.Errorf("user knowledge service not configured / 用户知识库服务未配置")
	}
	if id <= 0 {
		return nil, fmt.Errorf("id must be positive / id 必须为正数")
	}
	user, err := s.loadUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	record, err := s.knowledge.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if !record.Show {
		return nil, ErrNotFound
	}
	article := s.toArticle(ctx, record, user)
	return &article, nil
}

func (s *userKnowledgeService) List(ctx context.Context, userID string, input UserKnowledgeListInput) (map[string][]UserKnowledgeArticle, error) {
	if s == nil || s.knowledge == nil || s.users == nil {
		return nil, fmt.Errorf("user knowledge service not configured / 用户知识库服务未配置")
	}
	language := strings.TrimSpace(input.Language)
	if language == "" {
		return nil, fmt.Errorf("language is required / 语言不能为空")
	}
	if len([]rune(strings.TrimSpace(input.Keyword))) > 255 {
		return nil, fmt.Errorf("keyword exceeds maximum length / 关键词超出长度限制")
	}
	user, err := s.loadUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	filter := repository.KnowledgeVisibleFilter{Language: language, Keyword: strings.TrimSpace(input.Keyword)}
	records, err := s.knowledge.ListVisible(ctx, filter)
	if err != nil {
		return nil, err
	}
	grouped := make(map[string][]UserKnowledgeArticle)
	for _, record := range records {
		if record == nil {
			continue
		}
		article := s.toArticle(ctx, record, user)
		grouped[article.Category] = append(grouped[article.Category], article)
	}
	return grouped, nil
}

func (s *userKnowledgeService) Categories(ctx context.Context, language string) ([]string, error) {
	if s == nil || s.knowledge == nil {
		return nil, fmt.Errorf("user knowledge service not configured / 用户知识库服务未配置")
	}
	var categories []string
	filter := repository.KnowledgeVisibleFilter{Language: strings.TrimSpace(language)}
	records, err := s.knowledge.ListVisible(ctx, filter)
	if err != nil {
		return nil, err
	}
	categorySet := make(map[string]struct{})
	for _, record := range records {
		cat := strings.TrimSpace(record.Category)
		if cat == "" {
			continue
		}
		categorySet[cat] = struct{}{}
	}
	for cat := range categorySet {
		categories = append(categories, cat)
	}
	sort.Strings(categories)
	return categories, nil
}

func (s *userKnowledgeService) loadUser(ctx context.Context, userID string) (*repository.User, error) {
	uid, err := parseUserID(userID)
	if err != nil {
		return nil, err
	}
	user, err := s.users.FindByID(ctx, uid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return user, nil
}

func (s *userKnowledgeService) toArticle(ctx context.Context, record *repository.Knowledge, user *repository.User) UserKnowledgeArticle {
	article := UserKnowledgeArticle{
		ID:        record.ID,
		Category:  strings.TrimSpace(record.Category),
		Title:     record.Title,
		UpdatedAt: record.UpdatedAt,
	}
	if processed := s.processBody(ctx, record.Body, user); processed != "" {
		val := processed
		article.Body = &val
	}
	return article
}

var accessBlockRegex = regexp.MustCompile(`(?s)<!--access start-->(.*?)<!--access end-->`)

func (s *userKnowledgeService) processBody(ctx context.Context, body string, user *repository.User) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return ""
	}
	if !s.isUserAvailable(user) {
		trimmed = accessBlockRegex.ReplaceAllString(trimmed, `<div class="v2board-no-access">You must have a valid subscription to view content in this area</div>`)
	}
	subscribeURL := s.subscribeURL(ctx, user)
	replacements := []string{
		"{{siteName}}", s.settingString(ctx, "app_name", "MGPanel"),
		"{{subscribeUrl}}", subscribeURL,
		"{{urlEncodeSubscribeUrl}}", url.QueryEscape(subscribeURL),
		"{{safeBase64SubscribeUrl}}", safeBase64(subscribeURL),
	}
	replacer := strings.NewReplacer(replacements...)
	return replacer.Replace(trimmed)
}

func (s *userKnowledgeService) isUserAvailable(user *repository.User) bool {
	if user == nil {
		return false
	}
	now := s.now().Unix()
	if user.Banned {
		return false
	}
	if user.TransferEnable <= 0 {
		return false
	}
	if user.ExpiredAt != 0 && user.ExpiredAt <= now {
		return false
	}
	return true
}

func (s *userKnowledgeService) subscribeURL(ctx context.Context, user *repository.User) string {
	token := ""
	if user != nil {
		token = strings.TrimSpace(user.Token)
	}
	path := "/api/v1/client/subscribe"
	if token != "" {
		path += "?token=" + url.QueryEscape(token)
	}
	base := strings.TrimSpace(s.settingString(ctx, "subscribe_url", ""))
	if base == "" {
		base = strings.TrimSpace(s.settingString(ctx, "app_url", ""))
	}
	if base == "" {
		return path
	}
	return strings.TrimRight(base, "/") + path
}

func (s *userKnowledgeService) settingString(ctx context.Context, key, def string) string {
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
	return setting.Value
}

func safeBase64(input string) string {
	if input == "" {
		return ""
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(input))
	encoded = strings.ReplaceAll(encoded, "+", "-")
	encoded = strings.ReplaceAll(encoded, "/", "_")
	return strings.TrimRight(encoded, "=")
}
