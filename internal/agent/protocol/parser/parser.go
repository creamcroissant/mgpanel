package parser

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
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

// stripComments 移除 JSON 中的 // 与 /* */ 风格注释。
func stripComments(content []byte) []byte {
	// 移除单行注释（// ...）
	singleLine := regexp.MustCompile(`(?m)^\s*//.*$`)
	content = singleLine.ReplaceAll(content, []byte{})

	// 移除行内注释
	inlineComment := regexp.MustCompile(`//[^\n]*`)
	content = inlineComment.ReplaceAll(content, []byte{})

	// 移除多行注释（/* ... */）
	multiLine := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	content = multiLine.ReplaceAll(content, []byte{})

	// 去掉首尾空白与空行
	content = bytes.TrimSpace(content)

	return content
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
