// Package template 提供配置模板渲染与校验。
package template

// TemplateContext 是传入模板的根数据结构。
type TemplateContext struct {
	// Inbounds 包含该 Agent 的全部入站配置
	Inbounds []InboundConfig `json:"inbounds"`

	// Outbounds 包含出站配置（direct, block, dns 等）
	Outbounds []OutboundConfig `json:"outbounds,omitempty"`

	// Users 包含该 Agent 服务器组的活跃用户
	Users []UserConfig `json:"users"`

	// Agent 包含 Agent 主机信息
	Agent AgentInfo `json:"agent"`

	// Server 包含全局服务配置
	Server ServerInfo `json:"server"`

	// DNS 包含 DNS 配置
	DNS *DNSConfig `json:"dns,omitempty"`

	// Route 包含路由规则
	Route *RouteConfig `json:"route,omitempty"`

	// Experimental 包含实验特性配置
	Experimental *ExperimentalConfig `json:"experimental,omitempty"`
}

// InboundConfig 表示单个入站监听配置。
type InboundConfig struct {
	Type       string `json:"type"`        // vless, vmess, shadowsocks, trojan, hysteria2, tuic
	Tag        string `json:"tag"`         // Inbound 标签
	Listen     string `json:"listen"`      // 监听地址
	ListenPort int    `json:"listen_port"` // 监听端口

	// Users 为该入站注入的用户（由模板引擎生成）
	Users []InboundUser `json:"users,omitempty"`

	// Transport 传输层配置
	Transport *TransportConfig `json:"transport,omitempty"`

	// TLS TLS 配置
	TLS *TLSConfig `json:"tls,omitempty"`

	// Multiplex 复用配置
	Multiplex *MultiplexConfig `json:"multiplex,omitempty"`

	// Options 协议相关选项
	Options map[string]interface{} `json:"options,omitempty"`

	// RequiredCapabilities 为该入站所需能力（不序列化，仅用于过滤）
	RequiredCapabilities []string `json:"-"`
}

// InboundUser 表示入站配置中的用户。
type InboundUser struct {
	UUID     string `json:"uuid,omitempty"`
	Name     string `json:"name,omitempty"`     // 通常为邮箱
	Password string `json:"password,omitempty"` // 用于 SS/Trojan/Hysteria2
	Flow     string `json:"flow,omitempty"`     // 用于 VLESS（xtls-rprx-vision）
	Method   string `json:"method,omitempty"`   // 用于 Shadowsocks 加密方式
}

// OutboundConfig 表示出站连接配置。
type OutboundConfig struct {
	Type string `json:"type"` // direct, block, dns, selector
	Tag  string `json:"tag"`
}

// UserConfig 表示 Panel 数据库中的用户。
type UserConfig struct {
	ID          int64  `json:"id"`
	UUID        string `json:"uuid"`
	Email       string `json:"email"`
	SpeedLimit  int64  `json:"speed_limit,omitempty"`
	DeviceLimit int64  `json:"device_limit,omitempty"`
	Enabled     bool   `json:"enabled"`
}

// AgentInfo 包含 Agent 主机元信息。
type AgentInfo struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Host         string   `json:"host"`
	CoreType     string   `json:"core_type"`    // sing-box, xray
	CoreVersion  string   `json:"core_version"` // 例如: "1.10.0"
	Capabilities []string `json:"capabilities"` // 例如: ["reality", "multiplex", "brutal"]
	BuildTags    []string `json:"build_tags"`   // 例如: ["with_v2ray_api"]
}

// ServerInfo 包含全局服务配置。
type ServerInfo struct {
	LogLevel     string `json:"log_level"`     // debug, info, warn, error
	ListenAddr   string `json:"listen_addr"`   // 默认监听地址
	DNSServer    string `json:"dns_server"`    // 默认 DNS 服务器
	StatsEnabled bool   `json:"stats_enabled"` // 启用 v2ray_api 统计
}

