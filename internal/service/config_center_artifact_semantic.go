package service

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/support/stringutil"
	"github.com/creamcroissant/mgpanel/internal/template"
)

func buildUnifiedInboundFromSemantic(tag string, semantic *inboundSemanticSpec, semanticObject map[string]any) (template.UnifiedInbound, error) {
	if semantic == nil {
		return template.UnifiedInbound{}, fmt.Errorf("semantic spec is nil")
	}

	inbound := template.UnifiedInbound{
		Tag:      normalizeTag(stringutil.FirstNonEmpty(tag, semantic.Tag)),
		Protocol: strings.TrimSpace(semantic.Protocol),
		Listen:   normalizeListen(semantic.Listen),
		Port:     semantic.Port,
	}

	if artifactRawObjectPresent(semantic.TLS) {
		tlsObject, err := artifactDecodeJSONObject(semantic.TLS)
		if err != nil {
			return template.UnifiedInbound{}, fmt.Errorf("parse semantic_spec.tls: %w", err)
		}
		inbound.TLS = artifactBuildUnifiedTLS(tlsObject)
	}
	if artifactRawObjectPresent(semantic.Transport) {
		transportObject, err := artifactDecodeJSONObject(semantic.Transport)
		if err != nil {
			return template.UnifiedInbound{}, fmt.Errorf("parse semantic_spec.transport: %w", err)
		}
		inbound.Transport = artifactBuildUnifiedTransport(transportObject)
	}
	if artifactRawObjectPresent(semantic.Multiplex) {
		multiplexObject, err := artifactDecodeJSONObject(semantic.Multiplex)
		if err != nil {
			return template.UnifiedInbound{}, fmt.Errorf("parse semantic_spec.multiplex: %w", err)
		}
		inbound.Multiplex = artifactBuildUnifiedMultiplex(multiplexObject)
	}
	if artifactRawObjectPresent(semantic.Sniffing) {
		sniffingObject, err := artifactDecodeJSONObject(semantic.Sniffing)
		if err != nil {
			return template.UnifiedInbound{}, fmt.Errorf("parse semantic_spec.sniffing: %w", err)
		}
		inbound.Sniffing = artifactBuildUnifiedSniffing(sniffingObject)
	}

	// Extract users and options from raw semanticObject (data not in typed struct fields)
	if semanticObject != nil {
		inbound.Users = artifactBuildUnifiedUsers(semanticObject)
		if optionsRaw, ok := semanticObject["options"]; ok {
			if optionsMap, ok := optionsRaw.(map[string]any); ok {
				inbound.Options = optionsMap
			}
		}
		// sockopt/fallbacks 经 _raw 透传：前端可视化填写后直通渲染器。
		if sockoptRaw, ok := semanticObject["sockopt"]; ok {
			if sockoptMap, ok := sockoptRaw.(map[string]any); ok && len(sockoptMap) > 0 {
				inbound.Sockopt = sockoptMap
			}
		}
		if fbRaw, ok := semanticObject["fallbacks"]; ok {
			if fbArr, ok := fbRaw.([]any); ok {
				for _, item := range fbArr {
					if m, ok := item.(map[string]any); ok && len(m) > 0 {
						inbound.Fallbacks = append(inbound.Fallbacks, m)
					}
				}
			}
		}
	}

	// Reality 密钥不下发：私钥/公钥/short_id 由每台 agent 按「配置」本地独立生成并注入
	// （见 internal/agent/secrets），面板只下发 reality 开关与伪装域名等公共参数。
	// 这样同一 spec 绑定多台机器时不会共用同一套密钥。
	if inbound.TLS != nil && inbound.TLS.Reality != nil {
		inbound.TLS.Reality.PrivateKey = ""
		inbound.TLS.Reality.PublicKey = ""
		inbound.TLS.Reality.ShortIDs = nil
	}

	return inbound, nil
}

