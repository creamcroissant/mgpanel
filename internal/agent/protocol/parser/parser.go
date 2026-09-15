package parser

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// Parser 定义协议配置解析器接口。
type Parser interface {
	// Name 返回解析器标识（如 "sing-box", "xray"）
	Name() string

	// CanParse 判断解析器是否能处理该内容
	CanParse(content []byte) bool

	// Parse 解析配置内容并提取协议详情
	Parse(filename string, content []byte) ([]ProtocolDetails, error)
}

// Registry 保存已注册的解析器列表。
type Registry struct {
	parsers []Parser
}

// NewRegistry 创建解析器注册表（包含默认解析器）。
func NewRegistry() *Registry {
	return &Registry{
		parsers: []Parser{
			NewSingBoxParser(),
			NewXrayParser(),
		},
	}
}

// Register 注册新的解析器。
func (r *Registry) Register(p Parser) {
	r.parsers = append(r.parsers, p)
}

// stripComments 移除 JSON 中的 // 与 /* */ 注释，且**不触碰字符串字面量内部**。
//
// 背景（生产实测）：早先用正则在全局删除 //，会把字符串里的 URL（如 loadbalance 健康检查
// "url": "https://..."）也删掉，导致含 URL 的配置片段被判为 invalid JSON，语义 diff 与
// 清单解析随之失败（apply 时出现 semantic_diff_unavailable: invalid JSON in file）。
func stripComments(content []byte) []byte {
	out := make([]byte, 0, len(content))
	inString := false
	escaped := false
	for i := 0; i < len(content); i++ {
		c := content[i]
		if inString {
			out = append(out, c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == '/' && i+1 < len(content) {
			if content[i+1] == '/' {
				for i < len(content) && content[i] != '\n' {
					i++
				}
				if i < len(content) {
					out = append(out, content[i]) // 保留换行，维持行结构
				}
				continue
			}
			if content[i+1] == '*' {
				i += 2
				for i+1 < len(content) && !(content[i] == '*' && content[i+1] == '/') {
					i++
				}
				i++ // 跳过结尾 '/'
				continue
			}
		}
		out = append(out, c)
	}
	return bytes.TrimSpace(out)
}

// Parse 依次尝试所有解析器，返回第一个成功结果。
func (r *Registry) Parse(filename string, content []byte) ([]ProtocolDetails, error) {
	// 解析前先移除注释
	content = stripComments(content)

	// 先检查是否为合法 JSON
	if !json.Valid(content) {
		return nil, fmt.Errorf("invalid JSON in file: %s", filename)
	}

	var parseErrs []error
	for _, p := range r.parsers {
		if p.CanParse(content) {
			details, err := p.Parse(filename, content)
			if err != nil {
				parseErrs = append(parseErrs, fmt.Errorf("%s parser: %w", p.Name(), err))
				continue
			}
			return details, nil
		}
	}

	if len(parseErrs) > 0 {
		return nil, errors.Join(parseErrs...)
	}

	// 未匹配到解析器则返回空结果（可能是 outbounds 或 routes 配置）
	return nil, nil
}

// ParseAll 使用所有可用解析器解析内容并合并结果。
func (r *Registry) ParseAll(filename string, content []byte) []ProtocolDetails {
	var allDetails []ProtocolDetails

	// 解析前先移除注释
	content = stripComments(content)

	if !json.Valid(content) {
		return allDetails
	}

	for _, p := range r.parsers {
		if p.CanParse(content) {
			details, err := p.Parse(filename, content)
			if err == nil && len(details) > 0 {
				allDetails = append(allDetails, details...)
			}
		}
	}

	return allDetails
}
