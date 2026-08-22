package service

import (
    "context"
    "errors"
    "fmt"

    "github.com/creamcroissant/mgpanel/internal/repository"
)

// UserNoticeService exposes user-facing notice operations.
type UserNoticeService interface {
    GetUnreadPopupNotice(ctx context.Context, userID string) (*UserNoticeView, error)
    MarkNoticeRead(ctx context.Context, userID string, noticeID int64) error
}

// UserNoticeView models notice payload returned to users.
type UserNoticeView struct {
    ID        int64    `json:"id"`
    Title     string   `json:"title"`
    Content   string   `json:"content"`
    ImgURL    string   `json:"img_url"`
    Tags      []string `json:"tags"`
    CreatedAt int64    `json:"created_at"`
    UpdatedAt int64    `json:"updated_at"`
}

type userNoticeService struct {
    notices repository.NoticeRepository
    reads   repository.UserNoticeReadsRepository
}

// NewUserNoticeService constructs a user-facing notice service.
func NewUserNoticeService(notices repository.NoticeRepository, reads repository.UserNoticeReadsRepository) UserNoticeService {
    return &userNoticeService{notices: notices, reads: reads}
}

func (s *userNoticeService) GetUnreadPopupNotice(ctx context.Context, userID string) (*UserNoticeView, error) {
    if s == nil || s.notices == nil || s.reads == nil {
        return nil, fmt.Errorf("user notice service not configured / 用户公告服务未配置")
    }
    uid, err := parseUserID(userID)
    if err != nil {
        return nil, err
    }
    ids, err := s.reads.GetUnreadPopupNoticeIDs(ctx, uid)
    if err != nil {
        return nil, err
    }
    for _, id := range ids {
        if id <= 0 {
            continue
        }
        record, err := s.notices.FindByID(ctx, id)
        if err != nil {
            if errors.Is(err, repository.ErrNotFound) {
                continue
            }
            return nil, err
        }
        if record == nil || !record.Show || !record.Popup {
            continue
        }
        view := mapUserNotice(record)
        return &view, nil
    }
    return nil, ErrNotFound
}

func (s *userNoticeService) MarkNoticeRead(ctx context.Context, userID string, noticeID int64) error {
    if s == nil || s.notices == nil || s.reads == nil {
        return fmt.Errorf("user notice service not configured / 用户公告服务未配置")
    }
    if noticeID <= 0 {
        return fmt.Errorf("notice id must be positive / notice id 必须为正数")
    }
    uid, err := parseUserID(userID)
    if err != nil {
        return err
    }
    if _, err := s.notices.FindByID(ctx, noticeID); err != nil {
        if errors.Is(err, repository.ErrNotFound) {
            return ErrNotFound
        }
        return err
    }
    return s.reads.MarkRead(ctx, uid, noticeID)
}

func mapUserNotice(record *repository.Notice) UserNoticeView {
    if record == nil {
        return UserNoticeView{}
    }
    tags := make([]string, len(record.Tags))
    copy(tags, record.Tags)
    return UserNoticeView{
        ID:        record.ID,
        Title:     record.Title,
        Content:   record.Content,
        ImgURL:    record.ImgURL,
        Tags:      tags,
        CreatedAt: record.CreatedAt,
        UpdatedAt: record.UpdatedAt,
    }
}
