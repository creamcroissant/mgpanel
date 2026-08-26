import type {
  EdgeKind,
  NodeKind,
  TopoEdge,
  TopoNode,
  TopoNodeData,
  TopologyGraph,
  TopologySnapshot,
} from "./types";

/** 节点 id 前缀：避免不同实体间 id 冲突 */
const PREFIX = {
  agent: "agent",
  inbound: "inbound",
  rule: "rule",
  set: "set",
  direct: "direct",
  fallback: "fallback",
} as const;

function node(kind: NodeKind, key: string | number, data: TopoNodeData): TopoNode {
  return { id: `${PREFIX[kind]}-${key}`, kind, data };
}

/**
 * 把服务端拓扑快照组装为画布图。
 * 规则（见 plan 冻结契约）：
 * - policies 按 priority 升序串成 eval-order 链；
 * - 每条 policy 一条 routes-to 边到其 target set 节点；
 * - 每个 enabled spec 一条 default-exit 边到其 exit_node_set / exit_agent 对应节点，
 *   都没有则指向 direct 兜底节点（仅当存在此类 spec 时才生成 direct 节点）。
 */
export function assembleTopologyGraph(snapshot: TopologySnapshot): TopologyGraph {
  const nodes: TopoNode[] = [];
  const edges: TopoEdge[] = [];

  // ---- 出口集合节点 ----
  for (const set of snapshot.exit_sets) {
    const membersLine =
      set.members.map((m) => `${m.name}×${m.weight}`).join(" / ") || "无成员";
    nodes.push(node("set", set.id, {
      label: set.name,
      strategy: set.strategy,
      enabled: set.enabled,
      membersLine,
    }));
  }

  // ---- agent 物理节点（有 mesh 边或被 spec/成员引用才上画布，避免孤岛噪音）----
  const referencedAgentIds = new Set<number>();
  for (const e of snapshot.mesh.edges) {
    referencedAgentIds.add(e.from_agent_id);
    referencedAgentIds.add(e.to_agent_id);
  }
  for (const s of snapshot.specs) {
    if (s.agent_host_id != null) referencedAgentIds.add(s.agent_host_id);
    if (s.exit_agent_host_id != null) referencedAgentIds.add(s.exit_agent_host_id);
    if (s.relay_path_id != null && s.relay_path_id > 0) {
      const rp = (snapshot.relay_paths ?? []).find((p) => p.id === s.relay_path_id);
      const firstHop = rp?.nodes.find((n) => n.sequence === 0);
      if (firstHop) referencedAgentIds.add(firstHop.agent_host_id);
    }
  }
  for (const set of snapshot.exit_sets) {
    for (const m of set.members) referencedAgentIds.add(m.agent_host_id);
  }
  const agentById = new Map(snapshot.agents.map((a) => [a.id, a]));
  for (const aid of referencedAgentIds) {
    const a = agentById.get(aid);
    if (!a) continue;
    nodes.push(node("agent", a.id, {
      label: a.name,
      host: a.host,
      online: a.online,
      summary: a.cores.filter((c) => c.active).map((c) => c.core_type).join(","),
      cores: a.cores,
    }));
  }

  // ---- mesh 物理链路：agent↔agent 虚线边 + 延迟/握手元数据 ----
  for (const m of snapshot.mesh.edges) {
    if (!agentById.has(m.from_agent_id) || !agentById.has(m.to_agent_id)) continue;
    edges.push({
      id: `mesh-${m.from_agent_id}-${m.to_agent_id}`,
      source: `${PREFIX.agent}-${m.from_agent_id}`,
      target: `${PREFIX.agent}-${m.to_agent_id}`,
      kind: "mesh" satisfies EdgeKind,
      latencyMs: m.latency_ms,
      handshakeAgeSec: m.handshake_age_sec,
    });
  }

  // ---- 规则链节点 + eval-order + routes-to ----
  const orderedPolicies = [...snapshot.policies].sort(
    (a, b) => a.priority - b.priority
  );
  orderedPolicies.forEach((p, idx) => {
    const ruleNode = node("rule", p.id, {
      label: p.name || `#${p.id}`,
      matchType: p.match_type,
      summary: p.match_value,
      priority: p.priority,
      specId: p.spec_id ?? null,
      enabled: p.enabled,
    });
    nodes.push(ruleNode);

    // eval-order 链：相邻两条策略按求值顺序相连
    if (idx > 0) {
      const prev = orderedPolicies[idx - 1];
      edges.push({
        id: `eval-${prev.id}-${p.id}`,
        source: `${PREFIX.rule}-${prev.id}`,
        target: ruleNode.id,
        kind: "eval-order" satisfies EdgeKind,
        label: "未命中则下一条",
      });
    }

    // routes-to：命中后流向目标出口集
    if (p.target_set_id != null && p.target_set_id > 0) {
      edges.push({
        id: `routes-${p.id}-${p.target_set_id}`,
        source: ruleNode.id,
        target: `${PREFIX.set}-${p.target_set_id}`,
        kind: "routes-to" satisfies EdgeKind,
        label: "命中",
      });
    }
  });

  // ---- 入站节点 + default-exit ----
  let hasDirectFallback = false;
  const relayById = new Map((snapshot.relay_paths ?? []).map((p) => [p.id, p]));
  for (const s of snapshot.specs) {
    if (!s.enabled) continue;
    nodes.push(node("inbound", s.id, {
      label: s.tag,
      summary: `${s.protocol}:${s.port}${s.tls.reality ? " · reality" : s.tls.enabled ? " · tls" : ""}`,
      enabled: s.enabled,
    }));

    // 中继链路绑定优先：入站流量走 relay 首跳 agent（虚线区分，非直接出口）
    if (s.relay_path_id != null && s.relay_path_id > 0) {
      const rp = relayById.get(s.relay_path_id);
      const firstHop = rp?.nodes.find((n) => n.sequence === 0);
      if (rp && firstHop && agentById.has(firstHop.agent_host_id)) {
        edges.push({
          id: `relay-bind-${s.id}-${rp.id}`,
          source: `${PREFIX.inbound}-${s.id}`,
          target: `${PREFIX.agent}-${firstHop.agent_host_id}`,
          kind: "relay-bind" satisfies EdgeKind,
          label: `⛓ 中继链路:${rp.name}`,
        });
        continue;
      }
    }

    // hosted-on：入站与宿主 agent 的细虚线关联（视觉归属，非流量路径）
    if (s.agent_host_id != null && s.agent_host_id > 0 && agentById.has(s.agent_host_id)) {
      edges.push({
        id: `hosted-${s.id}-${s.agent_host_id}`,
        source: `${PREFIX.inbound}-${s.id}`,
        target: `${PREFIX.agent}-${s.agent_host_id}`,
        kind: "hosted-on" satisfies EdgeKind,
        label: "宿主",
      });
    }

    if (s.exit_node_set_id != null && s.exit_node_set_id > 0) {
      edges.push({
        id: `exit-set-${s.id}-${s.exit_node_set_id}`,
        source: `${PREFIX.inbound}-${s.id}`,
        target: `${PREFIX.set}-${s.exit_node_set_id}`,
        kind: "default-exit" satisfies EdgeKind,
        label: "默认出口",
      });
    } else if (s.exit_agent_host_id != null && s.exit_agent_host_id > 0) {
      edges.push({
        id: `exit-agent-${s.id}-${s.exit_agent_host_id}`,
        source: `${PREFIX.inbound}-${s.id}`,
        target: `${PREFIX.agent}-${s.exit_agent_host_id}`,
        kind: "default-exit" satisfies EdgeKind,
        label: "固定出口",
      });
    } else {
      hasDirectFallback = true;
      edges.push({
        id: `exit-direct-${s.id}`,
        source: `${PREFIX.inbound}-${s.id}`,
        target: `${PREFIX.direct}-0`,
        kind: "default-exit" satisfies EdgeKind,
        label: "本机直连出网",
      });
    }
  }

  if (hasDirectFallback) {
    nodes.push(node("direct", 0, { label: "direct", summary: "直连出网兜底" }));
  }

  return { nodes, edges };
}

