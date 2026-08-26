import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { ReactFlow, Background, Controls, type Edge, type Node } from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import type { MeshPeer } from "@/api/admin/mesh";

interface MeshTopologyViewProps {
  peers: MeshPeer[];
  /** agent_host_id → 显示名（来自探针页已加载的 agent 列表） */
  nameById?: Record<number, string>;
}

const NODE_W = 176;

/**
 * Mesh 组网拓扑视图：全互联 wgmesh0 隧道。
 * 节点 = peer（名称/WG地址/延迟），边 = C(n,2) 全互联；
 * 布局采用等分圆环（mesh 为对称全互联模型，圆环最直观且零依赖）。
 */
export function MeshTopologyView({ peers, nameById }: MeshTopologyViewProps) {
  const { t } = useTranslation();

  const { nodes, edges } = useMemo(() => {
    const n = peers.length;
    const radius = Math.max(150, n * 42);
    const cx = radius + NODE_W / 2 + 24;
    const cy = radius + 60;

    const nodes: Node[] = peers.map((p, i) => {
      const angle = (2 * Math.PI * i) / Math.max(n, 1) - Math.PI / 2;
      const label = nameById?.[p.agent_host_id] ?? `agent-${p.agent_host_id}`;
      const latency =
        typeof p.latency_ms === "number" && p.packet_loss !== 1
          ? `${Math.round(p.latency_ms)}ms`
          : undefined;
      return {
        id: `mp-${p.agent_host_id}`,
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
          border: p.online
            ? "1px solid hsl(var(--success))"
            : "1px dashed hsl(var(--muted-foreground))",
          color: "hsl(var(--foreground))",
        },
        // 延迟信息挂到节点副标题
        ...(latency ? {} : {}),
      } satisfies Node & { __latency?: string };
    });

    const latencyOf = new Map<number, string | undefined>(
      peers.map((p) => [
        p.agent_host_id,
        typeof p.latency_ms === "number" && p.packet_loss !== 1
          ? `${Math.round(p.latency_ms)}ms`
          : undefined,
      ])
    );

    const edges: Edge[] = [];
    for (let i = 0; i < peers.length; i++) {
      for (let j = i + 1; j < peers.length; j++) {
        const a = peers[i];
        const b = peers[j];
        const up = a.online && b.online;
        edges.push({
          id: `me-${a.agent_host_id}-${b.agent_host_id}`,
          source: `mp-${a.agent_host_id}`,
          target: `mp-${b.agent_host_id}`,
          label: latencyOf.get(a.agent_host_id) ?? latencyOf.get(b.agent_host_id),
          animated: up && !!(a.latency_ms || b.latency_ms),
          style: {
            stroke: up ? "hsl(var(--success))" : "hsl(var(--muted-foreground))",
            strokeWidth: 1.5,
            strokeDasharray: up ? undefined : "5 4",
            opacity: up ? 0.85 : 0.45,
          },
          labelStyle: { fontSize: 10, fill: "hsl(var(--muted-foreground))" },
          labelBgStyle: { fill: "hsl(var(--card))" },
        });
      }
    }
    return { nodes, edges };
  }, [peers, nameById]);

  if (peers.length === 0) {
    return (
      <p className="py-10 text-center text-sm text-muted-foreground">
        {t("admin.agents.meshTable.emptyTitle")}
      </p>
    );
  }

  return (
    <div className="h-[480px] overflow-hidden rounded-md border">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        fitView
        nodesConnectable={false}
        elementsSelectable={false}
        proOptions={{ hideAttribution: true }}
      >
        <Background gap={18} />
        <Controls showInteractive={false} />
      </ReactFlow>
    </div>
  );
}
