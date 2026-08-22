// 文件路径: internal/service/adminpath.go
// 模块说明: 这是 internal 模块里的 adminpath 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// AdminPathService resolves the current secure admin path used by React Admin routes.
type AdminPathService interface {
	SecurePath(ctx context.Context) (string, error)
}

type adminPathService struct {
	settings repository.SettingRepository
	fallback string
	ttl      time.Duration

	mu      sync.RWMutex
	value   string
	expires time.Time
}

// NewAdminPathService builds a cached resolver backed by settings storage.
func NewAdminPathService(settings repository.SettingRepository) AdminPathService {
	return &adminPathService{
		settings: settings,
		fallback: "admin",
		ttl:      30 * time.Second,
	}
}

func (s *adminPathService) SecurePath(ctx context.Context) (string, error) {
	if cached := s.cachedValue(); cached != "" {
		return cached, nil
	}

	value := s.fallback
	if s.settings != nil {
		setting, err := s.settings.Get(ctx, "secure_path")
		if err == nil && setting != nil {
			if trimmed := strings.TrimSpace(setting.Value); trimmed != "" {
				value = trimmed
			}
		} else if err != nil && err != repository.ErrNotFound {
			return "", err
		}
	}

	s.store(value)
	return value, nil
}

func (s *adminPathService) cachedValue() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.value != "" && time.Now().Before(s.expires) {
		return s.value
	}
	return ""
}

func (s *adminPathService) store(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value = value
	s.expires = time.Now().Add(s.ttl)
}
