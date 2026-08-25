import { Badge, type BadgeVariant } from "@/components/ui/badge";
import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

type StatusType = "online" | "offline" | "active" | "inactive" | "pending" | "success" | "warning" | "error";

interface StatusBadgeProps {
  status: StatusType;
  label?: string;
  showDot?: boolean;
  size?: "sm" | "md" | "lg";
  icon?: ReactNode;
}

const statusConfig: Record<
  StatusType,
  { variant: BadgeVariant; dotClass: string }
> = {
  online: {
    variant: "success",
    dotClass: "bg-success animate-pulse",
  },
  offline: {
    variant: "destructive",
    dotClass: "bg-destructive",
  },
  active: {
    variant: "success",
    dotClass: "bg-success",
  },
  inactive: {
    variant: "secondary",
    dotClass: "bg-muted-foreground",
  },
  pending: {
    variant: "warning",
    dotClass: "bg-warning animate-pulse",
  },
  success: {
    variant: "success",
    dotClass: "bg-success",
  },
  warning: {
    variant: "warning",
    dotClass: "bg-warning",
  },
  error: {
    variant: "destructive",
    dotClass: "bg-destructive",
  },
};

export default function StatusBadge({
  status,
  label,
  showDot = true,
  size = "md",
  icon,
}: StatusBadgeProps) {
  const config = statusConfig[status];
  // 文案由调用方传 t() 结果；未传时回退状态键（避免组件内硬编码英文）
  const displayLabel = label ?? status;

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
      variant={config.variant}
      className={cn("gap-1.5 border font-medium", sizeClasses[size])}
    >
      {showDot ? (
        <span className={cn(dotSizes[size], "rounded-full", config.dotClass)} />
      ) : icon ? (
        icon
      ) : null}
      {displayLabel}
    </Badge>
  );
}
