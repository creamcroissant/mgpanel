package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/agent/protocol"
)

var (
	ErrInboundSpecInvalid            = errors.New("service: invalid inbound spec / 入站配置无效")
	ErrInboundSpecTagConflict        = errors.New("service: inbound spec tag conflict / 入站标签冲突")
	ErrInboundSpecListenConflict     = errors.New("service: inbound spec listen conflict / 入站监听冲突")
	ErrInboundSpecProtocolIncompat   = errors.New("service: protocol not supported by target core / 协议不被目标核心支持")

	// sing-box 1.11+ 支持的入站协议
	singBoxInboundProtocols = map[string]struct{}{
		"direct": {}, "block": {}, "socks": {}, "http": {},
		"shadowsocks": {}, "vmess": {}, "trojan": {}, "vless": {},
		"hysteria": {}, "hysteria2": {}, "tuic": {}, "anytls": {}, "shadowtls": {},
		"dns": {}, "loopback": {}, "mixed": {}, "naive": {},
		"tun": {}, "redirect": {}, "tproxy": {}, "wireguard": {},
		"mtproto": {},
	}

	// xray 1.8+ 支持的入站协议
	xrayInboundProtocols = map[string]struct{}{
		"socks": {}, "http": {},
		"shadowsocks": {}, "vmess": {}, "trojan": {}, "vless": {},
		"hysteria2": {}, "tuic": {},
		"dokodemo-door": {}, "mixed": {},
	}
)

// isProtocolSupportedByCore checks whether a protocol is supported by the given core type.
func isProtocolSupportedByCore(protocol, coreType string) bool {
	normalized := normalizeCoreType(coreType)
	switch normalized {
	case "sing-box":
		_, ok := singBoxInboundProtocols[protocol]
		return ok
	case "xray":
		_, ok := xrayInboundProtocols[protocol]
		return ok
	default:
		return false
	}
}

// InboundSpecFieldViolation describes a deterministic field-level validation issue.
type InboundSpecFieldViolation struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// InboundSpecValidationError contains one or more field violations.
type InboundSpecValidationError struct {
	Violations []InboundSpecFieldViolation `json:"violations"`
}

func (e *InboundSpecValidationError) Error() string {
	if e == nil || len(e.Violations) == 0 {
		return ErrInboundSpecInvalid.Error()
	}
	parts := make([]string, 0, len(e.Violations))
	for _, item := range e.Violations {
		parts = append(parts, fmt.Sprintf("%s: %s", item.Field, item.Message))
	}
	return fmt.Sprintf("%s (%s)", ErrInboundSpecInvalid.Error(), strings.Join(parts, "; "))
}

func (e *InboundSpecValidationError) Is(target error) bool {
	return target == ErrInboundSpecInvalid
}

func (e *InboundSpecValidationError) add(field, message string) {
	if e == nil {
		return
	}
	e.Violations = append(e.Violations, InboundSpecFieldViolation{Field: field, Message: message})
}

func (e *InboundSpecValidationError) hasViolations() bool {
	return e != nil && len(e.Violations) > 0
}

// InboundSpecConflictError represents deterministic conflict details.
type InboundSpecConflictError struct {
	Kind           string `json:"kind"`
	Field          string `json:"field"`
	Value          string `json:"value"`
	ExistingSpecID int64  `json:"existing_spec_id"`
	ExistingTag    string `json:"existing_tag,omitempty"`
}

func (e *InboundSpecConflictError) Error() string {
	if e == nil {
		return ""
	}
	switch e.Kind {
	case "tag":
		return fmt.Sprintf("%s (field=%s value=%s existing_spec_id=%d)", ErrInboundSpecTagConflict.Error(), e.Field, e.Value, e.ExistingSpecID)
	case "listen":
		return fmt.Sprintf("%s (field=%s value=%s existing_spec_id=%d existing_tag=%s)", ErrInboundSpecListenConflict.Error(), e.Field, e.Value, e.ExistingSpecID, e.ExistingTag)
	default:
		return fmt.Sprintf("inbound spec conflict (field=%s value=%s existing_spec_id=%d)", e.Field, e.Value, e.ExistingSpecID)
	}
}

func (e *InboundSpecConflictError) Is(target error) bool {
	if e == nil {
		return false
	}
	if target == ErrInboundSpecTagConflict {
		return e.Kind == "tag"
	}
	if target == ErrInboundSpecListenConflict {
		return e.Kind == "listen"
	}
	return false
}

type inboundSemanticSpec struct {
	Tag       string          `json:"tag,omitempty"`
	Protocol  string          `json:"protocol"`
	Listen    string          `json:"listen"`
	Port      int             `json:"port"`
	TLS       json.RawMessage `json:"tls,omitempty"`
	Transport json.RawMessage `json:"transport,omitempty"`
	Multiplex json.RawMessage `json:"multiplex,omitempty"`
	Sniffing  json.RawMessage `json:"sniffing,omitempty"`
}

