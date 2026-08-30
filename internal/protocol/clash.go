// 文件路径: internal/protocol/clash.go
// 模块说明: 这是 internal 模块里的 clash 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package protocol

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/template"
	"gopkg.in/yaml.v3"
)

const (
	defaultClashProfileName  = "MGPanel"
	clashCustomTemplatePath  = "resources/rules/custom.clash.yaml"
	clashDefaultTemplatePath = "resources/rules/default.clash.yaml"
)

type ClashBuilder struct {
	base *BaseBuilder
	cdn  *CDNConfig
}

func NewClashBuilder() *ClashBuilder {
	base := NewBaseBuilder()
	base.Allow("shadowsocks", "vmess", "trojan", "vless", "socks", "http")
	return &ClashBuilder{base: base}
}

func (b *ClashBuilder) Flags() []string {
	return []string{"clash", "mihomo", "clash-meta", "clashmeta"}
}

func (b *ClashBuilder) Build(req BuildRequest) (*Result, error) {
	nodes := req.Nodes
	if b.base != nil {
		nodes = b.base.FilterNodes(req)
	}
	b.cdn = req.CDN
	proxies := make([]map[string]any, 0, len(nodes))
	proxyNames := make([]string, 0, len(nodes))
	for _, node := range nodes {
		proxy := buildClashProxy(node, b.cdn)
		if proxy == nil {
			continue
		}
		proxies = append(proxies, proxy)
		proxyNames = append(proxyNames, node.Name)
	}
	profileTitle := strings.TrimSpace(req.AppName)
	if profileTitle == "" {
		profileTitle = defaultClashProfileName
	}
	config := b.loadTemplateConfig(req.Templates["clash"], profileTitle)
	config["proxies"] = append(cloneProxyMaps(config["proxies"]), proxies...)
	b.mergeProxyGroups(config, proxyNames, profileTitle)
	b.applyRules(config, req.Host, profileTitle)
	payload, err := yaml.Marshal(config)
	if err != nil {
		return nil, err
	}
	content := strings.ReplaceAll(string(payload), "$app_name", profileTitle)
	content = b.applyTemplateVariables(content, req)
	headers := enrichClashHeaders(buildUserHeaders(req.User, req.Lang, req.I18n), profileTitle, req.AppURL)
	return &Result{
		Payload:     []byte(content),
		ContentType: "text/yaml",
		Headers:     headers,
	}, nil
}

func (b *ClashBuilder) loadTemplateConfig(rawTemplate string, profile string) map[string]any {
	trimmed := strings.TrimSpace(rawTemplate)
	if trimmed == "" {
		if data, err := os.ReadFile(clashCustomTemplatePath); err == nil {
			trimmed = string(data)
		} else if data, err := os.ReadFile(clashDefaultTemplatePath); err == nil {
			trimmed = string(data)
		}
	}
	if trimmed == "" {
		return defaultClashTemplate(profile)
	}
	var cfg map[string]any
	if err := yaml.Unmarshal([]byte(trimmed), &cfg); err != nil {
		return defaultClashTemplate(profile)
	}
	if cfg == nil {
		cfg = map[string]any{}
	}
	return cfg
}

func (b *ClashBuilder) mergeProxyGroups(config map[string]any, proxies []string, profile string) {
	groups := cloneGroupMaps(config["proxy-groups"])
	if len(groups) == 0 {
		groups = []map[string]any{{
			"name":    profile,
			"type":    "select",
			"proxies": []string{},
		}}
	}
	for _, group := range groups {
		existing := toStringSlice(group["proxies"])
		var merged []string
		replaced := false
		for _, entry := range existing {
			if re, ok := compileClashRegex(entry); ok {
				replaced = true
				for _, candidate := range proxies {
					if re.MatchString(candidate) {
						merged = append(merged, candidate)
					}
				}
			} else {
				merged = append(merged, entry)
			}
		}
		if replaced {
			group["proxies"] = uniqueStrings(merged)
		} else {
			group["proxies"] = uniqueStrings(append(merged, proxies...))
		}
	}
	filtered := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		values := toStringSlice(group["proxies"])
		if len(values) == 0 {
			continue
		}
		group["proxies"] = values
		filtered = append(filtered, group)
	}
	config["proxy-groups"] = filtered
}

