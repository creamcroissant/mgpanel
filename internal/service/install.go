// 文件路径: internal/service/install.go
// 模块说明: 这是负责安装向导业务的 service，第一次部署时用于创建管理员账号。
package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/support/hash"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

// InstallInput 描述初始化向导中需要收集的字段。
type InstallInput struct {
	Email    string
	Username string
	Password string
}

// InstallService 用于判断系统是否需要初始化，并创建首个管理员。
type InstallService interface {
	NeedsBootstrap(ctx context.Context) (bool, error)
	CreateAdmin(ctx context.Context, input InstallInput) (*repository.User, error)
	I18n() *i18n.Manager
}


type installService struct {
	users  repository.UserRepository
	hasher hash.Hasher
	i18n   *i18n.Manager

	cacheTTL time.Duration
	mu       sync.RWMutex
	cached   bool
	valid    bool
	expires  time.Time
}

// NewInstallService 构建安装向导服务。
func NewInstallService(users repository.UserRepository, hasher hash.Hasher, i18n *i18n.Manager) InstallService {
	return &installService{
		users:    users,
		hasher:   hasher,
		i18n:     i18n,
		cacheTTL: 15 * time.Second,
	}
}

func (s *installService) I18n() *i18n.Manager {
	return s.i18n
}

func (s *installService) NeedsBootstrap(ctx context.Context) (bool, error) {
	if s == nil || s.users == nil {
		return false, fmt.Errorf("install service not configured / 安装服务未配置")
	}
	if need, ok := s.cachedValue(); ok {
		return need, nil
	}
	hasAdmin, err := s.users.HasAdmin(ctx)
	if err != nil {
		return false, err
	}
	need := !hasAdmin
	s.storeCache(need)
	return need, nil
}

func (s *installService) CreateAdmin(ctx context.Context, input InstallInput) (*repository.User, error) {
	if s == nil || s.users == nil || s.hasher == nil {
		return nil, fmt.Errorf("install service not configured / 安装服务未配置")
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
	need, err := s.NeedsBootstrap(ctx)
	if err != nil {
		return nil, err
	}
	if !need {
		return nil, ErrAlreadyInitialized
	}
	if email != "" {
		if _, err := s.users.FindByEmail(ctx, email); err == nil {
			return nil, ErrEmailExists
		} else if err != nil && err != repository.ErrNotFound {
			return nil, err
		}
	}
	if username != "" {
		if _, err := s.users.FindByUsername(ctx, username); err == nil {
			return nil, ErrUsernameExists
		} else if err != nil && err != repository.ErrNotFound {
			return nil, err
		}
	}
	hashValue, err := s.hasher.Hash(password)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	user := &repository.User{
		UUID:              newUserUUID(),
		Token:             newUserToken(),
		Username:          username,
		Email:             email,
		Password:          hashValue,
		BalanceCents:      0,
		PlanID:            0,
		GroupID:           0,
		ExpiredAt:         0,
		U:                 0,
		D:                 0,
		TransferEnable:    0,
		IsAdmin:           true,
		Status:            1,
		Banned:            false,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	created, err := s.users.Create(ctx, user)
	if err != nil {
		return nil, err
	}
	s.invalidate()
	return created, nil
}

func (s *installService) cachedValue() (bool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.valid || time.Now().After(s.expires) {
		return false, false
	}
	return s.cached, true
}

func (s *installService) storeCache(need bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cached = need
	s.valid = true
	s.expires = time.Now().Add(s.cacheTTL)
}

func (s *installService) invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cached = false
	s.valid = false
	s.expires = time.Time{}
}
