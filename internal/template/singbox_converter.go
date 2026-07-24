package template

import (
	"encoding/json"
)

// SingBoxConverter 在统一入站与 sing-box 配置 JSON 之间转换。
type SingBoxConverter struct{}

// TargetCore 返回目标核心标识。
func (c *SingBoxConverter) TargetCore() string {
	return "sing-box"
}

// FromUnified 将统一入站转换为 sing-box 配置对象。
func (c *SingBoxConverter) FromUnified(inbounds []UnifiedInbound) ([]byte, error) {
	inboundsList := make([]map[string]any, 0, len(inbounds))
	for _, inbound := range inbounds {
		inboundsList = append(inboundsList, buildSingBoxInbound(inbound))
	}
	return json.Marshal(map[string]any{"inbounds": inboundsList})
}

// ToUnified 解析 sing-box 配置并返回统一入站。
func (c *SingBoxConverter) ToUnified(configJSON []byte) ([]UnifiedInbound, error) {
	var inbounds []singBoxInbound
	if err := json.Unmarshal(configJSON, &inbounds); err == nil {
		return convertSingBoxInbounds(inbounds), nil
	}

	var config singBoxConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return nil, err
	}

	return convertSingBoxInbounds(config.Inbounds), nil
}

func convertSingBoxInbounds(inbounds []singBoxInbound) []UnifiedInbound {
	result := make([]UnifiedInbound, 0, len(inbounds))
	for _, inbound := range inbounds {
		result = append(result, parseSingBoxInbound(inbound))
	}
	return result
}

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
	Sniff      *singBoxSniff     `json:"sniff"`

	Method   string `json:"method"`
	Password string `json:"password"`
	Network  string `json:"network"`

	Version               int               `json:"version"`
	Detour                string            `json:"detour"`
	Handshake             *singBoxHandshake `json:"handshake"`
	CongestionControl     string            `json:"congestion_control"`
	ZeroRTTHandshake      bool              `json:"zero_rtt_handshake"`
	IgnoreClientBandwidth bool              `json:"ignore_client_bandwidth"`
}

type singBoxUser struct {
	UUID     string `json:"uuid"`
	Name     string `json:"name"`
	Password string `json:"password"`
	Flow     string `json:"flow"`
	Method   string `json:"method"`
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
	PublicKey  string            `json:"public_key"`
	ShortID    []string          `json:"short_id"`
	ServerName string            `json:"server_name"`
	Fingerprint string           `json:"fingerprint"`
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
	Host        []string          `json:"host"`
}

type singBoxMultiplex struct {
	Enabled   bool            `json:"enabled"`
	Protocol  string          `json:"protocol"`
	MaxStreams int            `json:"max_streams"`
	Padding   bool            `json:"padding"`
	Brutal    *singBoxBrutal `json:"brutal"`
}

type singBoxBrutal struct {
	Enabled  bool `json:"enabled"`
	UpMbps   int  `json:"up_mbps"`
	DownMbps int  `json:"down_mbps"`
}

type singBoxSniff struct {
	Enabled        bool     `json:"enabled"`
	DestOverride   []string `json:"dest_override"`
	MetadataOnly   bool     `json:"metadata_only"`
	DomainsExcluded []string `json:"domains_excluded"`
	RouteOnly      bool     `json:"route_only"`
}

