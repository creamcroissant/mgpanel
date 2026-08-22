package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type LoadOptions struct {
	ConfigPath string
	WorkingDir string
}

type EnsureDefaultConfigOptions struct {
	ConfigPath string
	WorkingDir string
}

func Load() (*Config, error) {
	return LoadWithOptions(LoadOptions{})
}

func LoadWithOptions(opts LoadOptions) (*Config, error) {
	v := viper.New()
	setDefaults(v)
	v.SetConfigType("yaml")
	if err := configureConfigFile(v, opts); err != nil {
		return nil, err
	}
	v.SetEnvPrefix("MGPANEL")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	if err := bindEnv(v); err != nil {
		return nil, err
	}
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}
	if legacyAddr := strings.TrimSpace(v.GetString("grpc.address")); legacyAddr != "" {
		v.Set("grpc.addr", legacyAddr)
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if configDir := configuredDir(v.ConfigFileUsed()); configDir != "" {
		resolveRelativePaths(&cfg, configDir)
	} else {
		cfg.DB.Path = resolveRelativePath(effectiveWorkingDir(opts.WorkingDir), cfg.DB.Path)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return &cfg, nil
}

func EnsureDefaultConfig(opts EnsureDefaultConfigOptions) (string, error) {
	if strings.TrimSpace(opts.ConfigPath) != "" {
		return "", nil
	}

	workingDir := effectiveWorkingDir(opts.WorkingDir)
	if strings.TrimSpace(workingDir) == "" {
		return "", fmt.Errorf("resolve working directory")
	}

	configPath := filepath.Join(workingDir, "config.yml")
	if _, err := os.Stat(configPath); err == nil {
		return configPath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat config file: %w", err)
	}

	if err := writeDefaultConfigFile(configPath); err != nil {
		return "", err
	}

	return configPath, nil
}

func writeDefaultConfigFile(configPath string) error {
	payload := map[string]any{
		"http": map[string]any{
			"addr": "0.0.0.0:8080",
		},
		"database": map[string]any{
			"driver": "sqlite",
			"path":   "data/mgpanel.db",
		},
		"auth": map[string]any{
			"signing_key": "change-me",
		},
		"log": map[string]any{
			"level":       "info",
			"format":      "json",
			"environment": "production",
		},
		"ui": map[string]any{
			"admin": map[string]any{
				"enabled":        true,
				"dir":            "web/user-vite/dist",
				"title":          "MGPanel Admin",
				"version":        "1.0.0",
				"logo":           "https://mgpanel.io/images/logo.png",
				"hidden_modules": []string{"ticket", "gift-card", "plugin", "theme"},
			},
			"user": map[string]any{
				"enabled": true,
				"dir":     "web/user-vite/dist",
				"title":   "MGPanel",
			},
			"install": map[string]any{
				"enabled": true,
				"dir":     "web/install",
			},
		},
		"grpc": map[string]any{
			"enabled":         true,
			"addr":            "0.0.0.0:8080",
			"reuse_http_port": true,
		},
		"scheduler": map[string]any{
			"stat_user_hourly": "@every 5m",
			"traffic_fetch":    "@every 1m",
			"email_notify":     "@every 1m",
			"telegram_notify":  "@every 1m",
		},
	}

	content, err := yaml.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal default config: %w", err)
	}
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		return fmt.Errorf("write default config: %w", err)
	}
	return nil
}

func configureConfigFile(v *viper.Viper, opts LoadOptions) error {
	if v == nil {
		return fmt.Errorf("viper is nil")
	}
	if configPath := strings.TrimSpace(opts.ConfigPath); configPath != "" {
		absPath, err := filepath.Abs(configPath)
		if err != nil {
			return fmt.Errorf("resolve config path: %w", err)
		}
		v.SetConfigFile(absPath)
		return nil
	}
	v.SetConfigName("config")
	workingDir := effectiveWorkingDir(opts.WorkingDir)
	if workingDir != "" {
		v.AddConfigPath(workingDir)
		v.AddConfigPath(filepath.Join(workingDir, "etc"))
	}
	v.AddConfigPath("/etc/mgpanel/")
	return nil
}

