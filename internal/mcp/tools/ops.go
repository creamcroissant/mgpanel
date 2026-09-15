package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/service"
)

// 本文件提供 MCP 的**写操作**能力（scope = ops）：配置渲染与发布、出口集内核分发模式、
// 路由策略与出口集维护、就绪状态读取。所有写操作都会落一条操作日志（审计）。

// auditAppender 审计落日志（可为 nil）。
type auditAppender struct {
	logs service.OperationLogService
}

func (a auditAppender) append(ctx context.Context, tool string, payload map[string]any) {
	if a.logs == nil {
		return
	}
	raw, _ := json.Marshal(payload)
	_, err := a.logs.Append(ctx, service.AppendOperationLogRequest{
		Scope:    "mcp.ops",
		TargetID: tool,
		Message:  "MCP 写操作: " + tool,
		Payload:  raw,
	})
	_ = err // 审计失败不阻断操作（已有 slog 兜底）
}

func paramMap(params any) (map[string]any, error) {
	if params == nil {
		return map[string]any{}, nil
	}
	m, ok := params.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("params must be object")
	}
	return m, nil
}

func paramInt64(m map[string]any, key string) (int64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case int64:
		return t, true
	case int:
		return int64(t), true
	case string:
		var parsed int64
		if _, err := fmt.Sscanf(strings.TrimSpace(t), "%d", &parsed); err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func paramString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func paramBool(m map[string]any, key string) (bool, bool) {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b, true
		}
	}
	return false, false
}

// ————————————————————————————————————————————————————————————————
// config_render：按当前配置渲染指定主机的产物（不发布），用于预览/干跑。
// ————————————————————————————————————————————————————————————————

type ConfigRenderHandler struct {
	compiler service.ArtifactCompilerService
	diff     service.DriftAndDiffService
	audit    auditAppender
}

func NewConfigRenderHandler(compiler service.ArtifactCompilerService, diff service.DriftAndDiffService, logs service.OperationLogService) *ConfigRenderHandler {
	return &ConfigRenderHandler{compiler: compiler, diff: diff, audit: auditAppender{logs: logs}}
}

func (h *ConfigRenderHandler) Name() string { return ToolConfigRender }
func (h *ConfigRenderHandler) Description() string {
	return "渲染指定主机核心配置产物（不发布），返回文件名与哈希"
}
func (h *ConfigRenderHandler) RequiredScope() string { return ScopeOps }
func (h *ConfigRenderHandler) InputSchema() map[string]any {
	return objectSchema(map[string]any{
		"agent_host_id": intSchema("目标主机 ID"),
		"core_type":     strSchema("sing-box | xray", "sing-box"),
		"revision":      intSchema("目标修订号（省略则用当前最新）"),
	}, []string{"agent_host_id"})
}

func (h *ConfigRenderHandler) Handle(ctx context.Context, params any) (*ToolCallResult, error) {
	m, err := paramMap(params)
	if err != nil {
		return nil, err
	}
	hostID, ok := paramInt64(m, "agent_host_id")
	if !ok || hostID <= 0 {
		return nil, fmt.Errorf("agent_host_id is required")
	}
	coreType := paramString(m, "core_type")
	if coreType == "" {
		coreType = service.CoreTypeSingBox
	}
	revision, _ := paramInt64(m, "revision")
	if revision <= 0 {
		// 省略修订号时取该主机当前修订（与发布流程一致：spec 修订号即目标修订号）
		latest, lerr := h.compiler.GetLatestRevision(ctx, hostID, coreType)
		if lerr != nil {
			return nil, fmt.Errorf("resolve revision: %w", lerr)
		}
		if latest <= 0 {
			return nil, fmt.Errorf("no revision available for host %d (%s)", hostID, coreType)
		}
		revision = latest
	}
	res, err := h.compiler.RenderArtifacts(ctx, service.RenderArtifactsRequest{
		AgentHostID: hostID, CoreType: coreType, DesiredRevision: revision,
	})
	if err != nil {
		return nil, err
	}
	h.audit.append(ctx, h.Name(), map[string]any{
		"agent_host_id": hostID, "core_type": coreType, "revision": res.DesiredRevision, "artifact_count": res.ArtifactCount,
	})
	// 产物清单以库内实际落盘为准（渲染结果的 metadata 只覆盖 spec 派生部分，不含 mesh 产物）。
	var files []map[string]any
	if h.diff != nil {
		listed, lerr := h.diff.ListArtifacts(ctx, service.ListDesiredArtifactsRequest{
			AgentHostID: hostID, CoreType: coreType, DesiredRevision: res.DesiredRevision, Limit: 500,
		})
		if lerr == nil && listed != nil {
			for _, a := range listed.Items {
				files = append(files, map[string]any{"filename": a.Filename, "source_tag": a.SourceTag})
			}
		}
	}
	if files == nil {
		files = make([]map[string]any, 0, len(res.Artifacts))
		for _, a := range res.Artifacts {
			files = append(files, map[string]any{"filename": a.Filename, "source_tag": a.SourceTag})
		}
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: map[string]any{
		"revision":       res.DesiredRevision,
		"artifact_count": res.ArtifactCount,
		"artifacts":      files,
		"warnings":       res.Warnings,
	}}}}, nil
}

