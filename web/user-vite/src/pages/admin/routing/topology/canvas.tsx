import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Background,
  BackgroundVariant,
  Controls,
  MiniMap,
  ReactFlow,
  applyNodeChanges,
  type Connection,
  type Edge,
  type Node,
  type NodeChange,
  type NodeTypes,
  type XYPosition,
} from "@xyflow/react";
import { toast } from "sonner";
import "@xyflow/react/dist/style.css";
import "./minimap.css";

import type { TopoEdge, TopoNode, TopologyIssue } from "@/lib/topology/types";
import { layoutTopology, buildPhysicalLane } from "./layout";
import { MeshEdgeView } from "./MeshEdgeView";
import { AgentNodeView } from "./nodes/AgentNodeView";
import { DirectNodeView } from "./nodes/DirectNodeView";
import { InboundNodeView } from "./nodes/InboundNodeView";
import { LaneNodeView } from "./nodes/LaneNodeView";
import { ExitSetNodeView } from "./nodes/ExitSetNodeView";
import { RuleNodeView } from "./nodes/RuleNodeView";

/** kind → 自定义节点组件注册表 */
const nodeTypes: NodeTypes = {
  agent: AgentNodeView,
  inbound: InboundNodeView,
  rule: RuleNodeView,
  set: ExitSetNodeView,
  direct: DirectNodeView,
  fallback: DirectNodeView,
  lane: LaneNodeView,
};

/** 自定义边组件注册表（mesh 物理链路带延迟徽章与陈旧警示） */
const edgeTypes = { mesh: MeshEdgeView };

/** 边语义 → 样式（项目 CSS 变量为裸 HSL 三元组，SVG 内联需 hsl() 包裹） */
function edgeStyle(kind: TopoEdge["kind"]): Partial<Edge> {
  switch (kind) {
    case "eval-order":
      return {
        animated: false,
        style: { stroke: "hsl(var(--muted-foreground))", strokeDasharray: "4 3" },
        labelStyle: { fill: "hsl(var(--muted-foreground))", fontSize: 11 },
      };
    case "routes-to":
      return { animated: true, style: { stroke: "hsl(var(--primary))" } };
    case "default-exit":
      return { style: { stroke: "hsl(var(--success))" } };
    case "mesh":
      // 自定义组件渲染（延迟徽章/陈旧警示），样式在 MeshEdgeView 内部
      return { type: "mesh" as const };
    case "hosted-on":
      return {
        style: {
          stroke: "hsl(var(--muted-foreground))",
          strokeWidth: 1,
          strokeDasharray: "2 3",
          opacity: 0.7,
        },
      };
    case "relay-bind":
      // 入站绑定中继链路：虚线+主色，区别于直接出口实线
      return {
        animated: true,
        style: { stroke: "hsl(var(--primary))", strokeDasharray: "6 4" },
        labelStyle: { fill: "hsl(var(--primary))", fontSize: 11 },
      };
    default:
      return {};
  }
}

export interface TopologyCanvasProps {
  nodes: TopoNode[];
  edges: TopoEdge[];
  /** 当前选中节点 id（Drawer 打开来源），画布内高亮 */
  selectedId?: string | null;
  /** validate 结果按节点 id 分组，error 级红框高亮 */
  issuesByNode?: Record<string, TopologyIssue[]>;
  /** 仅允许 rule→set 连线；非法方向由内部 toast 拒绝 */
  onConnectRuleToSet?: (sourceNodeId: string, targetNodeId: string) => void;
  /** 服务器拓扑模式：agent→agent 连线建中继链路 */
  onConnectAgents?: (sourceNodeId: string, targetNodeId: string) => void;
  /** 节点点击（父层据此开 Drawer / 定位 issue） */
  onNodeClick?: (nodeId: string, kind: string) => void;
  /** 中继边点击（父层据此打开该链路 Drawer） */
  onRelayEdgeClick?: (pathId: number) => void;
  /** 筛选器命中的节点 id 集合；filterActive 时未命中节点降透明 */
  highlightIds?: Set<string>;
  /** 筛选器是否激活 */
  filterActive?: boolean;
  /** 布局持久化命名空间（随 core_type 切换重置）*/
  resetKey?: string;
}

