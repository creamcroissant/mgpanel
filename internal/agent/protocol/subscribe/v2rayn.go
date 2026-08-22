package subscribe

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/template"
)

// ParseV2RayN parses V2RayN URL format (base64 encoded or plain URLs).
func ParseV2RayN(content []byte) ([]ClientConfig, error) {
	// Try to decode as base64 first
	decoded, err := base64.StdEncoding.DecodeString(string(content))
	if err != nil {
		// Not base64, use as-is
		decoded = content
	}

	lines := strings.Split(string(decoded), "\n")
	var configs []ClientConfig

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		config := parseV2RayNURL(line)
		if config.Name != "" {
			configs = append(configs, config)
		}
	}

	return configs, nil
}

// parseV2RayNURL parses a single V2RayN URL.
func parseV2RayNURL(urlStr string) ClientConfig {
	config := ClientConfig{}

	// Parse URL scheme
	if strings.HasPrefix(urlStr, "vless://") {
		config = parseVLESSURL(urlStr)
	} else if strings.HasPrefix(urlStr, "vmess://") {
		config = parseVMessURL(urlStr)
	} else if strings.HasPrefix(urlStr, "ss://") {
		config = parseShadowsocksURL(urlStr)
	} else if strings.HasPrefix(urlStr, "trojan://") {
		config = parseTrojanURL(urlStr)
	} else if strings.HasPrefix(urlStr, "hysteria2://") || strings.HasPrefix(urlStr, "hy2://") {
		config = parseHysteria2URL(urlStr)
	} else if strings.HasPrefix(urlStr, "tuic://") {
		config = parseTUICURL(urlStr)
	} else if strings.HasPrefix(urlStr, "anytls://") {
		config = parseAnyTLSURL(urlStr)
	}

	return config
}

// parseVLESSURL parses VLESS URL format.
func parseVLESSURL(urlStr string) ClientConfig {
	config := ClientConfig{Protocol: "vless"}

	// vless://uuid@server:port?params#name
	u, err := url.Parse(urlStr)
	if err != nil {
		return config
	}

	config.UUID = u.User.Username()
	config.Server = u.Hostname()
	config.Port, _ = strconv.Atoi(u.Port())
	config.Name, _ = url.QueryUnescape(u.Fragment)

	params := u.Query()
	config.Flow = params.Get("flow")
	config.Network = template.NormalizeXHTTPNetwork(params.Get("type"))
	if config.Network == "" {
		config.Network = "tcp"
	}

	// Security
	security := params.Get("security")
	if security == "tls" || security == "reality" {
		config.TLS = true
	}
	if security == "reality" {
		config.RealityEnabled = true
		config.RealityPublicKey = params.Get("pbk")
		config.RealityShortID = params.Get("sid")
	}

	config.SNI = params.Get("sni")
	config.Fingerprint = params.Get("fp")
	config.ALPN = params.Get("alpn")

	// Transport specific
	if config.Network == "ws" {
		config.Path = params.Get("path")
	} else if config.Network == "grpc" {
		config.ServiceName = params.Get("serviceName")
	} else if config.Network == "xhttp" {
		config.Path = params.Get("path")
		config.Host = params.Get("host")
		config.Mode = template.NormalizeXHTTPMode(params.Get("mode"))
		config.Extra = parseJSONMapParam(params.Get("extra"))
		config.XMux = parseJSONMapParam(params.Get("xmux"))
		config.DownloadSettings = parseJSONMapParam(params.Get("downloadSettings"))
	}

	return config
}