func bindEnv(v *viper.Viper) error {
	bindings := map[string][]string{
		"grpc.enabled":               {"MGPANEL_GRPC_ENABLED"},
		"grpc.addr":                  {"MGPANEL_GRPC_ADDR"},
		"grpc.reuse_http_port":       {"MGPANEL_GRPC_REUSE_HTTP_PORT"},
		"grpc.tls.enabled":           {"MGPANEL_GRPC_TLS_ENABLED"},
		"grpc.tls.cert_file":         {"MGPANEL_GRPC_TLS_CERT_FILE"},
		"grpc.tls.key_file":          {"MGPANEL_GRPC_TLS_KEY_FILE"},
		"ui.install.enabled":         {"MGPANEL_UI_INSTALL_ENABLED", "MGPANEL_INSTALL_UI_ENABLED", "INSTALL_UI_ENABLED"},
		"ui.install.dir":             {"MGPANEL_UI_INSTALL_DIR", "MGPANEL_INSTALL_UI_DIR", "INSTALL_UI_DIR"},
		"ui.admin.logo":              {"MGPANEL_UI_ADMIN_LOGO", "MGPANEL_ADMIN_UI_LOGO", "ADMIN_UI_LOGO"},
		"ui.admin.deploy_script_url": {"MGPANEL_UI_ADMIN_DEPLOY_SCRIPT_URL"},
		"scheduler.stat_user_hourly": {"MGPANEL_SCHEDULER_STAT_USER_HOURLY"},
		"scheduler.traffic_fetch":    {"MGPANEL_SCHEDULER_TRAFFIC_FETCH"},
		"scheduler.email_notify":     {"MGPANEL_SCHEDULER_EMAIL_NOTIFY"},
		"scheduler.telegram_notify":  {"MGPANEL_SCHEDULER_TELEGRAM_NOTIFY"},
	"mcp.enabled":                      {"MGPANEL_MCP_ENABLED"},
	"mcp.api_key":                      {"MGPANEL_MCP_API_KEY"},
	"mcp.max_agent_log_lines":          {"MGPANEL_MCP_MAX_AGENT_LOG_LINES"},
	"mcp.agent_log_upload_interval_seconds": {"MGPANEL_MCP_AGENT_LOG_UPLOAD_INTERVAL"},
	"mcp.server_log_max_lines":         {"MGPANEL_MCP_SERVER_LOG_MAX_LINES"},
	}
	for key, envs := range bindings {
		args := append([]string{key}, envs...)
		if err := v.BindEnv(args...); err != nil {
			return fmt.Errorf("bind env %s: %w", key, err)
		}
	}
	return nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("http.addr", "0.0.0.0:8080")
	v.SetDefault("http.shutdown_timeout", "15s")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
	v.SetDefault("log.environment", "production")
	v.SetDefault("log.log_dir", "logs")
	v.SetDefault("log.max_days", 7)
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.path", "data/mgpanel.db")
	v.SetDefault("auth.signing_key", "change-me")
	v.SetDefault("auth.token_ttl", "24h")
	v.SetDefault("auth.issuer", "mgpanel")
	v.SetDefault("auth.audience", "mgpanel-client")
	v.SetDefault("auth.leeway", "30s")
	v.SetDefault("auth.bcrypt_cost", 12)
	v.SetDefault("ui.admin.enabled", true)
	v.SetDefault("ui.admin.dir", "web/user-vite/dist")
	v.SetDefault("ui.admin.title", "MGPanel Admin")
	v.SetDefault("ui.admin.version", "1.0.0")
	v.SetDefault("ui.admin.logo", "https://mgpanel.io/images/logo.png")
	v.SetDefault("ui.admin.hidden_modules", []string{"ticket", "gift-card", "plugin", "theme"})
	v.SetDefault("ui.admin.deploy_script_url", "https://raw.githubusercontent.com/creamcroissant/mgpanel/main/deploy/agent.sh")
	v.SetDefault("ui.user.enabled", true)
	v.SetDefault("ui.user.dir", "web/user-vite/dist")
	v.SetDefault("ui.user.title", "MGPanel")
	v.SetDefault("ui.install.enabled", true)
	v.SetDefault("ui.install.dir", "web/install")
	v.SetDefault("grpc.reuse_http_port", true)
	v.SetDefault("grpc.addr", "0.0.0.0:8080")
	v.SetDefault("scheduler.stat_user_hourly", "@every 5m")
	v.SetDefault("scheduler.traffic_fetch", "@every 1m")
	v.SetDefault("scheduler.email_notify", "@every 1m")
	v.SetDefault("scheduler.telegram_notify", "@every 1m")
	v.SetDefault("agent_task.core_operation_claim_timeout", "2m")
	v.SetDefault("agent_task.lifecycle_operation_claim_timeout", "2m")
	v.SetDefault("agent_task.apply_run_claim_timeout", "10m")

	// MCP defaults
	v.SetDefault("mcp.enabled", false)
	v.SetDefault("mcp.api_key", "")
	v.SetDefault("mcp.max_agent_log_lines", 50)
	v.SetDefault("mcp.agent_log_upload_interval_seconds", 60)
	v.SetDefault("mcp.server_log_max_lines", 500)
}

func configuredDir(configPath string) string {
	if strings.TrimSpace(configPath) == "" {
		return ""
	}
	return filepath.Dir(configPath)
}

func resolveRelativePaths(cfg *Config, baseDir string) {
	if cfg == nil || baseDir == "" {
		return
	}
	cfg.DB.Path = resolveRelativePath(baseDir, cfg.DB.Path)
	cfg.UI.Admin.Dir = resolveRelativePath(baseDir, cfg.UI.Admin.Dir)
	cfg.UI.User.Dir = resolveRelativePath(baseDir, cfg.UI.User.Dir)
	cfg.UI.Install.Dir = resolveRelativePath(baseDir, cfg.UI.Install.Dir)
	cfg.GRPC.TLS.CertFile = resolveRelativePath(baseDir, cfg.GRPC.TLS.CertFile)
	cfg.GRPC.TLS.KeyFile = resolveRelativePath(baseDir, cfg.GRPC.TLS.KeyFile)
}

func resolveRelativePath(baseDir, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || filepath.IsAbs(trimmed) {
		return trimmed
	}
	return filepath.Clean(filepath.Join(baseDir, trimmed))
}

func effectiveWorkingDir(workingDir string) string {
	trimmed := strings.TrimSpace(workingDir)
	if trimmed != "" {
		return trimmed
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
}
