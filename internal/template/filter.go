package template

import (
	"fmt"
	"strconv"
	"strings"
)

// CapabilityFilter 根据 Agent 能力过滤配置。
type CapabilityFilter struct {
	agentCaps *AgentCapabilities
}

// NewCapabilityFilter 为指定 Agent 创建过滤器。
func NewCapabilityFilter(caps *AgentCapabilities) *CapabilityFilter {
	return &CapabilityFilter{agentCaps: caps}
}

// FilterContext 从模板上下文移除不支持的特性。
// 返回过滤后的上下文和被移除特性的警告列表。
func (f *CapabilityFilter) FilterContext(ctx *TemplateContext) (*TemplateContext, []string) {
	warnings := []string{}
	filtered := *ctx // 浅拷贝

	// 过滤入站
	filteredInbounds := make([]InboundConfig, 0, len(ctx.Inbounds))
	for _, inbound := range ctx.Inbounds {
		// 检查是否满足全部所需能力
		supported := true
		for _, cap := range inbound.RequiredCapabilities {
			if !f.agentCaps.SupportsCapability(Capability(cap)) {
				supported = false
				warnings = append(warnings, fmt.Sprintf(
					"Inbound '%s' requires capability '%s' which is not supported by agent (version %s)",
					inbound.Tag, cap, f.agentCaps.CoreVersion,
				))
				break
			}
		}

		if supported {
			// 过滤入站内的具体特性
			filteredInbound := f.filterInbound(&inbound, &warnings)
			filteredInbounds = append(filteredInbounds, *filteredInbound)
		}
	}
	filtered.Inbounds = filteredInbounds

	// 过滤实验特性
	if ctx.Experimental != nil {
		filtered.Experimental = f.filterExperimental(ctx.Experimental, &warnings)
	}

	return &filtered, warnings
}

// filterInbound 过滤单个入站内的特性。
func (f *CapabilityFilter) filterInbound(inbound *InboundConfig, warnings *[]string) *InboundConfig {
	result := *inbound // 浅拷贝

	// 深拷贝 TLS，避免修改原对象
	if inbound.TLS != nil {
		tlsCopy := *inbound.TLS
		result.TLS = &tlsCopy

		// 不支持 Reality 时过滤掉
		if tlsCopy.Reality != nil && tlsCopy.Reality.Enabled {
			if !f.agentCaps.SupportsCapability(CapReality) {
				*warnings = append(*warnings, fmt.Sprintf(
					"Removing Reality from inbound '%s' - not supported by agent (%s)",
					inbound.Tag, f.getVersionRequirement(CapReality)))
				result.TLS.Reality = nil
			}
		}
	}

	// 深拷贝 Multiplex，避免修改原对象
	if inbound.Multiplex != nil {
		muxCopy := *inbound.Multiplex
		result.Multiplex = &muxCopy

		if !f.agentCaps.SupportsCapability(CapMultiplex) {
			*warnings = append(*warnings, fmt.Sprintf(
				"Removing Multiplex from inbound '%s' - not supported by agent (%s)",
				inbound.Tag, f.getVersionRequirement(CapMultiplex)))
			result.Multiplex = nil
		} else if muxCopy.Brutal != nil && muxCopy.Brutal.Enabled {
			if !f.agentCaps.SupportsCapability(CapBrutal) {
				*warnings = append(*warnings, fmt.Sprintf(
					"Removing Brutal from inbound '%s' - not supported by agent (%s)",
					inbound.Tag, f.getVersionRequirement(CapBrutal)))
				result.Multiplex.Brutal = nil
			}
		}
	}

	return &result
}

// getVersionRequirement 返回易读的版本要求字符串。
func (f *CapabilityFilter) getVersionRequirement(cap Capability) string {
	switch f.agentCaps.CoreType {
	case "sing-box":
		if minVer, ok := SingBoxVersionRequirements[cap]; ok {
			return fmt.Sprintf("requires sing-box >= %s", minVer)
		}
	case "xray":
		if minVer, ok := XrayVersionRequirements[cap]; ok {
			return fmt.Sprintf("requires xray >= %s", minVer)
		}
	}
	return "not supported"
}

// filterExperimental 过滤实验特性。
func (f *CapabilityFilter) filterExperimental(exp *ExperimentalConfig, warnings *[]string) *ExperimentalConfig {
	if exp == nil {
		return nil
	}

	result := *exp // 浅拷贝

	// 检查 v2ray_api 支持情况
	if exp.V2RayAPI != nil {
		switch f.agentCaps.CoreType {
		case "sing-box":
			// Sing-box 需要 v2ray_api 的 build tag
			if !f.agentCaps.SupportsCapability(CapV2RayAPI) {
				*warnings = append(*warnings, "Removing v2ray_api - not supported by agent (requires build tag 'with_v2ray_api')")
				result.V2RayAPI = nil
			} else if !f.agentCaps.HasBuildTag("with_v2ray_api") {
				*warnings = append(*warnings, "Removing v2ray_api - agent missing build tag 'with_v2ray_api'")
				result.V2RayAPI = nil
			}
		case "xray":
			// Xray 内建 stats，始终可用
			// Xray 无需过滤
		default:
			// 未知核心类型，过滤 v2ray_api
			*warnings = append(*warnings, "Removing v2ray_api - unknown core type")
			result.V2RayAPI = nil
		}
	}

	// 若全部被过滤则返回 nil
	if result.V2RayAPI == nil {
		return nil
	}

	return &result
}