func artifactBuildUnifiedTLS(raw map[string]any) *template.UnifiedTLS {
	if len(raw) == 0 {
		return nil
	}
	enabledValue, enabledFound := artifactLookupFirst(raw, "enabled")
	enabled := true
	if enabledFound {
		enabled = artifactToBool(enabledValue)
	}

	tls := &template.UnifiedTLS{
		Enabled:    enabled,
		ServerName: artifactStringByKeys(raw, "server_name", "serverName"),
		ALPN:       artifactStringSliceByKeys(raw, "alpn", "ALPN"),
		CertPath:   artifactStringByKeys(raw, "cert_path", "certPath", "certificateFile"),
		KeyPath:    artifactStringByKeys(raw, "key_path", "keyPath", "keyFile"),
	}

	realityRaw, ok := artifactMapByKeys(raw, "reality", "reality_settings", "realitySettings")
	if ok {
		// reality 的 private_key/public_key/short_ids 允许为空：
		// spec 不再持久化服务端密钥，由 agent 端 secrets 注入（见 docs/plans/20260828-inbound-v2rayapi.md P1）。
		// 非空时照旧透传以兼容存量 spec；渲染器仅在有值时输出对应字段。
		reality := &template.UnifiedReality{
			Enabled:         artifactToBoolWithDefault(artifactLookupFirstValue(realityRaw, "enabled"), true),
			PrivateKey:      artifactStringByKeys(realityRaw, "private_key", "privateKey"),
			PublicKey:       artifactStringByKeys(realityRaw, "public_key", "publicKey"),
			ShortIDs:        artifactStringSliceByKeys(realityRaw, "short_ids", "shortIds", "short_id", "shortId"),
			ServerNames:     artifactStringSliceByKeys(realityRaw, "server_names", "serverNames"),
			HandshakeServer: artifactStringByKeys(realityRaw, "handshake_server", "handshakeServer"),
			HandshakePort:   artifactIntByKeys(realityRaw, "handshake_port", "handshakePort"),
			Fingerprint:     artifactStringByKeys(realityRaw, "fingerprint"),
		}

		if len(reality.ServerNames) == 0 {
			singleServerName := artifactStringByKeys(realityRaw, "server_name", "serverName")
			if singleServerName != "" {
				reality.ServerNames = []string{singleServerName}
			}
		}
		if reality.HandshakeServer == "" {
			if dest := artifactStringByKeys(realityRaw, "dest"); dest != "" {
				host, port := artifactSplitHostPort(dest)
				reality.HandshakeServer = host
				if reality.HandshakePort == 0 {
					reality.HandshakePort = port
				}
			}
		}

		tls.Reality = reality
	}

	if !tls.Enabled && tls.ServerName == "" && len(tls.ALPN) == 0 && tls.CertPath == "" && tls.KeyPath == "" && tls.Reality == nil {
		return nil
	}
	return tls
}

func artifactBuildUnifiedTransport(raw map[string]any) *template.UnifiedTransport {
	if len(raw) == 0 {
		return nil
	}

	transport := &template.UnifiedTransport{
		Type:        strings.TrimSpace(artifactStringByKeys(raw, "type", "network")),
		Path:        artifactStringByKeys(raw, "path"),
		Host:        artifactStringByKeys(raw, "host"),
		ServiceName: artifactStringByKeys(raw, "service_name", "serviceName"),
		Headers:     artifactStringMapByKeys(raw, "headers"),
	}
	if transport.Type == "" {
		transport.Type = "tcp"
	}
	if transport.Host == "" {
		hostCandidates := artifactStringSliceByKeys(raw, "host")
		if len(hostCandidates) > 0 {
			transport.Host = hostCandidates[0]
		}
	}
	if len(transport.Headers) == 0 {
		transport.Headers = nil
	}

	return transport
}

// artifactBuildUnifiedMultiplex builds a UnifiedMultiplex from a decoded JSON object.
func artifactBuildUnifiedMultiplex(raw map[string]any) *template.UnifiedMultiplex {
	if len(raw) == 0 {
		return nil
	}
	multiplex := &template.UnifiedMultiplex{
		Enabled:    artifactToBoolWithDefault(artifactLookupFirstValue(raw, "enabled"), true),
		Protocol:   artifactStringByKeys(raw, "protocol"),
		MaxStreams: artifactIntByKeys(raw, "max_streams", "maxStreams"),
		Padding:    artifactToBoolWithDefault(artifactLookupFirstValue(raw, "padding"), false),
	}
	if brutalRaw, ok := artifactMapByKeys(raw, "brutal"); ok {
		multiplex.Brutal = &template.UnifiedBrutal{
			Enabled:  artifactToBoolWithDefault(artifactLookupFirstValue(brutalRaw, "enabled"), true),
			UpMbps:   artifactIntByKeys(brutalRaw, "up_mbps", "upMbps"),
			DownMbps: artifactIntByKeys(brutalRaw, "down_mbps", "downMbps"),
		}
	}
	return multiplex
}

