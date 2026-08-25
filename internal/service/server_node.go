// 文件路径: internal/service/server_node.go
// 模块说明: 这是 internal 模块里的 server_node 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// ServerNodeService exposes node configuration and user listings consumed by agents.
type ServerNodeService interface {
	Users(ctx context.Context, server *repository.Server) (*ServerNodeUsersResult, error)
	Config(ctx context.Context, server *repository.Server) (*ServerNodeConfigResult, error)
}

// ServerNodeUsersResult wraps the UniProxy-compatible user payload.
type ServerNodeUsersResult struct {
	Users []ServerNodeUser
	ETag  string
}

// ServerNodeUser mirrors the limited columns exposed to nodes.
type ServerNodeUser struct {
	ID          int64  `json:"id"`
	UUID        string `json:"uuid"`
	SpeedLimit  *int64 `json:"speed_limit,omitempty"`
	DeviceLimit *int64 `json:"device_limit,omitempty"`
}

// ServerNodeConfigResult captures the rendered config payload.
type ServerNodeConfigResult struct {
	Payload map[string]any
	ETag    string
}

type serverNodeService struct {
	users    repository.UserRepository
	routes   repository.ServerRouteRepository
	settings repository.SettingRepository
}

// NewServerNodeService constructs a node-facing service backed by repositories.
func NewServerNodeService(users repository.UserRepository, routes repository.ServerRouteRepository, settings repository.SettingRepository) ServerNodeService {
	return &serverNodeService{users: users, routes: routes, settings: settings}
}

func (s *serverNodeService) Users(ctx context.Context, server *repository.Server) (*ServerNodeUsersResult, error) {
	if server == nil {
		return nil, ErrNotFound
	}
	if s == nil || s.users == nil {
		return nil, errors.New("server node: user repository unavailable / 节点用户仓库不可用")
	}
	groupIDs := serverGroupIDs(server)
	now := time.Now().Unix()
	var repoUsers []*repository.NodeUser
	if len(groupIDs) > 0 {
		users, err := s.users.ListActiveForGroups(ctx, groupIDs, now)
		if err != nil {
			return nil, err
		}
		repoUsers = users
	}
	payload := make([]ServerNodeUser, 0, len(repoUsers))
	for _, user := range repoUsers {
		if user == nil {
			continue
		}
		payload = append(payload, ServerNodeUser{
			ID:          user.ID,
			UUID:        user.UUID,
			SpeedLimit:  cloneOptionalInt64(user.SpeedLimit),
			DeviceLimit: cloneOptionalInt64(user.DeviceLimit),
		})
	}
	eTag := hashServerUsersETag(server, repoUsers)
	return &ServerNodeUsersResult{Users: payload, ETag: eTag}, nil
}

func (s *serverNodeService) Config(ctx context.Context, server *repository.Server) (*ServerNodeConfigResult, error) {
	if server == nil {
		return nil, ErrNotFound
	}
	settings := decodeNodeSettings(server.Settings)
	payload := buildServerConfigPayload(server, settings)
	intervals := nodeIntervalConfig{
		Push: s.intervalSetting(ctx, "server_push_interval", 60),
		Pull: s.intervalSetting(ctx, "server_pull_interval", 60),
	}
	payload["base_config"] = map[string]int{
		"push_interval": intervals.Push,
		"pull_interval": intervals.Pull,
	}
	routes, err := s.fetchRoutes(ctx, server, settings)
	if err != nil {
		return nil, err
	}
	if len(routes) > 0 {
		payload["routes"] = serializeServerRoutes(routes)
	}
	eTag := hashServerConfigETag(server, intervals, routes)
	return &ServerNodeConfigResult{Payload: payload, ETag: eTag}, nil
}

func (s *serverNodeService) intervalSetting(ctx context.Context, key string, fallback int) int {
	if s == nil || s.settings == nil {
		return fallback
	}
	setting, err := s.settings.Get(ctx, key)
	if err != nil || setting == nil {
		return fallback
	}
	if value, ok := parseInt(setting.Value); ok && value >= 0 {
		return value
	}
	return fallback
}

func (s *serverNodeService) fetchRoutes(ctx context.Context, server *repository.Server, settings map[string]any) ([]*repository.ServerRoute, error) {
	if s == nil || s.routes == nil {
		return nil, nil
	}
	routeIDs := extractRouteIDs(server, settings)
	if len(routeIDs) == 0 {
		return nil, nil
	}
	return s.routes.FindByIDs(ctx, routeIDs)
}

type nodeIntervalConfig struct {
	Push int
	Pull int
}

