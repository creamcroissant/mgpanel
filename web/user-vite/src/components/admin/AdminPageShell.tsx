import type { ReactNode } from "react";
import { Card, CardContent } from "@/components/ui";
import { cn } from "@/lib/utils";

interface AdminPageShellProps {
  title: string;
  description?: ReactNode;
  actions?: ReactNode;
  toolbar?: ReactNode;
  stats?: ReactNode;
  children: ReactNode;
  framed?: boolean;
}

export default function AdminPageShell({
  title,
  description,
  actions,
  toolbar,
  stats,
  children,
  framed = false,
}: AdminPageShellProps) {
  const head = (
    <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
      <div className="space-y-1.5">
        <h1 className="text-2xl font-semibold tracking-tight text-foreground">{title}</h1>
        {description ? <div className="text-sm leading-6 text-muted-foreground">{description}</div> : null}
      </div>
      {actions ? <div className="flex flex-wrap gap-2">{actions}</div> : null}
    </div>
  );
  return (
    <div className="space-y-5 lg:space-y-6">
      {framed ? (
        <Card className="border-border">
          <CardContent className="p-5 sm:p-6">{head}</CardContent>
        </Card>
      ) : (
        head
      )}

      {toolbar ? <div className={cn("flex flex-col gap-3")}>{toolbar}</div> : null}
      {stats}
      {children}
    </div>
  );
}
