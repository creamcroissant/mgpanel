package tools

import (
	"context"

	"github.com/creamcroissant/mgpanel/internal/service"
)

type ServerListHandler struct {
	svc service.AdminServerService
}

func NewServerListHandler(svc service.AdminServerService) *ServerListHandler {
	return &ServerListHandler{svc: svc}
}

func (h *ServerListHandler) Name() string        { return ToolServerList }
func (h *ServerListHandler) Description() string { return "获取节点列表" }
func (h *ServerListHandler) Handle(ctx context.Context, _ any) (*ToolCallResult, error) {
	nodes, err := h.svc.Nodes(ctx)
	if err != nil {
		return nil, err
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: nodes}}}, nil
}

type ServerStatsHandler struct {
	svc service.AdminNodeStatService
}

func NewServerStatsHandler(svc service.AdminNodeStatService) *ServerStatsHandler {
	return &ServerStatsHandler{svc: svc}
}

func (h *ServerStatsHandler) Name() string        { return ToolServerStats }
func (h *ServerStatsHandler) Description() string { return "获取节点统计" }
func (h *ServerStatsHandler) Handle(ctx context.Context, _ any) (*ToolCallResult, error) {
	stats, err := h.svc.GetServerStats(ctx, 0, 0, 7)
	if err != nil {
		return nil, err
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: stats}}}, nil
}
