// 文件路径: internal/service/password.go
// 模块说明: 这是 internal 模块里的 password 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/cache"
	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/support/hash"
)

// PasswordResetInput 表示重置密码所需的请求参数。
type PasswordResetInput struct {
	Email     string
	EmailCode string
	Password  string
}

// PasswordService 负责密码找回流程。
type PasswordService interface {
	Reset(ctx context.Context, input PasswordResetInput) error
}

type passwordService struct {
	users  repository.UserRepository
	hasher hash.Hasher
	verify VerificationService
	limits cache.Store
}

const (
	forgetLimitMax = 3
	forgetLimitTTL = 5 * time.Minute
)

// NewPasswordService 组装密码重置流程所需依赖。
func NewPasswordService(users repository.UserRepository, hasher hash.Hasher, verify VerificationService, store cache.Store) PasswordService {
	var limits cache.Store
	if store != nil {
		limits = store.Namespace("auth:forget")
	}
	return &passwordService{
		users:  users,
		hasher: hasher,
		verify: verify,
		limits: limits,
	}
}

func (s *passwordService) Reset(ctx context.Context, input PasswordResetInput) error {
	if s == nil || s.users == nil || s.hasher == nil || s.verify == nil {
		return fmt.Errorf("password service not fully configured / 密码服务未完整配置")
	}
	email := normalizeEmail(input.Email)
	if email == "" {
		return ErrInvalidEmail
	}
	password := strings.TrimSpace(input.Password)
	if len(password) < 8 || !hasLetterAndNumber(password) {
		return ErrInvalidPassword
	}
	code := strings.TrimSpace(input.EmailCode)
	if code == "" {
		return ErrInvalidVerificationCode
	}

	if err := s.ensureLimit(ctx, email); err != nil {
		return err
	}

	if err := s.verify.ValidateEmailCode(ctx, email, code, true); err != nil {
		if errors.Is(err, ErrInvalidVerificationCode) {
			s.bumpLimit(ctx, email)
			return ErrInvalidVerificationCode
		}
		return err
	}

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	hashed, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}

	user.Password = hashed
	user.PasswordAlgo = ""
	user.PasswordSalt = ""
	user.UpdatedAt = time.Now().Unix()

	if err := s.users.Save(ctx, user); err != nil {
		return err
	}

	s.clearLimit(ctx, email)
	return nil
}

func (s *passwordService) ensureLimit(ctx context.Context, email string) error {
	if s.limits == nil {
		return nil
	}
	if count := s.limitCount(ctx, email); count >= forgetLimitMax {
		return ErrRateLimited
	}
	return nil
}

func (s *passwordService) bumpLimit(ctx context.Context, email string) {
	if s.limits == nil {
		return
	}
	key := forgetLimitCacheKey(email)
	if _, err := s.limits.Increment(ctx, key, 1, forgetLimitTTL); err != nil {
		return
	}
}

func (s *passwordService) clearLimit(ctx context.Context, email string) {
	if s.limits == nil {
		return
	}
	s.limits.Delete(ctx, forgetLimitCacheKey(email))
}

func (s *passwordService) limitCount(ctx context.Context, email string) int {
	if s.limits == nil {
		return 0
	}
	raw, ok := s.limits.Get(ctx, forgetLimitCacheKey(email))
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

func forgetLimitCacheKey(email string) string {
	if strings.TrimSpace(email) == "" {
		return "FORGET_REQUEST_LIMIT"
	}
	return "FORGET_REQUEST_LIMIT_" + email
}