func normalizeCoreType(input string) string {
	return protocol.NormalizeCoreType(input)
}

func normalizeTag(input string) string {
	return strings.TrimSpace(input)
}

func normalizeListen(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	if ip := net.ParseIP(trimmed); ip != nil {
		return ip.String()
	}
	return strings.ToLower(trimmed)
}

func canonicalizeJSONObject(raw json.RawMessage, fieldName string) (json.RawMessage, map[string]any, *InboundSpecValidationError) {
	violations := &InboundSpecValidationError{}
	normalized := raw
	if len(strings.TrimSpace(string(normalized))) == 0 {
		normalized = json.RawMessage("{}")
	}

	var object map[string]any
	if err := json.Unmarshal(normalized, &object); err != nil {
		violations.add(fieldName, "must be a valid JSON object / 必须是合法 JSON 对象")
		return nil, nil, violations
	}
	if object == nil {
		object = map[string]any{}
	}

	canonical, err := json.Marshal(object)
	if err != nil {
		violations.add(fieldName, "failed to canonicalize JSON object / JSON 对象规范化失败")
		return nil, nil, violations
	}
	return canonical, object, nil
}

func parseSemanticSpec(raw json.RawMessage) (*inboundSemanticSpec, error) {
	var spec inboundSemanticSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, err
	}
	spec.Tag = normalizeTag(spec.Tag)
	spec.Protocol = strings.TrimSpace(spec.Protocol)
	spec.Listen = normalizeListen(spec.Listen)
	return &spec, nil
}

func validateSpecInput(coreType, tag string, semanticSpec json.RawMessage, coreSpecific json.RawMessage) (json.RawMessage, json.RawMessage, *inboundSemanticSpec, error) {
	validationErr := &InboundSpecValidationError{}

	if normalizeCoreType(coreType) == "" {
		validationErr.add("core_type", "must be sing-box or xray / 必须是 sing-box 或 xray")
	}
	if normalizeTag(tag) == "" {
		validationErr.add("tag", "is required / 不能为空")
	}

	normalizedSemantic, _, semanticObjErr := canonicalizeJSONObject(semanticSpec, "semantic_spec")
	if semanticObjErr != nil {
		validationErr.Violations = append(validationErr.Violations, semanticObjErr.Violations...)
	}

	var semantic *inboundSemanticSpec
	if normalizedSemantic != nil {
		parsed, err := parseSemanticSpec(normalizedSemantic)
		if err != nil {
			validationErr.add("semantic_spec", "must be a valid object / 必须是合法对象")
		} else {
			semantic = parsed
			if parsed.Protocol == "" {
				validationErr.add("semantic_spec.protocol", "is required / 不能为空")
			} else if !isProtocolSupportedByCore(parsed.Protocol, coreType) {
				validationErr.add("semantic_spec.protocol", fmt.Sprintf(
					"protocol %q is not supported by %s / 协议 %q 不被 %s 支持",
					parsed.Protocol, coreType, parsed.Protocol, coreType,
				))
			}
			if parsed.Listen == "" {
				validationErr.add("semantic_spec.listen", "is required / 不能为空")
			}
			if parsed.Port <= 0 || parsed.Port > 65535 {
				validationErr.add("semantic_spec.port", "must be between 1 and 65535 / 必须在 1-65535 之间")
			}
			if parsed.Tag != "" && parsed.Tag != normalizeTag(tag) {
				validationErr.add("semantic_spec.tag", "must match request tag / 必须与请求 tag 一致")
			}
		}
	}

	normalizedCoreSpecific, coreSpecificObj, coreSpecificErr := canonicalizeJSONObject(coreSpecific, "core_specific")
	if coreSpecificErr != nil {
		validationErr.Violations = append(validationErr.Violations, coreSpecificErr.Violations...)
	}

	if coreSpecificObj != nil {
		normalizedCore := normalizeCoreType(coreType)
		if value, ok := coreSpecificObj["core_type"]; ok {
			valueCore := normalizeCoreType(fmt.Sprintf("%v", value))
			if valueCore == "" || valueCore != normalizedCore {
				validationErr.add("core_specific.core_type", "must match request core_type / 必须与请求 core_type 一致")
			}
		}

		if _, ok := coreSpecificObj["xray"]; ok && normalizedCore != "xray" {
			validationErr.add("core_specific.xray", "scope does not match request core_type / 作用域与请求 core_type 不一致")
		}
		if _, ok := coreSpecificObj["sing-box"]; ok && normalizedCore != "sing-box" {
			validationErr.add("core_specific.sing-box", "scope does not match request core_type / 作用域与请求 core_type 不一致")
		}
		if _, ok := coreSpecificObj["singbox"]; ok && normalizedCore != "sing-box" {
			validationErr.add("core_specific.singbox", "scope does not match request core_type / 作用域与请求 core_type 不一致")
		}
	}

	if validationErr.hasViolations() {
		return nil, nil, nil, validationErr
	}
	return normalizedSemantic, normalizedCoreSpecific, semantic, nil
}