// CheckTemplateCompatibility 检查模板与 Agent 的兼容性。
func (f *CapabilityFilter) CheckTemplateCompatibility(minVersion string, reqCaps []string) (bool, []string) {
	warnings := []string{}

	// 检查最低版本要求
	if minVersion != "" && !f.SupportsVersion(minVersion) {
		return false, []string{fmt.Sprintf(
			"Template requires version %s, agent has %s",
			minVersion, f.agentCaps.CoreVersion,
		)}
	}

	// 检查所需能力
	for _, cap := range reqCaps {
		if !f.agentCaps.SupportsCapability(Capability(cap)) {
			warnings = append(warnings, fmt.Sprintf(
				"Template requires capability '%s' which may not be fully supported", cap))
		}
	}

	return len(warnings) == 0, warnings
}

// SupportsVersion 判断 Agent 版本是否满足最低要求。
func (f *CapabilityFilter) SupportsVersion(required string) bool {
	return compareVersions(f.agentCaps.CoreVersion, required) >= 0
}

// compareVersions 比较两个语义化版本字符串。
// 返回值：
//   - 1 表示 a > b
//   - 0 表示 a == b
//   - -1 表示 a < b
func compareVersions(a, b string) int {
	aParts := parseVersion(a)
	bParts := parseVersion(b)

	// 逐段比较
	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}

	for i := 0; i < maxLen; i++ {
		aVal := 0
		bVal := 0

		if i < len(aParts) {
			aVal = aParts[i]
		}
		if i < len(bParts) {
			bVal = bParts[i]
		}

		if aVal > bVal {
			return 1
		}
		if aVal < bVal {
			return -1
		}
	}

	return 0
}

// parseVersion 将版本字符串解析为整数切片。
func parseVersion(version string) []int {
	// 移除前导 'v' 或 'V'
	version = strings.TrimPrefix(version, "v")
	version = strings.TrimPrefix(version, "V")

	// 按点号拆分
	parts := strings.Split(version, ".")
	result := make([]int, 0, len(parts))

	for _, part := range parts {
		// 处理预发布标签（如 "1.0.0-beta"）
		if idx := strings.IndexAny(part, "-+"); idx >= 0 {
			part = part[:idx]
		}

		// 解析为整数
		val, err := strconv.Atoi(part)
		if err != nil {
			continue
		}
		result = append(result, val)
	}

	return result
}

// DeriveCapabilities 依据版本和 build tag 推导能力列表。
func DeriveCapabilities(coreType, coreVersion string, buildTags []string) []Capability {
	caps := make([]Capability, 0)

	switch coreType {
	case "sing-box":
		return deriveSingBoxCapabilities(coreVersion, buildTags)
	case "xray":
		return deriveXrayCapabilities(coreVersion, buildTags)
	default:
		// 未知核心类型，返回空能力列表
		return caps
	}
}

// deriveSingBoxCapabilities 推导 Sing-box 能力列表。
func deriveSingBoxCapabilities(coreVersion string, buildTags []string) []Capability {
	caps := make([]Capability, 0)

	// 检查基于版本的能力
	for cap, minVer := range SingBoxVersionRequirements {
		if compareVersions(coreVersion, minVer) >= 0 {
			// v2ray_api 还需要 build tag
			if cap == CapV2RayAPI {
				hasBuildTag := false
				for _, tag := range buildTags {
					if tag == "with_v2ray_api" {
						hasBuildTag = true
						break
					}
				}
				if hasBuildTag {
					caps = append(caps, cap)
				}
			} else {
				caps = append(caps, cap)
			}
		}
	}

	// 根据 build tag 追加能力
	tagToCap := map[string]Capability{
		"with_quic":      CapQUIC,
		"with_wireguard": CapWireguard,
		"with_utls":      CapUTLS,
		"with_ech":       CapECH,
		"with_gvisor":    CapTUN,
		"with_dhcp":      CapDHCP,
	}

	for _, tag := range buildTags {
		if cap, ok := tagToCap[tag]; ok {
			// 避免重复添加
			if !hasCapability(caps, cap) {
				caps = append(caps, cap)
			}
		}
	}

	return caps
}

// deriveXrayCapabilities 推导 Xray-core 能力列表。
func deriveXrayCapabilities(coreVersion string, buildTags []string) []Capability {
	caps := make([]Capability, 0)

	// 检查基于版本的能力
	for cap, minVer := range XrayVersionRequirements {
		if compareVersions(coreVersion, minVer) >= 0 {
			caps = append(caps, cap)
		}
	}

	// Xray-core 编译为单一二进制，内置全部特性
	// 大多特性无需 build tag，但可用于识别定制构建
	for _, tag := range buildTags {
		switch tag {
		case "with_geoip":
			if !hasCapability(caps, CapGeoIP) {
				caps = append(caps, CapGeoIP)
			}
		case "with_geosite":
			if !hasCapability(caps, CapGeoSite) {
				caps = append(caps, CapGeoSite)
			}
		}
	}

	return caps
}

// hasCapability 判断能力切片是否包含指定能力。
func hasCapability(caps []Capability, cap Capability) bool {
	for _, c := range caps {
		if c == cap {
			return true
		}
	}
	return false
}
