package template

import "strings"

const (
	XHTTPNetwork       = "xhttp"
	SplitHTTPNetwork   = "splithttp"
	XHTTPModeAuto      = "auto"
	XHTTPModePacketUp  = "packet-up"
	XHTTPModeStreamUp  = "stream-up"
	XHTTPModeStreamOne = "stream-one"
)

// NormalizeXHTTPNetwork returns the canonical transport name for xhttp-compatible networks.
func NormalizeXHTTPNetwork(network string) string {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case XHTTPNetwork, SplitHTTPNetwork:
		return XHTTPNetwork
	default:
		return strings.ToLower(strings.TrimSpace(network))
	}
}

// IsXHTTPNetwork reports whether network is xhttp or its legacy splithttp alias.
func IsXHTTPNetwork(network string) bool {
	return NormalizeXHTTPNetwork(network) == XHTTPNetwork
}

// NormalizeXHTTPMode returns the canonical xhttp mode, defaulting empty values to auto.
func NormalizeXHTTPMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", XHTTPModeAuto:
		return XHTTPModeAuto
	case XHTTPModePacketUp:
		return XHTTPModePacketUp
	case XHTTPModeStreamUp:
		return XHTTPModeStreamUp
	case XHTTPModeStreamOne:
		return XHTTPModeStreamOne
	default:
		return strings.ToLower(strings.TrimSpace(mode))
	}
}

// MergeXHTTPConfig merges explicit xhttp settings with transport shorthand fields.
func MergeXHTTPConfig(settings *XHTTPConfig, host, path, mode string, headers map[string]string) XHTTPConfig {
	result := XHTTPConfig{}
	if settings != nil {
		result = *settings
		result.Headers = cloneXHTTPHeaders(settings.Headers)
		result.Extra = cloneXHTTPMap(settings.Extra)
		result.XMux = cloneXHTTPMap(settings.XMux)
		result.DownloadSettings = cloneXHTTPMap(settings.DownloadSettings)
		// Deep-clone pointer fields to avoid shared state
		if settings.NoGRPCHeader != nil {
			v := *settings.NoGRPCHeader
			result.NoGRPCHeader = &v
		}
		if settings.NoSSEHeader != nil {
			v := *settings.NoSSEHeader
			result.NoSSEHeader = &v
		}
		if settings.SCMaxBufferedPosts != nil {
			v := *settings.SCMaxBufferedPosts
			result.SCMaxBufferedPosts = &v
		}
		if settings.SCMaxEachPostBytes != nil {
			v := *settings.SCMaxEachPostBytes
			result.SCMaxEachPostBytes = &v
		}
		if settings.SCMinPostsInterval != nil {
			v := *settings.SCMinPostsInterval
			result.SCMinPostsInterval = &v
		}
		if settings.SCStreamUpServerSec != nil {
			v := *settings.SCStreamUpServerSec
			result.SCStreamUpServerSec = &v
		}
	}
	if strings.TrimSpace(result.Host) == "" {
		result.Host = strings.TrimSpace(host)
	}
	if strings.TrimSpace(result.Path) == "" {
		result.Path = strings.TrimSpace(path)
	}
	if strings.TrimSpace(result.Mode) == "" {
		result.Mode = strings.TrimSpace(mode)
	}
	if len(result.Headers) == 0 {
		result.Headers = cloneXHTTPHeaders(headers)
	}
	return result
}

// ValidateXHTTPSettings returns field-level validation errors for xhttp settings.
func ValidateXHTTPSettings(settings XHTTPConfig) map[string]string {
	fields := map[string]string{}
	mode := NormalizeXHTTPMode(settings.Mode)
	if !validXHTTPMode(mode) {
		fields["mode"] = "must be one of auto, packet-up, stream-up, stream-one"
	}
	if mode == XHTTPModeStreamOne && len(settings.DownloadSettings) > 0 {
		fields["downloadSettings"] = "cannot be used when mode is stream-one"
	}
	for key := range settings.Headers {
		if strings.EqualFold(strings.TrimSpace(key), "host") {
			fields["headers.Host"] = "use xhttp host instead of headers.Host"
		}
	}
	return fields
}

