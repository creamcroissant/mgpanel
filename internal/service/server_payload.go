// 文件路径: internal/service/server_payload.go
// 模块说明: 这是 internal 模块里的 server_payload 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"crypto/md5"
	crand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"strconv"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/plugin/hook"
	"github.com/creamcroissant/mgpanel/internal/protocol"
	"github.com/creamcroissant/mgpanel/internal/repository"
)

func buildProtocolNodes(servers []*repository.Server, user *repository.User) []protocol.Node {
	if len(servers) == 0 {
		return []protocol.Node{}
	}
	uuid := ensureUserUUID(user)
	nodes := make([]protocol.Node, 0, len(servers))
	for _, server := range servers {
		if server == nil {
			continue
		}
		settings := decodeNodeSettings(server.Settings)
		port, portRange := resolveServerPort(server, settings)
		nType := strings.ToLower(server.Type)
		if p := strings.ToLower(asString(settings["protocol"])); p != "" {
			nType = p
		}
		nodes = append(nodes, protocol.Node{
			ID:          server.ID,
			Name:        server.Name,
			Type:        nType,
			Host:        server.Host,
			Port:        port,
			ServerPort:  server.ServerPort,
			Rate:        server.Rate,
			Tags:        decodeStringArray(server.Tags),
			Ports:       portRange,
			Settings:    settings,
			RawSettings: cloneRawMessage(server.Settings),
			Password:    deriveServerPassword(server, uuid, settings),
		})
	}
	return nodes
}

func decodeNodeSettings(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	// 优先解析为 map（旧格式）
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err == nil {
		return payload
	}
	// 兼容数组格式（agent 上报的 protocol details 数组）
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return flattenProtocolDetails(arr[0])
	}
	return map[string]any{}
}

// flattenProtocolDetails 将嵌套的 protocol detail 扁平化为订阅渲染需要的 key 格式。
// 例如 {tls:{enabled:true, server_name:"x"}} → {tls:true, tls_settings.server_name:"x"}
func flattenProtocolDetails(detail map[string]any) map[string]any {
	out := make(map[string]any, len(detail)+8)
	for k, v := range detail {
		out[k] = v
	}
	// TLS 扁平化：渲染器期望嵌套结构 tls_settings.server_name 等
	if tls, ok := detail["tls"].(map[string]any); ok {
		out["tls"] = true
		if v, ok := tls["enabled"].(bool); ok && !v {
			out["tls"] = false
		}
		tlsSettings := make(map[string]any, 8)
		if sn, ok := tls["server_name"].(string); ok {
			tlsSettings["server_name"] = sn
		}
		if ai, ok := tls["allow_insecure"].(bool); ok {
			tlsSettings["allow_insecure"] = ai
		}
		// Reality 扁平化
		if reality, ok := tls["reality"].(map[string]any); ok {
			if reality != nil {
				out["reality"] = true
				if pk, ok := reality["public_key"].(string); ok {
					tlsSettings["public_key"] = pk
				}
				if sid, ok := reality["short_id"].(string); ok {
					tlsSettings["short_id"] = sid
				} else if sidArr, ok := reality["short_ids"].([]any); ok && len(sidArr) > 0 {
					if s, ok := sidArr[0].(string); ok {
						tlsSettings["short_id"] = s
					}
				}
				if fp, ok := reality["fingerprint"].(string); ok {
					tlsSettings["fingerprint"] = fp
				}
				if sn, ok := reality["server_name"].(string); ok {
					tlsSettings["server_name"] = sn
				}
			}
		}
		if len(tlsSettings) > 0 {
			out["tls_settings"] = tlsSettings
		}
	}
	// 从 users 数组提取第一个用户的 flow
	if users, ok := detail["users"].([]any); ok && len(users) > 0 {
		if u, ok := users[0].(map[string]any); ok {
			if flow, ok := u["flow"].(string); ok {
				out["flow"] = flow
			}
		}
	}
	// 网络/传输扁平化
	if transport, ok := detail["transport"].(map[string]any); ok {
		if tp, ok := transport["type"].(string); ok {
			out["network"] = tp
			out["network_settings"] = transport
		}
	}
	return out
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	dup := make([]byte, len(raw))
	copy(dup, raw)
	return json.RawMessage(dup)
}