func (b *ClashBuilder) applyRules(config map[string]any, host, profile string) {
	rules := toStringSlice(config["rules"])
	if host != "" {
		entry := fmt.Sprintf("DOMAIN,%s,DIRECT", host)
		if !containsString(rules, entry) {
			rules = append([]string{entry}, rules...)
		}
	}
	matchRule := fmt.Sprintf("MATCH,%s", profile)
	if !containsString(rules, matchRule) {
		rules = append(rules, matchRule)
	}
	config["rules"] = rules
}

func (b *ClashBuilder) applyTemplateVariables(content string, req BuildRequest) string {
	replacements := map[string]string{
		"{{app_name}}":    req.AppName,
		"{{app_url}}":     req.AppURL,
		"{{subs_link}}":   req.SubscribeURL,
		"{{subs_domain}}": req.Host,
	}
	for key, val := range replacements {
		content = strings.ReplaceAll(content, key, val)
	}
	return content
}

func cloneProxyMaps(value any) []map[string]any {
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

func cloneGroupMaps(value any) []map[string]any {
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

func toStringSlice(value any) []string {
	var result []string
	switch v := value.(type) {
	case []string:
		return append(result, v...)
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
	case string:
		result = append(result, v)
	}
	return result
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func compileClashRegex(expr string) (*regexp.Regexp, bool) {
	if len(expr) < 2 || !strings.HasPrefix(expr, "/") {
		return nil, false
	}
	last := strings.LastIndex(expr, "/")
	if last <= 0 {
		return nil, false
	}
	pattern := expr[1:last]
	flags := expr[last+1:]
	if strings.Contains(flags, "i") {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, false
	}
	return re, true
}

func enrichClashHeaders(headers map[string]string, profileTitle, appURL string) map[string]string {
	if headers == nil {
		headers = map[string]string{}
	}
	if _, ok := headers["profile-update-interval"]; !ok {
		headers["profile-update-interval"] = "24"
	}
	if profileTitle != "" {
		headers["profile-title"] = profileTitle
		encoded := url.PathEscape(profileTitle)
		headers["content-disposition"] = fmt.Sprintf("attachment;filename*=UTF-8''%s", encoded)
	}
	if appURL := strings.TrimSpace(appURL); appURL != "" {
		headers["profile-web-page-url"] = appURL
	}
	return headers
}

func defaultClashTemplate(profile string) map[string]any {
	if strings.TrimSpace(profile) == "" {
		profile = defaultClashProfileName
	}
	return map[string]any{
		"mixed-port": 7890,
		"allow-lan":  true,
		"mode":       "rule",
		"log-level":  "info",
		"proxies":    []map[string]any{},
		"proxy-groups": []map[string]any{
			{
				"name":    profile,
				"type":    "select",
				"proxies": []string{},
			},
		},
		"rules": []string{fmt.Sprintf("MATCH,%s", profile)},
	}
}

func buildClashProxy(node Node, cdn *CDNConfig) map[string]any {
	switch strings.ToLower(node.Type) {
	case "shadowsocks":
		return buildClashShadowsocks(node)
	case "vmess":
		return buildClashVmess(node)
	case "trojan":
		return buildClashTrojan(node)
	case "vless":
		return buildClashVless(node, cdn)
	case "socks":
		return buildClashSocks(node)
	case "http":
		return buildClashHTTP(node)
	default:
		return nil
	}
}

func buildClashShadowsocks(node Node) map[string]any {
	cipher := settingString(node.Settings, "cipher")
	if cipher == "" {
		return nil
	}
	proxy := map[string]any{
		"name":     node.Name,
		"type":     "ss",
		"server":   node.Host,
		"port":     node.Port,
		"cipher":   cipher,
		"password": node.Password,
		"udp":      true,
	}
	plugin := settingString(node.Settings, "plugin")
	if plugin != "" {
		proxy["plugin"] = plugin
		if opts := parsePluginOptions(settingString(node.Settings, "plugin_opts")); len(opts) > 0 {
			proxy["plugin-opts"] = opts
		}
	}
	return proxy
}

func buildClashVmess(node Node) map[string]any {
	proxy := map[string]any{
		"name":    node.Name,
		"type":    "vmess",
		"server":  node.Host,
		"port":    node.Port,
		"uuid":    node.Password,
		"alterId": 0,
		"cipher":  "auto",
		"udp":     true,
	}
	if settingBool(node.Settings, "tls") {
		proxy["tls"] = true
		proxy["skip-cert-verify"] = settingBool(node.Settings, "tls_settings.allow_insecure")
		if sni := settingString(node.Settings, "tls_settings.server_name"); sni != "" {
			proxy["servername"] = sni
		}
	}
	switch strings.ToLower(settingString(node.Settings, "network")) {
	case "ws":
		proxy["network"] = "ws"
		ws := map[string]any{}
		if path := settingString(node.Settings, "network_settings.path"); path != "" {
			ws["path"] = path
		}
		if host := settingString(node.Settings, "network_settings.headers.Host"); host != "" {
			ws["headers"] = map[string]any{"Host": host}
		}
		if len(ws) > 0 {
			proxy["ws-opts"] = ws
		}
	case "grpc":
		proxy["network"] = "grpc"
		if serviceName := settingString(node.Settings, "network_settings.serviceName"); serviceName != "" {
			proxy["grpc-opts"] = map[string]any{"grpc-service-name": serviceName}
		}
	default:
		proxy["network"] = "tcp"
	}
	return proxy
}

func buildClashTrojan(node Node) map[string]any {
	proxy := map[string]any{
		"name":     node.Name,
		"type":     "trojan",
		"server":   node.Host,
		"port":     node.Port,
		"password": node.Password,
		"udp":      true,
	}
	if sni := settingString(node.Settings, "server_name"); sni != "" {
		proxy["sni"] = sni
	}
	proxy["skip-cert-verify"] = settingBool(node.Settings, "allow_insecure")
	switch strings.ToLower(settingString(node.Settings, "network")) {
	case "ws":
		proxy["network"] = "ws"
		ws := map[string]any{}
		if path := settingString(node.Settings, "network_settings.path"); path != "" {
			ws["path"] = path
		}
		if host := settingString(node.Settings, "network_settings.headers.Host"); host != "" {
			ws["headers"] = map[string]any{"Host": host}
		}
		if len(ws) > 0 {
			proxy["ws-opts"] = ws
		}
	case "grpc":
		proxy["network"] = "grpc"
		if serviceName := settingString(node.Settings, "network_settings.serviceName"); serviceName != "" {
			proxy["grpc-opts"] = map[string]any{"grpc-service-name": serviceName}
		}
	default:
		proxy["network"] = "tcp"
	}
	return proxy
}

func buildClashSocks(node Node) map[string]any {
	proxy := map[string]any{
		"name":     node.Name,
		"type":     "socks5",
		"server":   node.Host,
		"port":     node.Port,
		"udp":      true,
		"username": node.Password,
		"password": node.Password,
	}
	if settingBool(node.Settings, "tls") {
		proxy["tls"] = true
		proxy["skip-cert-verify"] = settingBool(node.Settings, "tls_settings.allow_insecure")
	}
	return proxy
}

func buildClashHTTP(node Node) map[string]any {
	proxy := map[string]any{
		"name":     node.Name,
		"type":     "http",
		"server":   node.Host,
		"port":     node.Port,
		"username": node.Password,
		"password": node.Password,
	}
	if settingBool(node.Settings, "tls") {
		proxy["tls"] = true
		proxy["skip-cert-verify"] = settingBool(node.Settings, "tls_settings.allow_insecure")
	}
	return proxy
}

func parsePluginOptions(raw string) map[string]string {
	opts := map[string]string{}
	for _, pair := range strings.Split(raw, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		opts[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	if len(opts) == 0 {
		return nil
	}
	return opts
}

// buildClashVless builds a VLESS proxy config for Clash Meta (mihomo).
func buildClashVless(node Node, cdn *CDNConfig) map[string]any {
	host := node.Host
	port := node.Port

	proxy := map[string]any{
		"name":   node.Name,
		"type":   "vless",
		"server": host,
		"port":   port,
		"uuid":   node.Password,
		"udp":    true,
	}

	// Flow (xtls-rprx-vision, etc.)
	if flow := settingString(node.Settings, "flow"); flow != "" {
		proxy["flow"] = flow
	}

	// TLS settings
	if settingBool(node.Settings, "tls") {
		proxy["tls"] = true
		proxy["skip-cert-verify"] = settingBool(node.Settings, "tls_settings.allow_insecure")
		if sni := settingString(node.Settings, "tls_settings.server_name"); sni != "" {
			proxy["servername"] = sni
		}

		// Reality settings (Clash Meta specific)
		if settingBool(node.Settings, "reality") {
			proxy["reality-opts"] = map[string]any{
				"public-key": settingString(node.Settings, "tls_settings.public_key"),
				"short-id":   settingString(node.Settings, "tls_settings.short_id"),
			}
			fp := settingString(node.Settings, "tls_settings.fingerprint")
			if fp == "" {
				fp = "firefox"
			}
			proxy["client-fingerprint"] = fp
		}
	}

	// Network/Transport settings
	network := template.NormalizeXHTTPNetwork(settingString(node.Settings, "network"))
	switch network {
	case "ws":
		proxy["network"] = "ws"
		ws := map[string]any{}
		if path := settingString(node.Settings, "network_settings.path"); path != "" {
			ws["path"] = path
		}
		if host := settingString(node.Settings, "network_settings.headers.Host"); host != "" {
			ws["headers"] = map[string]any{"Host": host}
		}
		if len(ws) > 0 {
			proxy["ws-opts"] = ws
		}
	case "grpc":
		proxy["network"] = "grpc"
		if serviceName := settingString(node.Settings, "network_settings.serviceName"); serviceName != "" {
			proxy["grpc-opts"] = map[string]any{"grpc-service-name": serviceName}
		}
	case "h2", "http":
		proxy["network"] = "h2"
		h2 := map[string]any{}
		if path := settingString(node.Settings, "network_settings.path"); path != "" {
			h2["path"] = path
		}
		if host := settingString(node.Settings, "network_settings.host"); host != "" {
			h2["host"] = []string{host}
		}
		if len(h2) > 0 {
			proxy["h2-opts"] = h2
		}
	case "xhttp":
		proxy["network"] = "xhttp"
		if opts := buildXHTTPOpts(node.Settings); len(opts) > 0 {
			proxy["xhttp-opts"] = opts
		}
		// CDN 域名替换：仅对 xhttp 协议生效。只替换连接目标和传输层 host/path，
		// TLS/Reality servername 保留原节点配置，避免证书或 Reality 校验失败。
		if cdn != nil {
			proxy["server"] = cdn.Domain
			proxy["port"] = 443
			if settingBool(node.Settings, "tls") {
				if _, ok := proxy["servername"]; !ok && node.Host != "" {
					proxy["servername"] = node.Host
				}
			}
			xopts, _ := proxy["xhttp-opts"].(map[string]any)
			if xopts == nil {
				xopts = map[string]any{}
				proxy["xhttp-opts"] = xopts
			}
			if cdn.Host != "" {
				xopts["host"] = cdn.Host
			} else {
				xopts["host"] = cdn.Domain
			}
			if cdn.Path != "" {
				xopts["path"] = cdn.Path
			}
		}
	default:
		proxy["network"] = "tcp"
	}

	return proxy
}
