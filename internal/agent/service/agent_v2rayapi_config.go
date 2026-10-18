package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/creamcroissant/mgpanel/internal/agent/config"
	"github.com/creamcroissant/mgpanel/internal/agent/v2rayapi"
)

// v2rayAPIEndpoint returns the v2ray-api user-management endpoint for the given
// core type:
//   - xray: reuses the traffic API address (gRPC HandlerService on the same
//     endpoint as stats), falling back to 127.0.0.1:10085.
//   - sing-box: uses the injected experimental.v2ray_api listen fragment,
//     falling back to config default (127.0.0.1:19194).
//
// Empty core type yields an empty endpoint (client remains unavailable).
func v2rayAPIEndpoint(cfg *config.Config, coreType string) string {
	switch coreType {
	case v2rayapi.CoreTypeXray:
		if cfg != nil && strings.TrimSpace(cfg.Traffic.Address) != "" {
			return strings.TrimSpace(cfg.Traffic.Address)
		}
		return "127.0.0.1:10085"
	case v2rayapi.CoreTypeSingBox:
		if cfg != nil && strings.TrimSpace(cfg.Protocol.V2RayAPIListen) != "" {
			return strings.TrimSpace(cfg.Protocol.V2RayAPIListen)
		}
		return "127.0.0.1:19194"
	}
	return ""
}

// ensureV2RayAPIConfig writes an experimental_v2ray.json fragment into the
// sing-box config directory so that the sing-box core exposes the
// experimental.v2ray_api section (runtime user management via HandlerService).
//
// The fragment only carries the v2ray_api block; it deliberately does not
// touch experimental.json (owned by the access manager for clash_api) so the two
// injected sections never clobber each other. sing-box 'run -C <dir>' merges all
// JSON fragments in the directory.
func ensureV2RayAPIConfig(cfg *config.Config) error {
	dir := strings.TrimSpace(cfg.Protocol.ConfigDir)
	if dir == "" {
		if strings.TrimSpace(cfg.Protocol.ManagedConfigDir) != "" {
			dir = strings.TrimSpace(cfg.Protocol.ManagedConfigDir)
		} else {
			dir = "/etc/sing-box/conf"
		}
	}
	if testing.Testing() {
		dir = filepath.Join(os.TempDir(), fmt.Sprintf("singbox-conf-test-%d", os.Getpid()))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	listen := strings.TrimSpace(cfg.Protocol.V2RayAPIListen)
	if listen == "" {
		listen = "127.0.0.1:19194"
	}
	payload := map[string]any{
		"experimental": map[string]any{
			"v2ray_api": map[string]any{
				"listen": listen,
				"stats":  map[string]any{"enabled": true},
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "experimental_v2ray.json"), data, 0o644)
}