/** 手动拖拽位置持久化：按 resetKey 存 localStorage，自动布局可一键覆盖 */
const positionsKey = (k: string) => `topology-positions-${k}`;
function loadPositions(key: string): Record<string, XYPosition> {
  try {
    const raw = localStorage.getItem(positionsKey(key));
    return raw ? (JSON.parse(raw) as Record<string, XYPosition>) : {};
  } catch {
    return {};
  }
}

/**
 * 拓扑画布（交互装配波次）：
 * - 节点可拖动（位置存 localStorage，切 core_type 分命名空间）
 * - 节点可选中（点击开 Drawer 由父层处理 onNodeClick）
 * - rule 节点可拖出连线到 set 节点 = 改绑/建策略；其余方向拒绝
 * - 校验 issue 注入节点 className：红边 + 角标由节点视图按 data.issues 渲染
 * - 筛选器激活时命中节点 success 描边、未命中降透明
 * 基础布局以 dagre 为准，手动拖拽位置优先于布局结果。
 */
export function TopologyCanvas({
  nodes,
  edges,
  selectedId = null,
  issuesByNode = {},
  onConnectRuleToSet,
  onConnectAgents,
  onRelayEdgeClick,
  onNodeClick,
  highlightIds,
  filterActive = false,
  resetKey = "default",
}: TopologyCanvasProps) {
  const kindById = useMemo(() => {
    const m = new Map<string, string>();
    for (const n of nodes) m.set(n.id, n.kind);
    return m;
  }, [nodes]);

  // 手动拖拽位置：布局重建时用 dragRef 作为覆盖；拖拽过程由 applyNodeChanges
  // 实时写回 flowNodes state（节点跟随鼠标），松手时持久化到 localStorage。
  const dragRef = useRef<Record<string, XYPosition>>(loadPositions(resetKey));
  useEffect(() => {
    dragRef.current = loadPositions(resetKey);
  }, [resetKey]);

  // 拖拽实时跟随：React Flow 每帧下发 position change，applyNodeChanges 立即
  // 应用到 state —— 松手才瞬移的问题由此解决。
  const onNodesChange = useCallback((changes: NodeChange[]) => {
    setFlowNodes((nds) => applyNodeChanges(changes, nds));
  }, []);

  const onNodeDragStop = useCallback(
    (_event: React.MouseEvent | React.TouchEvent | MouseEvent | TouchEvent, node: Node) => {
      dragRef.current[node.id] = node.position;
      try {
        localStorage.setItem(positionsKey(resetKey), JSON.stringify(dragRef.current));
      } catch {
        /* 隐私模式等写入失败静默降级 */
      }
    },
    [resetKey]
  );

  const handleConnect = useCallback(
    (conn: Connection | Edge) => {
      const srcKind = kindById.get(conn.source);
      const dstKind = kindById.get(conn.target);
      if (srcKind === "agent" && dstKind === "agent") {
        if (!onConnectAgents) return;
        if (conn.source === conn.target) {
          toast.error("不能连接到自身");
          return;
        }
        onConnectAgents(conn.source, conn.target);
        return;
      }
      if (!onConnectRuleToSet) return;
      if (srcKind !== "rule" || dstKind !== "set") {
        toast.error("仅支持从规则节点连线到出口集节点");
        return;
      }
      onConnectRuleToSet(conn.source, conn.target);
    },
    [kindById, onConnectRuleToSet, onConnectAgents]
  );

  // flowNodes 为受控 state：拖拽时 onNodesChange 经 applyNodeChanges 实时写回，
  // 节点跟随鼠标；仅当拓扑数据/筛选/选中变化时才整体重建布局（重建时以
  // dragRef 保存的手动位置为准，不打断拖拽）。
  const [flowNodes, setFlowNodes] = useState<Node[]>([]);

  useEffect(() => {
    const laidOut = layoutTopology(
      nodes.map((n) => {
        const issues = issuesByNode[n.id];
        const hasError = issues?.some((i) => i.severity === "error") ?? false;
        return {
          id: n.id,
          type: n.kind,
          position: { x: 0, y: 0 },
          // 校验高亮走外层 className，不动节点内部实现
          className: hasError ? "ring-2 ring-destructive rounded-md" : undefined,
          data: {
            ...n.data,
            kind: n.kind,
            ...(issues && issues.length > 0
              ? {
                  issues: issues.map((i) => ({
                    severity: i.severity,
                    message: i.message,
                  })),
                }
              : {}),
          },
        };
      }),
      edges.map((e) => ({ id: e.id, source: e.source, target: e.target }))
    )
      // 手动拖拽位置优先于 dagre 布局结果
      .map((n) => (dragRef.current[n.id] ? { ...n, position: dragRef.current[n.id] } : n))
      // 筛选器：命中 success 描边，未命中降透明（保留可点）
      .map((n) => {
        if (!filterActive) return n;
        const matched = highlightIds?.has(n.id) ?? false;
        return {
          ...n,
          className: [
            n.className ?? "",
            matched ? "ring-2 ring-success rounded-md" : "opacity-30 saturate-50",
          ]
            .filter(Boolean)
            .join(" "),
        };
      })
      .map((n) =>
        n.id === selectedId
          ? { ...n, className: `${n.className ?? ""} outline outline-2 outline-primary` }
          : n
      );
    // 物理层泳道底板垫底（zIndex -1，见 buildPhysicalLane）
    setFlowNodes([...buildPhysicalLane(laidOut), ...laidOut]);
    // resetKey 参与重建：切换 core_type 时以新命名空间的拖拽位置重排
  }, [nodes, edges, issuesByNode, selectedId, filterActive, highlightIds, resetKey]);

  const flowEdges = useMemo<Edge[]>(
    () =>
      edges.map((e) => {
        const base = edgeStyle(e.kind);
        if (base.type === "mesh") {
          return {
            id: e.id,
            source: e.source,
            target: e.target,
            type: "mesh" as const,
            zIndex: -1,
            data: { latencyMs: e.latencyMs ?? null, handshakeAgeSec: e.handshakeAgeSec ?? null },
          };
        }
        return {
          id: e.id,
          source: e.source,
          target: e.target,
          label: e.label,
          ...base,
        };
      }),
    [edges]
  );

  return (
    <div id="topology-canvas" className="h-[560px] w-full overflow-hidden rounded-md border bg-background">
      <ReactFlow
        nodes={flowNodes}
        edges={flowEdges}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        fitView
        minZoom={0.2}
        maxZoom={1.6}
        proOptions={{ hideAttribution: true }}
        nodesDraggable
        onNodesChange={onNodesChange}
        nodesConnectable={onConnectRuleToSet != null || onConnectAgents != null}
        elementsSelectable
        onNodeClick={(_, node) => onNodeClick?.(node.id, String(node.type))}
        onEdgeClick={(_, edge) => {
          const m = /^relay-(\d+)-/.exec(edge.id);
          if (m) onRelayEdgeClick?.(Number(m[1]));
        }}
        onNodeDragStop={onNodeDragStop}
        isValidConnection={(conn) => {
          const s = kindById.get(String(conn.source));
          const t = kindById.get(String(conn.target));
          // 服务器链路画布：agent→agent 建中继；规则画布：rule→set
          if (s === "agent" && t === "agent") return conn.source !== conn.target;
          return s === "rule" && t === "set";
        }}
        onConnect={handleConnect}
      >
        <Background variant={BackgroundVariant.Dots} gap={20} size={1} />
        <Controls showInteractive={false} />
        <MiniMap
          pannable
          zoomable
          position="bottom-right"
          className="topology-minimap"
          nodeColor="hsl(var(--muted-foreground))"
          maskColor="hsl(var(--background) / 0.55)"
        />
      </ReactFlow>
    </div>
  );
}