// ————————————————————————————————————————————————————————————————
// config_apply：创建/复用发布任务（等价管理端「应用」按钮）。
// ————————————————————————————————————————————————————————————————

type ConfigApplyHandler struct {
	apply    service.ApplyOrchestratorService
	compiler service.ArtifactCompilerService
	audit    auditAppender
}

func NewConfigApplyHandler(apply service.ApplyOrchestratorService, compiler service.ArtifactCompilerService, logs service.OperationLogService) *ConfigApplyHandler {
	return &ConfigApplyHandler{apply: apply, compiler: compiler, audit: auditAppender{logs: logs}}
}

func (h *ConfigApplyHandler) Name() string { return ToolConfigApply }
func (h *ConfigApplyHandler) Description() string {
	return "对指定主机创建发布任务（先重渲染再下发），返回 run_id 与状态"
}
func (h *ConfigApplyHandler) RequiredScope() string { return ScopeOps }
func (h *ConfigApplyHandler) InputSchema() map[string]any {
	return objectSchema(map[string]any{
		"agent_host_id":     intSchema("目标主机 ID"),
		"core_type":         strSchema("sing-box | xray", "sing-box"),
		"target_revision":   intSchema("目标修订号（省略则取最新）"),
		"previous_revision": intSchema("上一修订号（用于回滚展示，可省略）"),
	}, []string{"agent_host_id"})
}

func (h *ConfigApplyHandler) Handle(ctx context.Context, params any) (*ToolCallResult, error) {
	m, err := paramMap(params)
	if err != nil {
		return nil, err
	}
	hostID, ok := paramInt64(m, "agent_host_id")
	if !ok || hostID <= 0 {
		return nil, fmt.Errorf("agent_host_id is required")
	}
	coreType := paramString(m, "core_type")
	if coreType == "" {
		coreType = service.CoreTypeSingBox
	}
	revision, _ := paramInt64(m, "target_revision")
	if revision <= 0 {
		latest, err := h.compiler.GetLatestRevision(ctx, hostID, coreType)
		if err != nil {
			return nil, fmt.Errorf("resolve revision: %w", err)
		}
		revision = latest
	}
	prev, _ := paramInt64(m, "previous_revision")
	run, err := h.apply.PrepareApplyRun(ctx, service.PrepareApplyRunRequest{
		AgentHostID: hostID, CoreType: coreType, TargetRevision: revision, PreviousRevision: prev,
	})
	if err != nil {
		return nil, err
	}
	h.audit.append(ctx, h.Name(), map[string]any{
		"agent_host_id": hostID, "core_type": coreType, "target_revision": revision, "run_id": run.RunID,
	})
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: run}}}, nil
}

// ————————————————————————————————————————————————————————————————
// config_apply_status：查询发布任务状态。
// ————————————————————————————————————————————————————————————————

type ConfigApplyStatusHandler struct {
	apply service.ApplyOrchestratorService
}

