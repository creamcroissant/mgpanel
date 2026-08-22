import * as React from "react";

import { cn } from "@/lib/utils";

export type BadgeVariant = "default" | "secondary" | "success" | "warning" | "danger" | "destructive" | "outline";

type BadgeProps = React.HTMLAttributes<HTMLSpanElement> & {
  variant?: BadgeVariant;
};

const variantClasses: Record<BadgeVariant, string> = {
  default: "bg-muted text-foreground",
  secondary: "bg-secondary text-secondary-foreground",
  success: "border border-success/20 bg-success/15 text-success dark:text-success",
  warning: "border border-warning/20 bg-warning/15 text-warning-foreground dark:text-warning",
  danger: "border border-destructive/20 bg-destructive/15 text-destructive",
  destructive: "border border-destructive/20 bg-destructive/15 text-destructive",
  outline: "border border-border bg-transparent text-foreground",
};

const Badge = React.forwardRef<HTMLSpanElement, BadgeProps>(
  ({ className, variant = "default", ...props }, ref) => (
    <span
      ref={ref}
      className={cn(
        "inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium",
        variantClasses[variant],
        className
      )}
      {...props}
    />
  )
);

Badge.displayName = "Badge";

export { Badge };