func parseJSONMapParam(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// parseVMessURL parses VMess URL format.
func parseVMessURL(urlStr string) ClientConfig {
	config := ClientConfig{Protocol: "vmess"}

	// vmess:// is usually base64 encoded JSON
	encoded := strings.TrimPrefix(urlStr, "vmess://")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		// Try URL-safe base64
		decoded, err = base64.URLEncoding.DecodeString(encoded)
		if err != nil {
			return config
		}
	}

	// Parse JSON - simplified extraction
	content := string(decoded)
	config.Name = extractJSONString(content, "ps")
	config.Server = extractJSONString(content, "add")
	config.Port, _ = strconv.Atoi(extractJSONString(content, "port"))
	config.UUID = extractJSONString(content, "id")
	config.Network = extractJSONString(content, "net")

	if extractJSONString(content, "tls") == "tls" {
		config.TLS = true
	}
	config.SNI = extractJSONString(content, "sni")
	config.Path = extractJSONString(content, "path")

	return config
}

// parseShadowsocksURL parses Shadowsocks URL format.
func parseShadowsocksURL(urlStr string) ClientConfig {
	config := ClientConfig{Protocol: "shadowsocks"}

	// ss://base64(method:password)@server:port#name
	// or ss://base64(method:password@server:port)#name
	urlStr = strings.TrimPrefix(urlStr, "ss://")

	// Split by # for name
	parts := strings.SplitN(urlStr, "#", 2)
	if len(parts) > 1 {
		config.Name, _ = url.QueryUnescape(parts[1])
	}
	urlStr = parts[0]

	// Try to parse as base64
	if strings.Contains(urlStr, "@") {
		// Format: base64(method:password)@server:port
		atParts := strings.SplitN(urlStr, "@", 2)
		decoded, err := base64.StdEncoding.DecodeString(atParts[0])
		if err == nil {
			methodPwd := strings.SplitN(string(decoded), ":", 2)
			if len(methodPwd) == 2 {
				config.Cipher = methodPwd[0]
				config.Password = methodPwd[1]
			}
		}
		if len(atParts) > 1 {
			hostPort := atParts[1]
			colonIdx := strings.LastIndex(hostPort, ":")
			if colonIdx > 0 {
				config.Server = hostPort[:colonIdx]
				config.Port, _ = strconv.Atoi(hostPort[colonIdx+1:])
			}
		}
	} else {
		// Format: base64(method:password@server:port)
		decoded, err := base64.StdEncoding.DecodeString(urlStr)
		if err == nil {
			content := string(decoded)
			atParts := strings.SplitN(content, "@", 2)
			if len(atParts) == 2 {
				methodPwd := strings.SplitN(atParts[0], ":", 2)
				if len(methodPwd) == 2 {
					config.Cipher = methodPwd[0]
					config.Password = methodPwd[1]
				}
				hostPort := atParts[1]
				colonIdx := strings.LastIndex(hostPort, ":")
				if colonIdx > 0 {
					config.Server = hostPort[:colonIdx]
					config.Port, _ = strconv.Atoi(hostPort[colonIdx+1:])
				}
			}
		}
	}

	return config
}

// parseTrojanURL parses Trojan URL format.
func parseTrojanURL(urlStr string) ClientConfig {
	config := ClientConfig{Protocol: "trojan", TLS: true}

	// trojan://password@server:port?params#name
	u, err := url.Parse(urlStr)
	if err != nil {
		return config
	}

	config.Password = u.User.Username()
	config.Server = u.Hostname()
	config.Port, _ = strconv.Atoi(u.Port())
	config.Name, _ = url.QueryUnescape(u.Fragment)

	params := u.Query()
	config.SNI = params.Get("sni")
	config.Network = params.Get("type")
	if config.Network == "" {
		config.Network = "tcp"
	}

	if params.Get("allowInsecure") == "1" || params.Get("insecure") == "1" {
		config.Insecure = true
	}

	return config
}

// parseHysteria2URL parses Hysteria2 URL format.
func parseHysteria2URL(urlStr string) ClientConfig {
	config := ClientConfig{Protocol: "hysteria2", TLS: true}

	// hysteria2://password@server:port?params#name
	urlStr = strings.Replace(urlStr, "hy2://", "hysteria2://", 1)
	u, err := url.Parse(urlStr)
	if err != nil {
		return config
	}

	config.Password = u.User.Username()
	config.Server = u.Hostname()
	config.Port, _ = strconv.Atoi(u.Port())
	config.Name, _ = url.QueryUnescape(u.Fragment)

	params := u.Query()
	config.SNI = params.Get("sni")
	config.ALPN = params.Get("alpn")

	if params.Get("insecure") == "1" || params.Get("allowInsecure") == "1" {
		config.Insecure = true
	}

	return config
}

