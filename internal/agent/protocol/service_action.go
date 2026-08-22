package protocol

import (
	"fmt"
	"strings"
)

// ServiceAction defines how config changes are applied to the running service.
type ServiceAction string

const (
	ReloadServiceAction  ServiceAction = "reload"
	RestartServiceAction ServiceAction = "restart"
	NoneServiceAction    ServiceAction = "none"
)

// NormalizeServiceAction resolves the final service action.
// Explicit service_action takes precedence; legacy auto_restart maps to reload/none.
func NormalizeServiceAction(raw string, autoRestart bool) (ServiceAction, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		if autoRestart {
			return ReloadServiceAction, nil
		}
		return NoneServiceAction, nil
	}

	action := ServiceAction(normalized)
	switch action {
	case ReloadServiceAction, RestartServiceAction, NoneServiceAction:
		return action, nil
	default:
		return "", fmt.Errorf("invalid service_action %q", raw)
	}
}

func normalizeServiceAction(cfg Config) (ServiceAction, error) {
	return NormalizeServiceAction(cfg.ServiceAction, cfg.AutoRestart)
}