func NewConfigApplyStatusHandler(apply service.ApplyOrchestratorService) *ConfigApplyStatusHandler {
	return &ConfigApplyStatusHandler{apply: apply}
}

func (h *ConfigApplyStatusHandler) Name() string { return ToolConfigApplyStatus }
func (h *ConfigApplyStatusHandler) Description() string {
	return "查询发布任务（列表或按 run_id 取详情）"
}
func (h *ConfigApplyStatusHandler) InputSchema() map[string]any {
	return objectSchema(map[string]any{
		"run_id":        strSchema("发布任务 ID（给定则返回详情）"),
		"agent_host_id": intSchema("按主机过滤"),
		"status":        strSchema("pending|applying|success|failed|rolled_back"),
		"limit":         intSchema("条数上限（默认 20）"),
	}, nil)
}

func (h *ConfigApplyStatusHandler) Handle(ctx context.Context, params any) (*ToolCallResult, error) {
	m, err := paramMap(params)
	if err != nil {
		return nil, err
	}
	if runID := paramString(m, "run_id"); runID != "" {
		detail, err := h.apply.GetApplyRunDetail(ctx, service.GetApplyRunDetailRequest{RunID: runID})
		if err != nil {
			return nil, err
		}
		return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: detail}}}, nil
	}
	limit, _ := paramInt64(m, "limit")
	if limit <= 0 {
		limit = 20
	}
	var hostID *int64
	if v, ok := paramInt64(m, "agent_host_id"); ok && v > 0 {
		hostID = &v
	}
	res, err := h.apply.ListApplyRuns(ctx, service.ListApplyRunsRequest{
		AgentHostID: hostID, CoreType: paramString(m, "core_type"), Status: paramString(m, "status"),
		Limit: int(limit),
	})
	if err != nil {
		return nil, err
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: res}}}, nil
}

// ————————————————————————————————————————————————————————————————
// egress_mode：读取/设置主机的出口集内核分发模式（inherit | socks | l3）。
// ————————————————————————————————————————————————————————————————

type EgressModeHandler struct {
	hosts service.AgentHostService
	audit auditAppender
}

func NewEgressModeHandler(hosts service.AgentHostService, logs service.OperationLogService) *EgressModeHandler {
	return &EgressModeHandler{hosts: hosts, audit: auditAppender{logs: logs}}
}

func (h *EgressModeHandler) Name() string { return ToolEgressMode }
func (h *EgressModeHandler) Description() string {
	return "读取或设置主机出口集分发模式（inherit/socks/l3）；带 mode 参数即写入"
}
func (h *EgressModeHandler) RequiredScope() string { return ScopeOps }
func (h *EgressModeHandler) InputSchema() map[string]any {
	return objectSchema(map[string]any{
		"agent_host_id": intSchema("目标主机 ID"),
		"mode":          strSchema("inherit | socks | l3；省略则只读"),
	}, []string{"agent_host_id"})
}