// parseTUICURL parses TUIC URL format.
func parseTUICURL(urlStr string) ClientConfig {
	config := ClientConfig{Protocol: "tuic", TLS: true}

	// tuic://uuid:password@server:port?params#name
	u, err := url.Parse(urlStr)
	if err != nil {
		return config
	}

	config.UUID = u.User.Username()
	if pwd, ok := u.User.Password(); ok {
		config.Password = pwd
	}
	config.Server = u.Hostname()
	config.Port, _ = strconv.Atoi(u.Port())
	config.Name, _ = url.QueryUnescape(u.Fragment)

	params := u.Query()
	config.SNI = params.Get("sni")
	config.ALPN = params.Get("alpn")
	config.CongestionControl = params.Get("congestion_control")

	if params.Get("insecure") == "1" || params.Get("allowInsecure") == "1" {
		config.Insecure = true
	}

	return config
}

// parseAnyTLSURL parses AnyTLS URL format.
func parseAnyTLSURL(urlStr string) ClientConfig {
	config := ClientConfig{Protocol: "anytls", TLS: true}

	// anytls://password@server:port?params#name
	u, err := url.Parse(urlStr)
	if err != nil {
		return config
	}

	config.Password = u.User.Username()
	config.Server = u.Hostname()
	config.Port, _ = strconv.Atoi(u.Port())
	config.Name, _ = url.QueryUnescape(u.Fragment)

	params := u.Query()
	config.SNI = params.Get("sni")
	config.Fingerprint = params.Get("fp")

	if params.Get("insecure") == "1" || params.Get("allowInsecure") == "1" {
		config.Insecure = true
	}

	return config
}

// extractJSONString extracts a string value from JSON content (simplified).
func extractJSONString(content, key string) string {
	// Simple extraction without full JSON parsing
	patterns := []string{
		`"` + key + `":"([^"]*)"`,
		`"` + key + `": "([^"]*)"`,
		`"` + key + `":([0-9]+)`,
		`"` + key + `": ([0-9]+)`,
	}

	for _, pattern := range patterns {
		re := strings.NewReader(pattern)
		_ = re // Placeholder - using simple string search instead

		// Simple approach
		searchKey := `"` + key + `":`
		idx := strings.Index(content, searchKey)
		if idx == -1 {
			searchKey = `"` + key + `": `
			idx = strings.Index(content, searchKey)
		}
		if idx >= 0 {
			start := idx + len(searchKey)
			// Skip whitespace
			for start < len(content) && (content[start] == ' ' || content[start] == '\t') {
				start++
			}
			if start < len(content) {
				if content[start] == '"' {
					// String value
					end := strings.Index(content[start+1:], `"`)
					if end >= 0 {
						return content[start+1 : start+1+end]
					}
				} else {
					// Number or other value
					end := start
					for end < len(content) && content[end] != ',' && content[end] != '}' && content[end] != ' ' {
						end++
					}
					return content[start:end]
				}
			}
		}
	}
	return ""
}

// extractV2RayNLine extracts a specific URL line matching the protocol name.
func extractV2RayNLine(content, name string) string {
	lines := strings.Split(content, "\n")
	encodedName := url.QueryEscape(name)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Check if line ends with #name or #encoded_name
		if strings.HasSuffix(line, "#"+name) || strings.HasSuffix(line, "#"+encodedName) {
			return line
		}
		// Also check URL-decoded fragment
		if idx := strings.LastIndex(line, "#"); idx >= 0 {
			fragment, _ := url.QueryUnescape(line[idx+1:])
			if fragment == name {
				return line
			}
		}
	}
	return ""
}
