package subscribe

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/template"
)

// ParseClash parses Clash YAML format proxies file.
// The proxies file uses a simplified YAML format with inline JSON-like structures.
func ParseClash(content []byte) ([]ClientConfig, error) {
	lines := strings.Split(string(content), "\n")
	var configs []ClientConfig

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- {") {
			continue
		}

		config := parseClashProxyLine(line)
		if config.Name != "" {
			configs = append(configs, config)
		}
	}

	return configs, nil
}

// parseClashProxyLine parses a single Clash proxy line.
func parseClashProxyLine(line string) ClientConfig {
	config := ClientConfig{}

	// Extract name
	config.Name = extractClashField(line, "name")

	// Extract type
	proxyType := extractClashField(line, "type")
	config.Protocol = proxyType

	// Extract server and port
	config.Server = extractClashField(line, "server")
	if portStr := extractClashField(line, "port"); portStr != "" {
		config.Port, _ = strconv.Atoi(portStr)
	}

	// Extract authentication based on type
	switch proxyType {
	case "vless", "vmess":
		config.UUID = extractClashField(line, "uuid")
		config.Flow = extractClashField(line, "flow")
		config.Network = template.NormalizeXHTTPNetwork(extractClashField(line, "network"))

		// Reality
		if strings.Contains(line, "reality-opts") {
			config.RealityEnabled = true
			config.RealityPublicKey = extractNestedField(line, "reality-opts", "public-key")
			config.RealityShortID = extractNestedField(line, "reality-opts", "short-id")
		}

		// TLS
		if extractClashField(line, "tls") == "true" {
			config.TLS = true
		}
		config.SNI = extractClashField(line, "servername")
		if config.SNI == "" {
			config.SNI = extractClashField(line, "sni")
		}

		// gRPC
		if config.Network == "grpc" {
			config.ServiceName = extractNestedField(line, "grpc-opts", "grpc-service-name")
		}

		if config.Network == "xhttp" {
			config.Path = extractNestedField(line, "xhttp-opts", "path")
			config.Host = extractNestedField(line, "xhttp-opts", "host")
			config.Mode = template.NormalizeXHTTPMode(extractNestedField(line, "xhttp-opts", "mode"))
			config.Headers = extractNestedStringMap(line, "xhttp-opts", "headers")
			// Use extractInlineBlock for nested brace fields (extra/xmux/download-settings)
			xhttpBlock := extractInlineBlock(line, "xhttp-opts")
			if rawExtra := extractInlineBlock(xhttpBlock, "extra"); rawExtra != "" {
				if parsed := parseInlineStringMap(rawExtra); len(parsed) > 0 {
					extra := make(map[string]any, len(parsed))
					for k, v := range parsed {
						extra[k] = v
					}
					config.Extra = extra
				}
			}
			if rawXMux := extractInlineBlock(xhttpBlock, "xmux"); rawXMux != "" {
				if parsed := parseInlineStringMap(rawXMux); len(parsed) > 0 {
					xmux := make(map[string]any, len(parsed))
					for k, v := range parsed {
						xmux[k] = v
					}
					config.XMux = xmux
				}
			}
			if rawDS := extractInlineBlock(xhttpBlock, "download-settings"); rawDS != "" {
				if parsed := parseInlineStringMap(rawDS); len(parsed) > 0 {
					ds := make(map[string]any, len(parsed))
					for k, v := range parsed {
						ds[k] = v
					}
					config.DownloadSettings = ds
				}
			}
		}
		// Multiplex
		if strings.Contains(line, "smux") {
			if extractNestedField(line, "smux", "enabled") == "true" {
				config.MuxEnabled = true
			}
			if extractNestedField(line, "smux", "padding") == "true" {
				config.MuxPadding = true
			}
		}

	case "hysteria2":
		config.Password = extractClashField(line, "password")
		config.SNI = extractClashField(line, "sni")
		config.TLS = true // Hysteria2 always uses TLS

		// Port hopping
		config.HopPorts = extractClashField(line, "ports")
		if intervalStr := extractClashField(line, "HopInterval"); intervalStr != "" {
			config.HopInterval, _ = strconv.Atoi(intervalStr)
		}

		// Bandwidth
		config.UpMbps = parseClashBandwidth(extractClashField(line, "up"))
		config.DownMbps = parseClashBandwidth(extractClashField(line, "down"))

		// Fingerprint
		config.Fingerprint = extractClashField(line, "fingerprint")

	case "tuic":
		config.UUID = extractClashField(line, "uuid")
		config.Password = extractClashField(line, "password")
		config.SNI = extractClashField(line, "sni")
		config.TLS = true // TUIC always uses TLS
		config.CongestionControl = extractClashField(line, "congestion-controller")
		config.Fingerprint = extractClashField(line, "fingerprint")

	case "ss":
		config.Protocol = "shadowsocks"
		config.Cipher = extractClashField(line, "cipher")
		config.Password = extractClashField(line, "password")

		// Check for ShadowTLS plugin
		if extractClashField(line, "plugin") == "shadow-tls" {
			config.ShadowTLSPassword = extractNestedField(line, "plugin-opts", "password")
			if versionStr := extractNestedField(line, "plugin-opts", "version"); versionStr != "" {
				config.ShadowTLSVersion, _ = strconv.Atoi(versionStr)
			}
			config.SNI = extractNestedField(line, "plugin-opts", "host")
		}

		// Multiplex
		if strings.Contains(line, "smux") {
			if extractNestedField(line, "smux", "enabled") == "true" {
				config.MuxEnabled = true
			}
			if extractNestedField(line, "smux", "padding") == "true" {
				config.MuxPadding = true
			}
		}

	case "trojan":
		config.Password = extractClashField(line, "password")
		config.SNI = extractClashField(line, "sni")
		config.TLS = true // Trojan always uses TLS
		config.Fingerprint = extractClashField(line, "fingerprint")

		// Multiplex
		if strings.Contains(line, "smux") {
			if extractNestedField(line, "smux", "enabled") == "true" {
				config.MuxEnabled = true
			}
		}

	case "anytls":
		config.Password = extractClashField(line, "password")
		config.SNI = extractClashField(line, "sni")
		config.TLS = true
		config.Fingerprint = extractClashField(line, "fingerprint")
	}

	// Skip cert verify
	if extractClashField(line, "skip-cert-verify") == "true" {
		config.Insecure = true
	}

	return config
}

