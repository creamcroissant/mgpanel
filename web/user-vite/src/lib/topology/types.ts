// 拓扑画布图模型与 API 快照类型。
// 字段名与 docs/plans/20260824-topology-editor.md【冻结契约】逐一对应。

// ===== API 快照（服务端聚合返回） =====

export interface TopologyAgent {
  id: number;
  name: string;
  host: string;
  online: boolean;
  cores: { core_type: string; active: boolean }[];
}

export interface MeshEdgeInfo {
  from_agent_id: number;
  to_agent_id: number;
  latency_ms: number | null;
  handshake_age_sec: number | null;
}

export interface TopologySpec {
  id: number;
  tag: string;
  core_type: string;
  agent_host_id: number | null;
  enabled: boolean;
  protocol: string;
  port: number;
  tls: { enabled: boolean; reality: boolean };
  exit_agent_host_id: number | null;
  exit_node_set_id: number | null;
  /** 绑定的服务器中继链路（优先于两个 exit 字段；null=未绑定）*/
  relay_path_id: number | null;
}

export interface TopologyPolicy {
  id: number;
  name: string;
  priority: number;
  match_type: string;
  match_value: string;
  action: string;
  target_set_id: number | null;
  /** 作用域：null/缺省 = 全局；非空 = 仅对该入站 spec 生效（渲染时排在全局之前） */
  spec_id?: number | null;
  enabled: boolean;
}

export interface ExitSetMember {
  agent_host_id: number;
  weight: number;
  name: string;
  host: string;
}

export interface TopologyExitSet {
  id: number;
  name: string;
  strategy: string;
  enabled: boolean;
  members: ExitSetMember[];
}

export interface UnlockProbeSummary {
  agent_host_id: number;
  platform: string;
  unlocked: boolean;
  region: string | null;
}

/** 中继链路节点：sequence 0 = 入口，N-1 = 出口 */
export interface RelayPathNode {
  sequence: number;
  agent_host_id: number;
}

/** 中继链路：受控服务器间的多跳转发路径（冻结契约，后端并行实现中） */
export interface RelayPathInfo {
  id: number;
  name: string;
  description: string;
  core_type: string;
  enabled: boolean;
  nodes: RelayPathNode[];
}

/** GET /api/v2/admin/topology 的 data 载荷 */
export interface TopologySnapshot {
  generated_at: number;
  agents: TopologyAgent[];
  mesh: { edges: MeshEdgeInfo[] };
  specs: TopologySpec[];
  policies: TopologyPolicy[];
  exit_sets: TopologyExitSet[];
  unlock: UnlockProbeSummary[];
  /** 服务器中继链路；旧后端无此键时 undefined（画布 C 降级为纯节点图） */
  relay_paths?: RelayPathInfo[];
}

/** POST /api/v2/admin/topology/validate 响应 */
export interface TopologyIssue {
  severity: "error" | "warning";
  code: string;
  message: string;
  entity_type: string;
  entity_id: number;
}

export interface TopologyValidation {
  valid: boolean;
  issues: TopologyIssue[];
}

// ===== 画布图模型 =====

export type NodeKind = "agent" | "inbound" | "rule" | "set" | "direct" | "fallback";

/**
 * 通用节点载荷：不同 kind 使用各自字段；stub 阶段统一用
 * 可选字段承载关键一行数据，后续波次实体化时细化。
 */
export interface TopoNodeData extends Record<string, unknown> {
  label: string;
  /** 节点种类回填：DirectNodeView 以此区分 direct/fallback 展示语义 */
  kind?: NodeKind;
  /** rule: 匹配类型图标语义 geosite|domain|ip_cidr */
  matchType?: string;
  /** rule/set: 启用状态 */
  enabled?: boolean;
  /** set: 均衡策略 round_robin|weighted_random|least_ping|random */
  strategy?: string;
  /** inbound: 协议+端口摘要 */
  summary?: string;
  /** agent: 主机地址与在线态 */
  host?: string;
  online?: boolean;
  /** agent: 核心实例列表（active 者渲染高亮徽章） */
  cores?: { core_type: string; active: boolean }[];
  /** set: 成员权重摘要行 */
  membersLine?: string;
  priority?: number;
  /** agent(画布C)：作为中继链路入口时绑定的入站数量角标 */
  relayBoundCount?: number;
  /** validate 注入的节点级问题（f4 交互层写入，节点视图渲染角标） */
  issues?: { severity: string; message: string }[];
}

export interface TopoNode {
  id: string;
  kind: NodeKind;
  data: TopoNodeData;
}

export type EdgeKind = "eval-order" | "routes-to" | "default-exit" | "mesh" | "hosted-on" | "set-member" | "relay" | "relay-bind";

export interface TopoEdge {
  id: string;
  source: string;
  target: string;
  kind: EdgeKind;
  label?: string;
  /** mesh 边专用：探测延迟（null=暂无数据） */
  latencyMs?: number | null;
  /** mesh 边专用：最近握手秒龄（>600 视为陈旧，边转警示色） */
  handshakeAgeSec?: number | null;
  /** 链路所属 path 禁用时边虚线灰显 */
  dimmed?: boolean;
}

/** 组装产物：ReactFlow 直接消费的节点/边集合 */
export interface TopologyGraph {
  nodes: TopoNode[];
  edges: TopoEdge[];
}
