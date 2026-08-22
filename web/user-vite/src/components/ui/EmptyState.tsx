import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

interface EmptyStateProps {
  icon?: ReactNode;
  title: string;
  description?: string;
  action?: ReactNode;
  size?: "sm" | "md" | "lg";
}

export default function EmptyState({
  icon,
  title,
  description,
  action,
  size = "md",
}: EmptyStateProps) {
  const sizeClasses = {
    sm: {
      container: "px-4 py-8",
      iconWrapper: "mb-3 h-12 w-12",
      iconSize: "h-6 w-6",
      title: "text-base",
      description: "text-xs",
    },
    md: {
      container: "px-6 py-16",
      iconWrapper: "mb-4 h-20 w-20",
      iconSize: "h-10 w-10",
      title: "text-lg",
      description: "text-sm",
    },
    lg: {
      container: "px-8 py-24",
      iconWrapper: "mb-6 h-28 w-28",
      iconSize: "h-14 w-14",
      title: "text-xl",
      description: "text-base",
    },
  };

  const classes = sizeClasses[size];

  return (
    <div className={cn("flex flex-col items-center justify-center text-center", classes.container)}>
      {icon && (
        <div
          className={cn(
            "flex items-center justify-center rounded-full bg-muted text-muted-foreground",
            classes.iconWrapper
          )}
        >
          <div className={classes.iconSize}>{icon}</div>
        </div>
      )}
      <h3 className={cn("mb-2 font-semibold text-foreground", classes.title)}>{title}</h3>
      {description && (
        <p className={cn("mb-6 max-w-md text-muted-foreground", classes.description)}>
          {description}
        </p>
      )}
      {action && <div>{action}</div>}
    </div>
  );
}
