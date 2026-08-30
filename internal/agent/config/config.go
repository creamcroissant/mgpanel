package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/protocol"
	"github.com/creamcroissant/mgpanel/internal/agent/updater"
	"gopkg.in/yaml.v3"
)

const (
	defaultProxyPortRangeStart    = 30000
	defaultProxyPortRangeEnd      = 40000
	defaultProxyMaxRetries        = 10
	defaultProxyHealthTimeout     = 10 * time.Second
	defaultProxyHealthInterval    = 500 * time.Millisecond
	defaultProxyDrainTimeout      = 5 * time.Second
	defaultProxyNftBin            = "/usr/sbin/nft"
	defaultProxyConntrackBin      = "conntrack"
	defaultProxyNftTableName      = "mgpanel_proxy"
	defaultProxyPIDDir            = "/var/run/mgpanel/cores"
	defaultProxyCgroupBasePath    = "/sys/fs/cgroup/mgpanel"

	defaultInstallScriptPath      = "/opt/mgpanel/deploy/agent.sh"
	defaultSingBoxBinaryPath      = "/opt/mgpanel/bin/sing-box"
	defaultXrayBinaryPath         = "/opt/mgpanel/bin/xray"
	defaultSingBoxReleaseRepo     = "creamcroissant/sing-box_with_api"
	defaultXrayReleaseRepo        = "XTLS/Xray-core"
	defaultCoreReleaseBaseURL     = "https://github.com"
	defaultCoreInstallDir         = "/opt/mgpanel/agent/cores"
	defaultUpdateHealthTimeout    = 2 * time.Minute
	defaultUpdateMaxCrashCount    = 3
	defaultUpdateJitterMax        = 30 * time.Second
	defaultUpdateMaxDownloadBytes = 200 * 1024 * 1024

	defaultSecretsFile = "/etc/mgpanel/agent/secrets.json"

	// defaultV2RayAPIListen is the default listen address for the sing-box
	// v2ray_api experimental section (runtime user management via HandlerService).
	defaultV2RayAPIListen = "127.0.0.1:19194"
)

type Config struct {
	Panel      PanelConfig      `yaml:"panel"`
	Server     ServerConfig     `yaml:"server"`
	GRPC       GRPCConfig       `yaml:"grpc"`
	GRPCServer GRPCServerConfig `yaml:"grpc_server"`
	Interval   IntervalConfig   `yaml:"interval"`
	Core       CoreConfig       `yaml:"core"`
	Traffic    TrafficConfig    `yaml:"traffic"`
	Protocol   ProtocolConfig   `yaml:"protocol"`
	Forwarding ForwardingConfig `yaml:"forwarding"`
	Proxy      ProxyConfig      `yaml:"proxy"`
	Update     UpdateConfig     `yaml:"update"`
	CDN        CDNConfig        `yaml:"cdn"`
	Log        LogConfig        `yaml:"log"`
	Mesh       MeshConfig       `yaml:"mesh"`
	Unlock     UnlockConfig     `yaml:"unlock"`
}

// UnlockConfig 控制流媒体解锁检测。
type UnlockConfig struct {
	// Enabled 控制是否启用解锁检测（每日自查）。
	Enabled bool `yaml:"enabled"`
	// IntervalHours 是每日自查间隔（小时），默认 24。
	IntervalHours int `yaml:"interval_hours"`
	// Services 是要检测的平台列表（netflix,disney_plus,youtube,tiktok,reddit,chatgpt）；空则全部。
	Services []string `yaml:"services"`
}

// MeshConfig holds configuration for WireGuard mesh networking.
type MeshConfig struct {
	// Enabled controls whether WireGuard mesh networking is active.
	Enabled bool `yaml:"enabled"`
	// ConfigDir is the directory for mesh WireGuard configuration files.
	ConfigDir string `yaml:"config_dir"`
	// ListenPort is the WireGuard listen port (default 51820).
	ListenPort int `yaml:"listen_port"`
	// WgBinary is the path to the wg binary (default "wg").
	WgBinary string `yaml:"wg_binary"`
	// NetworkCIDR is the mesh network CIDR (default "10.144.0.0/24").
	NetworkCIDR string          `yaml:"network_cidr"`
	Probe       MeshProbeConfig `yaml:"probe"`
}

