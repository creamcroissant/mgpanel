package template

// UnifiedInbound 是与核心无关的入站模型，供转换器使用。
type UnifiedInbound struct {
	Tag       string            `json:"tag"`
	Protocol  string            `json:"protocol"` // vless, vmess, shadowsocks, trojan
	Listen    string            `json:"listen"`
	Port      int               `json:"port"`
	Transport *UnifiedTransport `json:"transport,omitempty"`
	TLS       *UnifiedTLS       `json:"tls,omitempty"`
	Users     []UnifiedUser     `json:"users,omitempty"`
	Options   map[string]any    `json:"options,omitempty"`
	Multiplex *UnifiedMultiplex `json:"multiplex,omitempty"`
	Sniffing  *UnifiedSniffing  `json:"sniffing,omitempty"`
}

// UnifiedTransport 描述跨核心的传输配置。
type UnifiedTransport struct {
	Type        string            `json:"type"` // tcp, ws, grpc, http, h2, quic, xhttp
	Path        string            `json:"path,omitempty"`
	Host        string            `json:"host,omitempty"`
	ServiceName string            `json:"service_name,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Mode        string            `json:"mode,omitempty"`
	XHTTP       *XHTTPConfig      `json:"xhttp,omitempty"`
}

// UnifiedTLS 描述跨核心的 TLS 配置。
type UnifiedTLS struct {
	Enabled    bool            `json:"enabled"`
	ServerName string          `json:"server_name,omitempty"`
	ALPN       []string        `json:"alpn,omitempty"`
	CertPath   string          `json:"cert_path,omitempty"`
	KeyPath    string          `json:"key_path,omitempty"`
	Reality    *UnifiedReality `json:"reality,omitempty"`
}

// UnifiedReality 描述跨核心的 Reality 配置。
type UnifiedReality struct {
	Enabled         bool     `json:"enabled"`
	PrivateKey      string   `json:"private_key,omitempty"`
	PublicKey       string   `json:"public_key,omitempty"`
	ShortIDs        []string `json:"short_ids,omitempty"`
	ServerNames     []string `json:"server_names,omitempty"` // 支持多 SNI 防探测
	HandshakeServer string   `json:"handshake_server,omitempty"`
	HandshakePort   int      `json:"handshake_port,omitempty"`
	Fingerprint     string   `json:"fingerprint,omitempty"`
}

// UnifiedUser 描述跨核心的用户配置。
type UnifiedUser struct {
	UUID     string `json:"uuid,omitempty"`
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
	Flow     string `json:"flow,omitempty"`
	Method   string `json:"method,omitempty"`
}


// UnifiedMultiplex 描述跨核心的多路复用配置。
type UnifiedMultiplex struct {
	Enabled   bool            `json:"enabled"`
	Protocol  string          `json:"protocol,omitempty"`
	MaxStreams int            `json:"max_streams,omitempty"`
	Padding   bool            `json:"padding,omitempty"`
	Brutal    *UnifiedBrutal  `json:"brutal,omitempty"`
}

// UnifiedBrutal 描述 TCP Brutal 拥塞控制。
type UnifiedBrutal struct {
	Enabled  bool `json:"enabled"`
	UpMbps   int  `json:"up_mbps,omitempty"`
	DownMbps int  `json:"down_mbps,omitempty"`
}

// UnifiedSniffing 描述跨核心的连接嗅探配置。
type UnifiedSniffing struct {
	Enabled        bool     `json:"enabled"`
	DestOverride   []string `json:"dest_override,omitempty"`
	MetadataOnly   bool     `json:"metadata_only,omitempty"`
	DomainsExcluded []string `json:"domains_excluded,omitempty"`
	RouteOnly      bool     `json:"route_only,omitempty"`
}
