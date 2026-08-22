package template

import (
	"encoding/json"
	"fmt"
)

// ValidationResult 包含校验结果。
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// AddError 添加错误并标记为无效。
func (r *ValidationResult) AddError(format string, args ...interface{}) {
	r.Valid = false
	r.Errors = append(r.Errors, fmt.Sprintf(format, args...))
}

// AddWarning 添加告警信息。
func (r *ValidationResult) AddWarning(format string, args ...interface{}) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}

// Merge 合并另一个结果。
func (r *ValidationResult) Merge(other *ValidationResult) {
	if !other.Valid {
		r.Valid = false
	}
	r.Errors = append(r.Errors, other.Errors...)
	r.Warnings = append(r.Warnings, other.Warnings...)
}

// Validator 负责模板与配置校验。
type Validator struct {
	engine *Engine
}

// NewValidator 创建新的校验器。
func NewValidator() *Validator {
	return &Validator{
		engine: NewEngine(),
	}
}

// ValidateTemplate 按以下步骤校验模板：
// 1. 解析 Go 模板语法
// 2. 使用示例数据渲染
// 3. 校验输出 JSON
// 4. 检查 sing-box/xray 的特定要求
func (v *Validator) ValidateTemplate(content string, templateType string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// 第 1 步：使用示例数据尝试渲染
	output, err := v.engine.PreviewRender(content)
	if err != nil {
		result.AddError("Template render error: %v", err)
		return result
	}

	// 第 2 步：校验 JSON 结构
	var parsed interface{}
	if err := json.Unmarshal(output, &parsed); err != nil {
		result.AddError("Invalid JSON: %v", err)
		return result
	}

	// 第 3 步：按类型校验结构
	switch templateType {
	case "sing-box":
		v.validateSingBoxConfig(parsed, result)
	case "xray":
		v.validateXrayConfig(parsed, result)
	default:
		result.AddWarning("Unknown template type '%s', skipping type-specific validation", templateType)
	}

	return result
}

// validateSingBoxConfig 校验 sing-box 的配置结构。
func (v *Validator) validateSingBoxConfig(parsed interface{}, result *ValidationResult) {
	config, ok := parsed.(map[string]interface{})
	if !ok {
		result.AddError("Config must be a JSON object")
		return
	}

	// 检查必需部分
	if _, ok := config["inbounds"]; !ok {
		result.AddWarning("Missing 'inbounds' section - will be injected by system if using dynamic mode")
	}

	if _, ok := config["outbounds"]; !ok {
		result.AddWarning("Missing 'outbounds' section - recommend adding at least 'direct' and 'block'")
	}

	// 检查日志部分
	if _, ok := config["log"]; !ok {
		result.AddWarning("Missing 'log' section - recommend adding for debugging")
	}

	// Validate inbounds structure
	if inbounds, ok := config["inbounds"].([]interface{}); ok {
		for i, inbound := range inbounds {
			v.validateSingBoxInbound(inbound, i, result)
		}
	}

	// 校验出站结构
	if outbounds, ok := config["outbounds"].([]interface{}); ok {
		for i, outbound := range outbounds {
			v.validateSingBoxOutbound(outbound, i, result)
		}
	}
}

// validateSingBoxInbound 校验单个 sing-box 入站。
func (v *Validator) validateSingBoxInbound(inbound interface{}, index int, result *ValidationResult) {
	ib, ok := inbound.(map[string]interface{})
	if !ok {
		result.AddError("Inbound %d: must be a JSON object", index)
		return
	}

	// Check required fields
	if _, hasType := ib["type"]; !hasType {
		result.AddError("Inbound %d: missing 'type' field", index)
	}

	if _, hasTag := ib["tag"]; !hasTag {
		result.AddError("Inbound %d: missing 'tag' field", index)
	}

	// Validate type-specific requirements
	if ibType, ok := ib["type"].(string); ok {
		switch ibType {
		case "vless", "vmess", "trojan":
			// These require users or fallback
			if _, hasUsers := ib["users"]; !hasUsers {
				result.AddWarning("Inbound %d (%s): no 'users' defined - ensure users are injected", index, ibType)
			}
		case "shadowsocks":
			// Check for method
			if _, hasMethod := ib["method"]; !hasMethod {
				if users, hasUsers := ib["users"].([]interface{}); hasUsers && len(users) > 0 {
					// Check if users have method
				} else {
					result.AddWarning("Inbound %d (shadowsocks): consider specifying 'method' for cipher", index)
				}
			}
		case "hysteria2", "tuic":
			// Check for TLS
			if _, hasTLS := ib["tls"]; !hasTLS {
				result.AddWarning("Inbound %d (%s): typically requires 'tls' configuration", index, ibType)
			}
		}
	}

	// Check listen_port
	if port, hasPort := ib["listen_port"]; hasPort {
		switch p := port.(type) {
		case float64:
			if p <= 0 || p > 65535 {
				result.AddError("Inbound %d: invalid port %v", index, port)
			}
		case int:
			if p <= 0 || p > 65535 {
				result.AddError("Inbound %d: invalid port %v", index, port)
			}
		}
	}
}

