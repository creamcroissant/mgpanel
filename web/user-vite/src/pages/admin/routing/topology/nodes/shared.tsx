import type { LucideIcon } from "lucide-react";
import { Ban } from "lucide-react";

/** 画布节点统一宽度（与 layout.ts 布局常量保持一致） */
export const NODE_WIDTH = 208;

/**
 * 统一扁平外壳：rounded-md 无 shadow（设计系统卡片规范），
 * 语义令牌配色，暗色模式随 CSS 变量自适应。
 */
export const NODE_SHELL =
  "rounded-md border bg-card px-3 py-2 text-card-foreground";

/** 停用态：整节点半透明降饱和（配合 DisabledBadge 使用） */
export function disabledClass(enabled?: boolean): string {
  return enabled === false ? "opacity-55 saturate-50" : "";
}

/** 校验错误角标：validate issue 注入节点 data.issues 后由节点视图渲染 */
export function IssueBadge({ count = 1 }: { count?: number }) {
  return (
    <span
      className="absolute -left-2 -top-2 z-10 flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-bold leading-none text-destructive-foreground"
      title={`校验问题 ×${count}`}
    >
      !
      {count > 1 && <span className="ml-px">{count}</span>}
    </span>
  );
}

/** 从通用节点载荷提取校验问题（canvas 层注入） */
export function nodeIssues(d: Record<string, unknown>): { severity: string; message: string }[] {
  const raw = d.issues;
  return Array.isArray(raw) ? (raw as { severity: string; message: string }[]) : [];
}

/** 右上角“已停用”角标；父容器需 relative 定位 */
export function DisabledBadge({ label = "已停用" }: { label?: string }) {
  return (
    <span className="absolute -right-2 -top-2 z-10 flex items-center gap-0.5 rounded-sm border bg-muted px-1 py-px text-[10px] font-medium leading-tight text-muted-foreground">
      <Ban className="h-2.5 w-2.5" aria-hidden />
      {label}
    </span>
  );
}

/** 成员权重条：按 weight/maxWeight 比例填充（出口集节点专用） */
export function WeightBar({
  weight,
  maxWeight,
}: {
  weight: number;
  maxWeight: number;
}) {
  const pct =
    maxWeight > 0 ? Math.max(8, Math.round((weight / maxWeight) * 100)) : 8;
  return (
    <div
      className="h-1.5 w-14 shrink-0 overflow-hidden rounded-full bg-muted"
      role="img"
      aria-label={`权重 ${weight}`}
    >
      <div className="h-full rounded-full bg-success" style={{ width: `${pct}%` }} />
    </div>
  );
}

/** 图标兜底：确保 lucide 组件类型可安全索引 */
export type IconType = LucideIcon;
