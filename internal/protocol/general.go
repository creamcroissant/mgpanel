package protocol

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/template"
)

// GeneralBuilder emits a standard base64 subscription compatible with V2RayN, Shadowrocket, etc.
type GeneralBuilder struct {
	base *BaseBuilder
	cdn  *CDNConfig
}

// NewGeneralBuilder returns a ready-to-use general builder instance.
func NewGeneralBuilder() *GeneralBuilder {
	base := NewBaseBuilder()
	base.Allow("shadowsocks", "vmess", "trojan", "vless", "hysteria", "hysteria2", "tuic", "socks", "http")
	return &GeneralBuilder{base: base}
}

// Flags enumerates supported client identifiers for this builder.
func (b *GeneralBuilder) Flags() []string {
	return []string{"general", "v2rayn", "v2rayng", "passwall", "ssrplus", "sagernet", "shadowrocket", "quantumultx", "nekobox", "nekoray", "hiddify"}
}

// Build renders a newline-delimited list of scheme URIs (base64 encoded) for the provided nodes.
func (b *GeneralBuilder) Build(req BuildRequest) (*Result, error) {
	b.cdn = req.CDN
	nodes := req.Nodes
	if b.base != nil {
		nodes = b.base.FilterNodes(req)
	}

	var builder strings.Builder
	for _, node := range nodes {
		if node.Host == "" || node.Type == "" {
			continue
		}
		uri := b.buildURI(node)
		if uri != "" {
			builder.WriteString(uri)
			builder.WriteString("\n")
		}
	}
	payload := base64.StdEncoding.EncodeToString([]byte(builder.String()))
	headers := buildUserHeaders(req.User, req.Lang, req.I18n)
	return &Result{
		Payload:     []byte(payload),
		ContentType: "text/plain; charset=utf-8",
		Headers:     headers,
	}, nil
}

func (b *GeneralBuilder) buildURI(node Node) string {
	switch strings.ToLower(node.Type) {
	case "shadowsocks":
		return b.buildShadowsocksURI(node)
	case "vmess":
		return b.buildVmessURI(node)
	case "trojan":
		return b.buildTrojanURI(node)
	case "vless":
		return b.buildVlessURI(node)
	case "hysteria2":
		return b.buildHysteria2URI(node)
	case "hysteria":
		return b.buildHysteriaURI(node)
	case "tuic":
		return b.buildTuicURI(node)
	default:
		return ""
	}
}

func (b *GeneralBuilder) buildShadowsocksURI(node Node) string {
	cipher := settingString(node.Settings, "cipher")
	password := node.Password
	if cipher == "" || password == "" {
		return ""
	}
	userinfo := base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", cipher, password)))
	name := url.QueryEscape(node.Name)

	// SIP002 format: ss://userinfo@host:port#name
	// But plugin support usually requires legacy format or plugin param
	plugin := settingString(node.Settings, "plugin")
	if plugin != "" {
		pluginOpts := settingString(node.Settings, "plugin_opts")
		if pluginOpts != "" {
			plugin = fmt.Sprintf("%s;%s", plugin, pluginOpts)
		}
		encodedPlugin := url.QueryEscape(plugin)
		return fmt.Sprintf("ss://%s@%s:%d?plugin=%s#%s", userinfo, node.Host, node.Port, encodedPlugin, name)
	}
	return fmt.Sprintf("ss://%s@%s:%d#%s", userinfo, node.Host, node.Port, name)
}

func (b *GeneralBuilder) buildVmessURI(node Node) string {
	v := map[string]any{
		"v":    "2",
		"ps":   node.Name,
		"add":  node.Host,
		"port": node.Port,
		"id":   node.Password,
		"aid":  0,
		"scy":  "auto",
		"net":  "tcp",
		"type": "none",
		"tls":  "",
	}

	network := settingString(node.Settings, "network")
	if network != "" {
		v["net"] = network
	}
	if network == "ws" {
		if path := settingString(node.Settings, "network_settings.path"); path != "" {
			v["path"] = path
		}
		if host := settingString(node.Settings, "network_settings.headers.Host"); host != "" {
			v["host"] = host
		}
	} else if network == "grpc" {
		if serviceName := settingString(node.Settings, "network_settings.serviceName"); serviceName != "" {
			v["path"] = serviceName // V2RayN uses path for serviceName in grpc
		}
	}

	if settingBool(node.Settings, "tls") {
		v["tls"] = "tls"
		if sni := settingString(node.Settings, "tls_settings.server_name"); sni != "" {
			v["sni"] = sni
		}
		if settingBool(node.Settings, "tls_settings.allow_insecure") {
			// standard vmess link doesn't standardized allowInsecure well, mostly client config
		}
	}

	jsonBytes, _ := json.Marshal(v)
	return "vmess://" + base64.StdEncoding.EncodeToString(jsonBytes)
}

