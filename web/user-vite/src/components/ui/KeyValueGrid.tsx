import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/utils";

export interface KeyValueItem {
  label: ReactNode;
  value: ReactNode;
  hint?: ReactNode;
}

interface KeyValueGridProps extends HTMLAttributes<HTMLDListElement> {
  items: KeyValueItem[];
}

export function KeyValueGrid({ items, className, ...props }: KeyValueGridProps) {
  return (
    <dl className={cn("grid min-w-0 gap-3 sm:grid-cols-2 xl:grid-cols-3", className)} {...props}>
      {items.map((item, index) => (
        <div key={index} className="min-w-0 rounded-md border bg-muted/25 p-3">
          <dt className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground">
            {item.label}
          </dt>
          <dd className="mt-1 break-words text-sm font-semibold leading-6 text-foreground">{item.value}</dd>
          {item.hint && <dd className="mt-1 text-xs leading-5 text-muted-foreground">{item.hint}</dd>}
        </div>
      ))}
    </dl>
  );
}
