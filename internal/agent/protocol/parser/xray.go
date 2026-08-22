package parser

import (
	"encoding/json"
	"net"
	"strconv"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/template"
)

// XrayParser 解析 xray-core 配置文件。
type XrayParser struct{}

// NewXrayParser 创建 xray 解析器。
func NewXrayParser() *XrayParser {
	return &XrayParser{}
}

// Name 返回解析器标识。
func (p *XrayParser) Name() string {
	return "xray"
}

// CanParse 判断该解析器是否能处理该内容。
func (p *XrayParser) CanParse(content []byte) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(content, &raw); err != nil {
		return false
	}

	// xray 使用 "inbounds" 与 "protocol"，并可能包含 "streamSettings"
	inboundsRaw, hasInbounds := raw["inbounds"]
	if !hasInbounds {
		return false
	}

	// 判断是否为 xray 格式（inbounds 中存在 protocol/streamSettings）
	var inbounds []map[string]json.RawMessage
	if err := json.Unmarshal(inboundsRaw, &inbounds); err != nil {
		return false
	}

	for _, inbound := range inbounds {
		_, hasProtocol := inbound["protocol"]
		_, hasStreamSettings := inbound["streamSettings"]
		if hasProtocol || hasStreamSettings {
			return true
		}
	}

	return false
}

// Parse 解析 xray 配置并提取协议详情。
func (p *XrayParser) Parse(filename string, content []byte) ([]ProtocolDetails, error) {
	var config xrayConfig
	if err := json.Unmarshal(content, &config); err != nil {
		return nil, err
	}

	var results []ProtocolDetails
	for _, inbound := range config.Inbounds {
		details := ProtocolDetails{
			Protocol:   inbound.Protocol,
			Tag:        inbound.Tag,
			Listen:     inbound.Listen,
			Port:       inbound.Port,
			SourceFile: filename,
			CoreType:   "xray",
		}

		// 解析传输与 TLS 配置
		if inbound.StreamSettings != nil {
			details.Transport = p.parseTransport(inbound.StreamSettings)
			details.TLS = p.parseTLS(inbound.StreamSettings)
		} else {
			details.Transport = &TransportConfig{Type: "tcp"}
		}

		// 解析协议相关设置
		details.Users = p.parseSettings(inbound.Protocol, inbound.Settings)

		results = append(results, details)
	}

	return results, nil
}

// xray 配置解析所需的内部结构体

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
}

type xrayStreamSettings struct {
	Network         string               `json:"network"`
	Security        string               `json:"security"`
	TLSSettings     *xrayTLSSettings     `json:"tlsSettings"`
	RealitySettings *xrayRealitySettings `json:"realitySettings"`
	WSSettings      *xrayWSSettings      `json:"wsSettings"`
	GRPCSettings    *xrayGRPCSettings    `json:"grpcSettings"`
	TCPSettings     *xrayTCPSettings     `json:"tcpSettings"`
	HTTPSettings    *xrayHTTPSettings    `json:"httpSettings"`
	XHTTPSettings   map[string]any       `json:"xhttpSettings"`
}

type xrayTLSSettings struct {
	ServerName string   `json:"serverName"`
	ALPN       []string `json:"alpn"`
}

