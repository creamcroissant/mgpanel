// Package parser 提供不同代理核心的配置解析能力。
package parser

// ProtocolDetails 描述解析后的协议配置详情。
type ProtocolDetails struct {
	// 基础信息
	Protocol string `json:"protocol"` // vless, vmess, shadowsocks, trojan, hysteria, hysteria2, tuic, shadowtls, anytls
	Tag      string `json:"tag"`      // Inbound 标识
	Listen   string `json:"listen"`   // 监听地址
	Port     int    `json:"port"`     // 监听端口

	// 传输层
	Transport *TransportConfig `json:"transport,omitempty"`

	// TLS 配置
	TLS *TLSConfig `json:"tls,omitempty"`

	// 复用配置
	Multiplex *MultiplexConfig `json:"multiplex,omitempty"`

	// 用户信息
	Users []UserInfo `json:"users,omitempty"`

	// 协议特有选项（congestion_control, version, detour 等）
	Options map[string]any `json:"options,omitempty"`

	// 元数据
	SourceFile string `json:"source_file"` // 配置文件名
	CoreType   string `json:"core_type"`   // sing-box 或 xray
}

// TransportConfig 描述传输层设置。
type TransportConfig struct {
	Type             string            `json:"type"`                   // tcp, ws, grpc, http, quic, h2, xhttp
	Path             string            `json:"path,omitempty"`         // WebSocket/HTTP 路径
	Host             string            `json:"host,omitempty"`         // HTTP Host 头
	ServiceName      string            `json:"service_name,omitempty"` // gRPC 服务名
	Headers          map[string]string `json:"headers,omitempty"`
	Mode             string            `json:"mode,omitempty"`
	Extra            map[string]any    `json:"extra,omitempty"`
	XMux             map[string]any    `json:"xmux,omitempty"`
	DownloadSettings map[string]any    `json:"download_settings,omitempty"`
	XHTTPSettings    map[string]any    `json:"xhttp_settings,omitempty"`
}

// TLSConfig 描述 TLS 设置。
type TLSConfig struct {
	Enabled    bool     `json:"enabled"`
	ServerName string   `json:"server_name,omitempty"`
	ALPN       []string `json:"alpn,omitempty"`

	// Reality 相关
	Reality *RealityConfig `json:"reality,omitempty"`
}

// RealityConfig 描述 XTLS Reality 设置。
type RealityConfig struct {
	Enabled       bool     `json:"enabled"`
	ShortIDs      []string `json:"short_ids,omitempty"`
	ServerName    string   `json:"server_name,omitempty"`
	Fingerprint   string   `json:"fingerprint,omitempty"`
	HandshakeAddr string   `json:"handshake_addr,omitempty"` // 握手目标服务器
	HandshakePort int      `json:"handshake_port,omitempty"` // 握手目标端口
	PrivateKey    string   `json:"private_key,omitempty"`    // 服务端私钥
	PublicKey     string   `json:"public_key,omitempty"`     // 客户端公钥（由私钥推导）
}

// MultiplexConfig 描述复用（mux）设置。
type MultiplexConfig struct {
	Enabled bool          `json:"enabled"`
	Padding bool          `json:"padding,omitempty"`
	Brutal  *BrutalConfig `json:"brutal,omitempty"`
}

// BrutalConfig 描述 BBR Brutal 设置。
type BrutalConfig struct {
	Enabled  bool `json:"enabled"`
	UpMbps   int  `json:"up_mbps,omitempty"`
	DownMbps int  `json:"down_mbps,omitempty"`
}

// UserInfo 描述用户配置。
type UserInfo struct {
	UUID   string `json:"uuid,omitempty"`
	Flow   string `json:"flow,omitempty"` // xtls-rprx-vision 等
	Email  string `json:"email,omitempty"`
	Method string `json:"method,omitempty"` // Shadowsocks 加密方式
}

// ConfigFileWithDetails 描述包含解析详情的配置文件。
type ConfigFileWithDetails struct {
	Filename    string            `json:"filename"`
	ContentHash string            `json:"content_hash"`
	Protocols   []ProtocolDetails `json:"protocols"`
}
