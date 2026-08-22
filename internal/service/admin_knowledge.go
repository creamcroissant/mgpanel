// 文件路径: internal/service/admin_knowledge.go
// 模块说明: 这是 internal 模块里的 admin_knowledge 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

// AdminKnowledgeService exposes CRUD operations for knowledge base entries.
type AdminKnowledgeService interface {
	List(ctx context.Context) ([]AdminKnowledgeSummary, error)
	FindByID(ctx context.Context, id int64) (*AdminKnowledgeDetail, error)
	Categories(ctx context.Context) ([]string, error)
	Save(ctx context.Context, input AdminKnowledgeSaveInput) error
	Toggle(ctx context.Context, id int64) error
	Delete(ctx context.Context, id int64) error
	Sort(ctx context.Context, ids []int64) error
	I18n() *i18n.Manager
}

// AdminKnowledgeSummary represents the lightweight payload returned by fetch list.
type AdminKnowledgeSummary struct {
	ID        int64  `json:"id"`
	Language  string `json:"language"`
	Category  string `json:"category"`
	Title     string `json:"title"`
	Show      bool   `json:"show"`
	UpdatedAt int64  `json:"updated_at"`
}

// AdminKnowledgeDetail exposes the full article fields.
type AdminKnowledgeDetail struct {
	ID        int64  `json:"id"`
	Language  string `json:"language"`
	Category  string `json:"category"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Sort      int64  `json:"sort"`
	Show      bool   `json:"show"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// AdminKnowledgeSaveInput captures payload accepted by /knowledge/save.
type AdminKnowledgeSaveInput struct {
	ID       *int64 `json:"id"`
	Language string `json:"language"`
	Category string `json:"category"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	Sort     *int64 `json:"sort"`
	Show     *bool  `json:"show"`
}

type adminKnowledgeService struct {
	knowledge repository.KnowledgeRepository
	now       func() time.Time
	i18n      *i18n.Manager
}

// NewAdminKnowledgeService wires repository-backed CRUD operations.
func NewAdminKnowledgeService(repo repository.KnowledgeRepository, i18n *i18n.Manager) AdminKnowledgeService {
	return &adminKnowledgeService{knowledge: repo, now: time.Now, i18n: i18n}
}

func (s *adminKnowledgeService) I18n() *i18n.Manager {
	return s.i18n
}

func (s *adminKnowledgeService) List(ctx context.Context) ([]AdminKnowledgeSummary, error) {
	if s == nil || s.knowledge == nil {
		return nil, fmt.Errorf("admin knowledge service not configured / 管理知识库服务未配置")
	}
	records, err := s.knowledge.List(ctx)
	if err != nil {
		return nil, err
	}
	summaries := make([]AdminKnowledgeSummary, 0, len(records))
	for _, record := range records {
		summaries = append(summaries, AdminKnowledgeSummary{
			ID:        record.ID,
			Language:  record.Language,
			Category:  record.Category,
			Title:     record.Title,
			Show:      record.Show,
			UpdatedAt: record.UpdatedAt,
		})
	}
	return summaries, nil
}

func (s *adminKnowledgeService) FindByID(ctx context.Context, id int64) (*AdminKnowledgeDetail, error) {
	if s == nil || s.knowledge == nil {
		return nil, fmt.Errorf("admin knowledge service not configured / 管理知识库服务未配置")
	}
	if id <= 0 {
		return nil, fmt.Errorf("id must be positive / id 必须为正数")
	}
	record, err := s.knowledge.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &AdminKnowledgeDetail{
		ID:        record.ID,
		Language:  record.Language,
		Category:  record.Category,
		Title:     record.Title,
		Body:      record.Body,
		Sort:      record.Sort,
		Show:      record.Show,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}, nil
}

func (s *adminKnowledgeService) Categories(ctx context.Context) ([]string, error) {
	if s == nil || s.knowledge == nil {
		return nil, fmt.Errorf("admin knowledge service not configured / 管理知识库服务未配置")
	}
	return s.knowledge.Categories(ctx)
}

func (s *adminKnowledgeService) Save(ctx context.Context, input AdminKnowledgeSaveInput) error {
	if s == nil || s.knowledge == nil {
		return fmt.Errorf("admin knowledge service not configured / 管理知识库服务未配置")
	}
	language := strings.TrimSpace(input.Language)
	category := strings.TrimSpace(input.Category)
	title := strings.TrimSpace(input.Title)
	body := sanitizeHTML(input.Body)
	if language == "" {
		return fmt.Errorf("language is required / 语言不能为空")
	}
	if category == "" {
		return fmt.Errorf("category is required / 分类不能为空")
	}
	if title == "" {
		return fmt.Errorf("title is required / 标题不能为空")
	}
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("content is required / 内容不能为空")
	}
	sortValue := cleanSortValue(input.Sort)
	show := boolValue(input.Show)
	now := s.now().Unix()
	if input.ID == nil || *input.ID <= 0 {
		knowledge := &repository.Knowledge{
			Language:  language,
			Category:  category,
			Title:     title,
			Body:      body,
			Sort:      sortValue,
			Show:      show,
			CreatedAt: now,
			UpdatedAt: now,
		}
		_, err := s.knowledge.Create(ctx, knowledge)
		return err
	}
	record, err := s.knowledge.FindByID(ctx, *input.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	record.Language = language
	record.Category = category
	record.Title = title
	record.Body = body
	record.Sort = sortValue
	record.Show = show
	record.UpdatedAt = now
	return s.knowledge.Update(ctx, record)
}

func (s *adminKnowledgeService) Toggle(ctx context.Context, id int64) error {
	if s == nil || s.knowledge == nil {
		return fmt.Errorf("admin knowledge service not configured / 管理知识库服务未配置")
	}
	if id <= 0 {
		return fmt.Errorf("id must be positive / id 必须为正数")
	}
	record, err := s.knowledge.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	record.Show = !record.Show
	record.UpdatedAt = s.now().Unix()
	return s.knowledge.Update(ctx, record)
}

func (s *adminKnowledgeService) Delete(ctx context.Context, id int64) error {
	if s == nil || s.knowledge == nil {
		return fmt.Errorf("admin knowledge service not configured / 管理知识库服务未配置")
	}
	if id <= 0 {
		return fmt.Errorf("id must be positive / id 必须为正数")
	}
	if _, err := s.knowledge.FindByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return s.knowledge.Delete(ctx, id)
}

func (s *adminKnowledgeService) Sort(ctx context.Context, ids []int64) error {
	if s == nil || s.knowledge == nil {
		return fmt.Errorf("admin knowledge service not configured / 管理知识库服务未配置")
	}
	cleaned := uniquePositiveIDs(ids)
	if len(cleaned) == 0 {
		return fmt.Errorf("ids are required / ids 不能为空")
	}
	return s.knowledge.Sort(ctx, cleaned, s.now().Unix())
}

func cleanSortValue(sortPtr *int64) int64 {
	if sortPtr == nil || *sortPtr <= 0 {
		return 0
	}
	return *sortPtr
}
