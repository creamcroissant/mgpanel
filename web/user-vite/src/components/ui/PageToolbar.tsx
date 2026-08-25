import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/utils";

interface PageToolbarProps extends HTMLAttributes<HTMLDivElement> {
  leading?: ReactNode;
  filters?: ReactNode;
  actions?: ReactNode;
}

export function PageToolbar({ leading, filters, actions, children, className, ...props }: PageToolbarProps) {
  return (
    <div
      className={cn(
        "flex min-w-0 flex-col gap-3 rounded-md border bg-card p-3 md:flex-row md:items-center md:justify-between",
        className
      )}
      {...props}
    >
      <div className="flex min-w-0 flex-1 flex-col gap-3 md:flex-row md:items-center">
        {leading}
        {filters}
        {children}
      </div>
      {actions && <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>}
    </div>
  );
}
