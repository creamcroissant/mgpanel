import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/utils";

interface ResourceCardProps extends Omit<HTMLAttributes<HTMLElement>, "title"> {
  icon?: ReactNode;
  title: ReactNode;
  description?: ReactNode;
  meta?: ReactNode;
  status?: ReactNode;
  actions?: ReactNode;
  footer?: ReactNode;
}

export function ResourceCard({
  icon,
  title,
  description,
  meta,
  status,
  actions,
  footer,
  children,
  className,
  ...props
}: ResourceCardProps) {
  return (
    <article
      className={cn(
        "group min-w-0 rounded-md border bg-card p-4 text-card-foreground shadow-none transition-colors hover:border-primary/30",
        className
      )}
      {...props}
    >
      <div className="flex min-w-0 items-start gap-3">
        {icon && (
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md border bg-muted/40 text-muted-foreground">
            {icon}
          </div>
        )}
        <div className="min-w-0 flex-1 space-y-1">
          <div className="flex min-w-0 flex-wrap items-start justify-between gap-2">
            <h3 className="min-w-0 break-words text-base font-semibold leading-6 text-foreground">{title}</h3>
            {status && <div className="shrink-0">{status}</div>}
          </div>
          {description && <p className="text-sm leading-6 text-muted-foreground">{description}</p>}
          {meta && <div className="flex flex-wrap items-center gap-2 pt-1 text-xs text-muted-foreground">{meta}</div>}
        </div>
      </div>
      {children && <div className="mt-4 min-w-0">{children}</div>}
      {(actions || footer) && (
        <div className="mt-4 flex min-w-0 flex-col gap-3 border-t pt-3 sm:flex-row sm:items-center sm:justify-between">
          {footer && <div className="min-w-0 text-sm text-muted-foreground">{footer}</div>}
          {actions && <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>}
        </div>
      )}
    </article>
  );
}
