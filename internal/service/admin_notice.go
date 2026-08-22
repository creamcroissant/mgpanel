// 文件路径: internal/service/admin_notice.go
// 模块说明: 这是 internal 模块里的 admin_notice 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
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

// AdminNoticeService exposes CRUD operations for announcements.
type AdminNoticeService interface {
	List(ctx context.Context) ([]AdminNoticeView, error)
	GetByID(ctx context.Context, id int64) (*AdminNoticeView, error)
	Save(ctx context.Context, input AdminNoticeSaveInput) error
	Toggle(ctx context.Context, id int64) error
	Delete(ctx context.Context, id int64) error
	Sort(ctx context.Context, ids []int64) error
	I18n() *i18n.Manager
}

// AdminNoticeView mirrors the payload returned to admin clients.
type AdminNoticeView struct {
	ID        int64    `json:"id"`
	Sort      int64    `json:"sort"`
	Title     string   `json:"title"`
	Content   string   `json:"content"`
	ImgURL    string   `json:"img_url"`
	Tags      []string `json:"tags"`
	Show      bool     `json:"show"`
	Popup     bool     `json:"popup"`
	CreatedAt int64    `json:"created_at"`
	UpdatedAt int64    `json:"updated_at"`
}

// AdminNoticeSaveInput captures fields accepted by the save endpoint.
type AdminNoticeSaveInput struct {
	ID      *int64   `json:"id"`
	Title   string   `json:"title"`
	Content string   `json:"content"`
	ImgURL  string   `json:"img_url"`
	Tags    []string `json:"tags"`
	Show    *bool    `json:"show"`
	Popup   *bool    `json:"popup"`
}

type adminNoticeService struct {
	notices repository.NoticeRepository
	now     func() time.Time
	i18n    *i18n.Manager
}

// NewAdminNoticeService wires repository-backed notice operations.
func NewAdminNoticeService(notices repository.NoticeRepository, i18n *i18n.Manager) AdminNoticeService {
	return &adminNoticeService{notices: notices, now: time.Now, i18n: i18n}
}

func (s *adminNoticeService) I18n() *i18n.Manager {
	return s.i18n
}

func (s *adminNoticeService) List(ctx context.Context) ([]AdminNoticeView, error) {
	if s == nil || s.notices == nil {
		return nil, fmt.Errorf("admin notice service not configured / 管理公告服务未配置")
	}
	records, err := s.notices.List(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]AdminNoticeView, 0, len(records))
	for _, record := range records {
		views = append(views, mapNotice(record))
	}
	return views, nil
}

func (s *adminNoticeService) GetByID(ctx context.Context, id int64) (*AdminNoticeView, error) {
	if s == nil || s.notices == nil {
		return nil, fmt.Errorf("admin notice service not configured / 管理公告服务未配置")
	}
	if id <= 0 {
		return nil, ErrNotFound
	}
	record, err := s.notices.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	view := mapNotice(record)
	return &view, nil
}

func (s *adminNoticeService) Save(ctx context.Context, input AdminNoticeSaveInput) error {
	if s == nil || s.notices == nil {
		return fmt.Errorf("admin notice service not configured / 管理公告服务未配置")
	}
	title := strings.TrimSpace(input.Title)
	content := sanitizeHTML(strings.TrimSpace(input.Content))
	if title == "" {
		return fmt.Errorf("title is required / 标题不能为空")
	}
	if content == "" {
		return fmt.Errorf("content is required / 内容不能为空")
	}
	img := strings.TrimSpace(input.ImgURL)
	tags := sanitizeTags(input.Tags)
	show := boolValue(input.Show)
	popup := boolValue(input.Popup)
	now := s.now().Unix()
	if input.ID == nil || *input.ID <= 0 {
		notice := &repository.Notice{
			Title:     title,
			Content:   content,
			ImgURL:    img,
			Tags:      tags,
			Show:      show,
			Popup:     popup,
			CreatedAt: now,
			UpdatedAt: now,
		}
		_, err := s.notices.Create(ctx, notice)
		return err
	}
	notice, err := s.notices.FindByID(ctx, *input.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	notice.Title = title
	notice.Content = content
	notice.ImgURL = img
	notice.Tags = tags
	notice.Show = show
	notice.Popup = popup
	notice.UpdatedAt = now
	return s.notices.Update(ctx, notice)
}

func (s *adminNoticeService) Toggle(ctx context.Context, id int64) error {
	if s == nil || s.notices == nil {
		return fmt.Errorf("admin notice service not configured / 管理公告服务未配置")
	}
	if id <= 0 {
		return fmt.Errorf("id must be positive / id 必须为正数")
	}
	notice, err := s.notices.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	notice.Show = !notice.Show
	notice.UpdatedAt = s.now().Unix()
	return s.notices.Update(ctx, notice)
}

func (s *adminNoticeService) Delete(ctx context.Context, id int64) error {
	if s == nil || s.notices == nil {
		return fmt.Errorf("admin notice service not configured / 管理公告服务未配置")
	}
	if id <= 0 {
		return fmt.Errorf("id must be positive / id 必须为正数")
	}
	if _, err := s.notices.FindByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return s.notices.Delete(ctx, id)
}

func (s *adminNoticeService) Sort(ctx context.Context, ids []int64) error {
	if s == nil || s.notices == nil {
		return fmt.Errorf("admin notice service not configured / 管理公告服务未配置")
	}
	cleaned := uniquePositiveIDs(ids)
	if len(cleaned) == 0 {
		return fmt.Errorf("ids are required / ids 不能为空")
	}
	return s.notices.Sort(ctx, cleaned, s.now().Unix())
}

func mapNotice(record *repository.Notice) AdminNoticeView {
	if record == nil {
		return AdminNoticeView{}
	}
	tags := make([]string, len(record.Tags))
	copy(tags, record.Tags)
	return AdminNoticeView{
		ID:        record.ID,
		Sort:      record.Sort,
		Title:     record.Title,
		Content:   record.Content,
		ImgURL:    record.ImgURL,
		Tags:      tags,
		Show:      record.Show,
		Popup:     record.Popup,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
}

func sanitizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	clean := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		clean = append(clean, trimmed)
	}
	if len(clean) == 0 {
		return nil
	}
	return clean
}

func boolValue(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}

func uniquePositiveIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	result := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
