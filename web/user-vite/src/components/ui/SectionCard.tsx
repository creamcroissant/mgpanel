import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/utils";

interface SectionCardProps extends Omit<HTMLAttributes<HTMLElement>, "title"> {
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  footer?: ReactNode;
}

export function SectionCard({ title, description, actions, footer, children, className, ...props }: SectionCardProps) {
  return (
    <section className={cn("min-w-0 rounded-md border bg-card shadow-none", className)} {...props}>
      <div className="flex min-w-0 flex-col gap-3 border-b p-4 sm:flex-row sm:items-start sm:justify-between sm:p-5">
        <div className="min-w-0 space-y-1">
          <h2 className="break-words text-base font-semibold leading-6 text-foreground">{title}</h2>
          {description && <p className="text-sm leading-6 text-muted-foreground">{description}</p>}
        </div>
        {actions && <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>}
      </div>
      <div className="min-w-0 p-4 sm:p-5">{children}</div>
      {footer && <div className="border-t px-4 py-3 text-sm text-muted-foreground sm:px-5">{footer}</div>}
    </section>
  );
}
