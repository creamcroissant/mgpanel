// 文件路径: internal/bootstrap/config.go
// 模块说明: 这是 internal 模块里的 config 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package bootstrap

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config aggregates runtime settings consumed by the Go services.
type Config struct {
	HTTP HTTPConfig
	Log  LogConfig
	DB   DBConfig
	Auth AuthConfig
	UI   UIConfig
}

var defaultHiddenAdminModules = []string{"payment", "ticket", "gift-card", "plugin", "theme"}

// HTTPConfig stores listener and shutdown behavior.
type HTTPConfig struct {
	Addr            string
	ShutdownTimeout time.Duration
}

// LogConfig controls slog handler behavior.
type LogConfig struct {
	Level       slog.Level
	Format      string
	AddSource   bool
	Environment string
}

// DBConfig stores persistence layer settings.
type DBConfig struct {
	SQLitePath string
}

// AuthConfig controls token and password infrastructure defaults.
type AuthConfig struct {
	SigningKey string
	TokenTTL   time.Duration
	Issuer     string
	Audience   string
	Leeway     time.Duration
	BcryptCost int
}

// UIConfig controls how static assets are exposed.
type UIConfig struct {
	Admin   AdminUIConfig
	Install InstallUIConfig
}

// AdminUIConfig customizes the admin SPA bundle.
type AdminUIConfig struct {
	Enabled       bool
	Dir           string
	Title         string
	Version       string
	Logo          string
	BaseURL       string
	HiddenModules []string
}

// InstallUIConfig controls the lightweight安装向导静态页。
type InstallUIConfig struct {
	Enabled bool
	Dir     string
}