func buildSingBoxInbound(inbound UnifiedInbound) map[string]any {
	result := map[string]any{
		"type": inbound.Protocol,
		"tag":  inbound.Tag,
	}
	if inbound.Listen != "" {
		result["listen"] = inbound.Listen
	}
	if inbound.Port > 0 {
		result["listen_port"] = inbound.Port
	}

	// 判断是否启用 Reality（用于 Flow 语义清洗）
	hasReality := inbound.TLS != nil &&
		inbound.TLS.Reality != nil &&
		inbound.TLS.Reality.Enabled

	if len(inbound.Users) > 0 {
		users := make([]map[string]any, 0, len(inbound.Users))
		for _, user := range inbound.Users {
			users = append(users, buildSingBoxUser(inbound.Protocol, user, hasReality))
		}
		result["users"] = users
	}

	if inbound.Transport != nil {
		transport := map[string]any{
			"type": inbound.Transport.Type,
		}
		if inbound.Transport.Path != "" {
			transport["path"] = inbound.Transport.Path
		}
		if inbound.Transport.ServiceName != "" {
			transport["service_name"] = inbound.Transport.ServiceName
		}
		if inbound.Transport.Host != "" {
			switch inbound.Transport.Type {
			case "ws":
				transport["headers"] = map[string]string{"Host": inbound.Transport.Host}
			case "http":
				transport["host"] = []string{inbound.Transport.Host}
			}
		}
		if len(inbound.Transport.Headers) > 0 {
			transport["headers"] = inbound.Transport.Headers
		}
		result["transport"] = transport
	}

	if inbound.TLS != nil && inbound.TLS.Enabled {
		tls := map[string]any{
			"enabled": true,
		}
		if inbound.TLS.ServerName != "" {
			tls["server_name"] = inbound.TLS.ServerName
		}
		if len(inbound.TLS.ALPN) > 0 {
			tls["alpn"] = inbound.TLS.ALPN
		}
		if inbound.TLS.Reality != nil && inbound.TLS.Reality.Enabled {
			reality := map[string]any{
				"enabled": true,
			}
			if inbound.TLS.Reality.PrivateKey != "" {
				reality["private_key"] = inbound.TLS.Reality.PrivateKey
			}
			if inbound.TLS.Reality.PublicKey != "" {
				reality["public_key"] = inbound.TLS.Reality.PublicKey
			}
			// sing-box 只接受单个 server_name，取数组首个
			if len(inbound.TLS.Reality.ServerNames) > 0 {
				reality["server_name"] = inbound.TLS.Reality.ServerNames[0]
			}
			if inbound.TLS.Reality.Fingerprint != "" {
				reality["fingerprint"] = inbound.TLS.Reality.Fingerprint
			}
			if len(inbound.TLS.Reality.ShortIDs) > 0 {
				reality["short_id"] = inbound.TLS.Reality.ShortIDs
			}
			if inbound.TLS.Reality.HandshakeServer != "" {
				handshake := map[string]any{
					"server": inbound.TLS.Reality.HandshakeServer,
				}
				if inbound.TLS.Reality.HandshakePort > 0 {
					handshake["server_port"] = inbound.TLS.Reality.HandshakePort
				}
				reality["handshake"] = handshake
			}
			tls["reality"] = reality
		}
		result["tls"] = tls
	}

	if len(inbound.Options) > 0 {
		if value, ok := inbound.Options["method"]; ok {
			result["method"] = value
		}
		if value, ok := inbound.Options["network"]; ok {
			result["network"] = value
		}
		if value, ok := inbound.Options["version"]; ok {
			result["version"] = value
		}
		if value, ok := inbound.Options["detour"]; ok {
			result["detour"] = value
		}
		if value, ok := inbound.Options["handshake_server"]; ok {
			result["handshake"] = map[string]any{"server": value}
		}
		if value, ok := inbound.Options["handshake_port"]; ok {
			if handshake, ok := result["handshake"].(map[string]any); ok {
				handshake["server_port"] = value
			}
		}
		if value, ok := inbound.Options["congestion_control"]; ok {
			result["congestion_control"] = value
		}
		if value, ok := inbound.Options["zero_rtt_handshake"]; ok {
			result["zero_rtt_handshake"] = value
		}
		if value, ok := inbound.Options["ignore_client_bandwidth"]; ok {
			result["ignore_client_bandwidth"] = value
		}
	}

	if inbound.Multiplex != nil && inbound.Multiplex.Enabled {
		mux := map[string]any{
			"enabled": true,
		}
		if inbound.Multiplex.Protocol != "" {
			mux["protocol"] = inbound.Multiplex.Protocol
		}
		if inbound.Multiplex.MaxStreams > 0 {
			mux["max_streams"] = inbound.Multiplex.MaxStreams
		}
		if inbound.Multiplex.Padding {
			mux["padding"] = true
		}
		if inbound.Multiplex.Brutal != nil && inbound.Multiplex.Brutal.Enabled {
			brutal := map[string]any{
				"enabled": true,
			}
			if inbound.Multiplex.Brutal.UpMbps > 0 {
				brutal["up_mbps"] = inbound.Multiplex.Brutal.UpMbps
			}
			if inbound.Multiplex.Brutal.DownMbps > 0 {
				brutal["down_mbps"] = inbound.Multiplex.Brutal.DownMbps
			}
			mux["brutal"] = brutal
		}
		result["multiplex"] = mux
	}

	if inbound.Sniffing != nil && inbound.Sniffing.Enabled {
		sniffing := map[string]any{
			"enabled": true,
		}
		if len(inbound.Sniffing.DestOverride) > 0 {
			sniffing["dest_override"] = inbound.Sniffing.DestOverride
		}
		if inbound.Sniffing.MetadataOnly {
			sniffing["metadata_only"] = true
		}
		if len(inbound.Sniffing.DomainsExcluded) > 0 {
			sniffing["domains_excluded"] = inbound.Sniffing.DomainsExcluded
		}
		if inbound.Sniffing.RouteOnly {
			sniffing["route_only"] = true
		}
		result["sniff"] = sniffing
	}

	return result
}

