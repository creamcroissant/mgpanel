package proxy

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ConfigPatcher patches core config JSON while preserving unknown fields.
type ConfigPatcher struct {
	logger *slog.Logger
}

// NewConfigPatcher creates a ConfigPatcher instance.
func NewConfigPatcher(logger *slog.Logger) *ConfigPatcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &ConfigPatcher{logger: logger}
}

// Patch applies core-specific patching.
func (p *ConfigPatcher) Patch(coreType string, configJSON []byte, portMappings map[int]int) ([]byte, error) {
	core := strings.ToLower(strings.TrimSpace(coreType))
	switch core {
	case "sing-box", "singbox":
		return p.PatchSingBox(configJSON, portMappings)
	case "xray":
		return p.PatchXray(configJSON, portMappings)
	default:
		return nil, fmt.Errorf("unsupported core type: %s", coreType)
	}
}

// PatchSingBox updates sing-box inbound listen and listen_port fields.
func (p *ConfigPatcher) PatchSingBox(configJSON []byte, portMappings map[int]int) ([]byte, error) {
	return patchInbounds(configJSON, portMappings, "listen", "listen_port")
}

// PatchXray updates xray inbound listen and port fields.
func (p *ConfigPatcher) PatchXray(configJSON []byte, portMappings map[int]int) ([]byte, error) {
	return patchInbounds(configJSON, portMappings, "listen", "port")
}

func patchInbounds(configJSON []byte, portMappings map[int]int, listenKey, portKey string) ([]byte, error) {
	if len(portMappings) == 0 {
		return configJSON, nil
	}

	inbounds := gjson.GetBytes(configJSON, "inbounds")
	if !inbounds.Exists() || !inbounds.IsArray() {
		return nil, fmt.Errorf("config missing inbounds array")
	}

	patched := configJSON
	for idx, inbound := range inbounds.Array() {
		listenPort := int(inbound.Get(portKey).Int())
		internalPort, ok := portMappings[listenPort]
		if !ok {
			continue
		}

		pathListen := fmt.Sprintf("inbounds.%d.%s", idx, listenKey)
		pathPort := fmt.Sprintf("inbounds.%d.%s", idx, portKey)

		var err error
		patched, err = sjson.SetBytes(patched, pathListen, "::")
		if err != nil {
			return nil, fmt.Errorf("patch %s: %w", pathListen, err)
		}
		patched, err = sjson.SetBytes(patched, pathPort, internalPort)
		if err != nil {
			return nil, fmt.Errorf("patch %s: %w", pathPort, err)
		}
	}

	return patched, nil
}
