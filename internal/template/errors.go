package template

import (
	"errors"
	"fmt"
)

// 模板操作的哨兵错误。
var (
	ErrTemplateSyntax    = errors.New("template syntax error / 模板语法错误")
	ErrTemplateExecution = errors.New("template execution error / 模板执行错误")
	ErrInvalidJSON       = errors.New("invalid JSON output / JSON 输出无效")
	ErrValidationFailed  = errors.New("config validation failed / 配置校验失败")
	ErrVersionMismatch   = errors.New("version requirement mismatch / 版本要求不匹配")
	ErrMissingCapability = errors.New("required capability unavailable / 所需能力不可用")
)

// TemplateError 包装模板相关错误并附带上下文。
type TemplateError struct {
	Type    error  // 基础错误类型
	Message string // 错误信息
	Line    int    // 若可用则为行号
	Details string // 附加上下文（例如渲染输出）
}

// Error 实现 error 接口。
func (e *TemplateError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s 第 %d 行: %s", e.Type.Error(), e.Line, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Type.Error(), e.Message)
}

// Unwrap 返回底层错误类型。
func (e *TemplateError) Unwrap() error {
	return e.Type
}

// Is 判断 err 链中是否匹配目标错误。
func (e *TemplateError) Is(target error) bool {
	return errors.Is(e.Type, target)
}

// NewTemplateError 创建新的模板错误。
func NewTemplateError(errType error, message string) *TemplateError {
	return &TemplateError{
		Type:    errType,
		Message: message,
	}
}

// NewTemplateErrorWithLine 创建带行号的模板错误。
func NewTemplateErrorWithLine(errType error, message string, line int) *TemplateError {
	return &TemplateError{
		Type:    errType,
		Message: message,
		Line:    line,
	}
}

// NewTemplateErrorWithDetails 创建带详情的模板错误。
func NewTemplateErrorWithDetails(errType error, message, details string) *TemplateError {
	return &TemplateError{
		Type:    errType,
		Message: message,
		Details: details,
	}
}

// ValidationError 表示带字段信息的校验错误。
type ValidationError struct {
	Field   string // JSON 字段路径
	Message string // 错误描述
}

// Error 实现 error 接口。
func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}

// CapabilityError 表示能力相关错误。
type CapabilityError struct {
	Capability      Capability
	RequiredVersion string
	ActualVersion   string
	Message         string
}

// Error 实现 error 接口。
func (e *CapabilityError) Error() string {
	if e.RequiredVersion != "" && e.ActualVersion != "" {
		return fmt.Sprintf("能力 '%s' 需要版本 %s，Agent 当前为 %s",
			e.Capability, e.RequiredVersion, e.ActualVersion)
	}
	return fmt.Sprintf("能力 '%s': %s", e.Capability, e.Message)
}

// Unwrap 返回基础错误类型。
func (e *CapabilityError) Unwrap() error {
	return ErrMissingCapability
}
