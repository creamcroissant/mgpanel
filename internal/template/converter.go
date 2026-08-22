package template

import (
	"fmt"
	"strings"
)

// ConfigConverter 在统一入站与核心配置之间转换。
type ConfigConverter interface {
	TargetCore() string
	FromUnified(inbounds []UnifiedInbound) ([]byte, error)
	ToUnified(configJSON []byte) ([]UnifiedInbound, error)
}

// ConverterRegistry 管理已注册的转换器。
type ConverterRegistry struct {
	converters map[string]ConfigConverter
}

// NewConverterRegistry 创建注册表并可预注册转换器。
func NewConverterRegistry(converters ...ConfigConverter) *ConverterRegistry {
	registry := &ConverterRegistry{}
	for _, converter := range converters {
		registry.Register(converter)
	}
	return registry
}

// Register 将转换器注册到注册表。
func (r *ConverterRegistry) Register(converter ConfigConverter) {
	if converter == nil {
		return
	}
	coreType := normalizeCoreType(converter.TargetCore())
	if coreType == "" {
		return
	}
	if r.converters == nil {
		r.converters = make(map[string]ConfigConverter)
	}
	r.converters[coreType] = converter
}

// Convert 将统一入站转换为目标核心配置。
func (r *ConverterRegistry) Convert(inbounds []UnifiedInbound, targetCore string) ([]byte, error) {
	converter, err := r.getConverter(targetCore)
	if err != nil {
		return nil, err
	}
	return converter.FromUnified(inbounds)
}

// Parse 将核心配置转换为统一入站。
func (r *ConverterRegistry) Parse(configJSON []byte, sourceCore string) ([]UnifiedInbound, error) {
	converter, err := r.getConverter(sourceCore)
	if err != nil {
		return nil, err
	}
	return converter.ToUnified(configJSON)
}

func (r *ConverterRegistry) getConverter(coreType string) (ConfigConverter, error) {
	if r == nil || len(r.converters) == 0 {
		return nil, fmt.Errorf("no converters registered / 未注册任何转换器")
	}
	converter := r.converters[normalizeCoreType(coreType)]
	if converter == nil {
		return nil, fmt.Errorf("converter not registered: %s / 转换器未注册: %s", coreType, coreType)
	}
	return converter, nil
}

func normalizeCoreType(coreType string) string {
	return strings.ToLower(strings.TrimSpace(coreType))
}
