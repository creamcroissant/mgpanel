import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

interface ResponsiveListProps {
  label: string;
  children: ReactNode;
  className?: string;
}

interface ResponsiveListItemProps {
  children: ReactNode;
  className?: string;
}

interface ResponsiveListFieldProps {
  label: ReactNode;
  children: ReactNode;
  className?: string;
}

export function ResponsiveList({ label, children, className }: ResponsiveListProps) {
  return (
    <div role="list" aria-label={label} className={cn("space-y-3 md:hidden", className)}>
      {children}
    </div>
  );
}

export function ResponsiveListItem({ children, className }: ResponsiveListItemProps) {
  return (
    <article
      role="listitem"
      className={cn("rounded-md border bg-card p-4", className)}
    >
      {children}
    </article>
  );
}

export function ResponsiveListField({ label, children, className }: ResponsiveListFieldProps) {
  return (
    <div className={cn("space-y-1", className)}>
      <dt className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground">{label}</dt>
      <dd className="text-sm text-foreground">{children}</dd>
    </div>
  );
}
