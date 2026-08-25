import { memo } from "react";
import {
  BaseEdge,
  EdgeLabelRenderer,
  getBezierPath,
  type EdgeProps,
} from "@xyflow/react";

/**
 * Mesh 组网边（agent↔agent 物理层）：
 * - 虚线贝塞尔曲线，warning 语义色
 * - 标签徽章：latency_ms 有值显示 "45ms"，否则显示 "mesh"
 * - 握手秒龄 >600s 视为陈旧，整条边转 destructive 警示色
 */
const STALE_HANDSHAKE_SEC = 600;

export interface MeshEdgeData extends Record<string, unknown> {
  latencyMs?: number | null;
  handshakeAgeSec?: number | null;
}

export const MeshEdgeView = memo(function MeshEdgeView({
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  data,
}: EdgeProps) {
  const d = (data ?? {}) as MeshEdgeData;
  const [path, labelX, labelY] = getBezierPath({
    sourceX,
    sourceY,
    targetX,
    targetY,
    sourcePosition,
    targetPosition,
  });

  const stale =
    typeof d.handshakeAgeSec === "number" && d.handshakeAgeSec > STALE_HANDSHAKE_SEC;
  const stroke = stale ? "hsl(var(--destructive))" : "hsl(var(--warning))";
  const label =
    typeof d.latencyMs === "number" && d.latencyMs > 0
      ? `${d.latencyMs}ms`
      : "mesh";

  return (
    <>
      <BaseEdge path={path} style={{ stroke, strokeWidth: 1.5, strokeDasharray: "6 4" }} />
      <EdgeLabelRenderer>
        <div
          className="nodrag nopan absolute flex items-center gap-0.5 rounded-full border bg-background px-1.5 py-px text-[10px] font-medium leading-tight"
          style={{
            transform: `translate(-50%,-50%) translate(${labelX}px,${labelY}px)`,
            borderColor: stroke,
            color: stroke,
          }}
          title={
            stale
              ? `握手陈旧（${Math.round(d.handshakeAgeSec ?? 0)}s 前），疑似链路异常`
              : "mesh 组网链路"
          }
        >
          {label}
        </div>
      </EdgeLabelRenderer>
    </>
  );
});
