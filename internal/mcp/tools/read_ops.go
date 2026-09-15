package tools

import (
	"context"
	"fmt"

	"github.com/creamcroissant/mgpanel/internal/service"
)

// 专用只读工具（scope=read）：与写工具职责分离，保证 tools/list 中的作用域标注与能力一致。

// ——— egress_mode_get：读取主机出口集分发模式与能力新鲜度 ———

type EgressModeGetHandler struct {
	hosts service.AgentHostService
}

func NewEgressModeGetHandler(hosts service.AgentHostService) *EgressModeGetHandler {
	return &EgressModeGetHandler{hosts: hosts}
}

func (h *EgressModeGetHandler) Name() string { return ToolEgressModeGet }
func (h *EgressModeGetHandler) Description() string {
	return "读取主机出口集分发模式与最近同步时间（能力门控依据）"
}
func (h *EgressModeGetHandler) InputSchema() map[string]any {
	return objectSchema(map[string]any{"agent_host_id": intSchema("目标主机 ID")}, []string{"agent_host_id"})
}

func (h *EgressModeGetHandler) Handle(ctx context.Context, params any) (*ToolCallResult, error) {
	m, err := paramMap(params)
	if err != nil {
		return nil, err
	}
	hostID, ok := paramInt64(m, "agent_host_id")
	if !ok || hostID <= 0 {
		return nil, fmt.Errorf("agent_host_id is required")
	}
	host, err := h.hosts.GetByID(ctx, hostID)
	if err != nil {
		return nil, err
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: map[string]any{
		"agent_host_id":    host.ID,
		"name":             host.Name,
		"egress_dispatch":  host.EgressDispatch,
		"egress_synced_at": host.EgressSyncedAt,
	}}}}, nil
}

// ——— routing_policy_list ———

type RoutingPolicyListHandler struct {
	policies service.RoutingPolicyService
}

func NewRoutingPolicyListHandler(policies service.RoutingPolicyService) *RoutingPolicyListHandler {
	return &RoutingPolicyListHandler{policies: policies}
}

func (h *RoutingPolicyListHandler) Name() string { return ToolRoutingPolicyList }
func (h *RoutingPolicyListHandler) Description() string {
	return "列出路由策略（含绑定 spec、目标出口集、匹配值与粘性）"
}
func (h *RoutingPolicyListHandler) InputSchema() map[string]any {
	return objectSchema(map[string]any{"core_type": strSchema("sing-box | xray")}, nil)
}

func (h *RoutingPolicyListHandler) Handle(ctx context.Context, params any) (*ToolCallResult, error) {
	m, err := paramMap(params)
	if err != nil {
		return nil, err
	}
	items, err := h.policies.List(ctx, paramString(m, "core_type"))
	if err != nil {
		return nil, err
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: items}}}, nil
}

// ——— exit_node_set_list ———

type ExitNodeSetListHandler struct {
	sets service.ExitNodeSetService
}

func NewExitNodeSetListHandler(sets service.ExitNodeSetService) *ExitNodeSetListHandler {
	return &ExitNodeSetListHandler{sets: sets}
}

func (h *ExitNodeSetListHandler) Name() string        { return ToolExitNodeSetList }
func (h *ExitNodeSetListHandler) Description() string { return "列出出口集合及其成员" }
func (h *ExitNodeSetListHandler) Handle(ctx context.Context, _ any) (*ToolCallResult, error) {
	items, err := h.sets.List(ctx)
	if err != nil {
		return nil, err
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: items}}}, nil
}
