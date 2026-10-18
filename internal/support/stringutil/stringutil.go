// Package stringutil 收敛各包重复实现的字符串小工具。
//
// 背景：firstNonEmpty / uniqueStrings / fallback / sanitizeToken 在 service、bootstrap、
// api/handler、agent 等多处各写了一份（语义几乎相同），改一处容易漏一处。
// 这里提供唯一实现，调用方直接引用。
package stringutil

import "strings"

// FirstNonEmpty 返回第一个 trim 后非空的参数（返回 trim 后的值）；全空返回 ""。
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// UniqueStrings 去重：trim 后跳过空串，保留首次出现顺序；空输入返回 nil。
func UniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

// Fallback 在 value trim 后为空时返回 def。
func Fallback(value, def string) string {
	if strings.TrimSpace(value) == "" {
		return def
	}
	return value
}

// SanitizeToken 清理 token/标识符：去空白，保留 [a-zA-Z0-9_-]，其余字符替换为 '-'。
func SanitizeToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, value)
}
