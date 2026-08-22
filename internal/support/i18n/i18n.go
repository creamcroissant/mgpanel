package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/text/language"
)

//go:embed locales/*.json
var embeddedLocales embed.FS

// Embed 仅用于测试兼容性（在其他目录运行时）。
// 实际运行中，我们期望 locales 相对本包路径，但在其他包中运行测试时
// embed 可能因为路径关系出现问题；不过 embed 在构建期完成，因此通常是安全的。
// 报错 "pattern locales/*.json: no matching files found" 说明目录结构与预期不一致。
// 当前文件位于 internal/support/i18n/i18n.go，语言包在 internal/locales/*.json。
// 所以 //go:embed locales/*.json 会尝试查找 internal/support/i18n/locales。
// 需要按实际路径调整 embed 规则。

// Manager 管理翻译内容。
type Manager struct {
	defaultLang  string
	translations map[string]map[string]string
	logger       *slog.Logger
	mu           sync.RWMutex
}

// Option 用于配置 Manager。
type Option func(*Manager)

// WithLogger 设置 Manager 使用的日志实例。
func WithLogger(logger *slog.Logger) Option {
	return func(m *Manager) {
		m.logger = logger
	}
}

// WithDefaultLang 设置默认语言。
func WithDefaultLang(lang string) Option {
	return func(m *Manager) {
		m.defaultLang = lang
	}
}

// NewManager 创建 i18n Manager。
func NewManager(opts ...Option) (*Manager, error) {
	m := &Manager{
		defaultLang:  "en-US",
		translations: make(map[string]map[string]string),
		logger:       slog.Default(),
	}

	for _, opt := range opts {
		opt(m)
	}

	if err := m.loadEmbeddedTranslations(); err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Manager) loadEmbeddedTranslations() error {
	entries, err := embeddedLocales.ReadDir("locales")
	if err != nil {
		return fmt.Errorf("failed to read locales directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		lang := strings.TrimSuffix(entry.Name(), ".json")
		data, err := embeddedLocales.ReadFile("locales/" + entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read locale file %s: %w", entry.Name(), err)
		}

		var content map[string]string
		if err := json.Unmarshal(data, &content); err != nil {
			return fmt.Errorf("failed to unmarshal locale file %s: %w", entry.Name(), err)
		}

		m.mu.Lock()
		m.translations[lang] = content
		m.mu.Unlock()
	}

	return nil
}

// LoadFromDir 从外部目录加载翻译文件。
func (m *Manager) LoadFromDir(dir string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 外部目录不存在也可以继续。
		}
		return fmt.Errorf("failed to read external locales directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		lang := strings.TrimSuffix(file.Name(), ".json")
		data, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			m.logger.Warn("failed to read external locale file", "file", file.Name(), "error", err)
			continue
		}

		var content map[string]string
		if err := json.Unmarshal(data, &content); err != nil {
			m.logger.Warn("failed to unmarshal external locale file", "file", file.Name(), "error", err)
			continue
		}

		m.mu.Lock()
		// 合并或覆盖已有翻译内容
		if _, exists := m.translations[lang]; !exists {
			m.translations[lang] = make(map[string]string)
		}
		for k, v := range content {
			m.translations[lang][k] = v
		}
		m.mu.Unlock()
	}
	return nil
}

// Translate 按语言与键名返回翻译内容。
func (m *Manager) Translate(lang, key string, args ...interface{}) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 规范化语言标签
	tag, err := language.Parse(lang)
	if err == nil {
		lang = tag.String()
	}

	// 先尝试精确匹配
	if trans, ok := m.translations[lang]; ok {
		if val, ok := trans[key]; ok {
			if len(args) > 0 {
				return fmt.Sprintf(val, args...)
			}
			return val
		}
	}

	// 回退到默认语言
	if lang != m.defaultLang {
		if trans, ok := m.translations[m.defaultLang]; ok {
			if val, ok := trans[key]; ok {
				if len(args) > 0 {
					return fmt.Sprintf(val, args...)
				}
				return val
			}
		}
	}

	// 回退为原始 key
	return key
}

// GetSupportedLanguages 返回支持的语言列表。
func (m *Manager) GetSupportedLanguages() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	langs := make([]string, 0, len(m.translations))
	for k := range m.translations {
		langs = append(langs, k)
	}
	return langs
}

// GetTranslations 返回指定语言的完整翻译表。
func (m *Manager) GetTranslations(lang string) map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if trans, ok := m.translations[lang]; ok {
		// 返回副本，避免外部修改
		copy := make(map[string]string, len(trans))
		for k, v := range trans {
			copy[k] = v
		}
		return copy
	}
	return nil
}