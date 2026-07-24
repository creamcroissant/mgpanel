package template

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// XrayConverter 在统一入站与 Xray 配置 JSON 之间转换。
type XrayConverter struct{}

// TargetCore 返回目标核心标识。
func (c *XrayConverter) TargetCore() string {
	return "xray"
}

// stripJSONComments 移除 JSONC（带注释的 JSON）中的注释和末尾逗号。
// 支持 Xray 配置文件常见的 // 单行注释和 /* */ 多行注释。
func stripJSONComments(data []byte) []byte {
	s := string(data)

	// 移除单行注释 //...（但不移除字符串内的 //，简化处理）
	reLineComment := regexp.MustCompile(`(?m)^\s*//.*$|([^":])\s*//.*$`)
	s = reLineComment.ReplaceAllString(s, "$1")

	// 移除多行注释 /* ... */
	reBlockComment := regexp.MustCompile(`(?s)/\*.*?\*/`)
	s = reBlockComment.ReplaceAllString(s, "")

	// 移除末尾逗号 (,] 或 ,})
	reTrailingComma := regexp.MustCompile(`,\s*([}\]])`)
	s = reTrailingComma.ReplaceAllString(s, "$1")

	return []byte(s)
}

// FromUnified 将统一入站转换为 Xray 配置对象。
func (c *XrayConverter) FromUnified(inbounds []UnifiedInbound) ([]byte, error) {
	inboundsList := make([]map[string]any, 0, len(inbounds))
	for _, inbound := range inbounds {
		inboundsList = append(inboundsList, buildXrayInbound(inbound))
	}
	return json.Marshal(map[string]any{"inbounds": inboundsList})
}

// ToUnified 解析 Xray 配置并返回统一入站。
// 支持带注释和末尾逗号的 JSONC 格式。
func (c *XrayConverter) ToUnified(configJSON []byte) ([]UnifiedInbound, error) {
	// 预处理：移除注释和末尾逗号
	cleanJSON := stripJSONComments(configJSON)

	var config xrayConfig
	if err := json.Unmarshal(cleanJSON, &config); err == nil {
		if config.Inbounds != nil {
			return parseXrayInbounds(config.Inbounds), nil
		}
		return nil, nil
	}

	var inbounds []xrayInbound
	if err := json.Unmarshal(cleanJSON, &inbounds); err != nil {
		return nil, err
	}
	return parseXrayInbounds(inbounds), nil
}

type xrayConfig struct {
	Inbounds []xrayInbound `json:"inbounds"`
}

type xrayInbound struct {
	Protocol       string              `json:"protocol"`
	Tag            string              `json:"tag"`
	Listen         string              `json:"listen"`
	Port           int                 `json:"port"`
	Settings       json.RawMessage     `json:"settings"`
	StreamSettings *xrayStreamSettings `json:"streamSettings"`
	Sniffing       *xraySniffing       `json:"sniffing"`
}

type xrayStreamSettings struct {
	Network         string               `json:"network"`
	Security        string               `json:"security"`
	TLSSettings     *xrayTLSSettings     `json:"tlsSettings"`
	RealitySettings *xrayRealitySettings `json:"realitySettings"`
	WSSettings      *xrayWSSettings      `json:"wsSettings"`
	GRPCSettings    *xrayGRPCSettings    `json:"grpcSettings"`
	HTTPSettings    *xrayHTTPSettings    `json:"httpSettings"`
	XHTTPSettings   map[string]any       `json:"xhttpSettings"`
}

type xrayTLSSettings struct {
	ServerName   string               `json:"serverName"`
	ALPN         []string             `json:"alpn"`
	Certificates []xrayTLSCertificate `json:"certificates"`
}

type xrayTLSCertificate struct {
	CertificateFile string `json:"certificateFile"`
	KeyFile         string `json:"keyFile"`
}

type xraySniffing struct {
	Enabled        bool     `json:"enabled"`
	DestOverride   []string `json:"destOverride"`
	MetadataOnly   bool     `json:"metadataOnly"`
	DomainsExcluded []string `json:"domainsExcluded"`
	RouteOnly      bool     `json:"routeOnly"`
}