/**
 * 画布 A：规则分流。仅 rule/set/agent/direct/fallback 节点，
 * 不含 inbound/物理层/mesh 边/hosted-on。
 *
 * 规则链：
 * - policies 按 priority 升序串成 eval-order 链；
 * - 每条 policy 一条 routes-to 边到其 target set 节点；
 * - 最后一条规则→direct 兜底（未命中规则则直连）；
 * - 每个出口集以其成员 agent 节点展开，set→member 用 set-member 边关联。
 */
export function assembleCanvasA(snapshot: TopologySnapshot): TopologyGraph {
  const nodes: TopoNode[] = [];
  const edges: TopoEdge[] = [];

  // ---- 出口集节点 + set-member 边到成员 agent ----
  const agentById = new Map(snapshot.agents.map((a) => [a.id, a]));
  const refAgentIds = new Set<number>();
  for (const set of snapshot.exit_sets) {
    nodes.push(
      node("set", set.id, {
        label: set.name,
        strategy: set.strategy,
        enabled: set.enabled,
        membersLine:
          set.members.map((m) => `${m.name}×${m.weight}`).join(" / ") || "无成员",
      })
    );
    for (const m of set.members) refAgentIds.add(m.agent_host_id);
  }

  // 被固定出口 spec 引用的 agent（入站节点本身不出现，但 agent 作为出口目标可见）
  for (const s of snapshot.specs) {
    if (s.exit_agent_host_id != null) refAgentIds.add(s.exit_agent_host_id);
  }

  for (const aid of refAgentIds) {
    const a = agentById.get(aid);
    if (!a) continue;
    nodes.push(
      node("agent", a.id, {
        label: a.name,
        host: a.host,
        online: a.online,
        summary: a.cores
          .filter((c) => c.active)
          .map((c) => c.core_type)
          .join(","),
        cores: a.cores,
      })
    );
  }

  // 出口集→成员 agent 边（set-member 关系）
  for (const set of snapshot.exit_sets) {
    for (const m of set.members) {
      if (!agentById.has(m.agent_host_id)) continue;
      edges.push({
        id: `setmem-${set.id}-${m.agent_host_id}`,
        source: `${PREFIX.set}-${set.id}`,
        target: `${PREFIX.agent}-${m.agent_host_id}`,
        kind: "set-member" satisfies EdgeKind,
        label: `${m.weight}`,
      });
    }
  }

  // ---- 规则链：eval-order + routes-to ----
  const ordered = [...snapshot.policies].sort((a, b) => a.priority - b.priority);
  ordered.forEach((p, idx) => {
    const ruleNode = node("rule", p.id, {
      label: p.name || `#${p.id}`,
      matchType: p.match_type,
      summary: p.match_value,
      priority: p.priority,
      enabled: p.enabled,
    });
    nodes.push(ruleNode);

    if (idx > 0) {
      const prev = ordered[idx - 1];
      edges.push({
        id: `eval-${prev.id}-${p.id}`,
        source: `${PREFIX.rule}-${prev.id}`,
        target: ruleNode.id,
        kind: "eval-order" satisfies EdgeKind,
        label: "未命中则下一条",
      });
    }

    if (p.target_set_id != null && p.target_set_id > 0) {
      edges.push({
        id: `routes-${p.id}-${p.target_set_id}`,
        source: ruleNode.id,
        target: `${PREFIX.set}-${p.target_set_id}`,
        kind: "routes-to" satisfies EdgeKind,
        label: "命中",
      });
    }
  });

  // ---- 兜底 direct 节点：规则链的终端 ----
  if (ordered.length > 0) {
    const lastRule = ordered[ordered.length - 1];
    nodes.push(
      node("direct", 0, { label: "direct", summary: "未命中规则则直连出网兜底" })
    );
    edges.push({
      id: `failover-direct`,
      source: `${PREFIX.rule}-${lastRule.id}`,
      target: `${PREFIX.direct}-0`,
      kind: "default-exit" satisfies EdgeKind,
      label: "未命中直连",
    });
  }

  return { nodes, edges };
}