// validateSingBoxOutbound validates a single sing-box outbound.
func (v *Validator) validateSingBoxOutbound(outbound interface{}, index int, result *ValidationResult) {
	ob, ok := outbound.(map[string]interface{})
	if !ok {
		result.AddError("Outbound %d: must be a JSON object", index)
		return
	}

	// Check required fields
	if _, hasType := ob["type"]; !hasType {
		result.AddError("Outbound %d: missing 'type' field", index)
	}

	if _, hasTag := ob["tag"]; !hasTag {
		result.AddError("Outbound %d: missing 'tag' field", index)
	}
}

// validateXrayConfig validates Xray specific configuration.
func (v *Validator) validateXrayConfig(parsed interface{}, result *ValidationResult) {
	config, ok := parsed.(map[string]interface{})
	if !ok {
		result.AddError("Config must be a JSON object")
		return
	}

	// Check for required Xray sections
	if _, ok := config["inbounds"]; !ok {
		result.AddWarning("Missing 'inbounds' section - will be injected by system if using dynamic mode")
	}

	if _, ok := config["outbounds"]; !ok {
		result.AddWarning("Missing 'outbounds' section - recommend adding at least 'freedom' and 'blackhole'")
	}

	// Validate inbounds structure
	if inbounds, ok := config["inbounds"].([]interface{}); ok {
		for i, inbound := range inbounds {
			v.validateXrayInbound(inbound, i, result)
		}
	}
}

// validateXrayInbound validates a single Xray inbound.
func (v *Validator) validateXrayInbound(inbound interface{}, index int, result *ValidationResult) {
	ib, ok := inbound.(map[string]interface{})
	if !ok {
		result.AddError("Inbound %d: must be a JSON object", index)
		return
	}

	// Check Xray-specific required fields
	if _, hasProtocol := ib["protocol"]; !hasProtocol {
		result.AddError("Inbound %d: missing 'protocol' field", index)
	}

	if _, hasTag := ib["tag"]; !hasTag {
		result.AddError("Inbound %d: missing 'tag' field", index)
	}

	// Check for settings
	if _, hasSettings := ib["settings"]; !hasSettings {
		result.AddWarning("Inbound %d: missing 'settings' - ensure it's injected or defined", index)
	}
}

// ValidateFinalConfig validates a fully rendered configuration.
func (v *Validator) ValidateFinalConfig(configJSON []byte, configType string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Parse JSON
	var parsed interface{}
	if err := json.Unmarshal(configJSON, &parsed); err != nil {
		result.AddError("Invalid JSON: %v", err)
		return result
	}

	// Validate based on type
	switch configType {
	case "sing-box":
		v.validateSingBoxConfig(parsed, result)
	case "xray":
		v.validateXrayConfig(parsed, result)
	default:
		result.AddWarning("Unknown config type '%s', skipping type-specific validation", configType)
	}

	return result
}

// ValidateJSON validates that the content is valid JSON.
func (v *Validator) ValidateJSON(content []byte) *ValidationResult {
	result := &ValidationResult{Valid: true}

	var parsed interface{}
	if err := json.Unmarshal(content, &parsed); err != nil {
		result.AddError("Invalid JSON: %v", err)
	}

	return result
}