func (b *GeneralBuilder) buildTrojanURI(node Node) string {
	u := url.URL{
		Scheme:   "trojan",
		User:     url.User(node.Password),
		Host:     fmt.Sprintf("%s:%d", node.Host, node.Port),
		Fragment: node.Name,
	}
	q := u.Query()

	if sni := settingString(node.Settings, "server_name"); sni != "" {
		q.Set("sni", sni)
		q.Set("peer", sni)
	}
	if settingBool(node.Settings, "allow_insecure") {
		q.Set("allowInsecure", "1")
	}

	network := settingString(node.Settings, "network")
	if network == "ws" {
		q.Set("type", "ws")
		if path := settingString(node.Settings, "network_settings.path"); path != "" {
			q.Set("path", path)
		}
		if host := settingString(node.Settings, "network_settings.headers.Host"); host != "" {
			q.Set("host", host)
		}
	} else if network == "grpc" {
		q.Set("type", "grpc")
		if serviceName := settingString(node.Settings, "network_settings.serviceName"); serviceName != "" {
			q.Set("serviceName", serviceName)
		}
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func (b *GeneralBuilder) buildVlessURI(node Node) string {
	u := url.URL{
		Scheme:   "vless",
		User:     url.User(node.Password),
		Host:     fmt.Sprintf("%s:%d", node.Host, node.Port),
		Fragment: node.Name,
	}
	q := u.Query()
	q.Set("encryption", "none") // default for vless

	if settingBool(node.Settings, "tls") {
		q.Set("security", "tls")
		if sni := settingString(node.Settings, "tls_settings.server_name"); sni != "" {
			q.Set("sni", sni)
		}
		if settingBool(node.Settings, "reality") {
			q.Set("security", "reality")
			q.Set("pbk", settingString(node.Settings, "tls_settings.public_key"))
			q.Set("sid", settingString(node.Settings, "tls_settings.short_id"))
			if fp := settingString(node.Settings, "tls_settings.fingerprint"); fp != "" {
				q.Set("fp", fp)
			}
		}
	}

	network := template.NormalizeXHTTPNetwork(settingString(node.Settings, "network"))
	if network != "" {
		q.Set("type", network)
	}

	if network == "ws" {
		if path := settingString(node.Settings, "network_settings.path"); path != "" {
			q.Set("path", path)
		}
		if host := settingString(node.Settings, "network_settings.headers.Host"); host != "" {
			q.Set("host", host)
		}
	} else if network == "grpc" {
		if serviceName := settingString(node.Settings, "network_settings.serviceName"); serviceName != "" {
			q.Set("serviceName", serviceName)
		}
	} else if network == "xhttp" {
		applyVLESSXHTTPQuery(q, node.Settings)
	}

	// CDN 域名替换：仅对 xhttp 协议生效。只替换连接目标和传输层 host/path，
	// TLS/Reality SNI 保留原节点配置，避免证书或 Reality serverName 校验失败。
	if b.cdn != nil && network == "xhttp" {
		u.Host = fmt.Sprintf("%s:%d", b.cdn.Domain, 443)
		if b.cdn.Host != "" {
			q.Set("host", b.cdn.Host)
		} else {
			q.Set("host", b.cdn.Domain)
		}
		if b.cdn.Path != "" {
			q.Set("path", b.cdn.Path)
		}
	}

	if flow := settingString(node.Settings, "flow"); flow != "" {
		q.Set("flow", flow)
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func (b *GeneralBuilder) buildHysteria2URI(node Node) string {
	u := url.URL{
		Scheme:   "hysteria2",
		User:     url.User(node.Password),
		Host:     fmt.Sprintf("%s:%d", node.Host, node.Port),
		Fragment: node.Name,
	}
	q := u.Query()

	if sni := settingString(node.Settings, "tls.server_name"); sni != "" {
		q.Set("sni", sni)
	}
	if settingBool(node.Settings, "tls.allow_insecure") {
		q.Set("insecure", "1")
	}
	if obfs := settingString(node.Settings, "obfs"); obfs != "" {
		q.Set("obfs", "salamander")
		q.Set("obfs-password", obfs)
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func (b *GeneralBuilder) buildHysteriaURI(node Node) string {
	// Hysteria 1
	u := url.URL{
		Scheme:   "hysteria",
		Host:     fmt.Sprintf("%s:%d", node.Host, node.Port),
		Fragment: node.Name,
	}
	q := u.Query()
	q.Set("auth", node.Password)

	if sni := settingString(node.Settings, "tls.server_name"); sni != "" {
		q.Set("peer", sni)
		q.Set("server_name", sni)
	}
	if settingBool(node.Settings, "tls.allow_insecure") {
		q.Set("insecure", "1")
	}
	if protocol := settingString(node.Settings, "protocol"); protocol != "" {
		q.Set("protocol", protocol)
	}
	if up := settingString(node.Settings, "bandwidth.up"); up != "" {
		q.Set("up", up)
	}
	if down := settingString(node.Settings, "bandwidth.down"); down != "" {
		q.Set("down", down)
	}
	if obfs := settingString(node.Settings, "obfs"); obfs != "" {
		q.Set("obfs", obfs)
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func (b *GeneralBuilder) buildTuicURI(node Node) string {
	u := url.URL{
		Scheme:   "tuic",
		User:     url.User(node.Password), // TUIC usually uses UUID as user/password
		Host:     fmt.Sprintf("%s:%d", node.Host, node.Port),
		Fragment: node.Name,
	}
	q := u.Query()

	if sni := settingString(node.Settings, "server_name"); sni != "" {
		q.Set("sni", sni)
	}
	if settingBool(node.Settings, "allow_insecure") {
		q.Set("allow_insecure", "1")
	}
	if alpn := settingString(node.Settings, "alpn"); alpn != "" {
		q.Set("alpn", alpn)
	}
	q.Set("congestion_control", "bbr")
	q.Set("udp_relay_mode", "native")

	u.RawQuery = q.Encode()
	return u.String()
}