func buildServerConfigPayload(server *repository.Server, settings map[string]any) map[string]any {
	nodeType := serverNodeType(server)
	config := map[string]any{
		"protocol":        nodeType,
		"listen_ip":       "0.0.0.0",
		"server_port":     effectiveServerPort(server),
		"network":         nil,
		"networkSettings": nil,
	}
	if netVal, ok := settings["network"]; ok {
		config["network"] = netVal
	}
	if ns, ok := settings["network_settings"]; ok && ns != nil {
		config["networkSettings"] = ns
	}
	switch nodeType {
	case "shadowsocks":
		cipher := firstNonEmpty(asString(settings["cipher"]), server.Cipher)
		config["cipher"] = cipher
		config["plugin"] = asString(settings["plugin"])
		config["plugin_opts"] = asString(settings["plugin_opts"])
		config["server_key"] = serverNodeKey(cipher, server)
	case "vmess":
		config["tls"] = toInt(settings["tls"])
	case "trojan":
		config["host"] = server.Host
		config["server_name"] = asString(settings["server_name"])
	case "vless":
		tlsMode := toInt(settings["tls"])
		config["tls"] = tlsMode
		config["flow"] = asString(settings["flow"])
		if tlsMode == 2 {
			config["tls_settings"] = settings["reality_settings"]
		} else {
			config["tls_settings"] = settings["tls_settings"]
		}
	case "hysteria":
		config["server_port"] = effectiveServerPort(server)
		config["version"] = toInt(settings["version"])
		config["host"] = server.Host
		tlsSettings := asMap(settings["tls"])
		config["server_name"] = stringFromMap(tlsSettings, "server_name")
		bandwidth := asMap(settings["bandwidth"])
		config["up_mbps"] = toInt(bandwidth["up"])
		config["down_mbps"] = toInt(bandwidth["down"])
		obfs := asMap(settings["obfs"])
		version := toInt(settings["version"])
		if version == 1 {
			config["obfs"] = stringFromMap(obfs, "password")
		} else {
			if boolFromMap(obfs, "open") {
				config["obfs"] = stringFromMap(obfs, "type")
			}
			config["obfs-password"] = stringFromMap(obfs, "password")
		}
	case "tuic":
		config["version"] = toInt(settings["version"])
		config["server_port"] = effectiveServerPort(server)
		tlsSettings := asMap(settings["tls"])
		config["server_name"] = stringFromMap(tlsSettings, "server_name")
		config["congestion_control"] = asString(settings["congestion_control"])
		config["auth_timeout"] = "3s"
		config["zero_rtt_handshake"] = false
		config["heartbeat"] = "3s"
	case "anytls":
		config["server_port"] = effectiveServerPort(server)
		tlsSettings := asMap(settings["tls"])
		config["server_name"] = stringFromMap(tlsSettings, "server_name")
		config["padding_scheme"] = settings["padding_scheme"]
	case "socks":
		config["server_port"] = effectiveServerPort(server)
		config["tls"] = toInt(settings["tls"])
		config["tls_settings"] = settings["tls_settings"]
	case "naive":
		config["server_port"] = effectiveServerPort(server)
		config["tls"] = toInt(settings["tls"])
		config["tls_settings"] = settings["tls_settings"]
	case "http":
		config["server_port"] = effectiveServerPort(server)
		config["tls"] = toInt(settings["tls"])
		config["tls_settings"] = settings["tls_settings"]
	case "mieru":
		config["server_port"] = strconv.Itoa(effectiveServerPort(server))
		config["protocol"] = toInt(settings["protocol"])
	}
	return config
}

func serverNodeKey(cipher string, server *repository.Server) string {
	normalized := strings.TrimSpace(strings.ToLower(cipher))
	cfg, ok := shadowCipherConfig[normalized]
	if !ok || server == nil {
		return ""
	}
	return buildServerKey(server.CreatedAt, cfg.serverKeySize)
}

func serializeServerRoutes(routes []*repository.ServerRoute) []map[string]any {
	result := make([]map[string]any, 0, len(routes))
	for _, route := range routes {
		if route == nil {
			continue
		}
		result = append(result, map[string]any{
			"id":           route.ID,
			"remarks":      route.Remarks,
			"match":        cloneRawMessage(route.Match),
			"action":       route.Action,
			"action_value": route.ActionValue,
		})
	}
	return result
}

func serverGroupIDs(server *repository.Server) []int64 {
	if server == nil || server.GroupID <= 0 {
		return nil
	}
	return []int64{server.GroupID}
}

func serverNodeType(server *repository.Server) string {
	if server == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(server.Type))
}