func buildSingBoxUser(protocol string, user UnifiedUser, hasReality bool) map[string]any {
	result := map[string]any{}
	if user.Email != "" {
		result["name"] = user.Email
	}
	if user.UUID != "" {
		result["uuid"] = user.UUID
	}
	if user.Password != "" {
		result["password"] = user.Password
	}
	if user.Method != "" {
		result["method"] = user.Method
	}

	switch protocol {
	case "shadowsocks":
		if user.Method == "" {
			result["method"] = "aes-256-gcm"
		}
	case "vless":
		// Flow 语义清洗：仅当启用 Reality 时才设置 flow 字段
		// Sing-box 1.9+ 对未知 flow 字段敏感，非 Reality 场景不应设置
		if hasReality {
			flow := user.Flow
			if flow == "" {
				flow = "xtls-rprx-vision"
			}
			result["flow"] = flow
		}
		// 未启用 Reality 时不设置 flow 字段
	}

	return result
}

func parseSingBoxInbound(inbound singBoxInbound) UnifiedInbound {
	result := UnifiedInbound{
		Tag:      inbound.Tag,
		Protocol: inbound.Type,
		Listen:   inbound.Listen,
		Port:     inbound.ListenPort,
	}

	if inbound.Transport != nil {
		result.Transport = &UnifiedTransport{
			Type:        inbound.Transport.Type,
			Path:        inbound.Transport.Path,
			ServiceName: inbound.Transport.ServiceName,
			Headers:     inbound.Transport.Headers,
		}
		if inbound.Transport.Host != nil && len(inbound.Transport.Host) > 0 {
			result.Transport.Host = inbound.Transport.Host[0]
		}
		if inbound.Transport.Headers != nil {
			result.Transport.Headers = inbound.Transport.Headers
		}
	}

	if inbound.TLS != nil {
		result.TLS = &UnifiedTLS{
			Enabled:    inbound.TLS.Enabled,
			ServerName: inbound.TLS.ServerName,
			ALPN:       inbound.TLS.ALPN,
		}
		if inbound.TLS.Reality != nil && inbound.TLS.Reality.Enabled {
			reality := &UnifiedReality{
				Enabled:     true,
				ShortIDs:    inbound.TLS.Reality.ShortID,
				PublicKey:   inbound.TLS.Reality.PublicKey,
				PrivateKey:  inbound.TLS.Reality.PrivateKey,
				Fingerprint: inbound.TLS.Reality.Fingerprint,
			}
			// sing-box 只支持单个 ServerName，转为数组
			if inbound.TLS.Reality.ServerName != "" {
				reality.ServerNames = []string{inbound.TLS.Reality.ServerName}
			}
			if inbound.TLS.Reality.Handshake != nil {
				reality.HandshakeServer = inbound.TLS.Reality.Handshake.Server
				reality.HandshakePort = inbound.TLS.Reality.Handshake.ServerPort
			}
			result.TLS.Reality = reality
		}
	}

	if len(inbound.Users) > 0 {
		users := make([]UnifiedUser, 0, len(inbound.Users))
		for _, user := range inbound.Users {
			users = append(users, UnifiedUser{
				UUID:     user.UUID,
				Email:    user.Name,
				Password: user.Password,
				Flow:     user.Flow,
				Method:   user.Method,
			})
		}
		result.Users = users
	}

	options := map[string]any{}
	if inbound.Method != "" {
		options["method"] = inbound.Method
	}
	if inbound.Password != "" {
		options["password"] = inbound.Password
	}
	if inbound.Network != "" {
		options["network"] = inbound.Network
	}
	if inbound.Version > 0 {
		options["version"] = inbound.Version
	}
	if inbound.Detour != "" {
		options["detour"] = inbound.Detour
	}
	if inbound.CongestionControl != "" {
		options["congestion_control"] = inbound.CongestionControl
	}
	if inbound.ZeroRTTHandshake {
		options["zero_rtt_handshake"] = true
	}
	if inbound.IgnoreClientBandwidth {
		options["ignore_client_bandwidth"] = true
	}
	if len(options) > 0 {
		result.Options = options
	}

	if inbound.Multiplex != nil && inbound.Multiplex.Enabled {
		m := &UnifiedMultiplex{
			Enabled: true,
			Protocol: inbound.Multiplex.Protocol,
			MaxStreams: inbound.Multiplex.MaxStreams,
			Padding: inbound.Multiplex.Padding,
		}
		if inbound.Multiplex.Brutal != nil {
			m.Brutal = &UnifiedBrutal{
				Enabled: inbound.Multiplex.Brutal.Enabled,
				UpMbps: inbound.Multiplex.Brutal.UpMbps,
				DownMbps: inbound.Multiplex.Brutal.DownMbps,
			}
		}
		result.Multiplex = m
	}

	if inbound.Sniff != nil {
		result.Sniffing = &UnifiedSniffing{
			Enabled: inbound.Sniff.Enabled,
			DestOverride: inbound.Sniff.DestOverride,
			MetadataOnly: inbound.Sniff.MetadataOnly,
			DomainsExcluded: inbound.Sniff.DomainsExcluded,
			RouteOnly: inbound.Sniff.RouteOnly,
		}
	}

	return result
}