// BuildXHTTPSettingsMap builds a compact xhttpSettings object for Xray streamSettings.
func BuildXHTTPSettingsMap(settings XHTTPConfig) map[string]any {
	result := map[string]any{}
	if host := strings.TrimSpace(settings.Host); host != "" {
		result["host"] = host
	}
	if path := strings.TrimSpace(settings.Path); path != "" {
		result["path"] = path
	}
	if mode := NormalizeXHTTPMode(settings.Mode); mode != "" && mode != XHTTPModeAuto {
		result["mode"] = mode
	}
	if headers := cloneXHTTPHeaders(settings.Headers); len(headers) > 0 {
		result["headers"] = headers
	}
	if extra := cloneXHTTPMap(settings.Extra); len(extra) > 0 {
		result["extra"] = extra
	}
	if xmux := cloneXHTTPMap(settings.XMux); len(xmux) > 0 {
		result["xmux"] = xmux
	}
	if downloadSettings := cloneXHTTPMap(settings.DownloadSettings); len(downloadSettings) > 0 {
		result["downloadSettings"] = downloadSettings
	}
	if settings.NoGRPCHeader != nil {
		result["noGRPCHeader"] = *settings.NoGRPCHeader
	}
	if settings.NoSSEHeader != nil {
		result["noSSEHeader"] = *settings.NoSSEHeader
	}
	if settings.SCMaxBufferedPosts != nil {
		result["scMaxBufferedPosts"] = *settings.SCMaxBufferedPosts
	}
	if settings.SCMaxEachPostBytes != nil {
		result["scMaxEachPostBytes"] = *settings.SCMaxEachPostBytes
	}
	if settings.SCMinPostsInterval != nil {
		result["scMinPostsIntervalMs"] = *settings.SCMinPostsInterval
	}
	if settings.SCStreamUpServerSec != nil {
		result["scStreamUpServerSecs"] = *settings.SCStreamUpServerSec
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func xhttpConfigFromMap(settings map[string]any) XHTTPConfig {
	config := XHTTPConfig{}
	if len(settings) == 0 {
		return config
	}
	config.Host = stringFromXHTTPAny(settings["host"])
	config.Path = stringFromXHTTPAny(settings["path"])
	config.Mode = NormalizeXHTTPMode(stringFromXHTTPAny(settings["mode"]))
	config.Headers = stringMapFromXHTTPAny(settings["headers"])
	config.Extra = anyMapFromXHTTPAny(settings["extra"])
	config.XMux = anyMapFromXHTTPAny(settings["xmux"])
	config.DownloadSettings = anyMapFromXHTTPAny(settings["downloadSettings"])
	config.NoGRPCHeader = boolPtrFromXHTTPAny(settings["noGRPCHeader"])
	config.NoSSEHeader = boolPtrFromXHTTPAny(settings["noSSEHeader"])
	config.SCMaxBufferedPosts = intPtrFromXHTTPAny(settings["scMaxBufferedPosts"])
	config.SCMaxEachPostBytes = intPtrFromXHTTPAny(settings["scMaxEachPostBytes"])
	config.SCMinPostsInterval = intPtrFromXHTTPAny(settings["scMinPostsIntervalMs"])
	config.SCStreamUpServerSec = intPtrFromXHTTPAny(settings["scStreamUpServerSecs"])
	return config
}

func validXHTTPMode(mode string) bool {
	switch mode {
	case XHTTPModeAuto, XHTTPModePacketUp, XHTTPModeStreamUp, XHTTPModeStreamOne:
		return true
	default:
		return false
	}
}

func cloneXHTTPHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	result := make(map[string]string, len(headers))
	for key, value := range headers {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		result[key] = strings.TrimSpace(value)
	}
	return result
}

func cloneXHTTPMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		result[key] = cloneXHTTPAny(value)
	}
	return result
}

func cloneXHTTPAny(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneXHTTPMap(typed)
	case map[string]string:
		return cloneXHTTPHeaders(typed)
	case []any:
		result := make([]any, len(typed))
		for i, item := range typed {
			result[i] = cloneXHTTPAny(item)
		}
		return result
	case []string:
		result := make([]string, len(typed))
		copy(result, typed)
		return result
	default:
		return value
	}
}

func stringFromXHTTPAny(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func stringMapFromXHTTPAny(value any) map[string]string {
	switch typed := value.(type) {
	case map[string]string:
		return cloneXHTTPHeaders(typed)
	case map[string]any:
		result := map[string]string{}
		for key, raw := range typed {
			if text, ok := raw.(string); ok {
				key = strings.TrimSpace(key)
				if key == "" || strings.EqualFold(key, "host") {
					continue
				}
				result[key] = strings.TrimSpace(text)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return nil
}

func anyMapFromXHTTPAny(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return cloneXHTTPMap(typed)
	}
	return nil
}

func boolPtrFromXHTTPAny(value any) *bool {
	if typed, ok := value.(bool); ok {
		return &typed
	}
	return nil
}

func intPtrFromXHTTPAny(value any) *int {
	switch typed := value.(type) {
	case int:
		return &typed
	case int64:
		converted := int(typed)
		return &converted
	case float64:
		converted := int(typed)
		return &converted
	}
	return nil
}