type xrayRealitySettings struct {
	PublicKey   string   `json:"publicKey"`
	PrivateKey  string   `json:"privateKey"`
	ShortIds    []string `json:"shortIds"`
	ServerNames []string `json:"serverNames"`
	Fingerprint string   `json:"fingerprint"`
	Dest        string   `json:"dest"`
}

type xrayWSSettings struct {
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
}

type xrayGRPCSettings struct {
	ServiceName string `json:"serviceName"`
}

type xrayHTTPSettings struct {
	Path string   `json:"path"`
	Host []string `json:"host"`
}

func buildXrayInbound(inbound UnifiedInbound) map[string]any {
	result := map[string]any{
		"protocol": inbound.Protocol,
		"tag":      inbound.Tag,
	}
	if inbound.Listen != "" {
		result["listen"] = inbound.Listen
	}
	if inbound.Port > 0 {
		result["port"] = inbound.Port
	}

	settings := buildXraySettings(inbound.Protocol, inbound.Users, inbound.Options, inbound.Transport)
	if settings != nil {
		result["settings"] = settings
	}

	streamSettings := buildXrayStreamSettings(inbound.Transport, inbound.TLS)
	if streamSettings != nil {
		result["streamSettings"] = streamSettings
	}

	if inbound.Sniffing != nil && inbound.Sniffing.Enabled {
		sniffing := map[string]any{
			"enabled": true,
		}
		if len(inbound.Sniffing.DestOverride) > 0 {
			sniffing["destOverride"] = inbound.Sniffing.DestOverride
		}
		if inbound.Sniffing.MetadataOnly {
			sniffing["metadataOnly"] = true
		}
		if len(inbound.Sniffing.DomainsExcluded) > 0 {
			sniffing["domainsExcluded"] = inbound.Sniffing.DomainsExcluded
		}
		if inbound.Sniffing.RouteOnly {
			sniffing["routeOnly"] = true
		}
		result["sniffing"] = sniffing
	}

	return result
}

func buildXraySettings(protocol string, users []UnifiedUser, options map[string]any, transport *UnifiedTransport) map[string]any {
	settings := map[string]any{}

	switch protocol {
	case "vless":
		settings["decryption"] = "none"
		if clients := buildXrayClients(protocol, users, isVLESSXHTTPTransport(transport)); len(clients) > 0 {
			settings["clients"] = clients
		}
	case "vmess":
		if clients := buildXrayClients(protocol, users, false); len(clients) > 0 {
			settings["clients"] = clients
		}
	case "shadowsocks":
		method := optionString(options, "method")
		if method == "" {
			method = pickUserMethod(users)
		}
		if method != "" {
			settings["method"] = method
		}
		if network := optionString(options, "network"); network != "" {
			settings["network"] = network
		}
		if clients := buildXrayClients(protocol, users, false); len(clients) > 0 {
			settings["clients"] = clients
		} else if password := optionString(options, "password"); password != "" {
			settings["password"] = password
		}
	case "trojan":
		if clients := buildXrayClients(protocol, users, false); len(clients) > 0 {
			settings["clients"] = clients
		}
	}

	if len(settings) == 0 {
		return nil
	}
	return settings
}

func buildXrayClients(protocol string, users []UnifiedUser, omitDefaultVLESSFlow bool) []map[string]any {
	clients := make([]map[string]any, 0, len(users))
	for _, user := range users {
		client := map[string]any{}
		switch protocol {
		case "vless":
			if user.UUID == "" {
				continue
			}
			client["id"] = user.UUID
			client["email"] = user.Email
			client["level"] = 0
			flow := user.Flow
			if flow == "" && !omitDefaultVLESSFlow {
				flow = "xtls-rprx-vision"
			}
			if flow != "" {
				client["flow"] = flow
			}
		case "vmess":
			if user.UUID == "" {
				continue
			}
			client["id"] = user.UUID
			client["email"] = user.Email
			client["level"] = 0
			client["alterId"] = 0
		case "shadowsocks":
			password := pickUserPassword(user)
			if password == "" {
				continue
			}
			client["password"] = password
			client["email"] = user.Email
			client["level"] = 0
		case "trojan":
			password := pickUserPassword(user)
			if password == "" {
				continue
			}
			client["password"] = password
			client["email"] = user.Email
			client["level"] = 0
		}
		clients = append(clients, client)
	}
	return clients
}

