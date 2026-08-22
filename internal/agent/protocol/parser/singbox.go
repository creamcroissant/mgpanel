package parser

import (
	"encoding/base64"
	"encoding/json"

	"golang.org/x/crypto/curve25519"
)

// SingBoxParser 解析 sing-box 配置文件。
type SingBoxParser struct{}

// NewSingBoxParser 创建 sing-box 解析器。
func NewSingBoxParser() *SingBoxParser {
	return &SingBoxParser{}
}

// Name 返回解析器标识。
func (p *SingBoxParser) Name() string {
	return "sing-box"
}

// CanParse 判断该解析器是否能处理该内容。
func (p *SingBoxParser) CanParse(content []byte) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(content, &raw); err != nil {
		return false
	}

	inboundsRaw, hasInbounds := raw["inbounds"]
	if !hasInbounds {
		return false
	}

	var inbounds []map[string]json.RawMessage
	if err := json.Unmarshal(inboundsRaw, &inbounds); err != nil {
		return false
	}
	for _, inbound := range inbounds {
		if _, hasType := inbound["type"]; hasType {
			return true
		}
	}
	return false
}

// Parse 解析 sing-box 配置并提取协议详情。
func (p *SingBoxParser) Parse(filename string, content []byte) ([]ProtocolDetails, error) {
	var config singBoxConfig
	if err := json.Unmarshal(content, &config); err != nil {
		return nil, err
	}

	var results []ProtocolDetails
	for _, inbound := range config.Inbounds {
		details := ProtocolDetails{
			Protocol:   inbound.Type,
			Tag:        inbound.Tag,
			Listen:     inbound.Listen,
			Port:       inbound.ListenPort,
			SourceFile: filename,
			CoreType:   "sing-box",
		}

		// 解析 TLS
		if inbound.TLS != nil {
			details.TLS = p.parseTLS(inbound.TLS)
		}

		// 解析传输层
		details.Transport = p.parseTransport(&inbound)

		// 解析复用配置
		if inbound.Multiplex != nil {
			details.Multiplex = p.parseMultiplex(inbound.Multiplex)
		}

		// 解析用户（兼容不同认证方式）
		details.Users = p.parseUsers(&inbound)

		// 解析协议特有选项
		details.Options = p.parseProtocolOptions(&inbound)

		results = append(results, details)
	}

	return results, nil
}

// sing-box 配置解析所需的内部结构体

type singBoxConfig struct {
	Inbounds []singBoxInbound `json:"inbounds"`
}

type singBoxInbound struct {
	Type       string            `json:"type"`
	Tag        string            `json:"tag"`
	Listen     string            `json:"listen"`
	ListenPort int               `json:"listen_port"`
	Users      []singBoxUser     `json:"users"`
	TLS        *singBoxTLS       `json:"tls"`
	Transport  *singBoxTransport `json:"transport"`
	Multiplex  *singBoxMultiplex `json:"multiplex"`

	// Shadowsocks 专用字段（顶层）
	Method   string `json:"method"`
	Password string `json:"password"`
	Network  string `json:"network"` // tcp, udp, or empty for both

	// ShadowTLS 专用字段
	Version   int               `json:"version"`
	Detour    string            `json:"detour"`
	Handshake *singBoxHandshake `json:"handshake"`

	// TUIC/Hysteria2 专用字段
	CongestionControl     string `json:"congestion_control"`
	ZeroRTTHandshake      bool   `json:"zero_rtt_handshake"`
	IgnoreClientBandwidth bool   `json:"ignore_client_bandwidth"`
}

type singBoxUser struct {
	UUID     string `json:"uuid"`
	Name     string `json:"name"`
	Password string `json:"password"`
	Flow     string `json:"flow"`
}

type singBoxTLS struct {
	Enabled    bool            `json:"enabled"`
	ServerName string          `json:"server_name"`
	ALPN       []string        `json:"alpn"`
	Reality    *singBoxReality `json:"reality"`
}

type singBoxReality struct {
	Enabled    bool              `json:"enabled"`
	Handshake  *singBoxHandshake `json:"handshake"`
	PrivateKey string            `json:"private_key"`
	ShortID    []string          `json:"short_id"`
}

type singBoxHandshake struct {
	Server     string `json:"server"`
	ServerPort int    `json:"server_port"`
}

type singBoxTransport struct {
	Type        string            `json:"type"`
	Path        string            `json:"path"`
	Headers     map[string]string `json:"headers"`
	ServiceName string            `json:"service_name"`
}

type singBoxMultiplex struct {
	Enabled bool           `json:"enabled"`
	Padding bool           `json:"padding"`
	Brutal  *singBoxBrutal `json:"brutal"`
}

