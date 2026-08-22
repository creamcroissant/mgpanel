// Package subscribe provides parsing for client subscription configuration files.
package subscribe

// ClientConfig represents a single protocol's client configuration.
type ClientConfig struct {
	Name     string `json:"name"`     // Protocol name/tag
	Protocol string `json:"protocol"` // vless, vmess, hysteria2, tuic, ss, trojan, anytls
	Server   string `json:"server"`   // Server address
	Port     int    `json:"port"`     // Main port

	// Authentication
	UUID     string `json:"uuid,omitempty"`     // UUID for vless/vmess/tuic
	Password string `json:"password,omitempty"` // Password for hysteria2/trojan/ss/anytls

	// Transport
	Network          string            `json:"network,omitempty"`           // tcp, ws, grpc, http, quic, xhttp
	Path             string            `json:"path,omitempty"`              // WebSocket/HTTP path
	Host             string            `json:"host,omitempty"`              // HTTP/XHTTP host
	Mode             string            `json:"mode,omitempty"`              // XHTTP mode
	ServiceName      string            `json:"service_name,omitempty"`      // gRPC service name
	Headers          map[string]string `json:"headers,omitempty"`           // Transport headers
	Extra            map[string]any    `json:"extra,omitempty"`             // XHTTP extra settings
	XMux             map[string]any    `json:"xmux,omitempty"`              // XHTTP xmux settings
	DownloadSettings map[string]any    `json:"download_settings,omitempty"` // XHTTP downloadSettings

	// TLS
	TLS         bool   `json:"tls"`                   // TLS enabled
	SNI         string `json:"sni,omitempty"`         // Server Name Indication
	ALPN        string `json:"alpn,omitempty"`        // ALPN protocols
	Fingerprint string `json:"fingerprint,omitempty"` // Certificate fingerprint
	Insecure    bool   `json:"insecure,omitempty"`    // Skip certificate verification

	// Reality
	RealityEnabled   bool   `json:"reality_enabled,omitempty"`
	RealityPublicKey string `json:"reality_public_key,omitempty"` // Reality public key
	RealityShortID   string `json:"reality_short_id,omitempty"`   // Reality short ID

	// VLESS specific
	Flow string `json:"flow,omitempty"` // xtls-rprx-vision

	// Hysteria2 specific
	HopPorts    string `json:"hop_ports,omitempty"`    // Port hopping range (e.g., "50000-51000")
	HopInterval int    `json:"hop_interval,omitempty"` // Hop interval in seconds
	UpMbps      int    `json:"up_mbps,omitempty"`      // Upload bandwidth
	DownMbps    int    `json:"down_mbps,omitempty"`    // Download bandwidth

	// TUIC specific
	CongestionControl string `json:"congestion_control,omitempty"` // bbr, cubic, new_reno

	// Shadowsocks specific
	Cipher string `json:"cipher,omitempty"` // Encryption method

	// ShadowTLS specific
	ShadowTLSVersion  int    `json:"shadowtls_version,omitempty"`
	ShadowTLSPassword string `json:"shadowtls_password,omitempty"`

	// Multiplex
	MuxEnabled bool `json:"mux_enabled,omitempty"`
	MuxPadding bool `json:"mux_padding,omitempty"`

	// Raw configs for direct export (format -> content)
	RawConfigs map[string]string `json:"raw_configs,omitempty"`
}

// SubscribeData represents the parsed result of the subscribe directory.
type SubscribeData struct {
	Configs     []ClientConfig `json:"configs"`
	ContentHash string         `json:"content_hash"` // For change detection
}

// FormatType represents different subscription formats.
type FormatType string

const (
	FormatV2RayN       FormatType = "v2rayn"
	FormatClash        FormatType = "clash"
	FormatSingBoxPC    FormatType = "singbox-pc"
	FormatSingBoxPhone FormatType = "singbox-phone"
	FormatShadowrocket FormatType = "shadowrocket"
	FormatNeko         FormatType = "neko"
)

// RawSubscribeFile represents a raw subscription file content.
type RawSubscribeFile struct {
	Format  FormatType `json:"format"`
	Content string     `json:"content"`
}
