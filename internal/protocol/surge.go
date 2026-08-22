// 文件路径: internal/protocol/surge.go
// 模块说明: 这是 internal 模块里的 surge 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package protocol

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

const (
	surgeCustomTemplatePath  = "resources/rules/custom.surge.conf"
	surgeDefaultTemplatePath = "resources/rules/default.surge.conf"
)

type SurgeBuilder struct {
	base *BaseBuilder
}

func NewSurgeBuilder() *SurgeBuilder {
	base := NewBaseBuilder()
	base.Allow("shadowsocks", "vmess", "trojan", "hysteria")
	base.AddRequirement("surge", "hysteria", "protocol_settings.version", map[string]string{"2": "2398"}, false)
	return &SurgeBuilder{base: base}
}

func (b *SurgeBuilder) Flags() []string {
	return []string{"surge"}
}

func (b *SurgeBuilder) Build(req BuildRequest) (*Result, error) {
	nodes := req.Nodes
	if b.base != nil {
		nodes = b.base.FilterNodes(req)
	}
	proxyLines := make([]string, 0, len(nodes))
	proxyNames := make([]string, 0, len(nodes))
	for _, node := range nodes {
		line := buildSurgeProxyLine(node)
		if line == "" {
			continue
		}
		proxyLines = append(proxyLines, line)
		proxyNames = append(proxyNames, node.Name)
	}
	template := b.loadTemplate(req.Templates["surge"])
	var payload string
	if template == "" {
		payload = b.buildFallback(proxyLines, proxyNames)
	} else {
		payload = b.applyTemplate(template, req, proxyLines, proxyNames)
	}
	headers := enrichSurgeHeaders(buildUserHeaders(req.User, req.Lang, req.I18n), req.AppName)
	return &Result{
		Payload:     []byte(payload),
		ContentType: "application/octet-stream",
		Headers:     headers,
	}, nil
}

func (b *SurgeBuilder) loadTemplate(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed != "" {
		return trimmed
	}
	if data, err := os.ReadFile(surgeCustomTemplatePath); err == nil {
		return string(data)
	}
	if data, err := os.ReadFile(surgeDefaultTemplatePath); err == nil {
		return string(data)
	}
	return ""
}