func resolveServerPort(server *repository.Server, settings map[string]any) (int, string) {
	if server == nil {
		return 0, ""
	}
	if portRange := portRangeFromSettings(settings); portRange != "" {
		if port, ok := randomPortFromRange(portRange); ok {
			return port, portRange
		}
	}
	return server.Port, ""
}

func portRangeFromSettings(settings map[string]any) string {
	if len(settings) == 0 {
		return ""
	}
	for _, key := range []string{"ports", "port_range", "server_ports"} {
		if val, ok := settings[key]; ok {
			if str := strings.TrimSpace(asString(val)); strings.Contains(str, "-") {
				return str
			}
		}
	}
	return ""
}

func randomPortFromRange(rangeStr string) (int, bool) {
	parts := strings.Split(rangeStr, "-")
	if len(parts) != 2 {
		return 0, false
	}
	min, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, false
	}
	max, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || max < min {
		return 0, false
	}
	delta := max - min + 1
	nBig, err := crand.Int(crand.Reader, big.NewInt(int64(delta)))
	if err != nil {
		return 0, false
	}
	return min + int(nBig.Int64()), true
}

func deriveServerPassword(server *repository.Server, userUUID string, settings map[string]any) string {
	if server == nil {
		return userUUID
	}
	if strings.ToLower(server.Type) != "shadowsocks" {
		return userUUID
	}
	cipher := strings.TrimSpace(server.Cipher)
	if cipher == "" {
		cipher = strings.TrimSpace(asString(settings["cipher"]))
	}
	config, ok := shadowCipherConfig[cipher]
	if !ok {
		return userUUID
	}
	serverKey := buildServerKey(server.CreatedAt, config.serverKeySize)
	userKey := truncateAndBase64(userUUID, config.userKeySize)
	if serverKey == "" || userKey == "" {
		return userUUID
	}
	return fmt.Sprintf("%s:%s", serverKey, userKey)
}

func ensureUserUUID(user *repository.User) string {
	if user == nil {
		return ""
	}
	uuid := strings.TrimSpace(user.UUID)
	if uuid == "" {
		if user.ID == 0 {
			return ""
		}
		return fmt.Sprintf("user-%d", user.ID)
	}
	return uuid
}

func buildServerKey(timestamp int64, length int) string {
	if timestamp <= 0 || length <= 0 {
		return ""
	}
	hash := md5.Sum([]byte(strconv.FormatInt(timestamp, 10)))
	hexStr := fmt.Sprintf("%x", hash[:])
	if length > len(hexStr) {
		length = len(hexStr)
	}
	segment := hexStr[:length]
	return base64.StdEncoding.EncodeToString([]byte(segment))
}

func truncateAndBase64(src string, length int) string {
	if length <= 0 || src == "" {
		return ""
	}
	runes := []rune(src)
	if length > len(runes) {
		length = len(runes)
	}
	slice := runes[:length]
	return base64.StdEncoding.EncodeToString([]byte(string(slice)))
}

func asString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case fmt.Stringer:
		return v.String()
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	default:
		return ""
	}
}

var shadowCipherConfig = map[string]struct {
	serverKeySize int
	userKeySize   int
}{
	"2022-blake3-aes-128-gcm":       {serverKeySize: 16, userKeySize: 16},
	"2022-blake3-aes-256-gcm":       {serverKeySize: 32, userKeySize: 32},
	"2022-blake3-chacha20-poly1305": {serverKeySize: 32, userKeySize: 32},
}

type protocolServerHookPayload struct {
	User    *repository.User
	Servers []*repository.Server
}

func applyProtocolServerHooks(ctx context.Context, servers []*repository.Server, user *repository.User) []*repository.Server {
	payload := &protocolServerHookPayload{User: user, Servers: servers}
	result, err := hook.Apply(ctx, "protocol.servers.filtered", payload)
	if err != nil {
		slog.Default().Warn("protocol hook failed", "hook", "protocol.servers.filtered", "error", err)
		return servers
	}
	switch v := result.(type) {
	case *protocolServerHookPayload:
		if v != nil {
			return v.Servers
		}
	case protocolServerHookPayload:
		return v.Servers
	case []*repository.Server:
		return v
	default:
		return servers
	}
	return servers
}
