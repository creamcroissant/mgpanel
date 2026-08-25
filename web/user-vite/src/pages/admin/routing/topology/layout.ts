import type { Edge, Node } from "@xyflow/react";
import { RULE_NODE_HEIGHT } from "./nodes/RuleNodeView";

/** 布局常量：与节点实际渲染尺寸严格对应，防止 dagre 时代重叠问题复发 */
export const NODE_WIDTH = 208;
export const LAYOUT_GAP = {
  /** 主干行距（含呼吸空间） */
  rank: 36,
  /** 分支列与主干的水平距离 */
  branchX: 96,
  /** 分支列内纵向最小间距 */
  branchY: 28,
} as const;

const MARGIN = 24;

/**
 * 按种类估算渲染高度（px），必须与各 NodeView 实际 DOM 高度一致：
 * - rule: 固定 64（RuleNodeView）
 * - inbound/agent: 单行头+单行摘要 ≈ 64
 * - direct/fallback: 单行 ≈ 48
 * - set: 头行 + 成员行×N + 兜底行；结构化 members 缺失时回退单行文本
 */
function estimateHeight(node: Node): number {
  const d = node.data as Record<string, unknown>;
  switch (node.type ?? (d.kind as string)) {
    case "rule":
      return RULE_NODE_HEIGHT;
    case "set": {
      const members = Array.isArray(d.members)
        ? (d.members as unknown[]).length
        : 0;
      const fb = d.directFallback === true ? 24 : 0;
      // 头行 30 + 成员行 ~21/行（space-y-1 计入）；无成员回退一行文本 46
      return members > 0 ? 34 + members * 21 + fb : 46 + fb;
    }
    case "direct":
    case "fallback":
      return typeof d.summary === "string" && d.summary ? 64 : 48;
    default:
      return 64;
  }
}

interface Placed {
  id: string;
  x: number;
  y: number;
  h: number;
}

/**
 * 确定性分层布局（替代 dagre）：
 * - 左主干列：入站节点置顶，其后为 eval-order 规则链自上而下垂直排布；
 * - 右分支列：routes-to / default-exit 的目标（出口集/固定agent/直连兜底）
 *   与各自源节点垂直居中对齐，向右展开；纵向冲突时向下顺延避让。
 */
