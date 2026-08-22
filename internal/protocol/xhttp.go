package protocol

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/template"
)

func nodeUsesXHTTP(settings map[string]any) bool {
	return template.NormalizeXHTTPNetwork(settingString(settings, "network")) == "xhttp"
}

func applyVLESSXHTTPQuery(q url.Values, settings map[string]any) {
	if path := xhttpSettingString(settings, "path"); path != "" {
		q.Set("path", path)
	}
	if host := xhttpHost(settings); host != "" {
		q.Set("host", host)
	}
	if mode := template.NormalizeXHTTPMode(xhttpSettingString(settings, "mode")); mode != "" {
		q.Set("mode", mode)
	}
	if extra := encodedXHTTPExtra(settings); extra != "" {
		q.Set("extra", extra)
	}
}

func buildXHTTPOpts(settings map[string]any) map[string]any {
	opts := map[string]any{}
	if path := xhttpSettingString(settings, "path"); path != "" {
		opts["path"] = path
	}
	if host := xhttpHost(settings); host != "" {
		opts["host"] = host
	}
	if mode := template.NormalizeXHTTPMode(xhttpSettingString(settings, "mode")); mode != "" {
		opts["mode"] = mode
	}
	if headers := xhttpHeaders(settings); len(headers) > 0 {
		opts["headers"] = headers
	}
	if len(opts) == 0 {
		return nil
	}
	return opts
}

func xhttpSettingString(settings map[string]any, key string) string {
	if value := settingString(settings, "network_settings."+key); value != "" {
		return value
	}
	return settingString(settings, "network_settings.xhttp."+key)
}

func xhttpHost(settings map[string]any) string {
	if host := xhttpSettingString(settings, "host"); host != "" {
		return host
	}
	return settingString(settings, "network_settings.headers.Host")
}

func xhttpHeaders(settings map[string]any) map[string]any {
	headers := map[string]any{}
	mergeXHTTPHeaders(headers, settingMap(settings, "network_settings.headers"))
	mergeXHTTPHeaders(headers, settingMap(settings, "network_settings.xhttp.headers"))
	if len(headers) == 0 {
		return nil
	}
	return headers
}

func mergeXHTTPHeaders(dst, src map[string]any) {
	for key, value := range src {
		key = strings.TrimSpace(key)
		if key == "" || strings.EqualFold(key, "Host") {
			continue
		}
		dst[key] = value
	}
}

func encodedXHTTPExtra(settings map[string]any) string {
	extra := settingMap(settings, "network_settings.extra")
	if len(extra) == 0 {
		extra = settingMap(settings, "network_settings.xhttp.extra")
	}
	if len(extra) == 0 {
		return ""
	}
	payload, err := json.Marshal(extra)
	if err != nil {
		return ""
	}
	return string(payload)
}
