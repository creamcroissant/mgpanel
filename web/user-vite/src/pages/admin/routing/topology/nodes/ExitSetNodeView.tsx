import { memo } from "react";
import { Handle, Position, type NodeProps } from "@xyflow/react";
import type { ExitSetMember } from "@/lib/topology/types";
import type { FlowNodeData } from "./AgentNodeView";
import {
  DisabledBadge,
  IssueBadge,
  NODE_SHELL,
  WeightBar,
  disabledClass,
  nodeIssues,
} from "./shared";

/** 出口集节点本地扩展字段：结构化成员（assembler 未迁移时回退 membersLine 文本） */
interface SetNodeSpecifics {
  /** 成员列表（不含 direct 兜底行） */
  members?: ExitSetMember[];
  /** 是否渲染 direct 兜底行（渲染器约定：出口集 selector 末位固定 direct） */
  directFallback?: boolean;
}
type SetFlowData = FlowNodeData & SetNodeSpecifics;

const STRATEGY_META: Record<string, { short: string; full: string }> = {
  round_robin: { short: "RR", full: "轮询 round_robin" },
  weighted_random: { short: "WR", full: "加权随机 weighted_random" },
  random: { short: "RND", full: "随机 random" },
  least_ping: { short: "LB", full: "最低延迟 least_ping" },
};

/**
 * 出口集节点：绿系。
 * - strategy 徽章（RR/WR/RND/LB 缩写，title 全称）
 * - 成员列表每行 agent 名 + 权重条（weight/maxWeight 宽度百分比）
 * - direct 兜底行灰显置底
 * - enabled=false 时整节点半透明 + “已停用”角标
 */
function ExitSetNodeViewInner({ data }: NodeProps) {
  const d = data as SetFlowData;
  const off = d.enabled === false;
  const members = Array.isArray(d.members) ? d.members : [];
  const maxWeight = members.reduce((m, x) => Math.max(m, x.weight), 0);
  const meta =
    (typeof d.strategy === "string" && STRATEGY_META[d.strategy]) || null;

  return (
    <div
      className={`${NODE_SHELL} relative w-[208px] ${
        off ? "border-muted-foreground/40" : "border-success/60"
      } ${disabledClass(d.enabled)}`}
    >
      {off && <DisabledBadge />}
      {(() => {
        const errs = nodeIssues(d as unknown as Record<string, unknown>).filter(
          (i) => i.severity === "error"
        );
        return errs.length > 0 ? <IssueBadge count={errs.length} /> : null;
      })()}
      {/* 连线目标锚点：接收规则节点拖入的 route_to_set 改绑（f4 交互） */}
      <Handle type="target" position={Position.Left} isConnectable aria-label="接收规则连线" />
      <div className="flex items-center gap-1.5">
        <span className="truncate text-sm font-medium" title={String(d.label)}>
          {d.label}
        </span>
        {meta && (
          <span
            className="ml-auto shrink-0 rounded-sm bg-success/10 px-1.5 py-px text-[11px] font-semibold text-success"
            title={meta.full}
          >
            {meta.short}
          </span>
        )}
      </div>

      {members.length > 0 ? (
        <ul className="mt-1.5 space-y-1">
          {members.map((m) => (
            <li
              key={m.agent_host_id}
              className="flex items-center gap-1.5 text-xs text-muted-foreground"
              title={`${m.name} (${m.host}) · 权重 ${m.weight}`}
            >
              <span className="w-16 truncate">{m.name}</span>
              <WeightBar weight={m.weight} maxWeight={maxWeight} />
              <span className="tabular-nums">w{m.weight}</span>
            </li>
          ))}
        </ul>
      ) : (
        d.membersLine && (
          <p className="mt-1 truncate text-xs text-muted-foreground">
            {d.membersLine}
          </p>
        )
      )}

      {d.directFallback && (
        <p className="mt-1 flex items-center gap-1 border-t pt-1 text-xs text-muted-foreground/70">
          <span className="truncate">⊘ direct 兜底</span>
        </p>
      )}
    </div>
  );
}


/** 拖拽性能：memo 化避免 React Flow 每帧重绘全部节点 */
export const ExitSetNodeView = memo(ExitSetNodeViewInner);