/**
 * 画布 B：入站路由。每个 spec 一个 inbound 入口节点，连至其默认出口。
 *
 * - 无出口→direct（本机直连出网兜底）；
 * - 有 exit_node_set_id→set 节点；
 * - 有 exit_agent_host_id→agent 节点。
 * - 入站数据内嵌 protocol/port/tls 等协议链路字段供增强视图使用。
 * - 不含 eval-order/routes-to/mesh/hosted-on 边。
 */
export function assembleCanvasB(snapshot: TopologySnapshot): TopologyGraph {
  const nodes: TopoNode[] = [];
  const edges: TopoEdge[] = [];

  const agentById = new Map(snapshot.agents.map((a) => [a.id, a]));
  const setById = new Map(snapshot.exit_sets.map((s) => [s.id, s]));

  // ---- 先遍历 spec 收集被引用的出口目标 ----
  const refSetIds = new Set<number>();
  const refAgentIds = new Set<number>();
  let hasDirect = false;

  for (const s of snapshot.specs) {
    if (!s.enabled) continue;
    if (s.exit_node_set_id != null && s.exit_node_set_id > 0) {
      refSetIds.add(s.exit_node_set_id);
    } else if (s.exit_agent_host_id != null && s.exit_agent_host_id > 0) {
      refAgentIds.add(s.exit_agent_host_id);
    } else {
      hasDirect = true;
    }
  }

  // ---- 出口目标节点 ----
  for (const sid of refSetIds) {
    const set = setById.get(sid);
    if (!set) continue;
    nodes.push(
      node("set", set.id, {
        label: set.name,
        strategy: set.strategy,
        enabled: set.enabled,
        membersLine:
          set.members.map((m) => `${m.name}×${m.weight}`).join(" / ") || "无成员",
      })
    );
  }

  for (const aid of refAgentIds) {
    const a = agentById.get(aid);
    if (!a) continue;
    nodes.push(
      node("agent", a.id, {
        label: a.name,
        host: a.host,
        online: a.online,
        summary: a.cores
          .filter((c) => c.active)
          .map((c) => c.core_type)
          .join(","),
        cores: a.cores,
      })
    );
  }

  if (hasDirect) {
    nodes.push(
      node("direct", 0, { label: "direct", summary: "本机直连出网兜底" })
    );
  }

  // ---- 入站入口节点 + default-exit 边 ----
  for (const s of snapshot.specs) {
    if (!s.enabled) continue;
    nodes.push(
      node("inbound", s.id, {
        label: s.tag,
        summary: `${s.protocol}:${s.port}${s.tls.reality ? " · reality" : s.tls.enabled ? " · tls" : ""}`,
        enabled: s.enabled,
        // 协议链路字段供增强节点视图读取
        protocol: s.protocol,
        port: s.port,
        tls: s.tls,
        agent_host_id: s.agent_host_id,
        exit_agent_host_id: s.exit_agent_host_id,
        exit_node_set_id: s.exit_node_set_id,
      })
    );

    const sourceId = `${PREFIX.inbound}-${s.id}`;
    if (s.exit_node_set_id != null && s.exit_node_set_id > 0) {
      const targetId = `${PREFIX.set}-${s.exit_node_set_id}`;
      if (setById.has(s.exit_node_set_id)) {
        edges.push({
          id: `exit-set-${s.id}-${s.exit_node_set_id}`,
          source: sourceId,
          target: targetId,
          kind: "default-exit" satisfies EdgeKind,
          label: "默认出口",
        });
      }
    } else if (s.exit_agent_host_id != null && s.exit_agent_host_id > 0) {
      const targetId = `${PREFIX.agent}-${s.exit_agent_host_id}`;
      if (agentById.has(s.exit_agent_host_id)) {
        edges.push({
          id: `exit-agent-${s.id}-${s.exit_agent_host_id}`,
          source: sourceId,
          target: targetId,
          kind: "default-exit" satisfies EdgeKind,
          label: "固定出口",
        });
      }
    } else {
      edges.push({
        id: `exit-direct-${s.id}`,
        source: sourceId,
        target: `${PREFIX.direct}-0`,
        kind: "default-exit" satisfies EdgeKind,
        label: "本机直连出网",
      });
    }
  }

  return { nodes, edges };
}

