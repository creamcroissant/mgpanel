package protocol

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	singboxCustomTemplatePath  = "resources/rules/custom.sing-box.json"
	singboxDefaultTemplatePath = "resources/rules/default.sing-box.json"
)

type SingboxBuilder struct {
	base *BaseBuilder
	cdn  *CDNConfig
}

func NewSingboxBuilder() *SingboxBuilder {
	base := NewBaseBuilder()
	base.Allow("shadowsocks", "vmess", "trojan", "vless", "hysteria", "tuic", "socks", "http")
	return &SingboxBuilder{base: base}
}

func (b *SingboxBuilder) Flags() []string {
	return []string{"sing-box", "singbox"}
}

func (b *SingboxBuilder) Build(req BuildRequest) (*Result, error) {
	nodes := req.Nodes
	if b.base != nil {
		nodes = b.base.FilterNodes(req)
	}
	b.cdn = req.CDN

	outbounds := make([]map[string]any, 0, len(nodes))
	proxyTags := make([]string, 0, len(nodes))

	for _, node := range nodes {
		outbound := buildSingboxOutbound(node)
		if outbound == nil {
			continue
		}
		outbounds = append(outbounds, outbound)
		if tag, ok := outbound["tag"].(string); ok {
			proxyTags = append(proxyTags, tag)
		}
	}

	config := b.loadTemplateConfig(req.Templates["sing-box"])

	// Merge outbounds
	existingOutbounds := cloneOutbounds(config["outbounds"])

	// Filter existing outbounds to find Selector/URLTest groups to inject proxies into
	selectorTags := []string{}
	for _, out := range existingOutbounds {
		outType, _ := out["type"].(string)
		if outType == "selector" || outType == "urltest" {
			if tag, ok := out["tag"].(string); ok {
				selectorTags = append(selectorTags, tag)
			}
		}
	}

	// If no selectors found, create a default one
	if len(selectorTags) == 0 {
		defaultSelector := map[string]any{
			"type":      "selector",
			"tag":       "proxy",
			"outbounds": proxyTags,
			"default":   "auto",
		}
		existingOutbounds = append([]map[string]any{defaultSelector}, existingOutbounds...)
	} else {
		// Inject into existing selectors
		for i, out := range existingOutbounds {
			outType, _ := out["type"].(string)
			if outType == "selector" || outType == "urltest" {
				existingOutbounds[i]["outbounds"] = append(toStringSlice(out["outbounds"]), proxyTags...)
			}
		}
	}

	// Append proxy outbounds
	config["outbounds"] = append(existingOutbounds, outbounds...)

	payload, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, err
	}

	headers := enrichSingboxHeaders(buildUserHeaders(req.User, req.Lang, req.I18n), req.AppName)
	return &Result{
		Payload:     payload,
		ContentType: "application/json",
		Headers:     headers,
	}, nil
}

func (b *SingboxBuilder) loadTemplateConfig(rawTemplate string) map[string]any {
	trimmed := strings.TrimSpace(rawTemplate)
	if trimmed == "" {
		if data, err := os.ReadFile(singboxCustomTemplatePath); err == nil {
			trimmed = string(data)
		} else if data, err := os.ReadFile(singboxDefaultTemplatePath); err == nil {
			trimmed = string(data)
		}
	}

	var cfg map[string]any
	if trimmed != "" {
		if err := json.Unmarshal([]byte(trimmed), &cfg); err != nil {
			// Log error?
		}
	}

	if cfg == nil {
		cfg = defaultSingboxTemplate()
	}
	return cfg
}

func defaultSingboxTemplate() map[string]any {
	return map[string]any{
		"log": map[string]any{
			"level":     "info",
			"timestamp": true,
		},
		"route": map[string]any{
			"rules": []map[string]any{
				{
					"protocol": "dns",
					"outbound": "dns-out",
				},
				{
					"clash_mode": "Direct",
					"outbound":   "direct",
				},
				{
					"clash_mode": "Global",
					"outbound":   "proxy",
				},
			},
			"auto_detect_interface": true,
		},
		"outbounds": []map[string]any{
			{
				"type": "selector",
				"tag":  "proxy",
				"outbounds": []string{
					"auto",
					"direct",
				},
				"default": "auto",
			},
			{
				"type":      "urltest",
				"tag":       "auto",
				"outbounds": []string{},
				"url":       "http://www.gstatic.com/generate_204",
				"interval":  "10m",
				"tolerance": 50,
			},
			{
				"type": "direct",
				"tag":  "direct",
			},
			{
				"type": "dns",
				"tag":  "dns-out",
			},
		},
	}
}