// MeshProbeConfig holds configuration for mesh peer latency probing.
type MeshProbeConfig struct {
	Interval   int `yaml:"interval"`
	Timeout    int `yaml:"timeout"`
	WindowSize int `yaml:"window_size"`
}

// LogConfig holds agent log settings.
type LogConfig struct {
	// Dir is the directory for persisted daily logs (relative to working dir).
	// Default "logs". Set empty to disable file output.
	Dir string `yaml:"dir"`
	// MaxDays controls how many days of log files to retain (default 7).
	MaxDays int `yaml:"max_days"`

	// Upload controls agent log upload to Panel
	Upload LogUploadConfig `yaml:"upload"`
}

// LogUploadConfig controls periodic log upload via gRPC.
type LogUploadConfig struct {
	// Enabled controls whether log upload is active (default true).
	Enabled bool `yaml:"enabled"`
	// MaxLines is the max log lines per upload batch (default 50).
	MaxLines int `yaml:"max_lines"`
	// IntervalSeconds is the upload interval in seconds (default 60).
	IntervalSeconds int `yaml:"interval_seconds"`
	// Source identifies the log source: "agent" | "core" | "all" (default "all").
	Source string `yaml:"source"`
}

// GRPCConfig holds gRPC client configuration for connecting to Panel
type GRPCConfig struct {
	// Enabled controls whether gRPC transport is used (instead of HTTP)
	Enabled bool `yaml:"enabled"`

	// Address is the Panel gRPC server address (e.g., "panel.example.com:9090")
	Address string `yaml:"address"`

	// TLS configuration
	TLS TLSConfig `yaml:"tls"`

	// Keepalive configuration
	Keepalive KeepaliveConfig `yaml:"keepalive"`

	// Retry configuration
	Retry *RetryConfig `yaml:"retry"`

	// Timeout configuration
	Timeout TimeoutConfig `yaml:"timeout"`
}