func effectiveServerPort(server *repository.Server) int {
	if server == nil {
		return 0
	}
	if server.ServerPort > 0 {
		return server.ServerPort
	}
	return server.Port
}

func cloneOptionalInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func asMap(value any) map[string]any {
	if m, ok := value.(map[string]any); ok {
		return m
	}
	if m, ok := value.(map[string]interface{}); ok {
		return m
	}
	return nil
}

func stringFromMap(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	return asString(m[key])
}

func boolFromMap(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	return toBool(m[key])
}

func toBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		trimmed := strings.TrimSpace(strings.ToLower(v))
		return trimmed == "1" || trimmed == "true" || trimmed == "yes"
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i != 0
		}
	}
	return false
}

func toInt(value any) int {
	if v, ok := toInt64(value); ok {
		return int(v)
	}
	return 0
}

func toInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		return int64(v), true
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i, true
		}
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, false
		}
		if i, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			return i, true
		}
	}
	return 0, false
}

func parseInt(raw string) (int, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	if i, err := strconv.Atoi(trimmed); err == nil {
		return i, true
	}
	var decoded any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil {
		return toInt(decoded), true
	}
	return 0, false
}

func extractRouteIDs(server *repository.Server, settings map[string]any) []int64 {
	ids := make(map[int64]struct{})
	if server != nil && server.RouteID > 0 {
		ids[server.RouteID] = struct{}{}
	}
	if raw, ok := settings["route_ids"]; ok {
		for _, id := range coerceInt64Slice(raw) {
			if id > 0 {
				ids[id] = struct{}{}
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	result := make([]int64, 0, len(ids))
	for id := range ids {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func coerceInt64Slice(value any) []int64 {
	switch v := value.(type) {
	case []int:
		result := make([]int64, 0, len(v))
		for _, val := range v {
			result = append(result, int64(val))
		}
		return result
	case []int64:
		result := make([]int64, len(v))
		copy(result, v)
		return result
	case []float64:
		result := make([]int64, 0, len(v))
		for _, val := range v {
			result = append(result, int64(val))
		}
		return result
	case []string:
		result := make([]int64, 0, len(v))
		for _, val := range v {
			if parsed, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64); err == nil {
				result = append(result, parsed)
			}
		}
		return result
	case []any:
		result := make([]int64, 0, len(v))
		for _, item := range v {
			if parsed, ok := toInt64(item); ok {
				result = append(result, parsed)
			}
		}
		return result
	case json.RawMessage:
		var decoded any
		if err := json.Unmarshal(v, &decoded); err == nil {
			return coerceInt64Slice(decoded)
		}
	case string:
		trimmed := strings.TrimSpace(v)
		if strings.HasPrefix(trimmed, "[") {
			var decoded any
			if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil {
				return coerceInt64Slice(decoded)
			}
		}
		if parsed, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			return []int64{parsed}
		}
	}
	return nil
}

func hashServerUsersETag(server *repository.Server, users []*repository.NodeUser) string {
	hasher := sha1.New()
	fmt.Fprintf(hasher, "srv:%d:g:%d", server.ID, server.GroupID)
	for _, user := range users {
		if user == nil {
			continue
		}
		fmt.Fprintf(hasher, "|u:%d:%s", user.ID, strings.TrimSpace(user.UUID))
		if user.SpeedLimit != nil {
			fmt.Fprintf(hasher, ":s:%d", *user.SpeedLimit)
		}
		if user.DeviceLimit != nil {
			fmt.Fprintf(hasher, ":d:%d", *user.DeviceLimit)
		}
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func hashServerConfigETag(server *repository.Server, intervals nodeIntervalConfig, routes []*repository.ServerRoute) string {
	hasher := sha1.New()
	fmt.Fprintf(hasher, "srv:%d:%s:%d:%d:%d:%d", server.ID, serverNodeType(server), server.Port, server.ServerPort, server.UpdatedAt, intervals.Push)
	fmt.Fprintf(hasher, ":pull:%d", intervals.Pull)
	if len(server.Settings) > 0 {
		hasher.Write(server.Settings)
	}
	if len(server.ObfsSettings) > 0 {
		hasher.Write(server.ObfsSettings)
	}
	for _, route := range routes {
		if route == nil {
			continue
		}
		fmt.Fprintf(hasher, "|route:%d:%d", route.ID, route.UpdatedAt)
		if len(route.Match) > 0 {
			hasher.Write(route.Match)
		}
		hasher.Write([]byte(route.Action))
		hasher.Write([]byte(route.ActionValue))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}
