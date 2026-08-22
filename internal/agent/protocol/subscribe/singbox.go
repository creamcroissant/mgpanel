package subscribe

import (
	"encoding/json"
	"strings"
)

// extractSingBoxOutbound extracts a specific outbound configuration matching the tag.
func extractSingBoxOutbound(content []byte, tag string) string {
	var config struct {
		Outbounds []json.RawMessage `json:"outbounds"`
	}

	if err := json.Unmarshal(content, &config); err != nil {
		return ""
	}

	for _, outbound := range config.Outbounds {
		var basic struct {
			Tag string `json:"tag"`
		}
		if err := json.Unmarshal(outbound, &basic); err != nil {
			continue
		}
		if basic.Tag == tag {
			// Pretty print the JSON
			var formatted interface{}
			if err := json.Unmarshal(outbound, &formatted); err != nil {
				return string(outbound)
			}
			result, err := json.MarshalIndent(formatted, "", "  ")
			if err != nil {
				return string(outbound)
			}
			return string(result)
		}
	}

	return ""
}

// ParseSingBoxOutbounds parses sing-box client config and extracts outbound configs.
func ParseSingBoxOutbounds(content []byte) ([]ClientConfig, error) {
	var config struct {
		Outbounds []json.RawMessage `json:"outbounds"`
	}

	if err := json.Unmarshal(content, &config); err != nil {
		return nil, err
	}

	var configs []ClientConfig

	for _, outbound := range config.Outbounds {
		var basic struct {
			Type       string `json:"type"`
			Tag        string `json:"tag"`
			Server     string `json:"server"`
			ServerPort int    `json:"server_port"`
		}
		if err := json.Unmarshal(outbound, &basic); err != nil {
			continue
		}

		// Skip non-proxy outbounds
		if basic.Type == "direct" || basic.Type == "block" || basic.Type == "dns" ||
			basic.Type == "selector" || basic.Type == "urltest" {
			continue
		}

		cfg := ClientConfig{
			Name:     basic.Tag,
			Protocol: basic.Type,
			Server:   basic.Server,
			Port:     basic.ServerPort,
		}

		// Parse type-specific fields
		switch basic.Type {
		case "vless":
			cfg = parseSingBoxVLESS(outbound, cfg)
		case "vmess":
			cfg = parseSingBoxVMess(outbound, cfg)
		case "shadowsocks":
			cfg = parseSingBoxShadowsocks(outbound, cfg)
		case "trojan":
			cfg = parseSingBoxTrojan(outbound, cfg)
		case "hysteria2":
			cfg = parseSingBoxHysteria2(outbound, cfg)
		case "tuic":
			cfg = parseSingBoxTUIC(outbound, cfg)
		case "shadowtls":
			// ShadowTLS is usually paired with shadowsocks, skip for now
			continue
		}

		if cfg.Name != "" {
			configs = append(configs, cfg)
		}
	}

	return configs, nil
}

func parseSingBoxVLESS(data []byte, cfg ClientConfig) ClientConfig {
	var v struct {
		UUID      string `json:"uuid"`
		Flow      string `json:"flow"`
		TLS       *singBoxTLS `json:"tls"`
		Transport *singBoxTransport `json:"transport"`
		Multiplex *singBoxMux `json:"multiplex"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return cfg
	}

	cfg.UUID = v.UUID
	cfg.Flow = v.Flow

	if v.TLS != nil {
		cfg.TLS = v.TLS.Enabled
		cfg.SNI = v.TLS.ServerName
		if len(v.TLS.ALPN) > 0 {
			cfg.ALPN = strings.Join(v.TLS.ALPN, ",")
		}
		if v.TLS.Reality != nil && v.TLS.Reality.Enabled {
			cfg.RealityEnabled = true
			cfg.RealityPublicKey = v.TLS.Reality.PublicKey
			cfg.RealityShortID = v.TLS.Reality.ShortID
		}
		if v.TLS.UTLS != nil {
			cfg.Fingerprint = v.TLS.UTLS.Fingerprint
		}
	}

	if v.Transport != nil {
		cfg.Network = v.Transport.Type
		cfg.Path = v.Transport.Path
		cfg.ServiceName = v.Transport.ServiceName
	}

	if v.Multiplex != nil {
		cfg.MuxEnabled = v.Multiplex.Enabled
		cfg.MuxPadding = v.Multiplex.Padding
	}

	return cfg
}

func parseSingBoxVMess(data []byte, cfg ClientConfig) ClientConfig {
	var v struct {
		UUID      string `json:"uuid"`
		TLS       *singBoxTLS `json:"tls"`
		Transport *singBoxTransport `json:"transport"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return cfg
	}

	cfg.UUID = v.UUID

	if v.TLS != nil {
		cfg.TLS = v.TLS.Enabled
		cfg.SNI = v.TLS.ServerName
	}

	if v.Transport != nil {
		cfg.Network = v.Transport.Type
		cfg.Path = v.Transport.Path
	}

	return cfg
}

