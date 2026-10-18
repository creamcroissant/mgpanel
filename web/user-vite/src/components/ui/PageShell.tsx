import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/utils";

interface PageShellProps extends Omit<HTMLAttributes<HTMLDivElement>, "title"> {
  eyebrow?: ReactNode;
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  contentClassName?: string;
}

interface PageHeaderProps {
  eyebrow?: ReactNode;
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  className?: string;
}

/**
 * 页面头部（标题 + 描述 + 右侧操作）的唯一实现：PageShell 与 AdminPageShell
 * 都从这里渲染，避免两套外壳各写一份标题排版。
 */
export function PageHeader({ eyebrow, title, description, actions, className }: PageHeaderProps) {
  return (
    <header className={cn("flex min-w-0 flex-col gap-4 md:flex-row md:items-start md:justify-between", className)}>
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
  );
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
      <PageHeader eyebrow={eyebrow} title={title} description={description} actions={actions} />
      <div className={cn("min-w-0 space-y-5", contentClassName)}>{children}</div>
    </div>
  );
}