type singBoxBrutal struct {
	Enabled  bool `json:"enabled"`
	UpMbps   int  `json:"up_mbps"`
	DownMbps int  `json:"down_mbps"`
}

func (p *SingBoxParser) parseTLS(tls *singBoxTLS) *TLSConfig {
	if tls == nil {
		return nil
	}

	config := &TLSConfig{
		Enabled:    tls.Enabled,
		ServerName: tls.ServerName,
		ALPN:       tls.ALPN,
	}

	if tls.Reality != nil && tls.Reality.Enabled {
		config.Reality = &RealityConfig{
			Enabled:    true,
			ShortIDs:   tls.Reality.ShortID,
			PrivateKey: tls.Reality.PrivateKey,
			PublicKey:  deriveX25519PublicKey(tls.Reality.PrivateKey),
		}
		if tls.Reality.Handshake != nil {
			config.Reality.ServerName = tls.Reality.Handshake.Server
			config.Reality.HandshakeAddr = tls.Reality.Handshake.Server
			config.Reality.HandshakePort = tls.Reality.Handshake.ServerPort
		}
	}

	return config
}

func (p *SingBoxParser) parseTransport(inbound *singBoxInbound) *TransportConfig {
	// 仅在配置文件显式设置时才返回传输配置
	// 不推断默认值，保持与实际配置一致
	if inbound.Transport == nil {
		return nil
	}

	return &TransportConfig{
		Type:        inbound.Transport.Type,
		Path:        inbound.Transport.Path,
		ServiceName: inbound.Transport.ServiceName,
		Headers:     inbound.Transport.Headers,
	}
}

func (p *SingBoxParser) parseUsers(inbound *singBoxInbound) []UserInfo {
	var result []UserInfo

	// 处理顶层密码/方法（Shadowsocks 无 users 数组）
	if inbound.Method != "" && inbound.Password != "" && len(inbound.Users) == 0 {
		result = append(result, UserInfo{
			Method: inbound.Method,
			// 注意：出于安全原因不保存密码
		})
		return result
	}

	// 处理 users 数组
	for _, u := range inbound.Users {
		user := UserInfo{
			UUID:  u.UUID,
			Flow:  u.Flow,
			Email: u.Name,
		}

		// 部分协议使用 password 代替 UUID（hysteria2, tuic, trojan, shadowtls, anytls）
		// 使用 password 作为标识，但不暴露实际值
		if u.UUID == "" && u.Password != "" {
			// 对密码认证的协议，仅标记存在用户并隐藏密码
			user.UUID = "(password-auth)"
		}

		result = append(result, user)
	}

	return result
}

func (p *SingBoxParser) parseProtocolOptions(inbound *singBoxInbound) map[string]any {
	opts := make(map[string]any)

	switch inbound.Type {
	case "shadowsocks":
		if inbound.Method != "" {
			opts["method"] = inbound.Method
		}
		if inbound.Network != "" {
			opts["network"] = inbound.Network
		}

	case "shadowtls":
		if inbound.Version > 0 {
			opts["version"] = inbound.Version
		}
		if inbound.Detour != "" {
			opts["detour"] = inbound.Detour
		}
		if inbound.Handshake != nil {
			opts["handshake_server"] = inbound.Handshake.Server
			opts["handshake_port"] = inbound.Handshake.ServerPort
		}

	case "tuic":
		if inbound.CongestionControl != "" {
			opts["congestion_control"] = inbound.CongestionControl
		}
		if inbound.ZeroRTTHandshake {
			opts["zero_rtt_handshake"] = true
		}

	case "hysteria2":
		if inbound.IgnoreClientBandwidth {
			opts["ignore_client_bandwidth"] = true
		}
	}

	if len(opts) == 0 {
		return nil
	}
	return opts
}

func (p *SingBoxParser) parseMultiplex(mux *singBoxMultiplex) *MultiplexConfig {
	if mux == nil {
		return nil
	}

	config := &MultiplexConfig{
		Enabled: mux.Enabled,
		Padding: mux.Padding,
	}

	if mux.Brutal != nil {
		config.Brutal = &BrutalConfig{
			Enabled:  mux.Brutal.Enabled,
			UpMbps:   mux.Brutal.UpMbps,
			DownMbps: mux.Brutal.DownMbps,
		}
	}

	return config
}


// deriveX25519PublicKey 从 sing-box Reality 私钥推导 X25519 公钥。
// 私钥是 base64 URL-safe 编码的 32 字节。
func deriveX25519PublicKey(privateKey string) string {
	if privateKey == "" {
		return ""
	}
	priv, err := base64.RawURLEncoding.DecodeString(privateKey)
	if err != nil || len(priv) != 32 {
		return ""
	}
	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(pub)
}