func (h *EgressModeHandler) Handle(ctx context.Context, params any) (*ToolCallResult, error) {
	m, err := paramMap(params)
	if err != nil {
		return nil, err
	}
	hostID, ok := paramInt64(m, "agent_host_id")
	if !ok || hostID <= 0 {
		return nil, fmt.Errorf("agent_host_id is required")
	}
	mode := paramString(m, "mode")
	if mode != "" {
		if !service.IsValidEgressDispatchMode(mode) {
			return nil, fmt.Errorf("invalid mode %q (inherit|socks|l3)", mode)
		}
		if !hasOps(ctx) {
			return nil, fmt.Errorf("scope ops required for write")
		}
		if err := h.hosts.Update(ctx, hostID, service.UpdateAgentHostRequest{EgressDispatch: &mode}); err != nil {
			return nil, err
		}
		h.audit.append(ctx, h.Name(), map[string]any{"agent_host_id": hostID, "mode": mode})
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

// ————————————————————————————————————————————————————————————————
// routing_policy：列出/创建更新/删除路由策略。
// ————————————————————————————————————————————————————————————————

type RoutingPolicyHandler struct {
	policies service.RoutingPolicyService
	audit    auditAppender
}

func NewRoutingPolicyHandler(policies service.RoutingPolicyService, logs service.OperationLogService) *RoutingPolicyHandler {
	return &RoutingPolicyHandler{policies: policies, audit: auditAppender{logs: logs}}
}

func (h *RoutingPolicyHandler) Name() string { return ToolRoutingPolicy }
func (h *RoutingPolicyHandler) Description() string {
	return "路由策略：action=list|upsert|delete"
}
func (h *RoutingPolicyHandler) RequiredScope() string { return ScopeOps }
func (h *RoutingPolicyHandler) InputSchema() map[string]any {
	return objectSchema(map[string]any{
		"action":        strSchema("list | upsert | delete", "list"),
		"id":            intSchema("策略 ID（upsert 更新 / delete 必填）"),
		"name":          strSchema("策略名"),
		"core_type":     strSchema("sing-box | xray", "sing-box"),
		"match_type":    strSchema("domain | geosite | ip_cidr"),
		"match_value":   strSchema("匹配值，多值逗号分隔（如 ipinfo.io）"),
		"target_set_id": intSchema("目标出口集 ID"),
		"spec_id":       intSchema("限定入站 spec（省略为全局策略）"),
		"sticky":        boolSchema("池内粘性（默认 true）"),
		"enabled":       boolSchema("是否启用"),
		"priority":      intSchema("优先级（越小越先）"),
		"target_action": strSchema("动作（默认 route_to_set）"),
	}, []string{"action"})
}

func (h *RoutingPolicyHandler) Handle(ctx context.Context, params any) (*ToolCallResult, error) {
	m, err := paramMap(params)
	if err != nil {
		return nil, err
	}
	action := paramString(m, "action")
	switch action {
	case "list", "":
		items, err := h.policies.List(ctx, paramString(m, "core_type"))
		if err != nil {
			return nil, err
		}
		return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: items}}}, nil

	case "upsert":
		if !hasOps(ctx) {
			return nil, fmt.Errorf("scope ops required for write")
		}
		req := service.RoutingPolicyUpsertRequest{
			ID:         mustInt(m, "id"),
			Name:       paramString(m, "name"),
			CoreType:   paramString(m, "core_type"),
			Priority:   int(mustInt(m, "priority")),
			MatchType:  paramString(m, "match_type"),
			MatchValue: paramString(m, "match_value"),
			Action:     paramString(m, "target_action"),
		}
		if req.Action == "" {
			req.Action = "route_to_set"
		}
		if v, ok := paramInt64(m, "target_set_id"); ok {
			req.TargetSetID = &v
		}
		if v, ok := paramInt64(m, "spec_id"); ok {
			req.SpecID = &v
		}
		if v, ok := paramBool(m, "sticky"); ok {
			req.Sticky = &v
		}
		if v, ok := paramBool(m, "enabled"); ok {
			req.Enabled = &v
		}
		var policy any
		if req.ID > 0 {
			policy, err = h.policies.Update(ctx, req)
		} else {
			policy, err = h.policies.Create(ctx, req)
		}
		if err != nil {
			return nil, err
		}
		h.audit.append(ctx, h.Name(), map[string]any{"action": "upsert", "policy": policy})
		return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: policy}}}, nil

	case "delete":
		if !hasOps(ctx) {
			return nil, fmt.Errorf("scope ops required for write")
		}
		id := mustInt(m, "id")
		if id <= 0 {
			return nil, fmt.Errorf("id is required for delete")
		}
		if err := h.policies.Delete(ctx, id); err != nil {
			return nil, err
		}
		h.audit.append(ctx, h.Name(), map[string]any{"action": "delete", "id": id})
		return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: map[string]any{"deleted": id}}}}, nil

	default:
		return nil, fmt.Errorf("unknown action %q (list|upsert|delete)", action)
	}
}

// ————————————————————————————————————————————————————————————————
// exit_node_set：列出集合、增删成员。
// ————————————————————————————————————————————————————————————————

type ExitNodeSetHandler struct {
	sets  service.ExitNodeSetService
	audit auditAppender
}

