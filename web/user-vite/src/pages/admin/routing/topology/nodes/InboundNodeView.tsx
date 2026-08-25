import { memo } from "react";
import type { NodeProps } from "@xyflow/react";
import { Lock, ShieldCheck } from "lucide-react";
import type { FlowNodeData } from "./AgentNodeView";
import { DisabledBadge, NODE_SHELL, disabledClass } from "./shared";

/** 入站节点本地扩展字段：协议/TLS/reality/端口结构化标记（缺省回退 summary 文本解析） */
interface InboundNodeSpecifics {
  protocol?: string;
  port?: number;
  tlsEnabled?: boolean;
  reality?: boolean;
}
type InboundFlowData = FlowNodeData & InboundNodeSpecifics;

/**
 * 入站 spec 节点：紫系分类色。
 * - 首行：色点 + tag + enabled 角标
 * - 链路标签行：协议徽章 + TLS/Reality 徽章 + :port（tabular-nums 右对齐）
 * - reality 优先级高于普通 TLS（两者同时为真只显示 Reality）
 *
 * 色彩说明：violet-500 为节点类别色（非语义状态色），双主题下对比度达标；
 * 语义态（成功/警告/危险）仍一律走 success/warning/destructive 令牌。
 */
function InboundNodeViewInner({ data }: NodeProps) {
  const d = data as InboundFlowData;
  const off = d.enabled === false;

  // 结构化字段优先，缺省回退 summary 文本解析（如 "vless:28001 · reality"）
  const summary = String(d.summary ?? "");
  const hasTls = d.tlsEnabled === true || /tls/i.test(summary);
  const isReality = d.reality === true || /reality/i.test(summary);
  const protocol = d.protocol ?? summary.split(":")[0]?.trim() ?? "";
  const port =
    d.port ??
    (() => {
      const m = summary.match(/:\s*(\d+)/);
      return m ? Number(m[1]) : null;
    })();

  return (
    <div
      className={`${NODE_SHELL} relative w-[208px] ${
        off ? "border-muted-foreground/40" : "border-violet-500/60"
      } ${disabledClass(d.enabled)}`}
    >
      {off && <DisabledBadge />}
      <div className="flex items-center gap-1.5">
        {/* 分类色：入站=紫（见组件注释） */}
        <span className="h-2 w-2 shrink-0 rounded-sm bg-violet-500" aria-hidden />
        <span className="truncate text-sm font-medium" title={String(d.label)}>
          {d.label}
        </span>
      </div>
      {/* 协议链路标签行：协议徽章 + TLS/Reality 徽章 + :port */}
      <div className="mt-1.5 flex flex-wrap items-center gap-1">
        {protocol && (
          <span className="rounded-md border border-border bg-muted/50 px-1.5 py-0.5 text-[11px] font-medium leading-none">
            {protocol}
          </span>
        )}
        {isReality ? (
          <span
            className="flex items-center gap-1 rounded-md border border-border bg-muted/50 px-1.5 py-0.5 text-[11px] leading-none text-violet-600 dark:text-violet-300"
            title="Reality"
          >
            <ShieldCheck className="h-3 w-3" />
            Reality
          </span>
        ) : hasTls ? (
          <span
            className="flex items-center gap-1 rounded-md border border-border bg-muted/50 px-1.5 py-0.5 text-[11px] leading-none text-muted-foreground"
            title="TLS"
          >
            <Lock className="h-3 w-3" />
            TLS
          </span>
        ) : null}
        {port != null && (
          <span className="ml-auto rounded-md border border-border bg-muted/50 px-1.5 py-0.5 text-[11px] tabular-nums leading-none text-muted-foreground">
            :{port}
          </span>
        )}
      </div>
    </div>
  );
}


/** 拖拽性能：memo 化避免 React Flow 每帧重绘全部节点 */
export const InboundNodeView = memo(InboundNodeViewInner);