func parseSingBoxShadowsocks(data []byte, cfg ClientConfig) ClientConfig {
	var v struct {
		Method    string `json:"method"`
		Password  string `json:"password"`
		Multiplex *singBoxMux `json:"multiplex"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return cfg
	}

	cfg.Cipher = v.Method
	cfg.Password = v.Password

	if v.Multiplex != nil {
		cfg.MuxEnabled = v.Multiplex.Enabled
		cfg.MuxPadding = v.Multiplex.Padding
	}

	return cfg
}

func parseSingBoxTrojan(data []byte, cfg ClientConfig) ClientConfig {
	var v struct {
		Password  string `json:"password"`
		TLS       *singBoxTLS `json:"tls"`
		Multiplex *singBoxMux `json:"multiplex"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return cfg
	}

	cfg.Password = v.Password
	cfg.TLS = true // Trojan always uses TLS

	if v.TLS != nil {
		cfg.SNI = v.TLS.ServerName
		if v.TLS.UTLS != nil {
			cfg.Fingerprint = v.TLS.UTLS.Fingerprint
		}
	}

	if v.Multiplex != nil {
		cfg.MuxEnabled = v.Multiplex.Enabled
		cfg.MuxPadding = v.Multiplex.Padding
	}

	return cfg
}

func parseSingBoxHysteria2(data []byte, cfg ClientConfig) ClientConfig {
	var v struct {
		Password    string   `json:"password"`
		ServerPorts []string `json:"server_ports"`
		UpMbps      int      `json:"up_mbps"`
		DownMbps    int      `json:"down_mbps"`
		TLS         *singBoxTLS `json:"tls"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return cfg
	}

	cfg.Password = v.Password
	cfg.TLS = true // Hysteria2 always uses TLS
	cfg.UpMbps = v.UpMbps
	cfg.DownMbps = v.DownMbps

	// Port hopping
	if len(v.ServerPorts) > 0 {
		cfg.HopPorts = strings.Join(v.ServerPorts, ",")
		// Convert "50000:51000" format to "50000-51000"
		cfg.HopPorts = strings.ReplaceAll(cfg.HopPorts, ":", "-")
	}

	if v.TLS != nil {
		cfg.SNI = v.TLS.ServerName
		if len(v.TLS.ALPN) > 0 {
			cfg.ALPN = strings.Join(v.TLS.ALPN, ",")
		}
	}

	return cfg
}

func parseSingBoxTUIC(data []byte, cfg ClientConfig) ClientConfig {
	var v struct {
		UUID              string `json:"uuid"`
		Password          string `json:"password"`
		CongestionControl string `json:"congestion_control"`
		TLS               *singBoxTLS `json:"tls"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return cfg
	}

	cfg.UUID = v.UUID
	cfg.Password = v.Password
	cfg.CongestionControl = v.CongestionControl
	cfg.TLS = true // TUIC always uses TLS

	if v.TLS != nil {
		cfg.SNI = v.TLS.ServerName
		if len(v.TLS.ALPN) > 0 {
			cfg.ALPN = strings.Join(v.TLS.ALPN, ",")
		}
	}

	return cfg
}

// Helper types for parsing sing-box config
type singBoxTLS struct {
	Enabled    bool     `json:"enabled"`
	ServerName string   `json:"server_name"`
	ALPN       []string `json:"alpn"`
	UTLS       *struct {
		Enabled     bool   `json:"enabled"`
		Fingerprint string `json:"fingerprint"`
	} `json:"utls"`
	Reality *struct {
		Enabled   bool   `json:"enabled"`
		PublicKey string `json:"public_key"`
		ShortID   string `json:"short_id"`
	} `json:"reality"`
}

type singBoxTransport struct {
	Type        string `json:"type"`
	Path        string `json:"path"`
	ServiceName string `json:"service_name"`
}

type singBoxMux struct {
	Enabled bool `json:"enabled"`
	Padding bool `json:"padding"`
}
