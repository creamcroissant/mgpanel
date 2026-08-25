package config

import (
	"fmt"
	"log/slog"
	"time"
)

// Config 汇总应用的全部配置。
type Config struct {
	HTTP      HTTPConfig      `mapstructure:"http"`
	GRPC      GRPCConfig      `mapstructure:"grpc"`
	Log       LogConfig       `mapstructure:"log"`
	DB        DBConfig        `mapstructure:"database"`
	Auth      AuthConfig      `mapstructure:"auth"`
	Security  SecurityConfig  `mapstructure:"security"`
	Metrics   MetricsConfig   `mapstructure:"metrics"`
	UI        UIConfig        `mapstructure:"ui"`
	Scheduler SchedulerConfig `mapstructure:"scheduler"`
	Cores     []CoreConfig    `mapstructure:"cores"`
	Nodes     []NodeConfig    `mapstructure:"nodes"`
	Mesh      MeshConfig      `mapstructure:"mesh"`
	AgentTask AgentTaskConfig `mapstructure:"agent_task"`
	MCP       MCPConfig       `mapstructure:"mcp"`
}

// MeshConfig 定义面板侧 mesh 网络 ACL 配置。
type MeshConfig struct {
	// AllowedNetworks 允许 agent 加入的 mesh 网络 ID 白名单；空 = 仅允许 default。
	AllowedNetworks []string `mapstructure:"allowed_networks"`
}

// GRPCConfig 定义 Agent 通信所需的 gRPC 服务配置。
type GRPCConfig struct {
	Enabled       bool          `mapstructure:"enabled"`
	Addr          string        `mapstructure:"addr"`
	ReuseHTTPPort bool          `mapstructure:"reuse_http_port"`
	TLS           GRPCTLSConfig `mapstructure:"tls"`
}

// GRPCTLSConfig 定义 gRPC 服务的 TLS 配置。
type GRPCTLSConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
}

// SecurityConfig 定义安全相关配置。
type SecurityConfig struct {
	SubscribeObfuscation bool     `mapstructure:"subscribe_obfuscation"`
	CORSAllowedOrigins   []string `mapstructure:"cors_allowed_origins"`
	// TrustedProxies 显式可信反代 CIDR 白名单；为空时不信任任何 XFF 头
	// （客户端 IP 取直连地址）。反向代理部署必须包含代理地址，如 127.0.0.1/32。
	TrustedProxies []string `mapstructure:"trusted_proxies"`
}

// MetricsConfig 定义 Prometheus 指标配置。
type MetricsConfig struct {
	Enabled   bool      `mapstructure:"enabled"`
	Namespace string    `mapstructure:"namespace"`
	Subsystem string    `mapstructure:"subsystem"`
	Token     string    `mapstructure:"token"`
	Buckets   []float64 `mapstructure:"buckets"`
}