// TLSConfig holds TLS settings for gRPC
type TLSConfig struct {
	// Enabled controls whether TLS is used
	Enabled bool `yaml:"enabled"`

	// CertFile is the path to the client certificate (for mutual TLS)
	CertFile string `yaml:"cert_file"`

	// KeyFile is the path to the client key (for mutual TLS)
	KeyFile string `yaml:"key_file"`

	// CAFile is the path to the CA certificate
	CAFile string `yaml:"ca_file"`

	// InsecureSkipVerify skips server certificate verification
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`
}

// GRPCServerConfig holds gRPC server configuration for Agent.
type GRPCServerConfig struct {
	// Enabled controls whether the Agent gRPC server is started.
	Enabled bool `yaml:"enabled"`

	// Listen address (e.g., ":19090", "127.0.0.1:19090")
	Listen string `yaml:"listen"`

	// AuthToken for gRPC authentication (uses host_token if empty)
	AuthToken string `yaml:"auth_token"`

	// TLS configuration
	TLS ServerTLSConfig `yaml:"tls"`
}

// ServerTLSConfig holds TLS settings for gRPC server.
type ServerTLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// KeepaliveConfig holds keepalive settings for gRPC
type KeepaliveConfig struct {
	// Time is the interval between keepalive pings
	Time time.Duration `yaml:"time"`

	// Timeout is the timeout for keepalive pings
	Timeout time.Duration `yaml:"timeout"`
}

// RetryConfig holds retry settings for gRPC calls.
type RetryConfig struct {
	Enabled         bool          `yaml:"enabled"`
	MaxRetries      int           `yaml:"max_retries"`
	InitialInterval time.Duration `yaml:"initial_interval"`
	MaxInterval     time.Duration `yaml:"max_interval"`
	Multiplier      float64       `yaml:"multiplier"`
}

// TimeoutConfig holds timeout settings for gRPC calls.
type TimeoutConfig struct {
	Default time.Duration `yaml:"default"`
	Connect time.Duration `yaml:"connect"`
}

type ServerConfig struct {
	// Enabled controls whether the HTTP server is started
	Enabled bool `yaml:"enabled"`

	// Listen address (e.g., ":8081", "127.0.0.1:8081")
	Listen string `yaml:"listen"`

	// AuthToken for API authentication (uses host_token if empty)
	AuthToken string `yaml:"auth_token"`
}

type ProtocolConfig struct {
	// ConfigDir is the primary configuration directory for backward compatibility.
	ConfigDir string `yaml:"config_dir"`

	// LegacyConfigDir is the legacy configuration directory.
	LegacyConfigDir string `yaml:"legacy_config_dir"`

	// ManagedConfigDir is the managed configuration directory.
	ManagedConfigDir string `yaml:"managed_config_dir"`

	// MergeOutputFile is the merged output filename for single-file core mode.
	MergeOutputFile string `yaml:"merge_output_file"`

	// SubscribeDir is the directory containing client subscription files
	SubscribeDir string `yaml:"subscribe_dir"`

	// ServiceName is the name of the sing-box service
	ServiceName string `yaml:"service_name"`

	// InitSystem overrides auto-detection: "systemd", "openrc", "runit", "custom"
	InitSystem string `yaml:"init_system"`

	// ValidateCmd is the command to validate config before applying (optional)
	ValidateCmd string `yaml:"validate_cmd"`

	// ServiceAction controls whether changes trigger reload/restart/none.
	ServiceAction string `yaml:"service_action"`

	// AutoRestart is a legacy compatibility field: true -> reload, false -> none.
	AutoRestart bool `yaml:"auto_restart"`

	// PreHook is executed before applying config changes (optional)
	PreHook string `yaml:"pre_hook"`

	// PostHook is executed after applying config changes (optional)
	PostHook string `yaml:"post_hook"`

	// SecretsFile is the path to the node-level secrets file. When empty it
	// defaults to /etc/mgpanel/agent/secrets.json. Never stored in the panel spec.
	SecretsFile string `yaml:"secrets_file"`

	// Custom commands for custom init system
	CustomCommands CustomCommands `yaml:"custom_commands"`

	// V2RayAPIListen is the listen address for the sing-box experimental.v2ray_api
	// section (runtime user management). When empty it defaults to
	// defaultV2RayAPIListen (127.0.0.1:19194). Xray ignores this field (it reuses
	// Traffic.Address for the HandlerService gRPC endpoint).
	V2RayAPIListen string `yaml:"v2ray_api_listen"`
}

type CustomCommands struct {
	Start   string `yaml:"start"`
	Stop    string `yaml:"stop"`
	Restart string `yaml:"restart"`
	Reload  string `yaml:"reload"`
	Status  string `yaml:"status"`
	Enable  string `yaml:"enable"`
	Disable string `yaml:"disable"`
}

type PanelConfig struct {
	URL              string `yaml:"url"`               // Panel base URL for initial registration (optional, defaults from grpc.address)
	Token            string `yaml:"token"`             // DEPRECATED: Legacy V2bX node token, no longer supported
	HostToken        string `yaml:"host_token"`        // Agent Host Token (long-lived auth)
	CommunicationKey string `yaml:"communication_key"` // One-time registration key for first boot
	AdvertiseHost    string `yaml:"advertise_host"`    // Override public IP reported to panel (auto-detected if empty)
	NodeID           int    `yaml:"node_id"`           // DEPRECATED: Legacy V2bX node ID, no longer supported
	NodeType         string `yaml:"node_type"`         // DEPRECATED: Legacy V2bX node type, no longer supported
}

type ForwardingConfig struct {
	// Enabled controls forwarding sync and apply
	Enabled bool `yaml:"enabled"`

	// SyncInterval is the sync interval for forwarding rules
	SyncInterval time.Duration `yaml:"sync_interval"`

	// TableName is the nftables table name
	TableName string `yaml:"table_name"`

	// NftBin is the nft executable path
	NftBin string `yaml:"nft_bin"`
}

type ProxyConfig struct {
	Enabled        bool          `yaml:"enabled"`
	PortRangeStart int           `yaml:"port_range_start"`
	PortRangeEnd   int           `yaml:"port_range_end"`
	MaxRetries     int           `yaml:"max_retries"`
	HealthTimeout  time.Duration `yaml:"health_timeout"`
	HealthInterval time.Duration `yaml:"health_interval"`
	DrainTimeout   time.Duration `yaml:"drain_timeout"`
	NftBin         string        `yaml:"nft_bin"`
	ConntrackBin   string        `yaml:"conntrack_bin"`
	NftTableName   string        `yaml:"nft_table_name"`
	PIDDir         string        `yaml:"pid_dir"`
	CgroupBasePath string        `yaml:"cgroup_base_path"`
}

type IntervalConfig struct {
	Sync   int `yaml:"sync"`   // Seconds
	Report int `yaml:"report"` // Seconds
}

// CDNConfig holds Caddy CDN configuration.
type CDNConfig struct {
	// Enabled controls whether CDN management is active.
	Enabled bool `yaml:"enabled"`

	// BinPath is the path to the caddy binary.
	BinPath string `yaml:"bin_path"`

	// ConfigDir is the directory containing the Caddyfile.
	ConfigDir string `yaml:"config_dir"`

	// AdminAddr is the Caddy admin API address (e.g., "localhost:2019").
	AdminAddr string `yaml:"admin_addr"`
}

type CoreConfig struct {
	TemplatePath      string `yaml:"template_path"`
	OutputPath        string `yaml:"output_path"`
	ReloadCmd         string `yaml:"reload_cmd"`
	InstallScriptPath string `yaml:"install_script_path"`
	SingBoxBinaryPath  string `yaml:"singbox_binary_path"`
	XrayBinaryPath     string `yaml:"xray_binary_path"`
	SingBoxReleaseRepo string `yaml:"singbox_release_repo"`
	XrayReleaseRepo    string `yaml:"xray_release_repo"`
	ReleaseBaseURL     string `yaml:"release_base_url"`
	CoreInstallDir     string `yaml:"core_install_dir"`
}

type TrafficConfig struct {
	Type      string `yaml:"type"`      // "netio", "none", "dummy", "xray_api"
	Interface string `yaml:"interface"` // Network interface name, e.g., "eth0"; empty for all
	Address   string `yaml:"address"`   // API address for xray_api type, e.g., "127.0.0.1:10085"
}

type UpdateConfig struct {
	AutoEnabled      bool          `yaml:"auto_enabled"`
	CurrentVersion   string        `yaml:"current_version"`
	BinaryPath       string        `yaml:"binary_path"`
	StatePath        string        `yaml:"state_path"`
	BackupDir        string        `yaml:"backup_dir"`
	ReleaseBaseURL   string        `yaml:"release_base_url"`
	ReleaseRepo      string        `yaml:"release_repo"`
	ReleaseTag       string        `yaml:"release_tag"`
	HealthTimeout    time.Duration `yaml:"health_timeout"`
	MaxCrashCount    int           `yaml:"max_crash_count"`
	JitterMin        time.Duration `yaml:"jitter_min"`
	JitterMax        time.Duration `yaml:"jitter_max"`
	MaxDownloadBytes int64         `yaml:"max_download_bytes"`
}

// Load reads configuration from file
func Load(path string) (*Config, error) {
	cfg, err := loadFromPath(path)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(cfg.Panel.HostToken) == "" {
		if err := ensureHostToken(path, cfg); err != nil {
			return nil, err
		}
	}

	if err := applyDefaults(cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func loadFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	return cfg, nil
}

func applyDefaults(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	// Set defaults
	if cfg.Interval.Sync <= 0 {
		cfg.Interval.Sync = 60
	}
	if cfg.Interval.Report <= 0 {
		cfg.Interval.Report = 60
	}

	// Server defaults
	if cfg.Server.Listen == "" {
		cfg.Server.Listen = ":8081"
	}
	if cfg.Server.AuthToken == "" && cfg.Panel.HostToken != "" {
		cfg.Server.AuthToken = cfg.Panel.HostToken
	}

	// gRPC server defaults are retired with agent-grpc-retirement; keep values untouched so legacy configs do not become required.

	// Protocol defaults
	if cfg.Protocol.ConfigDir == "" {
		switch {
		case cfg.Protocol.ManagedConfigDir != "":
			cfg.Protocol.ConfigDir = cfg.Protocol.ManagedConfigDir
		case cfg.Protocol.LegacyConfigDir != "":
			cfg.Protocol.ConfigDir = cfg.Protocol.LegacyConfigDir
		default:
			cfg.Protocol.ConfigDir = "/etc/sing-box/conf"
		}
	}
	if cfg.Protocol.LegacyConfigDir == "" {
		cfg.Protocol.LegacyConfigDir = cfg.Protocol.ConfigDir
	}
	if cfg.Protocol.ManagedConfigDir == "" {
		cfg.Protocol.ManagedConfigDir = cfg.Protocol.ConfigDir
	}
	if cfg.Protocol.MergeOutputFile == "" {
		cfg.Protocol.MergeOutputFile = "config.json"
	}
	if cfg.Protocol.SubscribeDir == "" {
		cfg.Protocol.SubscribeDir = "/etc/sing-box/subscribe"
	}
	if cfg.Protocol.ServiceName == "" {
		cfg.Protocol.ServiceName = "sing-box"
	}
	if cfg.Protocol.SecretsFile == "" {
		cfg.Protocol.SecretsFile = defaultSecretsFile
	}
	if cfg.Protocol.V2RayAPIListen == "" {
		cfg.Protocol.V2RayAPIListen = defaultV2RayAPIListen
	}

	// Core defaults
	if strings.TrimSpace(cfg.Core.InstallScriptPath) == "" {
		cfg.Core.InstallScriptPath = defaultInstallScriptPath
	}
	if strings.TrimSpace(cfg.Core.SingBoxBinaryPath) == "" {
		cfg.Core.SingBoxBinaryPath = defaultSingBoxBinaryPath
	}
	if strings.TrimSpace(cfg.Core.XrayBinaryPath) == "" {
		cfg.Core.XrayBinaryPath = defaultXrayBinaryPath
	}

	if strings.TrimSpace(cfg.Core.SingBoxReleaseRepo) == "" {
		cfg.Core.SingBoxReleaseRepo = defaultSingBoxReleaseRepo
	}
	if strings.TrimSpace(cfg.Core.XrayReleaseRepo) == "" {
		cfg.Core.XrayReleaseRepo = defaultXrayReleaseRepo
	}
	if strings.TrimSpace(cfg.Core.ReleaseBaseURL) == "" {
		cfg.Core.ReleaseBaseURL = defaultCoreReleaseBaseURL
	}
	if strings.TrimSpace(cfg.Core.CoreInstallDir) == "" {
		cfg.Core.CoreInstallDir = defaultCoreInstallDir
	}

	if strings.TrimSpace(cfg.Update.ReleaseBaseURL) == "" {
		cfg.Update.ReleaseBaseURL = updater.DefaultReleaseBaseURL
	}
	if strings.TrimSpace(cfg.Update.ReleaseRepo) == "" {
		cfg.Update.ReleaseRepo = updater.DefaultReleaseRepo
	}
	if strings.TrimSpace(cfg.Update.ReleaseTag) == "" {
		cfg.Update.ReleaseTag = updater.DefaultReleaseTag
	}
	if cfg.Update.HealthTimeout == 0 {
		cfg.Update.HealthTimeout = defaultUpdateHealthTimeout
	}
	if cfg.Update.MaxCrashCount == 0 {
		cfg.Update.MaxCrashCount = defaultUpdateMaxCrashCount
	}
	if cfg.Update.JitterMax == 0 {
		cfg.Update.JitterMax = defaultUpdateJitterMax
	}
	if cfg.Update.MaxDownloadBytes == 0 {
		cfg.Update.MaxDownloadBytes = defaultUpdateMaxDownloadBytes
	}

	// gRPC defaults
	if cfg.GRPC.Keepalive.Time == 0 {
		cfg.GRPC.Keepalive.Time = 30 * time.Second
	}
	if cfg.GRPC.Keepalive.Timeout == 0 {
		cfg.GRPC.Keepalive.Timeout = 10 * time.Second
	}
	if cfg.GRPC.Timeout.Default == 0 {
		cfg.GRPC.Timeout.Default = 10 * time.Second
	}
	if cfg.GRPC.Timeout.Connect == 0 {
		cfg.GRPC.Timeout.Connect = 5 * time.Second
	}
	if cfg.GRPC.Retry == nil {
		cfg.GRPC.Retry = &RetryConfig{
			Enabled:         true,
			MaxRetries:      3,
			InitialInterval: 500 * time.Millisecond,
			MaxInterval:     5 * time.Second,
			Multiplier:      2,
		}
	} else {
		if cfg.GRPC.Retry.MaxRetries == 0 {
			cfg.GRPC.Retry.MaxRetries = 3
		}
		if cfg.GRPC.Retry.InitialInterval == 0 {
			cfg.GRPC.Retry.InitialInterval = 500 * time.Millisecond
		}
		if cfg.GRPC.Retry.MaxInterval == 0 {
			cfg.GRPC.Retry.MaxInterval = 5 * time.Second
		}
		if cfg.GRPC.Retry.Multiplier == 0 {
			cfg.GRPC.Retry.Multiplier = 2
		}
	}

	// Forwarding defaults
	if cfg.Forwarding.SyncInterval == 0 {
		cfg.Forwarding.SyncInterval = 30 * time.Second
	}
	if cfg.Forwarding.TableName == "" {
		cfg.Forwarding.TableName = "mgpanel_forwarding"
	}
	if cfg.Forwarding.NftBin == "" {
		cfg.Forwarding.NftBin = defaultProxyNftBin
	}

	// Proxy defaults
	if cfg.Proxy.Enabled {
		if cfg.Proxy.PortRangeStart == 0 {
			cfg.Proxy.PortRangeStart = defaultProxyPortRangeStart
		}
		if cfg.Proxy.PortRangeEnd == 0 {
			cfg.Proxy.PortRangeEnd = defaultProxyPortRangeEnd
		}
		if cfg.Proxy.MaxRetries == 0 {
			cfg.Proxy.MaxRetries = defaultProxyMaxRetries
		}
		if cfg.Proxy.HealthTimeout == 0 {
			cfg.Proxy.HealthTimeout = defaultProxyHealthTimeout
		}
		if cfg.Proxy.HealthInterval == 0 {
			cfg.Proxy.HealthInterval = defaultProxyHealthInterval
		}
		if cfg.Proxy.DrainTimeout == 0 {
			cfg.Proxy.DrainTimeout = defaultProxyDrainTimeout
		}
		if cfg.Proxy.NftBin == "" {
			cfg.Proxy.NftBin = defaultProxyNftBin
		}
		if cfg.Proxy.ConntrackBin == "" {
			cfg.Proxy.ConntrackBin = defaultProxyConntrackBin
		}
		if cfg.Proxy.NftTableName == "" {
			cfg.Proxy.NftTableName = defaultProxyNftTableName
		}
		if cfg.Proxy.PIDDir == "" {
			cfg.Proxy.PIDDir = defaultProxyPIDDir
		}
		if cfg.Proxy.CgroupBasePath == "" {
			cfg.Proxy.CgroupBasePath = defaultProxyCgroupBasePath
		}

	}

	// CDN defaults
	if strings.TrimSpace(cfg.CDN.BinPath) == "" {
		cfg.CDN.BinPath = "/opt/mgpanel/caddy/caddy"
	}
	if strings.TrimSpace(cfg.CDN.ConfigDir) == "" {
		cfg.CDN.ConfigDir = "/opt/mgpanel/caddy"
	}
	if strings.TrimSpace(cfg.CDN.AdminAddr) == "" {
		cfg.CDN.AdminAddr = "localhost:2019"
	}

	// Log defaults
	if cfg.Log.Dir == "" {
		cfg.Log.Dir = "logs"
	}
	if cfg.Log.MaxDays <= 0 {
		cfg.Log.MaxDays = 7
	}

	// Log upload defaults
	if cfg.Log.Upload.IntervalSeconds <= 0 {
		cfg.Log.Upload.IntervalSeconds = 60
	}
	if cfg.Log.Upload.MaxLines <= 0 {
		cfg.Log.Upload.MaxLines = 50
	}
	if cfg.Log.Upload.Source == "" {
		cfg.Log.Upload.Source = "all"
	}

	return nil
}

func ensureHostToken(path string, cfg *Config) error {
	commKey := strings.TrimSpace(cfg.Panel.CommunicationKey)
	if commKey == "" {
		return nil
	}

	registerURL, err := resolveRegisterEndpoint(cfg)
	if err != nil {
		return err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("resolve hostname: %w", err)
	}

	// 手动配置的 advertise_host 优先；否则尝试探测本机公网 IP。
	advertiseHost := strings.TrimSpace(cfg.Panel.AdvertiseHost)
	if advertiseHost == "" {
		advertiseHost = DetectPublicIP()
	}

	hostToken, err := registerAgentHost(registerURL, registerRequest{
		CommunicationKey: commKey,
		Hostname:         strings.TrimSpace(hostname),
		AdvertiseHost:    advertiseHost,
	})
	if err != nil {
		return err
	}
	cfg.Panel.HostToken = hostToken
	cfg.Panel.CommunicationKey = ""
	if err := save(path, cfg); err != nil {
		return err
	}
	return nil
}

func resolveRegisterEndpoint(cfg *Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config is nil")
	}

	if panelURL := strings.TrimSpace(cfg.Panel.URL); panelURL != "" {
		base, err := normalizePanelURL(panelURL)
		if err != nil {
			return "", fmt.Errorf("invalid panel.url: %w", err)
		}
		return joinRegisterPath(base), nil
	}

	grpcAddress := strings.TrimSpace(cfg.GRPC.Address)
	if grpcAddress == "" {
		return "", fmt.Errorf("grpc.address is required to infer panel register endpoint")
	}

	host, port, err := net.SplitHostPort(grpcAddress)
	if err != nil {
		return "", fmt.Errorf("parse grpc.address %q: %w", grpcAddress, err)
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return "", fmt.Errorf("grpc.address host is empty")
	}

	// Panel serves both HTTP API and gRPC on the same port via h2c.
	// Use the same host:port from grpc.address with the appropriate scheme.
	scheme := "http"
	if cfg.GRPC.TLS.Enabled {
		scheme = "https"
	}
	addr := net.JoinHostPort(host, strings.TrimSpace(port))
	return scheme + "://" + addr + "/api/v1/agent/register", nil
}

func normalizePanelURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("panel.url must include scheme and host")
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func joinRegisterPath(base string) string {
	return strings.TrimSuffix(base, "/") + "/api/v1/agent/register"
}

type registerRequest struct {
	CommunicationKey string `json:"communication_key"`
	Hostname         string `json:"hostname"`
	AdvertiseHost    string `json:"advertise_host,omitempty"`
}

type registerResponse struct {
	Data registerResponseData `json:"data"`
}

type registerResponseData struct {
	HostToken string `json:"host_token"`
}

func registerAgentHost(endpoint string, req registerRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal register request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	httpReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build register request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request register endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message := fmt.Sprintf("register endpoint returned status %d", resp.StatusCode)
		if payload, readErr := io.ReadAll(io.LimitReader(resp.Body, 512)); readErr == nil {
			trimmed := strings.TrimSpace(string(payload))
			if trimmed != "" {
				message = fmt.Sprintf("%s: %s", message, trimmed)
			}
		}
		return "", fmt.Errorf("%s", message)
	}

	var parsed registerResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("decode register response: %w", err)
	}

	hostToken := strings.TrimSpace(parsed.Data.HostToken)
	if hostToken == "" {
		return "", fmt.Errorf("register response missing host_token")
	}
	return hostToken, nil
}

func save(path string, cfg *Config) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("config path is empty")
	}
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config file: %w", err)
	}
	if len(data) == 0 {
		return fmt.Errorf("marshal config file: empty output")
	}
	if data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".agent_config_tmp_*")
	if err != nil {
		return fmt.Errorf("create temp config file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set temp config permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp config file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace config file: %w", err)
	}
	return nil
}

// Validate enforces gRPC-only mode and rejects legacy V2bX settings.
func (cfg *Config) Validate() error {
	if cfg.Panel.Token != "" || cfg.Panel.NodeID != 0 || cfg.Panel.NodeType != "" {
		return fmt.Errorf("legacy V2bX/UniProxy mode is no longer supported; migrate to gRPC with grpc.enabled=true, grpc.address, panel.host_token")
	}
	if !cfg.GRPC.Enabled || cfg.GRPC.Address == "" {
		return fmt.Errorf("gRPC transport is required; set grpc.enabled=true and grpc.address")
	}
	if cfg.Panel.HostToken == "" {
		return fmt.Errorf("panel.host_token is required for gRPC authentication (or provide panel.communication_key for first-boot registration)")
	}
	// Legacy grpc_server validation removed: core control now uses agent -> panel pull/report only.
	if _, err := protocol.NormalizeServiceAction(cfg.Protocol.ServiceAction, cfg.Protocol.AutoRestart); err != nil {
		return err
	}
	if err := cfg.validateUpdateConfig(); err != nil {
		return err
	}
	if cfg.Proxy.Enabled {
		if cfg.Proxy.PortRangeStart <= 0 || cfg.Proxy.PortRangeEnd <= 0 || cfg.Proxy.PortRangeEnd < cfg.Proxy.PortRangeStart {
			return fmt.Errorf("proxy port range is invalid")
		}
		if cfg.Proxy.MaxRetries <= 0 {
			return fmt.Errorf("proxy max_retries must be positive")
		}
		if cfg.Proxy.HealthTimeout <= 0 || cfg.Proxy.HealthInterval <= 0 || cfg.Proxy.DrainTimeout <= 0 {
			return fmt.Errorf("proxy timeouts must be positive")
		}
	}
	return nil
}

func (cfg *Config) validateUpdateConfig() error {
	if cfg.Update.HealthTimeout < 0 {
		return fmt.Errorf("update.health_timeout must be non-negative")
	}
	if cfg.Update.MaxCrashCount < 0 {
		return fmt.Errorf("update.max_crash_count must be non-negative")
	}
	if cfg.Update.JitterMin < 0 || cfg.Update.JitterMax < 0 {
		return fmt.Errorf("update jitter must be non-negative")
	}
	if cfg.Update.JitterMax > 0 && cfg.Update.JitterMax < cfg.Update.JitterMin {
		return fmt.Errorf("update.jitter_max must be greater than or equal to update.jitter_min")
	}
	if cfg.Update.MaxDownloadBytes < 0 {
		return fmt.Errorf("update.max_download_bytes must be non-negative")
	}
	if strings.TrimSpace(cfg.Update.ReleaseBaseURL) != "" {
		if _, err := updater.ParseBaseURL(cfg.Update.ReleaseBaseURL); err != nil {
			return fmt.Errorf("update.release_base_url: %w", err)
		}
	}
	return nil
}

// DetectPublicIP 尝试探测本机公网 IP，用于 agent 注册时上报 AdvertiseHost。
// 若失败返回空字符串（面板将回退到 pending-* 占位符）。
func DetectPublicIP() string {
	endpoints := []string{
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
		"https://icanhazip.com",
		"https://ipinfo.io/ip",
	}
	client := &http.Client{Timeout: 5 * time.Second}
	for _, ep := range endpoints {
		ip, err := fetchPublicIP(client, ep)
		if err == nil && ip != "" {
			return ip
		}
	}
	return ""
}

func fetchPublicIP(client *http.Client, endpoint string) (string, error) {
	resp, err := client.Get(endpoint)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("status %d from %s", resp.StatusCode, endpoint)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return "", err
	}
	ip := strings.TrimSpace(string(body))
	if ip == "" {
		return "", fmt.Errorf("empty ip from %s", endpoint)
	}
	return ip, nil
}