func isVLESSXHTTPTransport(transport *UnifiedTransport) bool {
	return transport != nil && IsXHTTPNetwork(transport.Type)
}

// buildXrayTransportStreamSettings 构建传输层 stream settings（不含 TLS）。
// 返回 settings map 和是否包含传输配置（transport 非 nil）。
func buildXrayTransportStreamSettings(transport *UnifiedTransport) (map[string]any, bool) {
	if transport == nil {
		return map[string]any{"network": "tcp"}, false
	}

	streamSettings := map[string]any{}
	network := NormalizeXHTTPNetwork(transport.Type)
	if network == "" {
		network = "tcp"
	}
	streamSettings["network"] = network

	switch network {
	case "ws":
		wsSettings := map[string]any{}
		if transport.Path != "" {
			wsSettings["path"] = transport.Path
		}
		if transport.Host != "" {
			wsSettings["headers"] = map[string]string{"Host": transport.Host}
		}
		if len(transport.Headers) > 0 {
			wsSettings["headers"] = transport.Headers
		}
		if len(wsSettings) > 0 {
			streamSettings["wsSettings"] = wsSettings
		}
	case "grpc":
		grpcSettings := map[string]any{}
		if transport.ServiceName != "" {
			grpcSettings["serviceName"] = transport.ServiceName
		}
		if len(grpcSettings) > 0 {
			streamSettings["grpcSettings"] = grpcSettings
		}
	case "http":
		httpSettings := map[string]any{}
		if transport.Path != "" {
			httpSettings["path"] = transport.Path
		}
		if transport.Host != "" {
			httpSettings["host"] = []string{transport.Host}
		}
		if len(httpSettings) > 0 {
			streamSettings["httpSettings"] = httpSettings
		}
	case XHTTPNetwork:
		xhttpSettings := BuildXHTTPSettingsMap(MergeXHTTPConfig(transport.XHTTP, transport.Host, transport.Path, transport.Mode, transport.Headers))
		if len(xhttpSettings) > 0 {
			streamSettings["xhttpSettings"] = xhttpSettings
		}
	case "tcp":
		// TCP is default, no additional settings needed
	}

	return streamSettings, true
}

func buildXrayStreamSettings(transport *UnifiedTransport, tls *UnifiedTLS) map[string]any {
	transportSS, hasTransport := buildXrayTransportStreamSettings(transport)
	streamSettings := transportSS
	hasSettings := hasTransport

	if tls != nil && tls.Enabled {
		hasSettings = true
		if tls.Reality != nil && tls.Reality.Enabled {
			streamSettings["security"] = "reality"
			realitySettings := map[string]any{}
			if tls.Reality.PrivateKey != "" {
				realitySettings["privateKey"] = tls.Reality.PrivateKey
			}
			if tls.Reality.PublicKey != "" {
				realitySettings["publicKey"] = tls.Reality.PublicKey
			}
			if len(tls.Reality.ShortIDs) > 0 {
				realitySettings["shortIds"] = tls.Reality.ShortIDs
			}
			// 支持多 SNI，直接使用数组；如果为空则回退到 HandshakeServer
			if len(tls.Reality.ServerNames) > 0 {
				realitySettings["serverNames"] = tls.Reality.ServerNames
			} else if tls.Reality.HandshakeServer != "" {
				realitySettings["serverNames"] = []string{tls.Reality.HandshakeServer}
			}
			if tls.Reality.Fingerprint != "" {
				realitySettings["fingerprint"] = tls.Reality.Fingerprint
			}
			if tls.Reality.HandshakeServer != "" {
				dest := tls.Reality.HandshakeServer
				if tls.Reality.HandshakePort > 0 {
					dest = dest + ":" + strconv.Itoa(tls.Reality.HandshakePort)
				}
				realitySettings["dest"] = dest
			}
			if len(realitySettings) > 0 {
				streamSettings["realitySettings"] = realitySettings
			}
		} else {
			streamSettings["security"] = "tls"
			tlsSettings := map[string]any{}
			if tls.ServerName != "" {
				tlsSettings["serverName"] = tls.ServerName
			}
			if len(tls.ALPN) > 0 {
				tlsSettings["alpn"] = tls.ALPN
			}
			if tls.CertPath != "" {
				tlsSettings["certificates"] = []map[string]any{{
					"certificateFile": tls.CertPath,
					"keyFile":         tls.KeyPath,
				}}
			}
			if len(tlsSettings) > 0 {
				streamSettings["tlsSettings"] = tlsSettings
			}
		}
	}

	if !hasSettings {
		return nil
	}
	return streamSettings
}