// HTTPConfig 定义 HTTP 服务配置。
type HTTPConfig struct {
	Addr            string        `mapstructure:"addr"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// LogConfig 定义日志配置。
type LogConfig struct {
	Level       string `mapstructure:"level"`
	Format      string `mapstructure:"format"`
	AddSource   bool   `mapstructure:"add_source"`
	Environment string `mapstructure:"environment"`
	// File path for persistent log storage (daily rotation, 7-day retention).
	// Empty disables file output (default). Example: /opt/mgpanel/logs
	LogDir string `mapstructure:"log_dir"`
	// MaxDays controls how many days of log files to retain (default 7).
	MaxDays int `mapstructure:"max_days"`
}

// DBConfig 定义数据库配置。
type DBConfig struct {
	Driver string `mapstructure:"driver"`
	Path   string `mapstructure:"path"`
}

// AuthConfig 定义认证配置。
type AuthConfig struct {
	SigningKey string        `mapstructure:"signing_key"`
	TokenTTL   time.Duration `mapstructure:"token_ttl"`
	Issuer     string        `mapstructure:"issuer"`
	Audience   string        `mapstructure:"audience"`
	Leeway     time.Duration `mapstructure:"leeway"`
	BcryptCost int           `mapstructure:"bcrypt_cost"`
}

// UIConfig 定义前端静态资源配置。
type UIConfig struct {
	Admin   AdminUIConfig   `mapstructure:"admin"`
	User    UserUIConfig    `mapstructure:"user"`
	Install InstallUIConfig `mapstructure:"install"`
}

type AdminUIConfig struct {
	Enabled         bool     `mapstructure:"enabled"`
	Dir             string   `mapstructure:"dir"`
	BaseURL         string   `mapstructure:"base_url"`
	Title           string   `mapstructure:"title"`
	Version         string   `mapstructure:"version"`
	Logo            string   `mapstructure:"logo"`
	DeployScriptURL string   `mapstructure:"deploy_script_url"`
	HiddenModules   []string `mapstructure:"hidden_modules"`
}

type UserUIConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Dir     string `mapstructure:"dir"`
	BaseURL string `mapstructure:"base_url"`
	Title   string `mapstructure:"title"`
}

type InstallUIConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Dir     string `mapstructure:"dir"`
}

// SchedulerConfig 定义 Panel 后台任务调度配置。
type SchedulerConfig struct {
	StatUserHourly string `mapstructure:"stat_user_hourly"`
	TrafficFetch   string `mapstructure:"traffic_fetch"`
	EmailNotify    string `mapstructure:"email_notify"`
	TelegramNotify string `mapstructure:"telegram_notify"`
}

// CoreConfig 定义代理核心配置（Xray/Sing-box）。
type CoreConfig struct {
	Type         string        `mapstructure:"type"`
	Name         string        `mapstructure:"name"`
	OriginalPath string        `mapstructure:"original_path"`
	Log          CoreLogConfig `mapstructure:"log"`
}

type CoreLogConfig struct {
	Level     string `mapstructure:"level"`
	Timestamp bool   `mapstructure:"timestamp"`
}

// NodeConfig 定义节点实例配置。
type NodeConfig struct {
	Core                   string     `mapstructure:"core"`
	NodeID                 int        `mapstructure:"node_id"`
	NodeType               string     `mapstructure:"node_type"`
	ApiHost                string     `mapstructure:"api_host"`
	ApiKey                 string     `mapstructure:"api_key"`
	Timeout                int        `mapstructure:"timeout"`
	ListenIP               string     `mapstructure:"listen_ip"`
	SendIP                 string     `mapstructure:"send_ip"`
	DeviceOnlineMinTraffic int64      `mapstructure:"device_online_min_traffic"`
	CertConfig             CertConfig `mapstructure:"cert_config"`
}

type CertConfig struct {
	CertMode   string `mapstructure:"cert_mode"`
	CertDomain string `mapstructure:"cert_domain"`
	Provider   string `mapstructure:"provider"`
}

// AgentTaskConfig 定义 agent 任务超时配置（server 端控制）。
type AgentTaskConfig struct {
	CoreOperationClaimTimeout     time.Duration `mapstructure:"core_operation_claim_timeout"`
	LifecycleOperationClaimTimeout time.Duration `mapstructure:"lifecycle_operation_claim_timeout"`
	ApplyRunClaimTimeout           time.Duration `mapstructure:"apply_run_claim_timeout"`
}

// MCPConfig defines MCP (Model Context Protocol) server settings.
type MCPConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	APIKey  string `mapstructure:"api_key"`

	// MaxAgentLogLines controls max cached log lines per agent host (default 50).
	MaxAgentLogLines int `mapstructure:"max_agent_log_lines"`

	// AgentLogUploadIntervalSeconds controls agent log upload interval (default 60).
	AgentLogUploadIntervalSeconds int `mapstructure:"agent_log_upload_interval_seconds"`

	// ServerLogMaxLines controls max lines returned by server_log_tail tool (default 500).
	ServerLogMaxLines int `mapstructure:"server_log_max_lines"`
}

func (c LogConfig) SlogLevel() slog.Level {
	switch c.Level {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	if c.GRPC.Enabled && c.GRPC.ReuseHTTPPort && c.GRPC.TLS.Enabled {
		return fmt.Errorf("grpc.tls.enabled is not supported when grpc.reuse_http_port=true")
	}
	return nil
}