func (b *SurgeBuilder) applyTemplate(template string, req BuildRequest, proxyLines, proxyNames []string) string {
	block := formatProxyBlock(proxyLines)
	group := formatProxyGroup(proxyNames)
	info := buildSubscribeInfo(req.AppName, req.User, req.Lang, req.I18n)
	replacements := map[string]string{
		"$proxies":        block,
		"$proxy_group":    group,
		"$subs_link":      req.SubscribeURL,
		"$subs_domain":    req.Host,
		"$subscribe_info": info,
		"{{app_name}}":    req.AppName,
		"{{app_url}}":     req.AppURL,
		"{{subs_link}}":   req.SubscribeURL,
		"{{subs_domain}}": req.Host,
	}
	result := template
	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

func (b *SurgeBuilder) buildFallback(proxyLines, proxyNames []string) string {
	builder := &strings.Builder{}
	builder.WriteString("[Proxy]\n")
	for _, line := range proxyLines {
		builder.WriteString(line)
		if !strings.HasSuffix(line, "\n") {
			builder.WriteString("\n")
		}
	}
	builder.WriteString("\n[Proxy Group]\n")
	groupValue := defaultClashProfileName
	if len(proxyNames) == 0 {
		proxyNames = []string{"DIRECT"}
	}
	builder.WriteString(fmt.Sprintf("%s = select, %s\n\n", groupValue, strings.Join(proxyNames, ", ")))
	builder.WriteString("[Rule]\n")
	builder.WriteString(fmt.Sprintf("FINAL, %s\n", groupValue))
	return builder.String()
}

func formatProxyBlock(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	builder := &strings.Builder{}
	for _, line := range lines {
		builder.WriteString(line)
		if !strings.HasSuffix(line, "\n") {
			builder.WriteString("\n")
		}
	}
	return builder.String()
}

func formatProxyGroup(names []string) string {
	if len(names) == 0 {
		return "DIRECT"
	}
	return strings.Join(names, ", ")
}

func buildSubscribeInfo(appName string, user *repository.User, lang string, i18nMgr *i18n.Manager) string {
	if user == nil {
		return ""
	}
	title := strings.TrimSpace(appName)
	if title == "" {
		title = defaultClashProfileName
	}
	upload := toGB(user.U)
	download := toGB(user.D)
	used := upload + download
	total := toGB(user.TransferEnable)
	unused := total - used
	if unused < 0 {
		unused = 0
	}
	expire := formatI18n(i18nMgr, lang, "subscription.surge.expire_never")
	if user.ExpiredAt > 0 {
		expire = time.Unix(user.ExpiredAt, 0).Format("2006-01-02 15:04:05")
	}
	subsInfo := formatI18n(i18nMgr, lang, "subscription.surge.info", title, upload, download, unused, total, expire)
	return subsInfo
}

func toGB(value int64) float64 {
	return float64(value) / (1024 * 1024 * 1024)
}

func enrichSurgeHeaders(headers map[string]string, appName string) map[string]string {
	if headers == nil {
		headers = map[string]string{}
	}
	title := strings.TrimSpace(appName)
	if title == "" {
		title = defaultClashProfileName
	}
	encoded := url.PathEscape(title)
	headers["content-disposition"] = fmt.Sprintf("attachment;filename*=UTF-8''%s.conf", encoded)
	return headers
}

func buildSurgeProxyLine(node Node) string {
	switch strings.ToLower(node.Type) {
	case "shadowsocks":
		return surgeShadowsocks(node)
	case "vmess":
		return surgeVmess(node)
	case "trojan":
		return surgeTrojan(node)
	case "hysteria":
		return surgeHysteria(node)
	default:
		return ""
	}
}

func surgeShadowsocks(node Node) string {
	cipher := settingString(node.Settings, "cipher")
	if cipher == "" {
		return ""
	}
	parts := []string{
		fmt.Sprintf("%s=ss", node.Name),
		node.Host,
		fmt.Sprintf("%d", node.Port),
		"tfo=true",
		"udp-relay=true",
		"encrypt-method=" + cipher,
		"password=" + node.Password,
	}
	if plugin := settingString(node.Settings, "plugin"); plugin != "" {
		parts = append(parts, "plugin="+plugin)
	}
	return strings.Join(parts, ",")
}

func surgeVmess(node Node) string {
	parts := []string{
		fmt.Sprintf("%s=vmess", node.Name),
		node.Host,
		fmt.Sprintf("%d", node.Port),
		"tfo=true",
		"udp-relay=true",
		"vmess-aead=true",
		"username=" + node.Password,
	}
	if settingBool(node.Settings, "tls") {
		parts = append(parts, "tls=true")
		if allow := settingBool(node.Settings, "tls_settings.allow_insecure"); allow {
			parts = append(parts, "skip-cert-verify=true")
		}
		if sni := settingString(node.Settings, "tls_settings.server_name"); sni != "" {
			parts = append(parts, "sni="+sni)
		}
	}
	if strings.ToLower(settingString(node.Settings, "network")) == "ws" {
		parts = append(parts, "ws=true")
		if path := settingString(node.Settings, "network_settings.path"); path != "" {
			parts = append(parts, "ws-path="+path)
		}
		if host := settingString(node.Settings, "network_settings.headers.Host"); host != "" {
			parts = append(parts, "ws-headers=Host:"+host)
		}
	}
	return strings.Join(parts, ",")
}

func surgeTrojan(node Node) string {
	parts := []string{
		fmt.Sprintf("%s=trojan", node.Name),
		node.Host,
		fmt.Sprintf("%d", node.Port),
		"tfo=true",
		"udp-relay=true",
		"password=" + node.Password,
	}
	if sni := settingString(node.Settings, "server_name"); sni != "" {
		parts = append(parts, "sni="+sni)
	}
	if allow := settingBool(node.Settings, "allow_insecure"); allow {
		parts = append(parts, "skip-cert-verify=true")
	}
	return strings.Join(parts, ",")
}

func surgeHysteria(node Node) string {
	if settingString(node.Settings, "version") != "2" {
		return ""
	}
	parts := []string{
		fmt.Sprintf("%s=hysteria2", node.Name),
		node.Host,
		fmt.Sprintf("%d", node.Port),
		"udp-relay=true",
		"password=" + node.Password,
	}
	if sni := settingString(node.Settings, "tls.server_name"); sni != "" {
		parts = append(parts, "sni="+sni)
	}
	if up := settingString(node.Settings, "bandwidth.up"); up != "" {
		parts = append(parts, "upload-bandwidth="+up)
	}
	if down := settingString(node.Settings, "bandwidth.down"); down != "" {
		parts = append(parts, "download-bandwidth="+down)
	}
	if allow := settingBool(node.Settings, "tls.allow_insecure"); allow {
		parts = append(parts, "skip-cert-verify=true")
	}
	return strings.Join(parts, ",")
}
