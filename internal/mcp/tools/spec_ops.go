package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/service"
)

// inbound_spec_upsert：创建/更新入站规格（含 reality / 中继绑定 / 出口集），scope=ops。
// 这是「批量开通节点」的核心写操作：spec 落地后由 agent 渲染并上报库存 → 自动生成订阅节点。

type InboundSpecUpsertHandler struct {
	specs service.InboundSpecService
	audit auditAppender
}

func NewInboundSpecUpsertHandler(specs service.InboundSpecService, logs service.OperationLogService) *InboundSpecUpsertHandler {
	return &InboundSpecUpsertHandler{specs: specs, audit: auditAppender{logs: logs}}
}

func (h *InboundSpecUpsertHandler) Name() string          { return ToolInboundSpecUpsert }
func (h *InboundSpecUpsertHandler) RequiredScope() string { return ScopeOps }
func (h *InboundSpecUpsertHandler) Description() string {
	return "创建/更新入站规格（vless+reality 等），可绑定中继链路或出口集；需 ops"
}

func (h *InboundSpecUpsertHandler) InputSchema() map[string]any {
	return objectSchema(map[string]any{
		"spec_id":            intSchema("更新时给出；省略为新建"),
		"agent_host_id":      intSchema("绑定主机（省略为模板 spec）"),
		"core_type":          strSchema("sing-box | xray", "sing-box"),
		"tag":                strSchema("入站 tag（也是节点协议标识）"),
		"semantic_spec":      objSchema("入站语义 JSON（listen/port/protocol/tls/users）"),
		"core_specific":      objSchema("核心专属配置（如 v2ray_api）"),
		"relay_path_id":      intSchema("绑定中继链路（入口须为该链路首跳）"),
		"exit_node_set_id":   intSchema("出口集合（负载均衡）"),
		"exit_agent_host_id": intSchema("固定出口主机"),
		"enabled":            boolSchema("是否启用（默认 true）"),
		"change_note":        strSchema("修订说明"),
	}, []string{"core_type", "tag", "semantic_spec"})
}

func (h *InboundSpecUpsertHandler) Handle(ctx context.Context, params any) (*ToolCallResult, error) {
	m, err := paramMap(params)
	if err != nil {
		return nil, err
	}
	req := service.UpsertInboundSpecRequest{
		CoreType:   paramString(m, "core_type"),
		Tag:        paramString(m, "tag"),
		ChangeNote: paramString(m, "change_note"),
	}
	if req.CoreType == "" {
		req.CoreType = service.CoreTypeSingBox
	}
	if strings.TrimSpace(req.Tag) == "" {
		return nil, fmt.Errorf("tag is required")
	}
	if req.SpecID, _ = paramInt64(m, "spec_id"); false {
	}
	req.SpecID = mustInt(m, "spec_id")
	if v, ok := paramInt64(m, "agent_host_id"); ok && v > 0 {
		req.AgentHostID = &v
	}
	if v, ok := paramInt64(m, "relay_path_id"); ok && v > 0 {
		req.RelayPathID = &v
	}
	if v, ok := paramInt64(m, "exit_node_set_id"); ok && v > 0 {
		req.ExitNodeSetID = &v
	}
	if v, ok := paramInt64(m, "exit_agent_host_id"); ok && v > 0 {
		req.ExitAgentHostID = &v
	}
	if v, ok := paramBool(m, "enabled"); ok {
		req.Enabled = &v
	}
	if raw, ok := m["semantic_spec"]; ok {
		encoded, merr := json.Marshal(raw)
		if merr != nil {
			return nil, fmt.Errorf("encode semantic_spec: %w", merr)
		}
		if !json.Valid(encoded) {
			return nil, fmt.Errorf("semantic_spec must be a JSON object")
		}
		req.SemanticSpec = encoded
	}
	if raw, ok := m["core_specific"]; ok {
		encoded, merr := json.Marshal(raw)
		if merr != nil {
			return nil, fmt.Errorf("encode core_specific: %w", merr)
		}
		req.CoreSpecific = encoded
	}
	specID, revision, err := h.specs.UpsertSpec(ctx, req)
	if err != nil {
		return nil, err
	}
	h.audit.append(ctx, h.Name(), map[string]any{
		"spec_id": specID, "agent_host_id": req.AgentHostID, "tag": req.Tag, "revision": revision,
		"relay_path_id": req.RelayPathID, "exit_node_set_id": req.ExitNodeSetID,
	})
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: map[string]any{
		"spec_id": specID, "revision": revision, "tag": req.Tag,
	}}}}, nil
}

func objSchema(desc string) map[string]any {
	return map[string]any{"type": "object", "description": desc}
}
