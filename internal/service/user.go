// 文件路径: internal/service/user.go
// 模块说明: 这是 internal 模块里的 user 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/support/hash"
)

// ChangePasswordInput 描述修改密码的输入。
type ChangePasswordInput struct {
	OldPassword string
	NewPassword string
}

// UserService 汇总用户资料与订阅相关逻辑（管理端/用户端通用）。
type UserService interface {
	Profile(ctx context.Context, userID string) (map[string]any, error)
	UpdateProfile(ctx context.Context, userID string, payload map[string]any) error
	ChangePassword(ctx context.Context, userID string, input ChangePasswordInput) error
	ResetSecurity(ctx context.Context, userID string) (string, error)
}

// NewUserService 组装用户服务依赖。
func NewUserService(users repository.UserRepository, settings repository.SettingRepository, hasher hash.Hasher) UserService {
	return &repoBackedUserService{users: users, settings: settings, hasher: hasher}
}

// repoBackedUserService 基于仓储实现 UserService。
type repoBackedUserService struct {
	users    repository.UserRepository
	settings repository.SettingRepository
	hasher   hash.Hasher
}

// Profile 返回用户资料与订阅链接信息。
func (s *repoBackedUserService) Profile(ctx context.Context, userID string) (map[string]any, error) {
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
	
	subscribeURL := ""
	if s.settings != nil {
		if base, err := s.settings.Get(ctx, "subscribe_url"); err == nil && base != nil && base.Value != "" {
			subscribeURL = base.Value
		} else if appURL, err := s.settings.Get(ctx, "app_url"); err == nil && appURL != nil {
			subscribeURL = appURL.Value
		}
	}

	fullSubscribeURL := ""
	if subscribeURL != "" && user.Token != "" {
		// 清理 base URL 末尾斜杠。
		subscribeURL = strings.TrimRight(strings.TrimSpace(subscribeURL), "/")
		fullSubscribeURL = fmt.Sprintf("%s/api/v1/client/subscribe?token=%s", subscribeURL, user.Token)
	}

	return map[string]any{
		"id":                 user.ID,
		"email":              user.Email,
		"plan_id":            user.PlanID,
		"expired_at":         user.ExpiredAt,
		"transfer_enable":    user.TransferEnable,
		"transfer_used":      user.U + user.D,
		"is_admin":           user.IsAdmin,
		"status":             user.Status,
		"subscribe_url":      fullSubscribeURL,
	}, nil
}

func (s *repoBackedUserService) UpdateProfile(ctx context.Context, userID string, payload map[string]any) error {
	uid, err := parseUserID(userID)
	if err != nil {
		return err
	}
	user, err := s.users.FindByID(ctx, uid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if email, ok := payload["email"].(string); ok && email != "" {
		user.Email = email
	}
	if planRaw, ok := payload["plan_id"]; ok {
		if planID, convErr := parseNumeric(planRaw); convErr == nil {
			user.PlanID = planID
		}
	}
	user.UpdatedAt = time.Now().Unix()
	return s.users.Save(ctx, user)
}

// parseUserID 将字符串 ID 解析为 int64。
func parseUserID(raw string) (int64, error) {
	if raw == "" {
		return 0, fmt.Errorf("user id is required / 缺少用户 id")
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid user id: %v / 无效的用户 id: %w", err, err)
	}
	return id, nil
}

// parseNumeric 将数值字段统一转换为 int64。
func parseNumeric(val any) (int64, error) {
	switch v := val.(type) {
	case float64:
		return int64(v), nil
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, fmt.Errorf("unsupported numeric type %T / 不支持的数值类型 %T", val, val)
	}
}

// ChangePassword 校验旧密码并更新为新密码。
func (s *repoBackedUserService) ChangePassword(ctx context.Context, userID string, input ChangePasswordInput) error {
	if s.hasher == nil {
		return fmt.Errorf("password hasher not configured / 密码哈希器未配置")
	}

	uid, err := parseUserID(userID)
	if err != nil {
		return err
	}

	oldPassword := strings.TrimSpace(input.OldPassword)
	newPassword := strings.TrimSpace(input.NewPassword)

	if oldPassword == "" {
		return ErrInvalidPassword
	}
	if len(newPassword) < 8 {
		return ErrInvalidPassword
	}

	user, err := s.users.FindByID(ctx, uid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	// 校验旧密码。
	if err := s.hasher.Compare(user.Password, oldPassword); err != nil {
		return ErrInvalidPassword
	}

	// 生成新密码哈希。
	hashed, err := s.hasher.Hash(newPassword)
	if err != nil {
		return fmt.Errorf("password hash failed: %v / 密码哈希失败: %w", err, err)
	}

	user.Password = hashed
	user.PasswordAlgo = ""
	user.PasswordSalt = ""
	user.UpdatedAt = time.Now().Unix()

	return s.users.Save(ctx, user)
}

// ResetSecurity 重新生成用户订阅 token，用于安全重置。
func (s *repoBackedUserService) ResetSecurity(ctx context.Context, userID string) (string, error) {
	uid, err := parseUserID(userID)
	if err != nil {
		return "", err
	}

	user, err := s.users.FindByID(ctx, uid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}

	// 生成新的随机 token（32 bytes = 64 hex chars）。
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate token: %v / 生成 token 失败: %w", err, err)
	}
	newToken := hex.EncodeToString(tokenBytes)

	user.Token = newToken
	user.UpdatedAt = time.Now().Unix()

	if err := s.users.Save(ctx, user); err != nil {
		return "", err
	}

	return newToken, nil
}