// TransportConfig 表示传输层配置。
type TransportConfig struct {
	Type        string            `json:"type"`                   // tcp, ws, grpc, http, quic, xhttp
	Path        string            `json:"path,omitempty"`         // WebSocket/HTTP 路径
	Host        string            `json:"host,omitempty"`         // HTTP Host 头
	ServiceName string            `json:"service_name,omitempty"` // gRPC 服务名
	Headers     map[string]string `json:"headers,omitempty"`
	Mode        string            `json:"mode,omitempty"`
	XHTTP       *XHTTPConfig      `json:"xhttp,omitempty"`
}

// XHTTPConfig 表示 Xray xhttp 传输配置。
type XHTTPConfig struct {
	Host                string            `json:"host,omitempty"`
	Path                string            `json:"path,omitempty"`
	Mode                string            `json:"mode,omitempty"`
	Headers             map[string]string `json:"headers,omitempty"`
	Extra               map[string]any    `json:"extra,omitempty"`
	XMux                map[string]any    `json:"xmux,omitempty"`
	DownloadSettings    map[string]any    `json:"downloadSettings,omitempty"`
	NoGRPCHeader        *bool             `json:"noGRPCHeader,omitempty"`
	NoSSEHeader         *bool             `json:"noSSEHeader,omitempty"`
	SCMaxBufferedPosts  *int              `json:"scMaxBufferedPosts,omitempty"`
	SCMaxEachPostBytes  *int              `json:"scMaxEachPostBytes,omitempty"`
	SCMinPostsInterval  *int              `json:"scMinPostsIntervalMs,omitempty"`
	SCStreamUpServerSec *int              `json:"scStreamUpServerSecs,omitempty"`
}

// TLSConfig 表示 TLS 配置。
type TLSConfig struct {
	Enabled     bool           `json:"enabled"`
	ServerName  string         `json:"server_name,omitempty"`
	ALPN        []string       `json:"alpn,omitempty"`
	Certificate string         `json:"certificate,omitempty"` // 证书路径
	Key         string         `json:"key,omitempty"`         // 密钥路径
	Reality     *RealityConfig `json:"reality,omitempty"`
}

// RealityConfig 表示 XTLS Reality 配置。
type RealityConfig struct {
	Enabled    bool             `json:"enabled"`
	PrivateKey string           `json:"private_key,omitempty"` // 服务端私钥
	PublicKey  string           `json:"public_key,omitempty"`  // 用于客户端配置生成
	ShortIDs   []string         `json:"short_id,omitempty"`
	Handshake  *HandshakeConfig `json:"handshake,omitempty"`
}

// HandshakeConfig 表示 Reality 握手配置。
type HandshakeConfig struct {
	Server     string `json:"server"`
	ServerPort int    `json:"server_port"`
}

// MultiplexConfig 表示复用配置。
type MultiplexConfig struct {
	Enabled bool          `json:"enabled"`
	Padding bool          `json:"padding,omitempty"`
	Brutal  *BrutalConfig `json:"brutal,omitempty"`
}

// BrutalConfig 表示 TCP Brutal 配置。
type BrutalConfig struct {
	Enabled  bool `json:"enabled"`
	UpMbps   int  `json:"up_mbps,omitempty"`
	DownMbps int  `json:"down_mbps,omitempty"`
}

// DNSConfig 表示 DNS 配置。
type DNSConfig struct {
	Servers []DNSServer `json:"servers,omitempty"`
	Rules   []DNSRule   `json:"rules,omitempty"`
}

// DNSServer 表示 DNS 服务器。
type DNSServer struct {
	Tag     string `json:"tag"`
	Address string `json:"address"`
	Detour  string `json:"detour,omitempty"`
}

// DNSRule 表示 DNS 规则。
type DNSRule struct {
	Outbound string   `json:"outbound,omitempty"`
	Server   string   `json:"server,omitempty"`
	Domain   []string `json:"domain,omitempty"`
}

// RouteConfig 表示路由规则配置。
type RouteConfig struct {
	Rules []RouteRule `json:"rules,omitempty"`
	Final string      `json:"final,omitempty"`
}

// RouteRule 表示路由规则。
type RouteRule struct {
	Inbound  []string `json:"inbound,omitempty"`
	Outbound string   `json:"outbound,omitempty"`
}

// ExperimentalConfig 表示实验特性配置。
type ExperimentalConfig struct {
	V2RayAPI *V2RayAPIConfig `json:"v2ray_api,omitempty"`
}