func parseXrayInbounds(inbounds []xrayInbound) []UnifiedInbound {
	result := make([]UnifiedInbound, 0, len(inbounds))
	for _, inbound := range inbounds {
		result = append(result, parseXrayInbound(inbound))
	}
	return result
}

func parseXrayInbound(inbound xrayInbound) UnifiedInbound {
	result := UnifiedInbound{
		Tag:      inbound.Tag,
		Protocol: inbound.Protocol,
		Listen:   inbound.Listen,
		Port:     inbound.Port,
	}

	if inbound.StreamSettings != nil {
		result.Transport = parseXrayTransport(inbound.StreamSettings)
		result.TLS = parseXrayTLS(inbound.StreamSettings)
	}

	if inbound.Sniffing != nil {
		result.Sniffing = &UnifiedSniffing{
			Enabled:         inbound.Sniffing.Enabled,
			DestOverride:    inbound.Sniffing.DestOverride,
			MetadataOnly:    inbound.Sniffing.MetadataOnly,
			DomainsExcluded: inbound.Sniffing.DomainsExcluded,
			RouteOnly:       inbound.Sniffing.RouteOnly,
		}
	}

	users, options := parseXraySettings(inbound.Protocol, inbound.Settings)
	if len(users) > 0 {
		result.Users = users
	}
	if len(options) > 0 {
		result.Options = options
	}

	return result
}

func parseXrayTransport(ss *xrayStreamSettings) *UnifiedTransport {
	if ss == nil {
		return nil
	}

	transport := &UnifiedTransport{
		Type: NormalizeXHTTPNetwork(ss.Network),
	}
	if transport.Type == "" {
		transport.Type = "tcp"
	}

	if ss.WSSettings != nil {
		transport.Path = ss.WSSettings.Path
		transport.Headers = ss.WSSettings.Headers
		if host, ok := ss.WSSettings.Headers["Host"]; ok && host != "" {
			transport.Host = host
		}
	}
	if ss.GRPCSettings != nil {
		transport.ServiceName = ss.GRPCSettings.ServiceName
	}
	if ss.HTTPSettings != nil {
		transport.Path = ss.HTTPSettings.Path
		if len(ss.HTTPSettings.Host) > 0 {
			transport.Host = ss.HTTPSettings.Host[0]
		}
	}
	if transport.Type == XHTTPNetwork && ss.XHTTPSettings != nil {
		xhttp := xhttpConfigFromMap(ss.XHTTPSettings)
		transport.XHTTP = &xhttp
		transport.Host = xhttp.Host
		transport.Path = xhttp.Path
		transport.Mode = xhttp.Mode
		transport.Headers = xhttp.Headers
	}

	return transport
}

