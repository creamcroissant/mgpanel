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
}

export default function AdminPageShell({
  title,
  description,
  actions,
  toolbar,
  stats,
  children,
}: AdminPageShellProps) {
  return (
    <div className="space-y-5 lg:space-y-6">
      <Card className="shadow-none">
        <CardContent className="flex flex-col gap-4 p-5 sm:p-6 lg:flex-row lg:items-center lg:justify-between">
          <div className="space-y-1.5">
            <h1 className="text-2xl font-semibold tracking-tight text-foreground">{title}</h1>
            {description ? <div className="text-sm leading-6 text-muted-foreground">{description}</div> : null}
          </div>
          {actions ? <div className="flex flex-wrap gap-2">{actions}</div> : null}
        </CardContent>
      </Card>

      {toolbar ? <div className={cn("flex flex-col gap-3")}>{toolbar}</div> : null}
      {stats}
      {children}
    </div>
  );
}
