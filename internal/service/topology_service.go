package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// 契约来源：docs/plans/20260824-topology-editor.md【冻结的 API 契约】。
// 字段名/结构不得偏离契约，前端拓扑画布按此渲染。

// PeerLatencySource 拓扑服务对 mesh 延迟数据的最小依赖（窄接口，
// 由 *agentMeshService.ListPeerLatencySnapshot 具体实现，避免扩大 AgentMeshService mock 面）。
type PeerLatencySource interface {
	ListPeerLatencySnapshot(ctx context.Context) ([]MeshPeerLatencyRecord, error)
}

// TopologyService 拓扑画布聚合查询与一致性校验。
type TopologyService interface {
	Snapshot(ctx context.Context, coreType string) (*TopologySnapshot, error)
	Validate(ctx context.Context) (*TopologyValidationReport, error)
	ReorderPolicies(ctx context.Context, orderedIDs []int64) (int64, error)
}

type topologyService struct {
	agentHosts repository.AgentHostRepository
	specs      repository.InboundSpecRepository
	policies   repository.RoutingPolicyRepository
	sets       ExitNodeSetService
	unlock     repository.UnlockProbeResultRepository
	meshPeers  repository.AgentMeshPeerRepository
	relayPaths repository.RelayPathRepository
	latencies  PeerLatencySource
	logger     *slog.Logger
}

// NewTopologyService 组装拓扑聚合服务；deps 任一为 nil 将在调用时报错，构造期不拦截。
func NewTopologyService(
	agentHosts repository.AgentHostRepository,
	specs repository.InboundSpecRepository,
	policies repository.RoutingPolicyRepository,
	sets ExitNodeSetService,
	unlock repository.UnlockProbeResultRepository,
	meshPeers repository.AgentMeshPeerRepository,
	relayPaths repository.RelayPathRepository,
	latencies PeerLatencySource,
	logger *slog.Logger,
) TopologyService {
	return &topologyService{
		agentHosts: agentHosts, specs: specs, policies: policies, sets: sets,
		unlock: unlock, meshPeers: meshPeers, relayPaths: relayPaths, latencies: latencies, logger: logger,
	}
}

// --- DTO（字段名对齐冻结契约，全部带 json tag）---

// TopologySnapshot 画布全量快照。
type TopologySnapshot struct {
	GeneratedAt int64               `json:"generated_at"`
	Agents      []TopologyAgent     `json:"agents"`
	Mesh        *TopologyMesh       `json:"mesh"`
	Specs       []TopologySpec      `json:"specs"`
	Policies    []*TopologyPolicy   `json:"policies"`
	ExitSets    []*TopologyExitSet  `json:"exit_sets"`
	RelayPaths  []*TopologyRelayPath `json:"relay_paths"`
	Unlock      []TopologyUnlockRow `json:"unlock"`
}

