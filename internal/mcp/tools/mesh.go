package tools

import (
	"context"
	"fmt"

	"github.com/creamcroissant/xboard/internal/service"
)

type MeshNetworkHandler struct {
	svc service.AgentMeshService
}

func NewMeshNetworkHandler(svc service.AgentMeshService) *MeshNetworkHandler {
	return &MeshNetworkHandler{svc: svc}
}

func (h *MeshNetworkHandler) Name() string        { return ToolMeshNetwork }
func (h *MeshNetworkHandler) Description() string { return "获取Mesh网络拓扑" }
func (h *MeshNetworkHandler) Handle(ctx context.Context, params any) (*ToolCallResult, error) {
	m, ok := params.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("params must be object")
	}
	networkID, ok := m["network_id"].(string)
	if !ok {
		return nil, fmt.Errorf("network_id is required")
	}
	peers, err := h.svc.ListNetworkPeers(ctx, networkID)
	if err != nil {
		return nil, err
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: peers}}}, nil
}
