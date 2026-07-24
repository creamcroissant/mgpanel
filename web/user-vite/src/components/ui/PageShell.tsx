import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/utils";

interface PageShellProps extends Omit<HTMLAttributes<HTMLDivElement>, "title"> {
  eyebrow?: ReactNode;
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  contentClassName?: string;
}

export function PageShell({
  eyebrow,
  title,
  description,
  actions,
  children,
  className,
  contentClassName,
  ...props
}: PageShellProps) {
  return (
    <div className={cn("space-y-6 lg:space-y-7", className)} {...props}>
      <header className="flex min-w-0 flex-col gap-4 md:flex-row md:items-start md:justify-between">
        <div className="min-w-0 space-y-1.5">
          {eyebrow && (
            <div className="text-xs font-semibold uppercase tracking-[0.12em] text-muted-foreground">
              {eyebrow}
            </div>
          )}
          <h1 className="break-words text-2xl font-semibold tracking-tight text-foreground md:text-3xl">
            {title}
          </h1>
          {description && <p className="max-w-3xl text-sm leading-6 text-muted-foreground">{description}</p>}
        </div>
        {actions && <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>}
      </header>
      <div className={cn("min-w-0 space-y-5", contentClassName)}>{children}</div>
    </div>
  );
}