func parseXrayTLS(ss *xrayStreamSettings) *UnifiedTLS {
	if ss == nil || ss.Security == "" || ss.Security == "none" {
		return nil
	}

	result := &UnifiedTLS{Enabled: true}

	if ss.TLSSettings != nil {
		result.ServerName = ss.TLSSettings.ServerName
		result.ALPN = ss.TLSSettings.ALPN
		if len(ss.TLSSettings.Certificates) > 0 {
			result.CertPath = ss.TLSSettings.Certificates[0].CertificateFile
			result.KeyPath = ss.TLSSettings.Certificates[0].KeyFile
		}
	}

	if ss.Security == "reality" && ss.RealitySettings != nil {
		reality := &UnifiedReality{
			Enabled:     true,
			PublicKey:   ss.RealitySettings.PublicKey,
			PrivateKey:  ss.RealitySettings.PrivateKey,
			ShortIDs:    ss.RealitySettings.ShortIds,
			Fingerprint: ss.RealitySettings.Fingerprint,
		}
		// 直接复制多 SNI 数组
		if len(ss.RealitySettings.ServerNames) > 0 {
			reality.ServerNames = ss.RealitySettings.ServerNames
		}
		if ss.RealitySettings.Dest != "" {
			host, port := splitHostPort(ss.RealitySettings.Dest)
			reality.HandshakeServer = host
			reality.HandshakePort = port
		}
		result.Reality = reality
	}

	return result
}

func parseXraySettings(protocol string, settings json.RawMessage) ([]UnifiedUser, map[string]any) {
	if len(settings) == 0 {
		return nil, nil
	}

	switch protocol {
	case "vless", "vmess":
		return parseXrayVMessSettings(settings)
	case "shadowsocks":
		return parseXrayShadowsocksSettings(settings)
	case "trojan":
		return parseXrayTrojanSettings(settings)
	default:
		return nil, nil
	}
}

func parseXrayVMessSettings(settings json.RawMessage) ([]UnifiedUser, map[string]any) {
	var payload struct {
		Clients []struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Flow  string `json:"flow"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(settings, &payload); err != nil {
		return nil, nil
	}

	users := make([]UnifiedUser, 0, len(payload.Clients))
	for _, client := range payload.Clients {
		users = append(users, UnifiedUser{
			UUID:  client.ID,
			Email: client.Email,
			Flow:  client.Flow,
		})
	}
	return users, nil
}

func parseXrayShadowsocksSettings(settings json.RawMessage) ([]UnifiedUser, map[string]any) {
	var payload struct {
		Method   string `json:"method"`
		Password string `json:"password"`
		Network  string `json:"network"`
		Clients  []struct {
			Password string `json:"password"`
			Email    string `json:"email"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(settings, &payload); err != nil {
		return nil, nil
	}

	options := map[string]any{}
	if payload.Method != "" {
		options["method"] = payload.Method
	}
	if payload.Network != "" {
		options["network"] = payload.Network
	}

	users := make([]UnifiedUser, 0, len(payload.Clients))
	for _, client := range payload.Clients {
		user := UnifiedUser{
			Password: client.Password,
			Email:    client.Email,
		}
		if payload.Method != "" {
			user.Method = payload.Method
		}
		users = append(users, user)
	}
	if len(users) == 0 && payload.Password != "" {
		users = append(users, UnifiedUser{Password: payload.Password})
	}

	if len(options) == 0 {
		options = nil
	}
	return users, options
}

func parseXrayTrojanSettings(settings json.RawMessage) ([]UnifiedUser, map[string]any) {
	var payload struct {
		Clients []struct {
			Password string `json:"password"`
			Email    string `json:"email"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(settings, &payload); err != nil {
		return nil, nil
	}

	users := make([]UnifiedUser, 0, len(payload.Clients))
	for _, client := range payload.Clients {
		users = append(users, UnifiedUser{
			Password: client.Password,
			Email:    client.Email,
		})
	}
	return users, nil
}

func optionString(options map[string]any, key string) string {
	if options == nil {
		return ""
	}
	value, ok := options[key]
	if !ok {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return ""
	}
}

func pickUserMethod(users []UnifiedUser) string {
	for _, user := range users {
		if user.Method != "" {
			return user.Method
		}
	}
	return ""
}

func pickUserPassword(user UnifiedUser) string {
	if user.Password != "" {
		return user.Password
	}
	if user.UUID != "" {
		return user.UUID
	}
	return ""
}

func splitHostPort(value string) (string, int) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", 0
	}
	idx := strings.LastIndex(trimmed, ":")
	if idx < 0 {
		return trimmed, 0
	}
	host := trimmed[:idx]
	portText := trimmed[idx+1:]
	port, err := strconv.Atoi(portText)
	if err != nil {
		return trimmed, 0
	}
	return host, port
}