// LoadConfig loads runtime environment variables into Config.
func LoadConfig() (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix("XBOARD")
	v.AutomaticEnv()

	cfg := &Config{}

	cfg.HTTP.Addr = fallback(v.GetString("HTTP_ADDR"), "0.0.0.0:8080")
	cfg.HTTP.ShutdownTimeout = parseDuration(fallback(v.GetString("SHUTDOWN_TIMEOUT"), "15s"), 15*time.Second)

	cfg.Log.Level = parseLogLevel(fallback(firstNonEmpty(
		v.GetString("LOG_LEVEL"),
		os.Getenv("LOG_LEVEL"),
	), "info"))
	cfg.Log.Format = strings.ToLower(fallback(firstNonEmpty(
		v.GetString("LOG_FORMAT"),
		os.Getenv("LOG_FORMAT"),
	), "json"))
	cfg.Log.AddSource = v.GetBool("LOG_ADD_SOURCE")
	cfg.Log.Environment = fallback(firstNonEmpty(
		v.GetString("ENV"),
		os.Getenv("APP_ENV"),
	), "development")

	defaultDBPath := filepath.Clean(fallback(v.GetString("DB_PATH"), filepath.Join("data", "xboard.db")))
	cfg.DB.SQLitePath = fallback(firstNonEmpty(
		v.GetString("DB_PATH"),
		os.Getenv("XBOARD_DB_PATH"),
	), defaultDBPath)

	cfg.Auth.SigningKey = fallback(firstNonEmpty(
		v.GetString("AUTH_SIGNING_KEY"),
		os.Getenv("XBOARD_AUTH_SIGNING_KEY"),
		os.Getenv("APP_KEY"),
	), "change-me")
	cfg.Auth.TokenTTL = parseDuration(fallback(firstNonEmpty(
		v.GetString("AUTH_TOKEN_TTL"),
		os.Getenv("XBOARD_AUTH_TOKEN_TTL"),
	), "24h"), 24*time.Hour)
	cfg.Auth.Issuer = fallback(firstNonEmpty(
		v.GetString("AUTH_ISSUER"),
		os.Getenv("XBOARD_AUTH_ISSUER"),
	), "xboard")
	cfg.Auth.Audience = fallback(firstNonEmpty(
		v.GetString("AUTH_AUDIENCE"),
		os.Getenv("XBOARD_AUTH_AUDIENCE"),
	), "xboard-client")
	cfg.Auth.Leeway = parseDuration(fallback(firstNonEmpty(
		v.GetString("AUTH_LEEWAY"),
		os.Getenv("XBOARD_AUTH_LEEWAY"),
	), "30s"), 30*time.Second)
	cfg.Auth.BcryptCost = parseBcryptCost(v)

	adminEnabledRaw := firstNonEmpty(
		v.GetString("ADMIN_UI_ENABLED"),
		os.Getenv("XBOARD_ADMIN_UI_ENABLED"),
	)
	cfg.UI.Admin.Enabled = parseBool(adminEnabledRaw, true)
	cfg.UI.Admin.Dir = fallback(firstNonEmpty(
		v.GetString("ADMIN_UI_DIR"),
		os.Getenv("XBOARD_ADMIN_UI_DIR"),
	), filepath.Join("web", "user-vite", "dist"))
	cfg.UI.Admin.Title = fallback(firstNonEmpty(
		v.GetString("ADMIN_UI_TITLE"),
		os.Getenv("XBOARD_ADMIN_UI_TITLE"),
	), "XBoard Admin")
	cfg.UI.Admin.Version = fallback(firstNonEmpty(
		v.GetString("ADMIN_UI_VERSION"),
		os.Getenv("XBOARD_ADMIN_UI_VERSION"),
	), "go-dev")
	cfg.UI.Admin.Logo = fallback(firstNonEmpty(
		v.GetString("ADMIN_UI_LOGO"),
		os.Getenv("XBOARD_ADMIN_UI_LOGO"),
	), "https://xboard.io/images/logo.png")
	cfg.UI.Admin.BaseURL = strings.TrimRight(fallback(firstNonEmpty(
		v.GetString("ADMIN_UI_BASE_URL"),
		os.Getenv("XBOARD_ADMIN_UI_BASE_URL"),
	), ""), "/")
	cfg.UI.Admin.HiddenModules = parseCSVList(fallback(firstNonEmpty(
		v.GetString("ADMIN_UI_HIDDEN_MODULES"),
		os.Getenv("XBOARD_ADMIN_UI_HIDDEN_MODULES"),
	), ""), defaultHiddenAdminModules)

	installEnabledRaw := firstNonEmpty(
		v.GetString("INSTALL_UI_ENABLED"),
		os.Getenv("XBOARD_INSTALL_UI_ENABLED"),
	)
	cfg.UI.Install.Enabled = parseBool(installEnabledRaw, true)
	cfg.UI.Install.Dir = fallback(firstNonEmpty(
		v.GetString("INSTALL_UI_DIR"),
		os.Getenv("XBOARD_INSTALL_UI_DIR"),
	), filepath.Join("web", "install"))

	return cfg, nil
}

func parseBcryptCost(v *viper.Viper) int {
	cost := v.GetInt("AUTH_BCRYPT_COST")
	if cost <= 0 {
		if raw := os.Getenv("XBOARD_AUTH_BCRYPT_COST"); raw != "" {
			if parsed, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
				cost = parsed
			}
		}
	}
	if cost <= 0 {
		cost = 12
	}
	return cost
}

func parseCSVList(raw string, def []string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return append([]string(nil), def...)
	}
	parts := strings.Split(trimmed, ",")
	unique := make(map[string]struct{})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.ToLower(strings.TrimSpace(part))
		if value == "" {
			continue
		}
		if _, exists := unique[value]; exists {
			continue
		}
		unique[value] = struct{}{}
		result = append(result, value)
	}
	if len(result) == 0 {
		return append([]string(nil), def...)
	}
	return result
}

func parseDuration(raw string, fallback time.Duration) time.Duration {
	if raw == "" {
		return fallback
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d
	}
	return fallback
}

func parseLogLevel(raw string) slog.Level {
	switch strings.ToLower(raw) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error", "err":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func fallback(value, def string) string {
	if value == "" {
		return def
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func parseBool(raw string, def bool) bool {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return def
	}
	switch trimmed {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}
