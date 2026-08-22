package template

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
)

// DefaultFuncMap 返回模板默认函数集合。
func DefaultFuncMap() template.FuncMap {
	return template.FuncMap{
		// JSON 编码
		"json": func(v interface{}) string {
			b, err := json.Marshal(v)
			if err != nil {
				return ""
			}
			return string(b)
		},

		// JSON 缩进输出
		"jsonIndent": func(v interface{}) string {
			b, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				return ""
			}
			return string(b)
		},

		// 默认值处理
		"default": func(def, val interface{}) interface{} {
			if val == nil {
				return def
			}
			switch v := val.(type) {
			case string:
				if v == "" {
					return def
				}
			case int:
				if v == 0 {
					return def
				}
			case int64:
				if v == 0 {
					return def
				}
			case bool:
				// 布尔值 false 也是合法值，直接返回
				return val
			}
			return val
		},

		// 连接字符串
		"join": func(sep string, items []string) string {
			return strings.Join(items, sep)
		},

		// 判断切片是否包含元素
		"contains": func(items []string, item string) bool {
			for _, i := range items {
				if i == item {
					return true
				}
			}
			return false
		},

		// JSON 字符串加引号
		"quote": func(s string) string {
			b, err := json.Marshal(s)
			if err != nil {
				return ""
			}
			return string(b)
		},

		// 按协议构造入站用户列表
		"usersForProtocol": func(users []UserConfig, protocol string) []InboundUser {
			result := make([]InboundUser, 0, len(users))
			for _, u := range users {
				if !u.Enabled {
					continue
				}
				user := InboundUser{
					UUID: u.UUID,
					Name: u.Email,
				}
				switch protocol {
				case "shadowsocks":
					user.Password = u.UUID // 使用 UUID 作为密码
					user.UUID = ""         // Shadowsocks 不使用 UUID
				case "trojan", "hysteria2", "tuic":
					user.Password = u.UUID // 使用 UUID 作为密码
				case "vless":
					user.Flow = "xtls-rprx-vision" // VLESS 默认流控
				case "vmess":
					// VMess 直接使用 UUID
				}
				result = append(result, user)
			}
			return result
		},

		// 生成 v2ray_api 统计用户列表
		"statsUsers": func(users []UserConfig) []string {
			result := make([]string, 0, len(users))
			for _, u := range users {
				if u.Enabled && u.Email != "" {
					result = append(result, u.Email)
				}
			}
			return result
		},

		// 生成入站标签列表
		"inboundTags": func(inbounds []InboundConfig) []string {
			result := make([]string, 0, len(inbounds))
			for _, in := range inbounds {
				if in.Tag != "" {
					result = append(result, in.Tag)
				}
			}
			return result
		},

		// 算术运算
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},

		// 条件判断
		"eq": func(a, b interface{}) bool { return a == b },
		"ne": func(a, b interface{}) bool { return a != b },
		"gt": func(a, b int) bool { return a > b },
		"lt": func(a, b int) bool { return a < b },
		"ge": func(a, b int) bool { return a >= b },
		"le": func(a, b int) bool { return a <= b },

		// 逻辑运算
		"and": func(a, b bool) bool { return a && b },
		"or":  func(a, b bool) bool { return a || b },
		"not": func(a bool) bool { return !a },

		// 字符串辅助函数
		"lower":     strings.ToLower,
		"upper":     strings.ToUpper,
		"trim":      strings.TrimSpace,
		"replace":   strings.ReplaceAll,
		"hasPrefix": strings.HasPrefix,
		"hasSuffix": strings.HasSuffix,

		// 切片辅助函数
		"first": func(items interface{}) interface{} {
			switch v := items.(type) {
			case []InboundConfig:
				if len(v) > 0 {
					return v[0]
				}
			case []UserConfig:
				if len(v) > 0 {
					return v[0]
				}
			case []string:
				if len(v) > 0 {
					return v[0]
				}
			}
			return nil
		},
		"last": func(items interface{}) interface{} {
			switch v := items.(type) {
			case []InboundConfig:
				if len(v) > 0 {
					return v[len(v)-1]
				}
			case []UserConfig:
				if len(v) > 0 {
					return v[len(v)-1]
				}
			case []string:
				if len(v) > 0 {
					return v[len(v)-1]
				}
			}
			return nil
		},
		"len": func(items interface{}) int {
			switch v := items.(type) {
			case []InboundConfig:
				return len(v)
			case []UserConfig:
				return len(v)
			case []InboundUser:
				return len(v)
			case []string:
				return len(v)
			case string:
				return len(v)
			}
			return 0
		},

		// 逗号分隔辅助
		"isLast": func(index, length int) bool {
			return index == length-1
		},
		"isFirst": func(index int) bool {
			return index == 0
		},

		// 生成 sing-box 入站配置
		"singboxInbound": func(inbound InboundConfig, users []UserConfig) map[string]interface{} {
			result := map[string]interface{}{
				"type": inbound.Type,
				"tag":  inbound.Tag,
			}

			// 监听地址与端口
			if inbound.Listen != "" {
				result["listen"] = inbound.Listen
			} else {
				result["listen"] = "::"
			}
			if inbound.ListenPort > 0 {
				result["listen_port"] = inbound.ListenPort
			}

			// 按协议填充用户
			if len(users) > 0 {
				usersList := make([]map[string]interface{}, 0, len(users))
				for _, u := range users {
					if !u.Enabled {
						continue
					}
					user := map[string]interface{}{
						"name": u.Email,
					}
					switch inbound.Type {
					case "vless":
						user["uuid"] = u.UUID
						user["flow"] = "xtls-rprx-vision"
					case "vmess":
						user["uuid"] = u.UUID
					case "shadowsocks":
						user["password"] = u.UUID
					case "trojan", "hysteria2", "tuic":
						user["password"] = u.UUID
					}
					usersList = append(usersList, user)
				}
				result["users"] = usersList
			}

			// 传输层配置
			if inbound.Transport != nil {
				transport := map[string]interface{}{
					"type": inbound.Transport.Type,
				}
				if inbound.Transport.Path != "" {
					transport["path"] = inbound.Transport.Path
				}
				if inbound.Transport.Host != "" {
					if inbound.Transport.Type == "ws" {
						transport["headers"] = map[string]string{"Host": inbound.Transport.Host}
					} else if inbound.Transport.Type == "http" {
						transport["host"] = []string{inbound.Transport.Host}
					}
				}
				if inbound.Transport.ServiceName != "" {
					transport["service_name"] = inbound.Transport.ServiceName
				}
				result["transport"] = transport
			}

			// TLS 配置
			if inbound.TLS != nil && inbound.TLS.Enabled {
				tls := map[string]interface{}{
					"enabled": true,
				}
				if inbound.TLS.ServerName != "" {
					tls["server_name"] = inbound.TLS.ServerName
				}
				if len(inbound.TLS.ALPN) > 0 {
					tls["alpn"] = inbound.TLS.ALPN
				}
				if inbound.TLS.Certificate != "" {
					tls["certificate_path"] = inbound.TLS.Certificate
				}
				if inbound.TLS.Key != "" {
					tls["key_path"] = inbound.TLS.Key
				}

				// Reality 配置
				if inbound.TLS.Reality != nil && inbound.TLS.Reality.Enabled {
					reality := map[string]interface{}{
						"enabled": true,
					}
					if inbound.TLS.Reality.PrivateKey != "" {
						reality["private_key"] = inbound.TLS.Reality.PrivateKey
					}
					if len(inbound.TLS.Reality.ShortIDs) > 0 {
						reality["short_id"] = inbound.TLS.Reality.ShortIDs
					}
					if inbound.TLS.Reality.Handshake != nil {
						reality["handshake"] = map[string]interface{}{
							"server":      inbound.TLS.Reality.Handshake.Server,
							"server_port": inbound.TLS.Reality.Handshake.ServerPort,
						}
					}
					tls["reality"] = reality
				}

				result["tls"] = tls
			}

			// 多路复用配置
			if inbound.Multiplex != nil && inbound.Multiplex.Enabled {
				mux := map[string]interface{}{
					"enabled": true,
				}
				if inbound.Multiplex.Padding {
					mux["padding"] = true
				}
				if inbound.Multiplex.Brutal != nil && inbound.Multiplex.Brutal.Enabled {
					mux["brutal"] = map[string]interface{}{
						"enabled":   true,
						"up_mbps":   inbound.Multiplex.Brutal.UpMbps,
						"down_mbps": inbound.Multiplex.Brutal.DownMbps,
					}
				}
				result["multiplex"] = mux
			}

			return result
		},

		// 生成 v2ray_api 实验配置
		"v2rayAPIConfig": func(listenAddr string, inbounds []InboundConfig, users []UserConfig) map[string]interface{} {
			if listenAddr == "" {
				listenAddr = "127.0.0.1:10085"
			}

			// 构建入站标签列表
			inboundTags := make([]string, 0, len(inbounds))
			for _, in := range inbounds {
				if in.Tag != "" {
					inboundTags = append(inboundTags, in.Tag)
				}
			}

			// 构建统计用户列表
			userNames := make([]string, 0, len(users))
			for _, u := range users {
				if u.Enabled && u.Email != "" {
					userNames = append(userNames, u.Email)
				}
			}

			return map[string]interface{}{
				"listen": listenAddr,
				"stats": map[string]interface{}{
					"enabled":   true,
					"inbounds":  inboundTags,
					"users":     userNames,
					"outbounds": []string{"direct"},
				},
			}
		},

		// 生成 sing-box 默认 DNS 配置
		"defaultDNS": func() map[string]interface{} {
			return map[string]interface{}{
				"servers": []map[string]interface{}{
					{
						"tag":     "dns-direct",
						"address": "local",
						"detour":  "direct",
					},
				},
			}
		},

		// 生成 sing-box 默认路由配置
		"defaultRoute": func(inbounds []InboundConfig) map[string]interface{} {
			inboundTags := make([]string, 0, len(inbounds))
			for _, in := range inbounds {
				if in.Tag != "" {
					inboundTags = append(inboundTags, in.Tag)
				}
			}

			return map[string]interface{}{
				"rules": []map[string]interface{}{
					{
						"inbound":  inboundTags,
						"outbound": "direct",
					},
				},
				"final": "direct",
			}
		},

		// 判断 capability 是否存在
		"hasCap": func(capabilities []string, cap string) bool {
			for _, c := range capabilities {
				if c == cap {
					return true
				}
			}
			return false
		},

		// 按协议类型过滤入站
		"filterByType": func(inbounds []InboundConfig, protoType string) []InboundConfig {
			result := make([]InboundConfig, 0)
			for _, in := range inbounds {
				if in.Type == protoType {
					result = append(result, in)
				}
			}
			return result
		},

		// 生成范围索引切片
		"range_": func(n int) []int {
			result := make([]int, n)
			for i := 0; i < n; i++ {
				result[i] = i
			}
			return result
		},

		// 生成 Xray 入站配置
		"xrayInbound": func(inbound InboundConfig, users []UserConfig) map[string]interface{} {
			result := map[string]interface{}{
				"protocol": inbound.Type,
				"tag":      inbound.Tag,
			}

			// 监听地址与端口
			if inbound.Listen != "" {
				result["listen"] = inbound.Listen
			}
			if inbound.ListenPort > 0 {
				result["port"] = inbound.ListenPort
			}

			// 按协议生成 settings
			settings := map[string]interface{}{}
			switch inbound.Type {
			case "vless":
				settings["decryption"] = "none"
				omitDefaultFlow := inbound.Transport != nil && IsXHTTPNetwork(inbound.Transport.Type)
				clients := make([]map[string]interface{}, 0, len(users))
				for _, u := range users {
					if !u.Enabled {
						continue
					}
					client := map[string]interface{}{
						"id":    u.UUID,
						"email": u.Email,
						"level": 0,
					}
					if !omitDefaultFlow {
						client["flow"] = "xtls-rprx-vision"
					}
					clients = append(clients, client)
				}
				settings["clients"] = clients

			case "vmess":
				clients := make([]map[string]interface{}, 0, len(users))
				for _, u := range users {
					if !u.Enabled {
						continue
					}
					client := map[string]interface{}{
						"id":      u.UUID,
						"email":   u.Email,
						"level":   0,
						"alterId": 0,
					}
					clients = append(clients, client)
				}
				settings["clients"] = clients

			case "shadowsocks":
				// Xray shadowsocks supports multi-user mode
				if len(users) > 0 {
					settings["method"] = "aes-256-gcm"
					settings["network"] = "tcp,udp"
					clients := make([]map[string]interface{}, 0, len(users))
					for _, u := range users {
						if !u.Enabled {
							continue
						}
						client := map[string]interface{}{
							"password": u.UUID,
							"email":    u.Email,
							"level":    0,
						}
						clients = append(clients, client)
					}
					settings["clients"] = clients
				}

			case "trojan":
				clients := make([]map[string]interface{}, 0, len(users))
				for _, u := range users {
					if !u.Enabled {
						continue
					}
					client := map[string]interface{}{
						"password": u.UUID,
						"email":    u.Email,
						"level":    0,
					}
					clients = append(clients, client)
				}
				settings["clients"] = clients
			}
			result["settings"] = settings

			// 传输/网络 — 复用 converter 的公共函数
			var unifiedTransport *UnifiedTransport
			if inbound.Transport != nil {
				unifiedTransport = &UnifiedTransport{
					Type:        inbound.Transport.Type,
					Path:        inbound.Transport.Path,
					Host:        inbound.Transport.Host,
					ServiceName: inbound.Transport.ServiceName,
					Headers:     inbound.Transport.Headers,
					Mode:        inbound.Transport.Mode,
					XHTTP:       inbound.Transport.XHTTP,
				}
			}
			streamSettings, hasStreamSettings := buildXrayTransportStreamSettings(unifiedTransport)

			// TLS 配置
			if inbound.TLS != nil && inbound.TLS.Enabled {
				hasStreamSettings = true

				if inbound.TLS.Reality != nil && inbound.TLS.Reality.Enabled {
					// Reality 配置
					streamSettings["security"] = "reality"
					realitySettings := map[string]interface{}{
						"show": false,
					}
					if inbound.TLS.Reality.PrivateKey != "" {
						realitySettings["privateKey"] = inbound.TLS.Reality.PrivateKey
					}
					if len(inbound.TLS.Reality.ShortIDs) > 0 {
						realitySettings["shortIds"] = inbound.TLS.Reality.ShortIDs
					}
					if inbound.TLS.Reality.Handshake != nil {
						realitySettings["dest"] = inbound.TLS.Reality.Handshake.Server
						if inbound.TLS.Reality.Handshake.ServerPort > 0 {
							realitySettings["dest"] = fmt.Sprintf("%s:%d",
								inbound.TLS.Reality.Handshake.Server,
								inbound.TLS.Reality.Handshake.ServerPort)
						}
						realitySettings["serverNames"] = []string{inbound.TLS.Reality.Handshake.Server}
					}
					streamSettings["realitySettings"] = realitySettings
				} else {
					// 标准 TLS 配置
					streamSettings["security"] = "tls"
					tlsSettings := map[string]interface{}{}
					if inbound.TLS.ServerName != "" {
						tlsSettings["serverName"] = inbound.TLS.ServerName
					}
					if len(inbound.TLS.ALPN) > 0 {
						tlsSettings["alpn"] = inbound.TLS.ALPN
					}
					if inbound.TLS.Certificate != "" {
						tlsSettings["certificates"] = []map[string]interface{}{
							{
								"certificateFile": inbound.TLS.Certificate,
								"keyFile":         inbound.TLS.Key,
							},
						}
					}
					if len(tlsSettings) > 0 {
						streamSettings["tlsSettings"] = tlsSettings
					}
				}
			} else {
				streamSettings["security"] = "none"
			}

			if hasStreamSettings {
				result["streamSettings"] = streamSettings
			}

			return result
		},

		// 生成 Xray API 配置
		"xrayAPIConfig": func(listenAddr string) map[string]interface{} {
			if listenAddr == "" {
				listenAddr = "127.0.0.1:10085"
			}

			// 解析 host:port
			parts := strings.Split(listenAddr, ":")
			host := "127.0.0.1"
			port := 10085
			if len(parts) == 2 {
				host = parts[0]
				// 简单解析端口
				p := 0
				for _, c := range parts[1] {
					if c >= '0' && c <= '9' {
						p = p*10 + int(c-'0')
					}
				}
				if p > 0 {
					port = p
				}
			}

			return map[string]interface{}{
				"stats": map[string]interface{}{},
				"api": map[string]interface{}{
					"tag":      "api",
					"listen":   listenAddr,
					"services": []string{"StatsService"},
				},
				"policy": map[string]interface{}{
					"levels": map[string]interface{}{
						"0": map[string]interface{}{
							"statsUserUplink":   true,
							"statsUserDownlink": true,
						},
					},
					"system": map[string]interface{}{
						"statsInboundUplink":    true,
						"statsInboundDownlink":  true,
						"statsOutboundUplink":   true,
						"statsOutboundDownlink": true,
					},
				},
				"apiInbound": map[string]interface{}{
					"tag":      "api",
					"port":     port,
					"listen":   host,
					"protocol": "dokodemo-door",
					"settings": map[string]interface{}{
						"address": host,
					},
				},
				"apiRoutingRule": map[string]interface{}{
					"inboundTag":  []string{"api"},
					"outboundTag": "api",
				},
			}
		},

		// 生成 Xray 默认出站配置
		"xrayDefaultOutbounds": func() []map[string]interface{} {
			return []map[string]interface{}{
				{
					"protocol": "freedom",
					"tag":      "direct",
				},
				{
					"protocol": "blackhole",
					"tag":      "block",
				},
			}
		},

		// 生成 Xray 默认路由配置
		"xrayDefaultRouting": func(inbounds []InboundConfig) map[string]interface{} {
			inboundTags := make([]string, 0, len(inbounds))
			for _, in := range inbounds {
				if in.Tag != "" {
					inboundTags = append(inboundTags, in.Tag)
				}
			}

			return map[string]interface{}{
				"domainStrategy": "AsIs",
				"rules": []map[string]interface{}{
					{
						"type":        "field",
						"inboundTag":  []string{"api"},
						"outboundTag": "api",
					},
				},
			}
		},
	}
}