func NewExitNodeSetHandler(sets service.ExitNodeSetService, logs service.OperationLogService) *ExitNodeSetHandler {
	return &ExitNodeSetHandler{sets: sets, audit: auditAppender{logs: logs}}
}

func (h *ExitNodeSetHandler) Name() string { return ToolExitNodeSet }
func (h *ExitNodeSetHandler) Description() string {
	return "出口集合：action=list|add_member|remove_member|set_member"
}
func (h *ExitNodeSetHandler) RequiredScope() string { return ScopeOps }
func (h *ExitNodeSetHandler) InputSchema() map[string]any {
	return objectSchema(map[string]any{
		"action":        strSchema("list | add_member | remove_member | set_member", "list"),
		"set_id":        intSchema("出口集合 ID"),
		"agent_host_id": intSchema("成员主机 ID"),
		"weight":        intSchema("权重（默认 1）"),
		"enabled":       boolSchema("成员是否启用"),
	}, []string{"action"})
}

func (h *ExitNodeSetHandler) Handle(ctx context.Context, params any) (*ToolCallResult, error) {
	m, err := paramMap(params)
	if err != nil {
		return nil, err
	}
	switch action := paramString(m, "action"); action {
	case "list", "":
		items, err := h.sets.List(ctx)
		if err != nil {
			return nil, err
		}
		return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: items}}}, nil

	case "add_member", "set_member", "remove_member":
		if !hasOps(ctx) {
			return nil, fmt.Errorf("scope ops required for write")
		}
		setID := mustInt(m, "set_id")
		hostID := mustInt(m, "agent_host_id")
		if setID <= 0 || hostID <= 0 {
			return nil, fmt.Errorf("set_id and agent_host_id are required")
		}
		if action == "remove_member" {
			if err := h.sets.RemoveMember(ctx, setID, hostID); err != nil {
				return nil, err
			}
		} else {
			req := service.ExitNodeSetMemberRequest{SetID: setID, AgentHostID: hostID, Weight: int(mustInt(m, "weight"))}
			if req.Weight <= 0 {
				req.Weight = 1
			}
			if v, ok := paramBool(m, "enabled"); ok {
				req.Enabled = &v
			}
			if action == "add_member" {
				err = h.sets.AddMember(ctx, req)
			} else {
				err = h.sets.UpdateMember(ctx, req)
			}
			if err != nil {
				return nil, err
			}
		}
		h.audit.append(ctx, h.Name(), map[string]any{"action": action, "set_id": setID, "agent_host_id": hostID})
		detail, err := h.sets.Get(ctx, setID)
		if err != nil {
			return nil, err
		}
		return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: detail}}}, nil

	default:
		return nil, fmt.Errorf("unknown action %q", action)
	}
}

// ————————————————————————————————————————————————————————————————
// egress_pairs / inbound_specs：就绪与拓扑读取。
// ————————————————————————————————————————————————————————————————

type EgressPairListHandler struct {
	pairs interface {
		ListAll(ctx context.Context) ([]*repository.EgressDispatchPair, error)
	}
}

func NewEgressPairListHandler(pairs interface {
	ListAll(ctx context.Context) ([]*repository.EgressDispatchPair, error)
}) *EgressPairListHandler {
	return &EgressPairListHandler{pairs: pairs}
}

func (h *EgressPairListHandler) Name() string { return ToolEgressPairList }
func (h *EgressPairListHandler) Description() string {
	return "列出出口集分发配对（入口↔成员、端口、隧道网段）"
}
func (h *EgressPairListHandler) Handle(ctx context.Context, _ any) (*ToolCallResult, error) {
	items, err := h.pairs.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: items}}}, nil
}

type InboundSpecListHandler struct {
	specs service.InboundSpecService
}

func NewInboundSpecListHandler(specs service.InboundSpecService) *InboundSpecListHandler {
	return &InboundSpecListHandler{specs: specs}
}