/**
 * 画布 C：精确出口链路。将链路终点精确到物理 agent。
 *
 * - 每个 enabled spec 一个 inbound 入口节点；
 * - exit_node_set_id → 中继 set 节点（仅作分组标识） + 集合内每个成员展开为独立
 *   agent 节点（含 host/在线态/权重），边：inbound→set(default-exit) + set→agent(set-member, 标签 wN)；
 * - exit_agent_host_id → 直连该 agent 节点(default-exit)；
 * - 两者皆空 → direct 节点(default-exit)。
 * - agent 节点 data 注入 weight（来自 ExitSetMember），供权重条/标签展示。
 * - 不含 eval-order/routes-to/mesh/hosted-on 边；只读视图，不承载编辑语义。
 */
/**
 * 画布C：纯服务器中继拓扑（与入口协议配置正交）。
 * - 节点 = 全量受控服务器（agent），显示名称/地址/在线态/活跃核心；
 * - 边 = relay_paths 内相邻节点对的有向中继链路：
 *   label 按序标注（末段=出口），链路禁用时整条灰虚线（dimmed）；
 * - relay_paths 缺省（旧后端）时退化为纯节点图。
 * 不含任何 inbound/rule/set 语义。
 */
export function assembleCanvasC(snapshot: TopologySnapshot): TopologyGraph {
  const nodes: TopoNode[] = [];
  const edges: TopoEdge[] = [];

  // 中继链路入口 agent 的入站绑定计数（spec.relay_path_id → path 首跳）
  const relayById = new Map((snapshot.relay_paths ?? []).map((p) => [p.id, p]));
  const entryBoundCounts = new Map<number, number>(); // agent_host_id → 绑定数
  for (const s of snapshot.specs) {
    if (s.relay_path_id == null || s.relay_path_id <= 0) continue;
    const rp = relayById.get(s.relay_path_id);
    const firstHop = rp?.nodes.find((n) => n.sequence === 0);
    if (!firstHop) continue;
    entryBoundCounts.set(firstHop.agent_host_id, (entryBoundCounts.get(firstHop.agent_host_id) ?? 0) + 1);
  }

  // ---- 服务器节点：全量 agent ----
  for (const a of snapshot.agents) {
    const bound = entryBoundCounts.get(a.id) ?? 0;
    nodes.push(
      node("agent", a.id, {
        label: a.name,
        host: a.host,
        online: a.online,
        cores: a.cores,
        summary: a.cores?.filter((c) => c.active).map((c) => c.core_type).join(",") || undefined,
        relayBoundCount: bound > 0 ? bound : undefined,
      })
    );
  }

  // ---- 中继链路有向边 ----
  for (const rp of snapshot.relay_paths ?? []) {
    const seq = [...rp.nodes].sort((x, y) => x.sequence - y.sequence);
    for (let i = 0; i + 1 < seq.length; i++) {
      edges.push({
        id: `relay-${rp.id}-${seq[i].sequence}`,
        source: `${PREFIX.agent}-${seq[i].agent_host_id}`,
        target: `${PREFIX.agent}-${seq[i + 1].agent_host_id}`,
        kind: "relay" satisfies EdgeKind,
        label: i + 2 === seq.length ? "出口" : `第${i + 1}跳`,
        dimmed: !rp.enabled,
      });
    }
  }

  return { nodes, edges };
}