// artifactBuildUnifiedSniffing builds a UnifiedSniffing from a decoded JSON object.
func artifactBuildUnifiedSniffing(raw map[string]any) *template.UnifiedSniffing {
	if len(raw) == 0 {
		return nil
	}
	return &template.UnifiedSniffing{
		Enabled:         artifactToBoolWithDefault(artifactLookupFirstValue(raw, "enabled"), true),
		DestOverride:    artifactStringSliceByKeys(raw, "dest_override", "destOverride"),
		MetadataOnly:    artifactToBoolWithDefault(artifactLookupFirstValue(raw, "metadata_only", "metadataOnly"), false),
		DomainsExcluded: artifactStringSliceByKeys(raw, "domains_excluded", "domainsExcluded"),
		RouteOnly:       artifactToBoolWithDefault(artifactLookupFirstValue(raw, "route_only", "routeOnly"), false),
	}
}

// artifactBuildUnifiedUsers extracts []UnifiedUser from a parsed semantic spec object.
// It looks for a "users" array where each entry has uuid/id, email/name, password, flow, method.
func artifactBuildUnifiedUsers(semanticObject map[string]any) []template.UnifiedUser {
	if semanticObject == nil {
		return nil
	}
	rawUsers, ok := semanticObject["users"]
	if !ok {
		return nil
	}
	usersArr, ok := rawUsers.([]any)
	if !ok {
		return nil
	}
	result := make([]template.UnifiedUser, 0, len(usersArr))
	for _, raw := range usersArr {
		userMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		user := template.UnifiedUser{
			UUID:     stringutil.FirstNonEmpty(artifactStringByKeys(userMap, "uuid", "id"), ""),
			Email:    artifactStringByKeys(userMap, "email", "name"),
			Password: artifactStringByKeys(userMap, "password", "pass"),
			Flow:     artifactStringByKeys(userMap, "flow"),
			Method:   artifactStringByKeys(userMap, "method"),
		}
		result = append(result, user)
	}
	return result
}

func artifactSingleInboundFromPayload(payload []byte) (map[string]any, error) {
	var document map[string]any
	if err := json.Unmarshal(payload, &document); err != nil {
		return nil, err
	}
	inboundsRaw, ok := document["inbounds"]
	if !ok {
		return nil, fmt.Errorf("render payload missing inbounds")
	}
	inboundsArray, ok := inboundsRaw.([]any)
	if !ok || len(inboundsArray) != 1 {
		return nil, fmt.Errorf("render payload must contain exactly one inbound")
	}
	inbound, ok := artifactToMap(inboundsArray[0])
	if !ok {
		return nil, fmt.Errorf("render payload inbound must be object")
	}
	return inbound, nil
}

func artifactMarshalInbound(inbound map[string]any) ([]byte, error) {
	document := map[string]any{
		"inbounds": []map[string]any{inbound},
	}
	return json.Marshal(document)
}

// cloneWithoutKey returns a shallow copy of m with key removed (key absent → same map).
// 用于渲染前剔除 v2ray_api 等仅由独立 fragment 承载的顶层键。
func cloneWithoutKey(m map[string]any, key string) map[string]any {
	if len(m) == 0 {
		return m
	}
	copied := make(map[string]any, len(m))
	for k, v := range m {
		if k == key {
			continue
		}
		copied[k] = v
	}
	return copied
}

func artifactHash(content []byte) string {
	h := md5.Sum(content)
	return hex.EncodeToString(h[:])
}

func artifactDecodeJSONObject(raw []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}, nil
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	if object == nil {
		object = map[string]any{}
	}
	return object, nil
}

func artifactRawObjectPresent(raw []byte) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return false
	}
	return string(trimmed) != "{}"
}