func (h *InboundSpecListHandler) Name() string { return ToolInboundSpecList }
func (h *InboundSpecListHandler) Description() string {
	return "列出入站 spec（含绑定主机、中继/出口引用）"
}
func (h *InboundSpecListHandler) InputSchema() map[string]any {
	return objectSchema(map[string]any{
		"agent_host_id": intSchema("按主机过滤"),
		"core_type":     strSchema("sing-box | xray"),
		"limit":         intSchema("条数上限（默认 100）"),
	}, nil)
}

func (h *InboundSpecListHandler) Handle(ctx context.Context, params any) (*ToolCallResult, error) {
	m, err := paramMap(params)
	if err != nil {
		return nil, err
	}
	limit := mustInt(m, "limit")
	if limit <= 0 {
		limit = 100
	}
	var hostID *int64
	if v, ok := paramInt64(m, "agent_host_id"); ok && v > 0 {
		hostID = &v
	}
	filter := service.ListInboundSpecFilter{AgentHostID: hostID, Limit: int(limit)}
	if core := paramString(m, "core_type"); core != "" {
		filter.CoreType = &core
	}
	specs, total, err := h.specs.ListSpecs(ctx, filter)
	if err != nil {
		return nil, err
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: map[string]any{
		"total": total,
		"items": specs,
	}}}}, nil
}

// ————————————————————————————————————————————————————————————————
// 小工具：schema 构造、上下文作用域、参数兜底
// ————————————————————————————————————————————————————————————————

func objectSchema(props map[string]any, required []string) map[string]any {
	if required == nil {
		required = []string{}
	}
	return map[string]any{"type": "object", "properties": props, "required": required}
}

func intSchema(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}
func boolSchema(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}
func strSchema(desc string, def ...string) map[string]any {
	schema := map[string]any{"type": "string", "description": desc}
	if len(def) > 0 && def[0] != "" {
		schema["default"] = def[0]
	}
	return schema
}

func mustInt(m map[string]any, key string) int64 {
	v, _ := paramInt64(m, key)
	return v
}

// hasOps 请求上下文是否具备 ops 作用域（写路径二次校验）。
func hasOps(ctx context.Context) bool { return HasScope(ctx, ScopeOps) }

// ————————————————————————————————————————————————————————————————
// config_sync：推进修订号并按当前全局配置重渲染（内容变更后使变更可交付）。
// ————————————————————————————————————————————————————————————————

type ConfigSyncHandler struct {
	specs service.InboundSpecService
	audit auditAppender
}

func NewConfigSyncHandler(specs service.InboundSpecService, logs service.OperationLogService) *ConfigSyncHandler {
	return &ConfigSyncHandler{specs: specs, audit: auditAppender{logs: logs}}
}

func (h *ConfigSyncHandler) Name() string { return ToolConfigSync }
func (h *ConfigSyncHandler) Description() string {
	return "推进主机修订号并用当前路由策略/出口集重渲染（使全局配置变更可交付），需 ops"
}
func (h *ConfigSyncHandler) RequiredScope() string { return ScopeOps }
func (h *ConfigSyncHandler) InputSchema() map[string]any {
	return objectSchema(map[string]any{
		"agent_host_id": intSchema("目标主机 ID"),
		"core_type":     strSchema("可选：仅该核心（sing-box | xray）"),
		"note":          strSchema("写入修订历史的说明（默认 auto: rendered output changed）"),
	}, []string{"agent_host_id"})
}

func (h *ConfigSyncHandler) Handle(ctx context.Context, params any) (*ToolCallResult, error) {
	m, err := paramMap(params)
	if err != nil {
		return nil, err
	}
	hostID, ok := paramInt64(m, "agent_host_id")
	if !ok || hostID <= 0 {
		return nil, fmt.Errorf("agent_host_id is required")
	}
	bumped, err := h.specs.BumpRevisionsForHost(ctx, hostID, paramString(m, "core_type"), paramString(m, "note"))
	if err != nil {
		return nil, err
	}
	h.audit.append(ctx, h.Name(), map[string]any{"agent_host_id": hostID, "bumped": bumped})
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: map[string]any{
		"agent_host_id": hostID, "bumped_specs": bumped,
	}}}}, nil
}
