import { memo } from "react";
import { Handle, Position, type NodeProps } from "@xyflow/react";
import { Globe, Link2, MapPin } from "lucide-react";
import type { FlowNodeData } from "./AgentNodeView";
import {
  DisabledBadge,
  IssueBadge,
  NODE_SHELL,
  disabledClass,
  nodeIssues,
  type IconType,
} from "./shared";

/** 规则节点本地扩展字段：order=按数据顺序的序号（区别于原始 priority 数值） */
interface RuleNodeSpecifics {
  order?: number;
  /** 作用域：非空 = 仅对该入站 spec 生效（渲染时排在全局之前） */
  specId?: number | null;
}
type RuleFlowData = FlowNodeData & RuleNodeSpecifics;

const RULE_NODE_WIDTH = "w-[208px]";

const MATCH_META: Record<string, { Icon: IconType; label: string }> = {
  geosite: { Icon: Globe, label: "geosite" },
  domain: { Icon: Link2, label: "domain" },
  ip_cidr: { Icon: MapPin, label: "ip_cidr" },
};

/**
 * 分流规则节点：蓝系主干。
 * - 匹配类型图标（geosite🌐/domain🔗/ip_cidr📍）
 * - 优先级序号徽标（#1/#2 按数据顺序；order 缺省回退 priority 原值）
 * - match_value 截断展示 + title 全文
 * - enabled=false 时整节点半透明 + “已停用”角标
 *连线建策略交互在 f4 波次接入（本组件只负责视觉）。
 */
function RuleNodeViewInner({ data }: NodeProps) {
  const d = data as RuleFlowData;
  const meta = MATCH_META[String(d.matchType)] ?? MATCH_META.geosite;
  const off = d.enabled === false;
  const seq = typeof d.order === "number" ? d.order : d.priority;
  const valueText = `${meta.label}: ${String(d.summary ?? "")}`;

  return (
    <div
      className={`${NODE_SHELL} relative ${
        off ? "border-muted-foreground/40" : "border-primary/60"
      } ${disabledClass(d.enabled)} ${RULE_NODE_WIDTH}`}
    >
      {off && <DisabledBadge />}
      {(() => {
        const errs = nodeIssues(d as unknown as Record<string, unknown>).filter(
          (i) => i.severity === "error"
        );
        return errs.length > 0 ? <IssueBadge count={errs.length} /> : null;
      })()}
      {/* 连线源锚点：拖到出口集节点即改绑/建策略（f4 交互） */}
      <Handle type="source" position={Position.Right} isConnectable aria-label="连线到出口集" />
      <div className="flex items-center gap-1.5">
        <meta.Icon className="h-3.5 w-3.5 shrink-0 text-primary" aria-hidden />
        <span className="truncate text-sm font-medium" title={String(d.label)}>
          {d.label}
        </span>
        {typeof d.specId === "number" && (
          <span
            className="shrink-0 rounded-sm bg-warning/15 px-1 py-px text-[11px] text-warning-foreground dark:text-warning"
            title={`仅对入站 #${d.specId} 生效（优先于全局规则）`}
          >
            🔒
          </span>
        )}
        {typeof seq === "number" && (
          <span
            className="ml-auto shrink-0 rounded-sm bg-primary/10 px-1.5 py-px text-[11px] font-semibold tabular-nums text-primary"
            title={`优先级 ${d.priority ?? seq}`}
          >
            #{seq}
          </span>
        )}
      </div>
      <p
        className="mt-1 truncate text-xs text-muted-foreground"
        title={valueText}
      >
        {valueText}
      </p>
    </div>
  );
}

/** 布局用：规则节点固定高度 */
export const RULE_NODE_HEIGHT = 64;


/** 拖拽性能：memo 化避免 React Flow 每帧重绘全部节点 */
export const RuleNodeView = memo(RuleNodeViewInner);
