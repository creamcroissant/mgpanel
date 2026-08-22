package tools

import (
	"context"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/service"
)

type SystemStatusHandler struct {
	svc service.AdminSystemService
}

func NewSystemStatusHandler(svc service.AdminSystemService) *SystemStatusHandler {
	return &SystemStatusHandler{svc: svc}
}

func (h *SystemStatusHandler) Name() string        { return ToolSystemStatus }
func (h *SystemStatusHandler) Description() string { return "获取面板系统运行状态" }
func (h *SystemStatusHandler) Handle(ctx context.Context, _ any) (*ToolCallResult, error) {
	status, err := h.svc.SystemStatus(ctx)
	if err != nil {
		return nil, err
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: status}}}, nil
}

type SystemSettingsHandler struct {
	svc service.AdminSystemSettingsService
}

func NewSystemSettingsHandler(svc service.AdminSystemSettingsService) *SystemSettingsHandler {
	return &SystemSettingsHandler{svc: svc}
}

func (h *SystemSettingsHandler) Name() string        { return ToolSystemSettings }
func (h *SystemSettingsHandler) Description() string { return "获取系统设置列表" }
func (h *SystemSettingsHandler) Handle(ctx context.Context, _ any) (*ToolCallResult, error) {
	settings, err := h.svc.GetByCategory(ctx, "")
	if err != nil {
		return nil, err
	}
	sensitive := []string{"token", "secret", "key", "password", "api_key", "private_key"}
	for k := range settings {
		for _, s := range sensitive {
			if strings.Contains(strings.ToLower(k), s) {
				settings[k] = "[REDACTED]"
				break
			}
		}
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: settings}}}, nil
}
