// 文件路径: internal/protocol/base.go
// 模块说明: 这是 internal 模块里的 base 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package protocol

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type BaseBuilder struct {
	allowed      map[string]struct{}
	requirements map[string]map[string][]fieldRequirement
}

type fieldRequirement struct {
	path    string
	allowed map[string]string
	strict  bool
}

func NewBaseBuilder() *BaseBuilder {
	return &BaseBuilder{
		allowed:      make(map[string]struct{}),
		requirements: make(map[string]map[string][]fieldRequirement),
	}
}

func (b *BaseBuilder) Allow(types ...string) {
	if b == nil {
		return
	}
	for _, t := range types {
		normalized := strings.ToLower(strings.TrimSpace(t))
		if normalized == "" {
			continue
		}
		b.allowed[normalized] = struct{}{}
	}
}

func (b *BaseBuilder) AddRequirement(client, serverType, path string, allowed map[string]string, strict bool) {
	if b == nil || len(allowed) == 0 {
		return
	}
	clientKey := strings.ToLower(strings.TrimSpace(client))
	if clientKey == "" {
		clientKey = "*"
	}
	typeKey := strings.ToLower(strings.TrimSpace(serverType))
	if typeKey == "" {
		return
	}
	rule := fieldRequirement{
		path:    strings.TrimSpace(path),
		allowed: normalizeAllowedMap(allowed),
		strict:  strict,
	}
	if rule.path == "" {
		return
	}
	if _, ok := b.requirements[clientKey]; !ok {
		b.requirements[clientKey] = make(map[string][]fieldRequirement)
	}
	b.requirements[clientKey][typeKey] = append(b.requirements[clientKey][typeKey], rule)
}

func (b *BaseBuilder) FilterNodes(req BuildRequest) []Node {
	if b == nil {
		return req.Nodes
	}
	nodes := make([]Node, 0, len(req.Nodes))
	for _, node := range req.Nodes {
		if node.Type == "" {
			continue
		}
		if len(b.allowed) > 0 {
			if _, ok := b.allowed[strings.ToLower(node.Type)]; !ok {
				continue
			}
		}
		if !b.isCompatible(node, req.ClientName, req.ClientVersion) {
			continue
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func (b *BaseBuilder) isCompatible(node Node, clientName, clientVersion string) bool {
	if b == nil || len(b.requirements) == 0 {
		return true
	}
	if !b.requirementsSatisfied("*", node, clientVersion) {
		return false
	}
	name := strings.ToLower(strings.TrimSpace(clientName))
	if name == "" {
		return true
	}
	return b.requirementsSatisfied(name, node, clientVersion)
}

func (b *BaseBuilder) requirementsSatisfied(clientKey string, node Node, clientVersion string) bool {
	if b == nil {
		return true
	}
	clientRules, ok := b.requirements[clientKey]
	if !ok {
		return true
	}
	rules := clientRules[strings.ToLower(node.Type)]
	for _, rule := range rules {
		if !rule.matches(node, clientVersion) {
			return false
		}
	}
	return true
}

func (r fieldRequirement) matches(node Node, clientVersion string) bool {
	if r.path == "" {
		return true
	}
	value := nodeValue(node, r.path)
	if value == "" {
		return !r.strict
	}
	versionRequired, ok := r.allowed[value]
	if !ok {
		return !r.strict
	}
	if versionRequired == "" || versionRequired == "0.0.0" || clientVersion == "" {
		return true
	}
	return !versionLess(clientVersion, versionRequired)
}

func normalizeAllowedMap(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		dst[key] = strings.TrimSpace(v)
	}
	return dst
}

func nodeValue(node Node, path string) string {
	data := map[string]any{
		"id":                node.ID,
		"name":              node.Name,
		"type":              node.Type,
		"host":              node.Host,
		"port":              node.Port,
		"rate":              node.Rate,
		"protocol_settings": node.Settings,
	}
	return stringify(nestedLookup(data, path))
}

func nestedLookup(obj any, path string) any {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	segments := strings.Split(path, ".")
	current := obj
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = m[segment]
		if !ok {
			return nil
		}
	}
	return current
}

func stringify(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case fmt.Stringer:
		return v.String()
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func versionLess(current, required string) bool {
	cParts := splitVersion(current)
	rParts := splitVersion(required)
	maxLen := len(cParts)
	if len(rParts) > maxLen {
		maxLen = len(rParts)
	}
	for i := 0; i < maxLen; i++ {
		var cVal, rVal int
		if i < len(cParts) {
			cVal = cParts[i]
		}
		if i < len(rParts) {
			rVal = rParts[i]
		}
		if cVal < rVal {
			return true
		}
		if cVal > rVal {
			return false
		}
	}
	return false
}

func splitVersion(input string) []int {
	trimmed := strings.TrimSpace(strings.TrimPrefix(input, "v"))
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ".")
	values := make([]int, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		if val, err := strconv.Atoi(part); err == nil {
			values = append(values, val)
		} else {
			values = append(values, 0)
		}
	}
	return values
}

// End of helper functions
