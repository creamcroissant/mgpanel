import type { ReactNode } from "react";
import { Card, CardContent, PageHeader } from "@/components/ui";
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
    <PageHeader
      title={title}
      description={description}
      actions={actions}
      className="lg:flex-row lg:items-center lg:justify-between"
    />
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
