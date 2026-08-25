import { memo } from "react";
import type { NodeProps } from "@xyflow/react";
import { ArrowDownToLine, CornerDownRight } from "lucide-react";
import type { FlowNodeData } from "./AgentNodeView";
import { NODE_SHELL } from "./shared";

/**
 * 直连/兜底节点：灰系虚线简洁卡。
 * direct（本机直连出网）与 fallback（默认出口）共用，以 data.kind 区分语义。
 */
function DirectNodeViewInner({ data }: NodeProps) {
  const d = data as FlowNodeData;
  const isFallback = d.kind === "fallback" || d.label === "fallback";
  const Icon = isFallback ? CornerDownRight : ArrowDownToLine;
  return (
    <div
      className={`${NODE_SHELL} w-[208px] border-dashed border-muted-foreground/40`}
    >
      <div className="flex items-center gap-1.5">
        <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden />
        <span className="truncate text-sm font-medium text-muted-foreground">
          {isFallback ? "默认出口兜底" : "direct 直连"}
        </span>
      </div>
      {d.summary ? (
        <p className="mt-1 truncate text-xs text-muted-foreground">{d.summary}</p>
      ) : null}
    </div>
  );
}


/** 拖拽性能：memo 化避免 React Flow 每帧重绘全部节点 */
export const DirectNodeView = memo(DirectNodeViewInner);
