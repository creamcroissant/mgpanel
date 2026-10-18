import type { ReactNode } from "react";
import { Badge, type BadgeVariant } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

/**
 * 状态徽章的唯一入口：全站的状态展示（在线/离线、运行/停止、成功/失败、
 * 待处理/进行中…）统一走这里，避免各页面自己写 `Badge + variant` 并各自维护一份
 * 「状态 → 颜色」映射。
 *
 * 两种用法：
 *   1. 内置状态：<StatusBadge status="running" label={t("...")} />
 *   2. 自定义状态：<StatusBadge tone="warning" label={t("...")} />
 */
export type StatusTone = "success" | "warning" | "danger" | "secondary" | "outline" | "info";

/** 内置状态 → 色调映射（覆盖 agent 上报、核心操作、版本状态等业务状态词）。 */
const toneByStatus: Record<string, StatusTone> = {
  // 正常/成功
  online: "success",
  active: "success",
  success: "success",
  running: "success",
  completed: "success",
  up_to_date: "success",
  healthy: "success",
  // 过渡/待处理
  pending: "warning",
  claimed: "warning",
  in_progress: "warning",
  applying: "warning",
  warning: "warning",
  upgrade_available: "warning",
  rollback_available: "warning",
  // 异常
  offline: "danger",
  error: "danger",
  failed: "danger",
  rolled_back: "danger",
  missing: "danger",
  unhealthy: "danger",
  // 中性/停用
  inactive: "secondary",
  stopped: "secondary",
  unknown: "secondary",
};

const variantByTone: Record<StatusTone, BadgeVariant> = {
  success: "success",
  warning: "warning",
  danger: "danger",
  secondary: "secondary",
  outline: "outline",
  info: "default",
};

const dotByTone: Record<StatusTone, string> = {
  success: "bg-success",
  warning: "bg-warning",
  danger: "bg-destructive",
  secondary: "bg-muted-foreground",
  outline: "bg-border",
  info: "bg-primary",
};

interface StatusBadgeProps {
  /** 业务状态词；命中内置映射时自动选色调。与 tone 二选一。 */
  status?: string;
  /** 直接指定色调（自定义状态用）。 */
  tone?: StatusTone;
  /** 显示文案，由调用方传 t() 结果；缺省回退到 status 原文。 */
  label?: string;
  /** 是否显示状态圆点（默认不显示，保持与普通标签一致的紧凑外观）。 */
  showDot?: boolean;
  size?: "sm" | "md" | "lg";
  icon?: ReactNode;
  className?: string;
}

export default function StatusBadge({
  status,
  tone,
  label,
  showDot = false,
  size = "md",
  icon,
  className,
}: StatusBadgeProps) {
  const resolvedTone: StatusTone = tone ?? toneByStatus[(status ?? "").toLowerCase()] ?? "secondary";
  const displayLabel = label ?? status ?? "";

  const dotSizes = {
    sm: "h-1.5 w-1.5",
    md: "h-2 w-2",
    lg: "h-2.5 w-2.5",
  };

  const sizeClasses = {
    sm: "px-2 py-0.5 text-xs",
    md: "px-2.5 py-0.5 text-sm",
    lg: "px-3 py-1 text-base",
  };

  return (
    <Badge
      variant={variantByTone[resolvedTone]}
      className={cn("gap-1.5 border font-medium", sizeClasses[size], className)}
    >
      {showDot ? (
        <span className={cn(dotSizes[size], "rounded-full", dotByTone[resolvedTone])} />
      ) : icon ? (
        icon
      ) : null}
      {displayLabel}
    </Badge>
  );
}
