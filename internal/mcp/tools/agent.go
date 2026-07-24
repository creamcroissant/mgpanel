package tools

import (
	"context"
	"fmt"

	"github.com/creamcroissant/xboard/internal/service"
)

type AgentListHandler struct {
	svc service.AgentHostService
}

func NewAgentListHandler(svc service.AgentHostService) *AgentListHandler {
	return &AgentListHandler{svc: svc}
}

func (h *AgentListHandler) Name() string        { return ToolAgentList }
func (h *AgentListHandler) Description() string { return "获取Agent主机列表" }
func (h *AgentListHandler) Handle(ctx context.Context, _ any) (*ToolCallResult, error) {
	agents, err := h.svc.List(ctx)
	if err != nil {
		return nil, err
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: agents}}}, nil
}

type AgentStatusHandler struct {
	svc service.AgentHostService
}

func NewAgentStatusHandler(svc service.AgentHostService) *AgentStatusHandler {
	return &AgentStatusHandler{svc: svc}
}

func (h *AgentStatusHandler) Name() string        { return ToolAgentStatus }
func (h *AgentStatusHandler) Description() string { return "获取单个Agent状态" }
func (h *AgentStatusHandler) Handle(ctx context.Context, params any) (*ToolCallResult, error) {
	id, err := parseID(params)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	agent, err := h.svc.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: agent}}}, nil
}

type AgentConfigYAMLHandler struct {
	svc service.AgentHostService
}

func NewAgentConfigYAMLHandler(svc service.AgentHostService) *AgentConfigYAMLHandler {
	return &AgentConfigYAMLHandler{svc: svc}
}

func (h *AgentConfigYAMLHandler) Name() string        { return ToolAgentConfigYAML }
func (h *AgentConfigYAMLHandler) Description() string { return "获取Agent运行配置YAML" }
func (h *AgentConfigYAMLHandler) Handle(ctx context.Context, params any) (*ToolCallResult, error) {
	id, err := parseID(params)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	yaml, err := h.svc.GetConfigYAML(ctx, id)
	if err != nil {
		return nil, err
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "text", Text: yaml}}}, nil
}

type AgentLogsFetchHandler struct {
	cache *service.AgentLogCache
}

func NewAgentLogsFetchHandler(cache *service.AgentLogCache) *AgentLogsFetchHandler {
	return &AgentLogsFetchHandler{cache: cache}
}

func (h *AgentLogsFetchHandler) Name() string        { return ToolAgentLogsFetch }
func (h *AgentLogsFetchHandler) Description() string { return "获取Agent日志" }
func (h *AgentLogsFetchHandler) Handle(ctx context.Context, params any) (*ToolCallResult, error) {
	m, ok := params.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("params must be object")
	}
	hostID, ok := m["host_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("host_id is required")
	}
	level, _ := m["level"].(string)
	limit := 50
	if l, ok := m["limit"].(float64); ok {
		limit = int(l)
	}
	entries, err := h.cache.Fetch(ctx, int64(hostID), level, limit)
	if err != nil {
		return nil, err
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: entries}}}, nil
}