export function layoutTopology(nodes: Node[], edges: Edge[]): Node[] {
  const byId = new Map(nodes.map((n) => [n.id, n]));
  const heights = new Map(nodes.map((n) => [n.id, estimateHeight(n)]));

  // ---- 主干序列：先入站，后规则链（eval-order 顺序）----
  const outEdgesFrom = new Map<string, Edge[]>();
  const kindOf = (id: string) => {
    const n = byId.get(id);
    return n ? (n.type) : "";
  };
  for (const e of edges) {
    const list = outEdgesFrom.get(e.source) ?? [];
    list.push(e);
    outEdgesFrom.set(e.source, list);
  }

  // 规则链顺序遍历：沿 eval-order 从链头走到底
  const evalOrderEdges = edges.filter((e) => (e.type ?? (e.data as Record<string, unknown> | undefined)?.kind) === "eval-order");
  const hasIncomingEval = new Set(evalOrderEdges.map((e) => e.target));
  const ruleChain: string[] = [];
  const visited = new Set<string>();
  for (const start of nodes.filter(
    (n) => (n.type ?? "") === "rule" && !hasIncomingEval.has(n.id)
  )) {
    let cur: string | undefined = start.id;
    while (cur && !visited.has(cur)) {
      visited.add(cur);
      ruleChain.push(cur);
      const next = evalOrderEdges.find((e) => e.source === cur);
      cur = next?.target;
    }
  }

  const spineOrder: string[] = [
    ...nodes.filter((n) => (n.type ?? "") === "inbound").map((n) => n.id),
    ...ruleChain,
    ...nodes.filter((n) => (n.type ?? "") === "rule" && !visited.has(n.id)).map((n) => n.id),
  ];

  // ---- 主干定位 ----
  const positions = new Map<string, Placed>();
  let cursorY = MARGIN;
  for (const id of spineOrder) {
    const h = heights.get(id) ?? 64;
    positions.set(id, { id, x: MARGIN, y: cursorY, h });
    cursorY += h + LAYOUT_GAP.rank;
  }
  const spineBottom = cursorY;

  // ---- 分支定位：目标节点右移一列，与首个源节点垂直居中 ----
  const branchIds = new Set<string>();
  const sourceOfBranch = new Map<string, string>();
  for (const e of edges) {
    const ek = (e.type ?? (e.data as Record<string, unknown> | undefined)?.kind) as string;
    if (ek !== "routes-to" && ek !== "default-exit" && ek !== "mesh") {
      continue;
    }
    // agent 节点一律进物理层泳道（见下方 physical lane），不参与逻辑分支排布
    if (kindOf(e.target) === "agent") continue;
    if (!sourceOfBranch.has(e.target)) {
      sourceOfBranch.set(e.target, e.source);
    }
    if (!positions.has(e.target)) {
      branchIds.add(e.target);
    }
  }

  const desired: Placed[] = [];
  for (const id of branchIds) {
    const srcId = sourceOfBranch.get(id)!;
    const src = positions.get(srcId);
    const h = heights.get(id) ?? 64;
    const srcCenter = src ? src.y + src.h / 2 : MARGIN;
    desired.push({ id, x: MARGIN + NODE_WIDTH + LAYOUT_GAP.branchX, y: srcCenter - h / 2, h });
  }
  // 纵向避让：按期望 y 排序后向下顺延
  desired.sort((a, b) => a.y - b.y);
  let lastBottom = -Infinity;
  for (const p of desired) {
    const y = Math.max(p.y, lastBottom + LAYOUT_GAP.branchY);
    positions.set(p.id, { ...p, y });
    lastBottom = y + p.h;
  }

  // ---- 孤岛（无任何边的节点）：底部区域按序堆放 ----
  const connected = new Set<string>();
  for (const e of edges) {
    connected.add(e.source);
    connected.add(e.target);
  }
  let islandY = Math.max(spineBottom, lastBottom + MARGIN);
  for (const n of nodes) {
    if (
      connected.has(n.id) ||
      positions.has(n.id) ||
      (n.type) === "agent"
    )
      continue;
    const h = heights.get(n.id) ?? 64;
    positions.set(n.id, { id: n.id, x: MARGIN, y: islandY, h });
    islandY += h + LAYOUT_GAP.rank;
  }

  // ---- 物理层泳道：agent 节点水平排布在画布底部独立色带内 ----
  const agentNodes = nodes.filter((n) => n.type === "agent");
  const laneTop = islandY + 48;
  const AGENT_GAP = 32;
  let maxAgentH = 64;
  agentNodes.forEach((n, i) => {
    const h = heights.get(n.id) ?? 64;
    maxAgentH = Math.max(maxAgentH, h);
    positions.set(n.id, {
      id: n.id,
      x: MARGIN + i * (NODE_WIDTH + AGENT_GAP),
      y: laneTop,
      h,
    });
  });

  return nodes.map((n) => {
    const pos = positions.get(n.id);
    if (!pos) return n;
    return { ...n, position: { x: pos.x, y: pos.y } };
  });
}

/**
 * 物理层泳道底板节点：覆盖 agent 节点包围盒（含呼吸留白），
 * 纯装饰背景，置于最底层。无 agent 时返回空数组。
 */
export function buildPhysicalLane(nodes: Node[]): Node[] {
  const agents = nodes.filter((n) => n.type === "agent");
  if (agents.length === 0) return [];
  const PAD = 14;
  const minX = Math.min(...agents.map((n) => n.position.x)) - PAD;
  const minY = Math.min(...agents.map((n) => n.position.y)) - PAD;
  const maxX =
    Math.max(...agents.map((n) => n.position.x)) + NODE_WIDTH + PAD;
  const maxY =
    Math.max(
      ...agents.map(
        (n) => n.position.y + estimateHeight(n)
      )
    ) + PAD;
  return [
    {
      id: "__lane__physical",
      type: "lane",
      position: { x: minX, y: minY },
      data: {},
      style: { width: maxX - minX, height: maxY - minY },
      zIndex: -1,
      selectable: false,
      draggable: false,
      connectable: false,
    },
  ];
}
