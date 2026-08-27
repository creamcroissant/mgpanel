import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import {
  ReactFlow,
  Background,
  Controls,
  ReactFlowProvider,
  applyNodeChanges,
  useReactFlow,
  type Edge,
  type Node,
  type NodeChange,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import type { MeshPeer } from "@/api/admin/mesh";
import { fetchTopology } from "@/lib/topology/api";

interface MeshTopologyViewProps {
  peers: MeshPeer[];
  /** agent_host_id → 显示名（来自探针页已加载的 agent 列表） */
  nameById?: Record<number, string>;
}

const NODE_W = 176;

/**
 * Mesh 组网拓扑视图（全互联 wgmesh0）：
 * - 直线连线、节点可拖动
 * - 点击节点：高亮其所有连线，并标注「该节点实测」到各邻居的有向延迟
 *   （数据源 /admin/topology 的 mesh.edges: from→to 单向探测值）
 */
export function MeshTopologyView(props: MeshTopologyViewProps) {
  return (
    <ReactFlowProvider>
      <MeshTopologyInner {...props} />
    </ReactFlowProvider>
  );
}

function MeshTopologyInner({ peers, nameById }: MeshTopologyViewProps) {
  const { t } = useTranslation();
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const wrapRef = useRef<HTMLDivElement>(null);
  const { screenToFlowPosition } = useReactFlow();

  // 画布在流坐标系中的安全矩形（拖拽开始时按当前缩放换算）
  const flowRectRef = useRef<{ minX: number; minY: number; maxX: number; maxY: number } | null>(null);
  const handleNodeDragStart = () => {
    const rect = wrapRef.current?.getBoundingClientRect();
    if (!rect) return;
    const pad = 28;
    const a = screenToFlowPosition({ x: rect.left + pad, y: rect.top + pad });
    const b = screenToFlowPosition({ x: rect.right - pad, y: rect.bottom - pad });
    flowRectRef.current = { minX: a.x, minY: a.y, maxX: b.x, maxY: b.y };
  };

  // 逐对有向延迟（from_agent 实测 → to_agent）
  const topoQuery = useQuery({
    queryKey: ["admin", "mesh-topo-edges"],
    queryFn: () => fetchTopology("sing-box"),
    staleTime: 30_000,
  });
  const dirLatency = useMemo(() => {
    const m = new Map<string, number | null>();
    for (const e of topoQuery.data?.mesh?.edges ?? []) {
      m.set(`${e.from_agent_id}-${e.to_agent_id}`, e.latency_ms);
    }
    return m;
  }, [topoQuery.data]);

  const { nodes: layoutNodes, edges } = useMemo(() => {
    const n = peers.length;
    const radius = Math.max(150, n * 42);
    const cx = radius + NODE_W / 2 + 24;
    const cy = radius + 60;

    const nodes: Node[] = peers.map((p, i) => {
      const angle = (2 * Math.PI * i) / Math.max(n, 1) - Math.PI / 2;
      const label = nameById?.[p.agent_host_id] ?? `agent-${p.agent_host_id}`;
      const isSel = selectedId === `mp-${p.agent_host_id}`;
      return {
        id: `mp-${p.agent_host_id}`,
        draggable: true,
        position: {
          x: cx + radius * Math.cos(angle) - NODE_W / 2,
          y: cy + radius * Math.sin(angle) - 32,
        },
        data: { label },
        style: {
          width: NODE_W,
          padding: "8px 10px",
          borderRadius: 10,
          fontSize: 12,
          background: p.online ? "hsl(var(--card))" : "hsl(var(--muted))",
          border: isSel
            ? "2px solid hsl(var(--primary))"
            : p.online
              ? "1px solid hsl(var(--success))"
              : "1px dashed hsl(var(--muted-foreground))",
          color: "hsl(var(--foreground))",
          boxShadow: isSel ? "0 0 0 3px hsl(var(--primary) / 0.25)" : undefined,
        },
      };
    });

    const fmt = (v: number | null | undefined) =>
      v == null || v <= 0 ? "—" : `${Math.round(v)}ms`;

    const other = (a: number, b: string) => Number(b.replace("mp-", "")) === a
      ? null
      : Number(b.replace("mp-", ""));

    const edges: Edge[] = [];
    for (let i = 0; i < peers.length; i++) {
      for (let j = i + 1; j < peers.length; j++) {
        const a = peers[i];
        const b = peers[j];
        const up = a.online && b.online;

        // 点击态：只标注被选节点实测方向的延迟，其余淡化
        let label: string | undefined;
        let dimmed = false;
        let highlighted = false;
        if (selectedId) {
          const selHost = Number(selectedId.replace("mp-", ""));
          const isIncident =
            `mp-${a.agent_host_id}` === selectedId ||
            `mp-${b.agent_host_id}` === selectedId;
          if (isIncident) {
            highlighted = true;
            const toOther =
              `mp-${a.agent_host_id}` === selectedId ? b.agent_host_id : a.agent_host_id;
            // 优先本节点实测方向；缺失时回退对向实测值(↔ 标记)，避免整条链路无延迟可读
            const dir = dirLatency.get(`${selHost}-${toOther}`);
            if (dir != null && dir > 0) {
              label = fmt(dir);
            } else {
              const rev = dirLatency.get(`${toOther}-${selHost}`);
              label = rev != null && rev > 0 ? `${Math.round(rev)}ms↔` : "—";
            }
          } else {
            dimmed = true;
          }
        }

        edges.push({
          id: `me-${a.agent_host_id}-${b.agent_host_id}`,
          source: `mp-${a.agent_host_id}`,
          target: `mp-${b.agent_host_id}`,
          type: "straight",
          label,
          animated: !selectedId && up,
          style: {
            stroke: !up
              ? "hsl(var(--muted-foreground))"
              : highlighted
                ? "hsl(var(--primary))"
                : "hsl(var(--success))",
            strokeWidth: highlighted ? 2 : 1.5,
            strokeDasharray: up ? undefined : "5 4",
            opacity: dimmed ? 0.25 : up ? 0.85 : 0.45,
          },
          labelStyle: {
            fontSize: 11,
            fontWeight: 600,
            fill: "hsl(var(--foreground))",
          },
          labelBgStyle: { fill: "hsl(var(--card))" },
          labelBgPadding: [4, 2] as [number, number],
          labelBgBorderRadius: 4,
        });
        void other;
      }
    }
    return { nodes, edges };
  }, [peers, nameById, selectedId, dirLatency]);

  // 受控模式必须自行应用拖拽产生的位置变更，否则节点拖不动
  const [nodes, setNodes] = useState<Node[]>(layoutNodes);
  useEffect(() => setNodes(layoutNodes), [layoutNodes]);
  const onNodesChange = (changes: NodeChange[]) =>
    setNodes((nds) => {
      const next = applyNodeChanges(changes, nds);
      const r = flowRectRef.current;
      if (!r) return next;
      return next.map((n) => {
        if (!r) return n;
        const x = Math.min(Math.max(n.position.x, r.minX), r.maxX);
        const y = Math.min(Math.max(n.position.y, r.minY), r.maxY);
        return x === n.position.x && y === n.position.y
          ? n
          : { ...n, position: { x, y } };
      });
    });

  if (peers.length === 0) {
    return (
      <p className="py-10 text-center text-sm text-muted-foreground">
        {t("admin.agents.meshTable.emptyTitle")}
      </p>
    );
  }

  return (
    <div ref={wrapRef} className="relative h-[480px] overflow-hidden rounded-md border">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onNodeDragStart={handleNodeDragStart}
        fitView
        nodesDraggable
        nodeDragThreshold={0}
        nodesConnectable={false}
        elementsSelectable
        proOptions={{ hideAttribution: true }}
        onNodeClick={(_, node) =>
          setSelectedId((cur) => (cur === node.id ? null : node.id))
        }
        onPaneClick={() => setSelectedId(null)}
      >
        <Background gap={18} />
        <Controls showInteractive={false} />
      </ReactFlow>
      {!selectedId && (
        <p className="pointer-events-none absolute bottom-2 left-1/2 -translate-x-1/2 rounded bg-card/90 px-2 py-1 text-xs text-muted-foreground shadow-sm">
          {t("admin.agents.meshTable.topologyHint")}
        </p>
      )}
    </div>
  );
}
