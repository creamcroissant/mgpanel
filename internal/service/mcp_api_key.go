package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

// KeyCheckFunc adapts MCPApiKeyService to mcp.KeyValidator.
type KeyCheckFunc func(rawKey string) (bool, error)

func (f KeyCheckFunc) Validate(rawKey string) (bool, error) { return f(rawKey) }

// MCPApiKeyService manages MCP API keys.
type MCPApiKeyService interface {
	Create(ctx context.Context, name string, createdBy int64) (*MCPApiKeyResult, error)
	List(ctx context.Context) ([]*MCPApiKeyResult, error)
	Revoke(ctx context.Context, id int64) error
	Delete(ctx context.Context, id int64) error
	Validate(ctx context.Context, rawKey string) (*MCPApiKeyResult, error)
}

// MCPApiKeyResult is the public view of an MCP API key.
type MCPApiKeyResult struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Prefix     string `json:"prefix"`
	Key        string `json:"key,omitempty"`  // only returned on create
	Enabled    bool   `json:"enabled"`
	LastUsedAt int64  `json:"last_used_at,omitempty"`
	CreatedBy  int64  `json:"created_by"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
}

type mcpApiKeyService struct {
	repo   repository.MCPApiKeyRepository
	logger *slog.Logger
}

func NewMCPApiKeyService(repo repository.MCPApiKeyRepository) MCPApiKeyService {
	return &mcpApiKeyService{repo: repo, logger: slog.Default()}
}

func generateMCPKey() (prefix, rawKey, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", "", fmt.Errorf("generate key: %w", err)
	}
	rawKey = "mcp_" + hex.EncodeToString(buf)
	prefix = rawKey[:12] // "mcp_" + first 8 hex chars

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(rawKey), bcrypt.DefaultCost)
	if err != nil {
		return "", "", "", fmt.Errorf("hash key: %w", err)
	}
	return prefix, rawKey, string(hashBytes), nil
}

func (s *mcpApiKeyService) Create(ctx context.Context, name string, createdBy int64) (*MCPApiKeyResult, error) {
	prefix, rawKey, hash, err := generateMCPKey()
	if err != nil {
		return nil, err
	}
	key := &repository.MCPApiKey{
		Name:      strings.TrimSpace(name),
		Prefix:    prefix,
		KeyHash:   hash,
		Enabled:   true,
		CreatedBy: createdBy,
	}
	if err := s.repo.Create(ctx, key); err != nil {
		return nil, err
	}
	s.logger.Info("mcp api key created", "prefix", prefix, "created_by", createdBy)
	return &MCPApiKeyResult{
		ID:        key.ID,
		Name:      key.Name,
		Prefix:    key.Prefix,
		Key:       rawKey,
		Enabled:   key.Enabled,
		CreatedBy: key.CreatedBy,
		CreatedAt: key.CreatedAt,
		UpdatedAt: key.UpdatedAt,
	}, nil
}

func (s *mcpApiKeyService) List(ctx context.Context) ([]*MCPApiKeyResult, error) {
	keys, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*MCPApiKeyResult, 0, len(keys))
	for _, k := range keys {
		result = append(result, &MCPApiKeyResult{
			ID:         k.ID,
			Name:       k.Name,
			Prefix:     k.Prefix,
			Enabled:    k.Enabled,
			LastUsedAt: k.LastUsedAt,
			CreatedBy:  k.CreatedBy,
			CreatedAt:  k.CreatedAt,
			UpdatedAt:  k.UpdatedAt,
		})
	}
	return result, nil
}

func (s *mcpApiKeyService) Revoke(ctx context.Context, id int64) error {
	key, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	key.Enabled = false
	if err := s.repo.Update(ctx, key); err != nil {
		return err
	}
	s.logger.Info("mcp api key revoked", "prefix", key.Prefix)
	return nil
}

func (s *mcpApiKeyService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.logger.Info("mcp api key deleted", "id", id)
	return nil
}

func (s *mcpApiKeyService) Validate(ctx context.Context, rawKey string) (*MCPApiKeyResult, error) {
	rawKey = strings.TrimSpace(rawKey)
	if rawKey == "" {
		return nil, fmt.Errorf("empty key")
	}
	prefix := rawKey
	if len(rawKey) > 12 {
		prefix = rawKey[:12]
	}
	key, err := s.repo.GetByPrefix(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("invalid key")
	}
	if !key.Enabled {
		return nil, fmt.Errorf("key is revoked")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(key.KeyHash), []byte(rawKey)); err != nil {
		return nil, fmt.Errorf("invalid key")
	}
		// Update last used time asynchronously with short timeout
		go func(id int64) {
			upCtx, upCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer upCancel()
			_ = s.repo.UpdateLastUsed(upCtx, id, time.Now().Unix())
		}(key.ID)
	return &MCPApiKeyResult{
		ID:         key.ID,
		Name:       key.Name,
		Prefix:     key.Prefix,
		Enabled:    key.Enabled,
		LastUsedAt: key.LastUsedAt,
		CreatedBy:  key.CreatedBy,
		CreatedAt:  key.CreatedAt,
		UpdatedAt:  key.UpdatedAt,
	}, nil
}