func artifactSemanticUnknownWarnings(coreType string, spec *repository.InboundSpec, semanticObject map[string]any, supported map[string]struct{}) []ArtifactRenderWarning {
	if len(semanticObject) == 0 {
		return nil
	}
	keys := make([]string, 0)
	for key := range semanticObject {
		if _, ok := supported[key]; ok {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	warnings := make([]ArtifactRenderWarning, 0, len(keys))
	for _, key := range keys {
		warnings = append(warnings, ArtifactRenderWarning{
			CoreType: coreType,
			SpecID:   spec.ID,
			Tag:      spec.Tag,
			Field:    fmt.Sprintf("semantic_spec.%s", key),
			Message:  "field is unsupported and ignored / 字段不受支持，已忽略",
		})
	}
	return warnings
}

func artifactExtractCoreSection(spec *repository.InboundSpec, coreType string, coreSpecific map[string]any, coreKeys ...string) (map[string]any, []ArtifactRenderWarning, error) {
	warnings := make([]ArtifactRenderWarning, 0)
	if len(coreSpecific) == 0 {
		return nil, warnings, nil
	}

	knownTop := map[string]struct{}{
		"core_type": {},
	}
	for _, key := range coreKeys {
		knownTop[key] = struct{}{}
	}

	unknownTop := make([]string, 0)
	for key := range coreSpecific {
		if _, ok := knownTop[key]; ok {
			continue
		}
		unknownTop = append(unknownTop, key)
	}
	sort.Strings(unknownTop)
	for _, key := range unknownTop {
		warnings = append(warnings, ArtifactRenderWarning{
			CoreType: coreType,
			SpecID:   spec.ID,
			Tag:      spec.Tag,
			Field:    fmt.Sprintf("core_specific.%s", key),
			Message:  "field is unsupported and ignored / 字段不受支持，已忽略",
		})
	}

	foundKeys := make([]string, 0, 1)
	for _, key := range coreKeys {
		if _, ok := coreSpecific[key]; ok {
			foundKeys = append(foundKeys, key)
		}
	}

	if len(foundKeys) == 0 {
		return nil, warnings, nil
	}
	if len(foundKeys) > 1 {
		return nil, warnings, &ArtifactUnsupportedFieldError{
			CoreType: coreType,
			SpecID:   spec.ID,
			Tag:      spec.Tag,
			Field:    "core_specific",
			Message:  fmt.Sprintf("ambiguous core section keys: %s / 核心扩展键冲突", strings.Join(foundKeys, ",")),
		}
	}

	sectionRaw := coreSpecific[foundKeys[0]]
	section, ok := artifactToMap(sectionRaw)
	if !ok {
		return nil, warnings, &ArtifactUnsupportedFieldError{
			CoreType: coreType,
			SpecID:   spec.ID,
			Tag:      spec.Tag,
			Field:    fmt.Sprintf("core_specific.%s", foundKeys[0]),
			Message:  "must be a JSON object / 必须是 JSON 对象",
		}
	}

	return section, warnings, nil
}

func artifactApplyCoreSection(
	inbound map[string]any,
	section map[string]any,
	reserved map[string]struct{},
	spec *repository.InboundSpec,
	coreType string,
	sectionField string,
) (string, error) {
	if len(section) == 0 {
		return "", nil
	}
	customFilename := ""

	keys := make([]string, 0, len(section))
	for key := range section {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := section[key]
		if key == "filename" {
			name, ok := value.(string)
			if !ok {
				return "", &ArtifactUnsupportedFieldError{
					CoreType: coreType,
					SpecID:   spec.ID,
					Tag:      spec.Tag,
					Field:    sectionField + ".filename",
					Message:  "must be a string / 必须是字符串",
				}
			}
			customFilename = strings.TrimSpace(name)
			continue
		}
		if _, exists := reserved[key]; exists {
			return "", &ArtifactUnsupportedFieldError{
				CoreType: coreType,
				SpecID:   spec.ID,
				Tag:      spec.Tag,
				Field:    sectionField + "." + key,
				Message:  "cannot override semantic core field / 不能覆盖语义层核心字段",
			}
		}
		inbound[key] = value
	}

	return customFilename, nil
}

func artifactResolveFilename(spec *repository.InboundSpec, customFilename string) (string, error) {
	if strings.TrimSpace(customFilename) != "" {
		if !artifactValidFilename(customFilename) {
			return "", &ArtifactUnsupportedFieldError{
				CoreType: spec.CoreType,
				SpecID:   spec.ID,
				Tag:      spec.Tag,
				Field:    "filename",
				Message:  "must be a safe .json filename / 必须是安全的 .json 文件名",
			}
		}
		return customFilename, nil
	}

	tag := normalizeTag(spec.Tag)
	if tag == "" {
		tag = "inbound"
	}
	safeTag := artifactFilenameSanitizer.ReplaceAllString(tag, "_")
	safeTag = strings.Trim(safeTag, "._-")
	if safeTag == "" {
		safeTag = "inbound"
	}
	return fmt.Sprintf("inbound-%d-%s.json", spec.ID, safeTag), nil
}

func artifactValidFilename(filename string) bool {
	trimmed := strings.TrimSpace(filename)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "/") || strings.Contains(trimmed, `\\`) {
		return false
	}
	if strings.Contains(trimmed, "..") {
		return false
	}
	if !strings.HasSuffix(strings.ToLower(trimmed), ".json") {
		return false
	}
	return true
}

func artifactLookupFirst(object map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		value, ok := object[key]
		if ok {
			return value, true
		}
	}
	return nil, false
}

