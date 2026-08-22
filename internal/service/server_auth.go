// 文件路径: internal/service/server_auth.go
// 模块说明: 这是 internal 模块里的 server_auth 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// ServerAuthService validates node credentials and resolves server metadata.
type ServerAuthService interface {
	Authenticate(ctx context.Context, token, nodeID, nodeType string) (*repository.Server, error)
}

type serverAuthService struct {
	settings repository.SettingRepository
	servers  repository.ServerRepository

	ttl     time.Duration
	mu      sync.RWMutex
	cached  string
	expires time.Time
}

// NewServerAuthService constructs a credential validator backed by settings + repository lookups.
func NewServerAuthService(settings repository.SettingRepository, servers repository.ServerRepository) ServerAuthService {
	return &serverAuthService{
		settings: settings,
		servers:  servers,
		ttl:      30 * time.Second,
	}
}

func (s *serverAuthService) Authenticate(ctx context.Context, token, nodeID, nodeType string) (*repository.Server, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrUnauthorized
	}
	expected, err := s.serverToken(ctx)
	if err != nil {
		return nil, err
	}
	if expected == "" || subtle.ConstantTimeCompare([]byte(expected), []byte(strings.TrimSpace(token))) != 1 {
		return nil, ErrUnauthorized
	}
	normalizedType, err := normalizeServerType(nodeType)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(nodeID) == "" {
		return nil, ErrNotFound
	}
	server, err := s.resolveServer(ctx, nodeID, normalizedType)
	if err != nil {
		return nil, err
	}
	return server, nil
}

func (s *serverAuthService) serverToken(ctx context.Context) (string, error) {
	if value, ok := s.cachedToken(); ok {
		return value, nil
	}
	if s.settings == nil {
		return "", errors.New("server auth: settings repository unavailable / 节点认证设置仓储不可用")
	}
	setting, err := s.settings.Get(ctx, "server_token") // DEPRECATED: Legacy node authentication. New Agent uses AgentHost token.
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.storeToken("")
			return "", nil
		}
		return "", err
	}
	token := strings.TrimSpace(setting.Value)
	s.storeToken(token)
	return token, nil
}

func (s *serverAuthService) cachedToken() (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if time.Now().Before(s.expires) {
		return s.cached, true
	}
	return "", false
}

func (s *serverAuthService) storeToken(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cached = value
	s.expires = time.Now().Add(s.ttl)
}

func (s *serverAuthService) resolveServer(ctx context.Context, identifier string, nodeType string) (*repository.Server, error) {
	if s.servers == nil {
		return nil, errors.New("server auth: server repository unavailable / 节点认证仓储不可用")
	}
	server, err := s.servers.FindByIdentifier(ctx, identifier, nodeType)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return server, nil
}

func normalizeServerType(raw string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "v2node" {
		trimmed = ""
	}
	if trimmed == "" {
		return "", nil
	}
	if alias, ok := serverTypeAliases[trimmed]; ok {
		trimmed = alias
	}
	if _, ok := validServerTypes[trimmed]; !ok {
		return "", ErrInvalidServerType
	}
	return trimmed, nil
}

var serverTypeAliases = map[string]string{
	"v2ray":     "vmess",
	"hysteria2": "hysteria",
}
