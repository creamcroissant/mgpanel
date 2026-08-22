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
  { variant: BadgeVariant; label: string; dotClass: string }
> = {
  online: {
    variant: "success",
    label: "Online",
    dotClass: "bg-success animate-pulse",
  },
  offline: {
    variant: "destructive",
    label: "Offline",
    dotClass: "bg-destructive",
  },
  active: {
    variant: "success",
    label: "Active",
    dotClass: "bg-success",
  },
  inactive: {
    variant: "secondary",
    label: "Inactive",
    dotClass: "bg-muted-foreground",
  },
  pending: {
    variant: "warning",
    label: "Pending",
    dotClass: "bg-warning animate-pulse",
  },
  success: {
    variant: "success",
    label: "Success",
    dotClass: "bg-success",
  },
  warning: {
    variant: "warning",
    label: "Warning",
    dotClass: "bg-warning",
  },
  error: {
    variant: "destructive",
    label: "Error",
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
  const displayLabel = label || config.label;

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