func artifactLookupFirstValue(object map[string]any, keys ...string) any {
	value, _ := artifactLookupFirst(object, keys...)
	return value
}

func artifactStringByKeys(object map[string]any, keys ...string) string {
	value, ok := artifactLookupFirst(object, keys...)
	if !ok {
		return ""
	}
	return artifactToString(value)
}

func artifactIntByKeys(object map[string]any, keys ...string) int {
	value, ok := artifactLookupFirst(object, keys...)
	if !ok {
		return 0
	}
	return artifactToInt(value)
}

func artifactStringSliceByKeys(object map[string]any, keys ...string) []string {
	value, ok := artifactLookupFirst(object, keys...)
	if !ok {
		return nil
	}
	return artifactToStringSlice(value)
}

func artifactMapByKeys(object map[string]any, keys ...string) (map[string]any, bool) {
	value, ok := artifactLookupFirst(object, keys...)
	if !ok {
		return nil, false
	}
	mapped, ok := artifactToMap(value)
	return mapped, ok
}

func artifactStringMapByKeys(object map[string]any, keys ...string) map[string]string {
	value, ok := artifactLookupFirst(object, keys...)
	if !ok {
		return nil
	}
	mapped, ok := artifactToMap(value)
	if !ok {
		return nil
	}
	result := make(map[string]string, 0)
	for key, item := range mapped {
		text := artifactToString(item)
		if text == "" {
			continue
		}
		result[key] = text
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func artifactToMap(value any) (map[string]any, bool) {
	if value == nil {
		return nil, false
	}
	mapped, ok := value.(map[string]any)
	if ok {
		return mapped, true
	}
	mappedAny, ok := value.(map[string]interface{})
	if !ok {
		return nil, false
	}
	result := make(map[string]any, len(mappedAny))
	for key, item := range mappedAny {
		result[key] = item
	}
	return result, true
}

func artifactToBool(value any) bool {
	switch item := value.(type) {
	case bool:
		return item
	case string:
		return strings.EqualFold(strings.TrimSpace(item), "true")
	case float64:
		return item != 0
	case int:
		return item != 0
	case int64:
		return item != 0
	default:
		return false
	}
}

func artifactToBoolWithDefault(value any, defaultValue bool) bool {
	if value == nil {
		return defaultValue
	}
	return artifactToBool(value)
}

func artifactToInt(value any) int {
	switch item := value.(type) {
	case int:
		return item
	case int8:
		return int(item)
	case int16:
		return int(item)
	case int32:
		return int(item)
	case int64:
		return int(item)
	case float64:
		return int(item)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(item))
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func artifactToString(value any) string {
	switch item := value.(type) {
	case string:
		return strings.TrimSpace(item)
	case json.Number:
		return item.String()
	default:
		return ""
	}
}

func artifactToStringSlice(value any) []string {
	switch item := value.(type) {
	case []string:
		result := make([]string, 0, len(item))
		for _, part := range item {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			result = append(result, trimmed)
		}
		if len(result) == 0 {
			return nil
		}
		return result
	case []any:
		result := make([]string, 0, len(item))
		for _, part := range item {
			trimmed := artifactToString(part)
			if trimmed == "" {
				continue
			}
			result = append(result, trimmed)
		}
		if len(result) == 0 {
			return nil
		}
		return result
	case string:
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			return nil
		}
		return []string{trimmed}
	default:
		return nil
	}
}

func artifactSplitHostPort(dest string) (string, int) {
	trimmed := strings.TrimSpace(dest)
	if trimmed == "" {
		return "", 0
	}
	index := strings.LastIndex(trimmed, ":")
	if index <= 0 || index >= len(trimmed)-1 {
		return trimmed, 0
	}
	host := strings.TrimSpace(trimmed[:index])
	portText := strings.TrimSpace(trimmed[index+1:])
	port, err := strconv.Atoi(portText)
	if err != nil {
		return trimmed, 0
	}
	return host, port
}

// boolPtr returns a pointer to a bool value.
func boolPtr(v bool) *bool {
	return &v
}