// V2RayAPIConfig 表示统计采集配置。
type V2RayAPIConfig struct {
	Listen string       `json:"listen"`
	Stats  *StatsConfig `json:"stats,omitempty"`
}

// StatsConfig 表示 stats 模块配置。
type StatsConfig struct {
	Enabled   bool     `json:"enabled"`
	Inbounds  []string `json:"inbounds,omitempty"`
	Outbounds []string `json:"outbounds,omitempty"`
	Users     []string `json:"users,omitempty"`
}

// Capability 表示可能受支持的能力。
type Capability string

const (
	CapReality   Capability = "reality"
	CapMultiplex Capability = "multiplex"
	CapBrutal    Capability = "brutal"
	CapECH       Capability = "ech"
	CapUTLS      Capability = "utls"
	CapQUIC      Capability = "quic"
	CapV2RayAPI  Capability = "v2ray_api"
	CapWireguard Capability = "wireguard"
	CapTUN       Capability = "tun"
	CapHTTP3     Capability = "http3"
	CapDHCP      Capability = "dhcp"
	CapGeoIP     Capability = "geoip"
	CapGeoSite   Capability = "geosite"

	// Xray 专属能力
	CapXTLS       Capability = "xtls"       // XTLS 流控
	CapSplitHTTP  Capability = "splithttp"  // SplitHTTP 传输
	CapMeek       Capability = "meek"       // Meek 传输
	CapMKCP       Capability = "mkcp"       // mKCP 传输
	CapDomainSock Capability = "domainsock" // Unix domain socket
	CapStats      Capability = "stats"      // Xray stats 模块
)

// SingBoxVersionRequirements 记录能力所需的最低版本。
var SingBoxVersionRequirements = map[Capability]string{
	CapReality:   "1.3.0",
	CapMultiplex: "1.3.0",
	CapBrutal:    "1.7.0",
	CapECH:       "1.8.0",
	CapV2RayAPI:  "1.0.0", // 需要 build tag
	CapQUIC:      "1.0.0",
	CapHTTP3:     "1.8.0",
}

// XrayVersionRequirements 记录能力所需的最低 Xray 版本。
// Xray-core 版本: https://github.com/XTLS/Xray-core/releases
var XrayVersionRequirements = map[Capability]string{
	CapReality:    "1.8.0",  // Reality 起始于 v1.8.0
	CapXTLS:       "1.0.0",  // XTLS 自 v1.0.0 可用
	CapStats:      "1.0.0",  // Stats 模块始终可用
	CapV2RayAPI:   "1.0.0",  // API 在 Xray 中始终可用
	CapSplitHTTP:  "1.8.11", // SplitHTTP 起始于 v1.8.11
	CapMeek:       "1.6.0",  // Meek 传输可用
	CapMKCP:       "1.0.0",  // mKCP 始终可用
	CapQUIC:       "1.3.0",  // QUIC 传输
	CapMultiplex:  "1.8.0",  // Mux.Cool 多路复用
	CapGeoIP:      "1.0.0",  // GeoIP 始终可用
	CapGeoSite:    "1.0.0",  // GeoSite 始终可用
	CapDomainSock: "1.0.0",  // Unix domain socket
	CapWireguard:  "1.8.0",  // WireGuard 出站
}

// AgentCapabilities 表示 Agent 支持的能力。
type AgentCapabilities struct {
	CoreType     string       `json:"core_type"`    // sing-box, xray
	CoreVersion  string       `json:"core_version"` // 例如: "1.10.0"
	BuildTags    []string     `json:"build_tags"`   // 例如: ["with_v2ray_api", "with_quic"]
	Capabilities []Capability `json:"capabilities"` // 由版本号与 build tags 推导
}

// SupportsCapability 判断 Agent 是否支持指定能力。
func (a *AgentCapabilities) SupportsCapability(cap Capability) bool {
	for _, c := range a.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// HasBuildTag 判断 Agent 是否包含指定 build tag。
func (a *AgentCapabilities) HasBuildTag(tag string) bool {
	for _, t := range a.BuildTags {
		if t == tag {
			return true
		}
	}
	return false
}