func buildSingboxOutbound(node Node) map[string]any {
	if nodeUsesXHTTP(node.Settings) {
		return nil
	}

	base := map[string]any{
		"tag": node.Name,
	}

	switch strings.ToLower(node.Type) {
	case "shadowsocks":
		base["type"] = "shadowsocks"
		base["server"] = node.Host
		base["server_port"] = node.Port
		base["method"] = settingString(node.Settings, "cipher")
		base["password"] = node.Password
		if plugin := settingString(node.Settings, "plugin"); plugin != "" {
			base["plugin"] = plugin
			base["plugin_opts"] = settingString(node.Settings, "plugin_opts") // Singbox might expect simpler opts
		}

	case "vmess":
		base["type"] = "vmess"
		base["server"] = node.Host
		base["server_port"] = node.Port
		base["uuid"] = node.Password
		base["alter_id"] = 0
		base["security"] = "auto"
		if settingBool(node.Settings, "tls") {
			tls := map[string]any{
				"enabled":  true,
				"insecure": settingBool(node.Settings, "tls_settings.allow_insecure"),
			}
			if sni := settingString(node.Settings, "tls_settings.server_name"); sni != "" {
				tls["server_name"] = sni
			}
			base["tls"] = tls
		}

		transport := map[string]any{}
		network := strings.ToLower(settingString(node.Settings, "network"))
		if network == "ws" {
			transport["type"] = "ws"
			if path := settingString(node.Settings, "network_settings.path"); path != "" {
				transport["path"] = path
			}
			if host := settingString(node.Settings, "network_settings.headers.Host"); host != "" {
				transport["headers"] = map[string]string{"Host": host}
			}
			base["transport"] = transport
		} else if network == "grpc" {
			transport["type"] = "grpc"
			if serviceName := settingString(node.Settings, "network_settings.serviceName"); serviceName != "" {
				transport["service_name"] = serviceName
			}
			base["transport"] = transport
		}

	case "vless":
		base["type"] = "vless"
		base["server"] = node.Host
		base["server_port"] = node.Port
		base["uuid"] = node.Password
		// VLESS needs flow
		if flow := settingString(node.Settings, "flow"); flow != "" {
			base["flow"] = flow
		}

		if settingBool(node.Settings, "tls") {
			tls := map[string]any{
				"enabled":  true,
				"insecure": settingBool(node.Settings, "tls_settings.allow_insecure"),
			}
			if sni := settingString(node.Settings, "tls_settings.server_name"); sni != "" {
				tls["server_name"] = sni
			}
			// Reality
			if settingBool(node.Settings, "reality") {
				tls["reality"] = map[string]any{
					"enabled":    true,
					"public_key": settingString(node.Settings, "tls_settings.public_key"),
					"short_id":   settingString(node.Settings, "tls_settings.short_id"),
				}
				// Reality 客户端必须指定 uTLS 指纹，缺省时用 firefox（与服务端惯例一致）
				fp := settingString(node.Settings, "tls_settings.fingerprint")
				if fp == "" {
					fp = "firefox"
				}
				tls["utls"] = map[string]any{
					"enabled":     true,
					"fingerprint": fp,
				}
			}
			base["tls"] = tls
		}

		transport := map[string]any{}
		network := strings.ToLower(settingString(node.Settings, "network"))
		if network == "ws" {
			transport["type"] = "ws"
			if path := settingString(node.Settings, "network_settings.path"); path != "" {
				transport["path"] = path
			}
			if host := settingString(node.Settings, "network_settings.headers.Host"); host != "" {
				transport["headers"] = map[string]string{"Host": host}
			}
			base["transport"] = transport
		} else if network == "grpc" {
			transport["type"] = "grpc"
			if serviceName := settingString(node.Settings, "network_settings.serviceName"); serviceName != "" {
				transport["service_name"] = serviceName
			}
			base["transport"] = transport
		}

	case "trojan":
		base["type"] = "trojan"
		base["server"] = node.Host
		base["server_port"] = node.Port
		base["password"] = node.Password

		tls := map[string]any{
			"enabled":  true,
			"insecure": settingBool(node.Settings, "allow_insecure"),
		}
		if sni := settingString(node.Settings, "server_name"); sni != "" {
			tls["server_name"] = sni
		}
		base["tls"] = tls

		transport := map[string]any{}
		network := strings.ToLower(settingString(node.Settings, "network"))
		if network == "ws" {
			transport["type"] = "ws"
			if path := settingString(node.Settings, "network_settings.path"); path != "" {
				transport["path"] = path
			}
			if host := settingString(node.Settings, "network_settings.headers.Host"); host != "" {
				transport["headers"] = map[string]string{"Host": host}
			}
			base["transport"] = transport
		} else if network == "grpc" {
			transport["type"] = "grpc"
			if serviceName := settingString(node.Settings, "network_settings.serviceName"); serviceName != "" {
				transport["service_name"] = serviceName
			}
			base["transport"] = transport
		}

	case "hysteria2", "hysteria":
		base["type"] = "hysteria2"
		base["server"] = node.Host
		base["server_port"] = node.Port
		base["password"] = node.Password

		if settingString(node.Settings, "version") == "2" || node.Type == "hysteria2" {
			// Hysteria 2
			tls := map[string]any{
				"enabled":  true,
				"insecure": settingBool(node.Settings, "tls.allow_insecure"),
			}
			if sni := settingString(node.Settings, "tls.server_name"); sni != "" {
				tls["server_name"] = sni
			}
			base["tls"] = tls

			if obfs := settingString(node.Settings, "obfs"); obfs != "" {
				base["obfs"] = map[string]any{
					"type":     "salamander",
					"password": obfs,
				}
			}

			// Bandwidth (optional in singbox hysteria2 but good to have)
			if up := settingString(node.Settings, "bandwidth.up"); up != "" {
				if upInt, err := strconv.Atoi(up); err == nil {
					base["up_mbps"] = upInt
				}
			}
			if down := settingString(node.Settings, "bandwidth.down"); down != "" {
				if downInt, err := strconv.Atoi(down); err == nil {
					base["down_mbps"] = downInt
				}
			}
		} else {
			// Legacy Hysteria 1 not fully supported in standard singbox hysteria2 outbound?
			// Sing-box has `hysteria` type for v1.
			if settingString(node.Settings, "version") == "1" {
				base["type"] = "hysteria"
				// Hysteria 1 config mapping...
				// Skipping for now, focusing on Hysteria 2
				return nil
			}
			// Default to Hysteria 2 if version not specified
			tls := map[string]any{
				"enabled":  true,
				"insecure": settingBool(node.Settings, "tls.allow_insecure"),
			}
			if sni := settingString(node.Settings, "tls.server_name"); sni != "" {
				tls["server_name"] = sni
			}
			base["tls"] = tls
		}

	case "tuic":
		base["type"] = "tuic"
		base["server"] = node.Host
		base["server_port"] = node.Port
		base["uuid"] = node.Password
		base["congestion_control"] = "bbr"

		tls := map[string]any{
			"enabled":  true,
			"insecure": settingBool(node.Settings, "allow_insecure"),
		}
		if sni := settingString(node.Settings, "server_name"); sni != "" {
			tls["server_name"] = sni
		}
		if alpn := settingString(node.Settings, "alpn"); alpn != "" {
			tls["alpn"] = []string{alpn}
		}
		base["tls"] = tls

	default:
		return nil
	}

	return base
}

func cloneOutbounds(value any) []map[string]any {
	var result []map[string]any
	switch v := value.(type) {
	case []map[string]any:
		for _, item := range v {
			result = append(result, item)
		}
	case []any:
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				result = append(result, m)
			}
		}
	}
	return result
}

func enrichSingboxHeaders(headers map[string]string, appName string) map[string]string {
	if headers == nil {
		headers = map[string]string{}
	}
	title := strings.TrimSpace(appName)
	if title == "" {
		title = "MGPanel"
	}
	encoded := url.PathEscape(title)
	headers["content-disposition"] = fmt.Sprintf("attachment;filename*=UTF-8''%s.json", encoded)
	return headers
}