type xrayRealitySettings struct {
	PublicKey   string   `json:"publicKey"`
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

type xrayTCPSettings struct {
	Header *xrayTCPHeader `json:"header"`
}

type xrayTCPHeader struct {
	Type string `json:"type"`
}

type xrayHTTPSettings struct {
	Path string   `json:"path"`
	Host []string `json:"host"`
}

func (p *XrayParser) parseTransport(ss *xrayStreamSettings) *TransportConfig {
	network := template.NormalizeXHTTPNetwork(ss.Network)
	if network == "" {
		network = "tcp"
	}

	config := &TransportConfig{
		Type: network,
	}

	if ss.WSSettings != nil {
		config.Path = ss.WSSettings.Path
		config.Headers = ss.WSSettings.Headers
	}

	if ss.GRPCSettings != nil {
		config.ServiceName = ss.GRPCSettings.ServiceName
	}

	if ss.HTTPSettings != nil {
		config.Path = ss.HTTPSettings.Path
		if len(ss.HTTPSettings.Host) > 0 {
			config.Host = ss.HTTPSettings.Host[0]
		}
	}

	if config.Type == "xhttp" && len(ss.XHTTPSettings) > 0 {
		config.XHTTPSettings = cloneAnyMap(ss.XHTTPSettings)
		config.Host = stringFromAny(ss.XHTTPSettings["host"])
		config.Path = stringFromAny(ss.XHTTPSettings["path"])
		config.Mode = template.NormalizeXHTTPMode(stringFromAny(ss.XHTTPSettings["mode"]))
		if headers := stringMapFromAny(ss.XHTTPSettings["headers"]); len(headers) > 0 {
			config.Headers = headers
		}
		if extra := anyMapFromAny(ss.XHTTPSettings["extra"]); len(extra) > 0 {
			config.Extra = extra
		}
		if xmux := anyMapFromAny(ss.XHTTPSettings["xmux"]); len(xmux) > 0 {
			config.XMux = xmux
		}
		if downloadSettings := anyMapFromAny(ss.XHTTPSettings["downloadSettings"]); len(downloadSettings) > 0 {
			config.DownloadSettings = downloadSettings
		}
	}

	return config
}

func stringFromAny(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func stringMapFromAny(value any) map[string]string {
	switch typed := value.(type) {
	case map[string]string:
		result := make(map[string]string, len(typed))
		for key, item := range typed {
			if strings.TrimSpace(key) != "" {
				result[key] = item
			}
		}
		return result
	case map[string]any:
		result := make(map[string]string, len(typed))
		for key, item := range typed {
			if text, ok := item.(string); ok && strings.TrimSpace(key) != "" {
				result[key] = text
			}
		}
		return result
	default:
		return nil
	}
}

func anyMapFromAny(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneAnyMap(typed)
	case map[string]string:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[key] = item
		}
		return result
	default:
		return nil
	}
}

func cloneAnyMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func parseRealityDest(dest string) (string, int) {
	dest = strings.TrimSpace(dest)
	if dest == "" {
		return "", 0
	}
	host, portText, err := net.SplitHostPort(dest)
	if err != nil {
		return dest, 0
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return host, 0
	}
	return host, port
}

func (p *XrayParser) parseTLS(ss *xrayStreamSettings) *TLSConfig {
	if ss.Security == "" || ss.Security == "none" {
		return nil
	}

	config := &TLSConfig{
		Enabled: true,
	}

	if ss.TLSSettings != nil {
		config.ServerName = ss.TLSSettings.ServerName
		config.ALPN = ss.TLSSettings.ALPN
	}

	if ss.Security == "reality" && ss.RealitySettings != nil {
		config.Reality = &RealityConfig{
			Enabled:     true,
			ShortIDs:    ss.RealitySettings.ShortIds,
			Fingerprint: ss.RealitySettings.Fingerprint,
			PublicKey:   ss.RealitySettings.PublicKey,
		}
		if len(ss.RealitySettings.ServerNames) > 0 {
			config.Reality.ServerName = ss.RealitySettings.ServerNames[0]
		}
		config.Reality.HandshakeAddr, config.Reality.HandshakePort = parseRealityDest(ss.RealitySettings.Dest)
	}

	return config
}

func (p *XrayParser) parseSettings(protocol string, settings json.RawMessage) []UserInfo {
	if settings == nil {
		return nil
	}

	switch protocol {
	case "vless", "vmess":
		return p.parseVMessSettings(settings)
	case "shadowsocks":
		return p.parseShadowsocksSettings(settings)
	case "trojan":
		return p.parseTrojanSettings(settings)
	default:
		return nil
	}
}

func (p *XrayParser) parseVMessSettings(settings json.RawMessage) []UserInfo {
	var s struct {
		Clients []struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Flow  string `json:"flow"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(settings, &s); err != nil {
		return nil
	}

	var users []UserInfo
	for _, c := range s.Clients {
		users = append(users, UserInfo{
			UUID:  c.ID,
			Email: c.Email,
			Flow:  c.Flow,
		})
	}
	return users
}

func (p *XrayParser) parseShadowsocksSettings(settings json.RawMessage) []UserInfo {
	var s struct {
		Method   string `json:"method"`
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	if err := json.Unmarshal(settings, &s); err != nil {
		return nil
	}

	return []UserInfo{{
		Method: s.Method,
		Email:  s.Email,
	}}
}

func (p *XrayParser) parseTrojanSettings(settings json.RawMessage) []UserInfo {
	var s struct {
		Clients []struct {
			Password string `json:"password"`
			Email    string `json:"email"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(settings, &s); err != nil {
		return nil
	}

	var users []UserInfo
	for _, c := range s.Clients {
		users = append(users, UserInfo{
			Email: c.Email,
		})
	}
	return users
}
