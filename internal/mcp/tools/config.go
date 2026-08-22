package tools

import (
	"context"

	"github.com/creamcroissant/mgpanel/internal/service"
)

type ConfigArtifactsHandler struct {
	svc service.DriftAndDiffService
}

func NewConfigArtifactsHandler(svc service.DriftAndDiffService) *ConfigArtifactsHandler {
	return &ConfigArtifactsHandler{svc: svc}
}

func (h *ConfigArtifactsHandler) Name() string        { return ToolConfigArtifacts }
func (h *ConfigArtifactsHandler) Description() string { return "获取配置产物列表" }
func (h *ConfigArtifactsHandler) Handle(ctx context.Context, _ any) (*ToolCallResult, error) {
	artifacts, err := h.svc.ListArtifacts(ctx, service.ListDesiredArtifactsRequest{})
	if err != nil {
		return nil, err
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: artifacts}}}, nil
}