// TopologyRelayPath 中继链路（服务器多跳走向，与入口协议正交）。
type TopologyRelayPath struct {
	ID          int64                    `json:"id"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	CoreType    string                   `json:"core_type"`
	Enabled     bool                     `json:"enabled"`
	Nodes       []TopologyRelayPathNode  `json:"nodes"`
}

// TopologyRelayPathNode 中继链路中的一跳（sequence 0=入口，N-1=出口）。
type TopologyRelayPathNode struct {
	Sequence    int   `json:"sequence"`
	AgentHostID int64 `json:"agent_host_id"`
}

// TopologyAgent 物理节点（在线判定镜像 IsNodeOnline 语义：status==1 或 300s 内有心跳）。
type TopologyAgent struct {
	ID     int64             `json:"id"`
	Name   string            `json:"name"`
	Host   string            `json:"host"`
	Online bool              `json:"online"`
	Cores  []TopologyCoreRef `json:"cores"`
}

// TopologyCoreRef agent 上运行的核心实例引用。
type TopologyCoreRef struct {
	CoreType string `json:"core_type"`
	Active   bool   `json:"active"`
}

// TopologyMesh mesh 组网边集合；Edges 为空时仍返回非 nil 空对象以稳定前端类型。
type TopologyMesh struct {
	Edges []TopologyMeshEdge `json:"edges"`
}

// TopologyMeshEdge 一条有向 mesh 链路及其探测延迟。
type TopologyMeshEdge struct {
	FromAgentID      int64    `json:"from_agent_id"`
	ToAgentID        int64    `json:"to_agent_id"`
	LatencyMs        *float64 `json:"latency_ms"`
	PacketLoss       *float64 `json:"packet_loss,omitempty"`
	HandshakeAgeSec  *int64   `json:"handshake_age_sec"`
}

// TopologySpec 入站 spec 的画布投影（协议/端口/TLS 从 semantic_spec 解析）。
type TopologySpec struct {
	ID              int64           `json:"id"`
	Tag             string          `json:"tag"`
	CoreType        string          `json:"core_type"`
	AgentHostID     *int64          `json:"agent_host_id"`
	Enabled         bool            `json:"enabled"`
	Protocol        string          `json:"protocol"`
	Port            int             `json:"port"`
	TLS             *TopologyTLS    `json:"tls,omitempty"`
	ExitAgentHostID *int64          `json:"exit_agent_host_id"`
	ExitNodeSetID   *int64          `json:"exit_node_set_id"`
}

// TopologyTLS TLS 概要；Reality 存在即视为 reality 启用标记由 Enabled 表达。
type TopologyTLS struct {
	Enabled bool `json:"enabled"`
	Reality bool `json:"reality"`
}

// TopologyPolicy 路由策略的画布投影。
type TopologyPolicy = repository.RoutingPolicy

// TopologyExitSet 出口集及成员（成员内联 name/host，契约要求扁平结构）。
type TopologyExitSet struct {
	ID          int64                  `json:"id"`
	Name        string                 `json:"name"`
	Strategy    string                 `json:"strategy"`
	Enabled     bool                   `json:"enabled"`
	Description string                 `json:"description"`
	Members     []TopologySetMemberRow `json:"members"`
}

// TopologySetMemberRow 成员行（内联 agent 显示信息）。
type TopologySetMemberRow struct {
	AgentHostID int64  `json:"agent_host_id"`
	Weight      int    `json:"weight"`
	Name        string `json:"name"`
	Host        string `json:"host"`
}

// TopologyUnlockRow 最新解锁状态行。
type TopologyUnlockRow struct {
	AgentHostID int64  `json:"agent_host_id"`
	Platform    string `json:"platform"`
	Unlocked    bool   `json:"unlocked"`
	Region      string `json:"region"`
}

// --- semantic_spec 最小解析（只取画布需要的字段，容忍缺失） ---

type topologySemanticProbe struct {
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
	TLS      *struct {
		Enabled bool `json:"enabled"`
		Reality *struct {
			Enabled bool `json:"enabled"`
		} `json:"reality"`
	} `json:"tls"`
}

// --- Snapshot ---

const onlineHeartbeatWindowSec int64 = 300 // 与 IsNodeOnline 的 300s 窗口语义一致

func (s *topologyService) Snapshot(ctx context.Context, coreType string) (*TopologySnapshot, error) {
	start := time.Now()
	snap := &TopologySnapshot{
		GeneratedAt: time.Now().Unix(),
		Agents:      []TopologyAgent{},
		Specs:       []TopologySpec{},
		Policies:    []*TopologyPolicy{},
		ExitSets:    []*TopologyExitSet{},
		RelayPaths:  []*TopologyRelayPath{},
		Unlock:      []TopologyUnlockRow{},
	}

	// 0. relay paths（nil repo 容错：未装配时留空数组，前端画布 C 不渲染链路边）
	if s.relayPaths != nil {
		relayList, err := s.relayPaths.List(ctx, coreType)
		if err != nil {
			return nil, fmt.Errorf("list relay paths: %w", err)
		}
		for _, rp := range relayList {
			nodes := make([]TopologyRelayPathNode, len(rp.Nodes))
			for i, n := range rp.Nodes {
				nodes[i] = TopologyRelayPathNode{Sequence: n.Sequence, AgentHostID: n.AgentHostID}
			}
			snap.RelayPaths = append(snap.RelayPaths, &TopologyRelayPath{
				ID: rp.ID, Name: rp.Name, Description: rp.Description,
				CoreType: rp.CoreType, Enabled: rp.Enabled, Nodes: nodes,
			})
		}
	}

	// 1. agents
	hosts, err := s.agentHosts.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list agent hosts: %w", err)
	}
	now := time.Now().Unix()
	nameByID := make(map[int64]string, len(hosts))
	hostAddrByID := make(map[int64]string, len(hosts))
	for _, h := range hosts {
		nameByID[h.ID] = h.Name
		hostAddrByID[h.ID] = h.Host
		online := h.Status == 1 || (h.LastHeartbeatAt > 0 && now-h.LastHeartbeatAt <= onlineHeartbeatWindowSec)
		snap.Agents = append(snap.Agents, TopologyAgent{
			ID: h.ID, Name: h.Name, Host: h.Host, Online: online,
			Cores: []TopologyCoreRef{}, // 核心实例清单属 runtime 态，画布不依赖时留空
		})
	}
	slog.Debug("topology: agents assembled", "count", len(hosts))

	// 2. specs（可选 core_type 过滤）
	specList, err := s.specs.List(ctx, repository.InboundSpecFilter{CoreType: strPtrIfNotEmpty(coreType)})
	if err != nil {
		return nil, fmt.Errorf("list inbound specs: %w", err)
	}
	for _, sp := range specList {
		ts := TopologySpec{
			ID:              sp.ID,
			Tag:             sp.Tag,
			CoreType:        sp.CoreType,
			AgentHostID:     sp.AgentHostID,
			Enabled:         sp.Enabled,
			ExitAgentHostID: sp.ExitAgentHostID,
			ExitNodeSetID:   sp.ExitNodeSetID,
		}
		var sem topologySemanticProbe
		if len(sp.SemanticSpec) > 0 && json.Unmarshal(sp.SemanticSpec, &sem) == nil {
			ts.Protocol = sem.Protocol
			ts.Port = sem.Port
			if sem.TLS != nil {
				reality := sem.TLS.Reality != nil && sem.TLS.Reality.Enabled
				ts.TLS = &TopologyTLS{Enabled: sem.TLS.Enabled || reality, Reality: reality}
			}
		} else if len(sp.SemanticSpec) > 0 {
			slog.Warn("topology: skip unparseable semantic_spec", "spec_id", sp.ID, "tag", sp.Tag)
		}
		snap.Specs = append(snap.Specs, ts)
	}
	slog.Debug("topology: specs assembled", "count", len(snap.Specs))

	// 3. policies（全量供编辑；前端自行区分 enabled）
	policies, err := s.policies.List(ctx, repository.RoutingPolicyFilter{CoreType: strPtrIfNotEmpty(coreType)})
	if err != nil {
		return nil, fmt.Errorf("list routing policies: %w", err)
	}
	for _, p := range policies {
		snap.Policies = append(snap.Policies, p)
	}
	slog.Debug("topology: policies assembled", "count", len(snap.Policies))

	// 4. exit sets + members（复用 ExitNodeSetService.List 的成员与命名组装）
	details, err := s.sets.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list exit node sets: %w", err)
	}
	for _, d := range details {
		if d.Set == nil {
			continue
		}
		set := &TopologyExitSet{
			ID:          d.Set.ID,
			Name:        d.Set.Name,
			Strategy:    d.Set.Strategy,
			Enabled:     d.Set.Enabled,
			Description: d.Set.Description,
			Members:     []TopologySetMemberRow{},
		}
		for _, m := range d.Members {
			row := TopologySetMemberRow{AgentHostID: m.AgentHostID, Weight: m.Weight}
			row.Name = nameByID[m.AgentHostID]
			row.Host = hostAddrByID[m.AgentHostID]
			set.Members = append(set.Members, row)
		}
		snap.ExitSets = append(snap.ExitSets, set)
	}
	slog.Debug("topology: exit sets assembled", "count", len(snap.ExitSets))

	// 5. unlock（ListAll 已是每 agent+service 一行的最新结果）
	probeRows, err := s.unlock.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list unlock probe results: %w", err)
	}
	for _, u := range probeRows {
		snap.Unlock = append(snap.Unlock, TopologyUnlockRow{
			AgentHostID: u.AgentHostID,
			Platform:    u.Service,
			Unlocked:    u.Status == "unlocked",
			Region:      u.Region,
		})
	}
	slog.Debug("topology: unlock rows assembled", "count", len(snap.Unlock))

	// 6. mesh 边：延迟记录的 pubkey → 目标 agent id；解析失败置 null 不报错
	meshEdges, err := s.buildMeshEdges(ctx, now)
	if err != nil {
		// mesh 数据缺失不应拖垮整个快照：降级为空边集合并告警
		slog.Warn("topology: mesh edges degraded to empty", "error", err)
		meshEdges = []TopologyMeshEdge{}
	}
	snap.Mesh = &TopologyMesh{Edges: meshEdges}

	slog.Info("topology snapshot assembled",
		"agents", len(snap.Agents), "specs", len(snap.Specs),
		"policies", len(snap.Policies), "sets", len(snap.ExitSets),
		"mesh_edges", len(snap.Mesh.Edges), "unlock", len(snap.Unlock),
		"elapsed_ms", time.Since(start).Milliseconds())
	return snap, nil
}

// buildMeshEdges 把带源延迟记录映射为有向边；pubkey 无法归属 peer 时跳过该条。
func (s *topologyService) buildMeshEdges(ctx context.Context, now int64) ([]TopologyMeshEdge, error) {
	if s.latencies == nil {
		return []TopologyMeshEdge{}, nil
	}
	records, err := s.latencies.ListPeerLatencySnapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("list peer latency snapshot: %w", err)
	}
	peers, err := s.meshPeers.ListByNetworkID(ctx, "default")
	if err != nil {
		return nil, fmt.Errorf("list mesh peers: %w", err)
	}
	keyToAgent := make(map[string]int64, len(peers))
	for _, p := range peers {
		keyToAgent[p.WGPublicKey] = p.AgentHostID
	}
	edges := make([]TopologyMeshEdge, 0, len(records))
	for _, rec := range records {
		toID, ok := keyToAgent[rec.PeerWGKey]
		if !ok {
			continue // 探测到已下线/未知 peer，不入图
		}
		lat := rec.LatencyMs
		loss := rec.PacketLoss
		age := now - rec.UpdatedAt
		if age < 0 {
			age = 0
		}
		edges = append(edges, TopologyMeshEdge{
			FromAgentID:     rec.SrcAgentID,
			ToAgentID:       toID,
			LatencyMs:       &lat,
			PacketLoss:      &loss,
			HandshakeAgeSec: &age, // 语义为最近一次探测数据的新鲜度
		})
	}
	return edges, nil
}

func strPtrIfNotEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// --- Validate ---

// TopologyValidationReport 校验报告。
type TopologyValidationReport struct {
	Valid  bool                     `json:"valid"`
	Issues []TopologyValidationIssue `json:"issues"`
}

// TopologyValidationIssue 单条问题；EntityType 取 policy|set|spec。
type TopologyValidationIssue struct {
	Severity   string `json:"severity"` // error | warning
	Code       string `json:"code"`
	Message    string `json:"message"`
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
}

func (s *topologyService) Validate(ctx context.Context) (*TopologyValidationReport, error) {
	report := &TopologyValidationReport{Valid: true, Issues: []TopologyValidationIssue{}}
	add := func(severity, code, msg, entityType string, id int64) {
		report.Issues = append(report.Issues, TopologyValidationIssue{
			Severity: severity, Code: code, Message: msg, EntityType: entityType, EntityID: id,
		})
	}

	sets, err := s.sets.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("validate: list sets: %w", err)
	}
	setByID := make(map[int64]*ExitNodeSetDetail, len(sets))
	for _, d := range sets {
		if d.Set != nil {
			setByID[d.Set.ID] = d
		}
	}

	specList, err := s.specs.List(ctx, repository.InboundSpecFilter{})
	if err != nil {
		return nil, fmt.Errorf("validate: list specs: %w", err)
	}

	// 1/2. 策略目标悬空或指向禁用集
	policies, err := s.policies.List(ctx, repository.RoutingPolicyFilter{})
	if err != nil {
		return nil, fmt.Errorf("validate: list policies: %w", err)
	}
	for _, p := range policies {
		if p.TargetSetID == nil || *p.TargetSetID <= 0 {
			continue
		}
		target, ok := setByID[*p.TargetSetID]
		if !ok {
			add("error", "dangling_target",
				fmt.Sprintf("策略#%d(%s)指向不存在的出口集 #%d", p.ID, p.Name, *p.TargetSetID),
				"policy", p.ID)
			continue
		}
		if target.Set != nil && !target.Set.Enabled {
			add("warning", "disabled_set_ref",
				fmt.Sprintf("策略#%d(%s)指向已禁用的出口集 %s", p.ID, p.Name, target.Set.Name),
				"policy", p.ID)
		}
	}

	// 3. 启用集无成员
	for _, d := range sets {
		if d.Set == nil || !d.Set.Enabled {
			continue
		}
		if len(d.Members) == 0 {
			add("warning", "empty_set",
				fmt.Sprintf("启用中的出口集 %s(#%d)没有任何成员", d.Set.Name, d.Set.ID),
				"set", d.Set.ID)
		}
	}

	// 4. 同 agent 同端口多启用的入站（模板 spec 无宿主，跳过）
	type portKey struct {
		agentID int64
		port    int
	}
	seen := make(map[portKey]string)
	for _, sp := range specList {
		if !sp.Enabled || sp.AgentHostID == nil {
			continue
		}
		var sem topologySemanticProbe
		if len(sp.SemanticSpec) == 0 || json.Unmarshal(sp.SemanticSpec, &sem) != nil || sem.Port == 0 {
			continue
		}
		k := portKey{agentID: *sp.AgentHostID, port: sem.Port}
		if prevTag, dup := seen[k]; dup {
			add("error", "port_conflict",
				fmt.Sprintf("agent#%d 上端口 %d 被 %s 与 %s 同时占用", k.agentID, k.port, prevTag, sp.Tag),
				"spec", sp.ID)
			continue
		}
		seen[k] = sp.Tag
	}

	report.Valid = true
	for _, iss := range report.Issues {
		if iss.Severity == "error" {
			report.Valid = false
			break
		}
	}
	slog.Info("topology validate done", "valid", report.Valid, "issues", len(report.Issues))
	return report, nil
}

// ReorderPolicies 按给定顺序重写策略优先级。
func (s *topologyService) ReorderPolicies(ctx context.Context, orderedIDs []int64) (int64, error) {
	updated, err := s.policies.ReorderPriorities(ctx, orderedIDs)
	if err != nil {
		return 0, fmt.Errorf("reorder policies: %w", err)
	}
	slog.Info("routing policies reordered", "requested", len(orderedIDs), "updated", updated)
	return updated, nil
}