// extractClashField extracts a field value from Clash proxy line.
func extractClashField(line, field string) string {
	// Pattern: field: value or field: "value"
	patterns := []string{
		field + `: "([^"]*)"`,         // quoted value
		field + `: '([^']*)'`,         // single quoted value
		field + `: ([^,}\s]+)`,        // unquoted value
		`"` + field + `": "([^"]*)"`,  // JSON style quoted
		`"` + field + `": ([^,}\s]+)`, // JSON style unquoted
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(line); len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}
	return ""
}

// extractNestedField extracts a field from a nested structure like "opts: { field: value }".
func extractNestedField(line, parent, field string) string {
	block := extractInlineBlock(line, parent)
	if block == "" {
		return ""
	}
	return extractClashField(block, field)
}

func extractNestedStringMap(line, parent, field string) map[string]string {
	block := extractInlineBlock(line, parent)
	if block == "" {
		return nil
	}
	nested := extractInlineBlock(block, field)
	if nested == "" {
		return nil
	}
	return parseInlineStringMap(nested)
}

func extractInlineBlock(content, field string) string {
	pattern := regexp.MustCompile(regexp.QuoteMeta(field) + `:\s*\{`)
	match := pattern.FindStringIndex(content)
	if len(match) != 2 {
		return ""
	}
	start := match[1]
	depth := 1
	for i := start; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return content[start:i]
			}
		}
	}
	return ""
}

func parseInlineStringMap(content string) map[string]string {
	result := map[string]string{}
	for _, part := range splitInlineMap(content) {
		key, value, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		key = trimInlineValue(key)
		value = trimInlineValue(value)
		if key != "" && value != "" {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func splitInlineMap(content string) []string {
	var parts []string
	start := 0
	depth := 0
	inDoubleQuote := false
	inSingleQuote := false
	for i := 0; i < len(content); i++ {
		// Handle escape sequences
		if content[i] == '\\' {
			i++ // skip the escaped character
			continue
		}
		switch content[i] {
		case '"':
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
			}
		case '\'':
			if !inDoubleQuote {
				inSingleQuote = !inSingleQuote
			}
		case '{':
			if !inDoubleQuote && !inSingleQuote {
				depth++
			}
		case '}':
			if !inDoubleQuote && !inSingleQuote {
				if depth > 0 {
					depth--
				}
			}
		case ',':
			if depth == 0 && !inDoubleQuote && !inSingleQuote {
				parts = append(parts, content[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, content[start:])
	return parts
}

func trimInlineValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return strings.TrimSpace(value)
}

// parseClashBandwidth parses bandwidth string like "200 Mbps" to int.
func parseClashBandwidth(s string) int {
	if s == "" {
		return 0
	}
	// Remove "Mbps" suffix and parse
	s = strings.TrimSuffix(s, " Mbps")
	s = strings.TrimSuffix(s, "Mbps")
	val, _ := strconv.Atoi(strings.TrimSpace(s))
	return val
}

// extractClashProxy extracts a specific proxy entry from Clash proxies content.
func extractClashProxy(content, name string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.Contains(line, `name: "`+name+`"`) || strings.Contains(line, `name: '`+name+`'`) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}
