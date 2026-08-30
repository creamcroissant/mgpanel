// 文件路径: internal/service/config.go
// 模块说明: 这是 internal 模块里的 config 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

// ConfigService describes configuration CRUD for admin endpoints.
type ConfigService interface {
	Fetch(ctx context.Context) (map[string]any, error)
	Save(ctx context.Context, payload map[string]any) error
	I18n() *i18n.Manager
}


type repoBackedConfigService struct {
	settings repository.SettingRepository
	i18n     *i18n.Manager
}

// NewConfigService wires config operations to Settings repository.
func NewConfigService(settings repository.SettingRepository, i18n *i18n.Manager) ConfigService {
	return &repoBackedConfigService{settings: settings, i18n: i18n}
}

func (s *repoBackedConfigService) I18n() *i18n.Manager {
	return s.i18n
}

func (s *repoBackedConfigService) Fetch(ctx context.Context) (map[string]any, error) {
	result := defaultConfigSnapshot()
	entries, err := s.settings.List(ctx)
	if err != nil {
		return nil, err
	}
	var latest int64
	for _, entry := range entries {
		result[entry.Key] = decodeSettingValue(entry.Value)
		if entry.UpdatedAt > latest {
			latest = entry.UpdatedAt
		}
	}
	if latest > 0 {
		result["updated_at"] = time.Unix(latest, 0).UTC().Format(time.RFC3339Nano)
	}
	return result, nil
}

func (s *repoBackedConfigService) Save(ctx context.Context, payload map[string]any) error {
	if payload == nil {
		return nil
	}
	now := time.Now().Unix()
	for key, val := range payload {
		serialized, err := encodeSettingValue(val)
		if err != nil {
			return fmt.Errorf("encode %s: %v / 编码 %s 失败: %w", key, err, key, err)
		}
		setting := &repository.Setting{
			Key:       key,
			Value:     serialized,
			Category:  categorizeSettingKey(key),
			UpdatedAt: now,
		}
		if err := s.settings.Upsert(ctx, setting); err != nil {
			return err
		}
	}
	return nil
}

func defaultConfigSnapshot() map[string]any {
	return map[string]any{
		"site_name":        "MGPanel",
		"support_email":    "support@example.com",
		"default_theme":    "v2board",
		"telegram_enabled": false,
		"updated_at":       "",
	}
}

func decodeSettingValue(raw string) any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var jsonVal any
	if err := json.Unmarshal([]byte(trimmed), &jsonVal); err == nil {
		return jsonVal
	}
	if i, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return f
	}
	switch strings.ToLower(trimmed) {
	case "true":
		return true
	case "false":
		return false
	}
	return raw
}

func encodeSettingValue(value any) (string, error) {
	switch v := value.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case fmt.Stringer:
		return v.String(), nil
	default:
		b, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}

func categorizeSettingKey(key string) string {
	switch {
	case strings.HasPrefix(key, "theme"):
		return "theme"
	default:
		return "general"
	}
}
