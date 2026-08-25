import * as React from "react";

import LanguageSwitcher from "@/components/LanguageSwitcher";
import ThemeToggle from "@/components/ThemeToggle";
import { cn } from "@/lib/utils";

interface AuthShellProps {
  children: React.ReactNode;
  /** 内容列宽度类，默认 max-w-md */
  maxWidth?: string;
}

/**
 * 认证/安装页统一外壳：背景装饰 + 右上角语言/主题切换 + 垂直居中。
 *
 * 居中采用 flex-col + m-auto 而非 items-center：内容高于视口时
 * auto margin 优雅退化为可滚动文档流，不会像 items-center 那样
 * 把顶部裁切到不可滚动（Install 长表单在小屏上的历史问题）。
 */
export function AuthShell({ children, maxWidth = "max-w-md" }: AuthShellProps) {
  return (
    <div className="relative flex min-h-svh flex-col bg-background px-4 py-10">
      <div className="pointer-events-none absolute -left-10 -top-10 h-40 w-40 rounded-full bg-primary/10 blur-2xl" />
      <div className="pointer-events-none absolute bottom-0 right-0 h-48 w-48 rounded-full bg-primary/5 blur-3xl" />

      <div className="absolute right-4 top-4 z-10 flex items-center gap-2">
        <LanguageSwitcher />
        <ThemeToggle />
      </div>

      <div className={cn("m-auto w-full", maxWidth)}>{children}</div>
    </div>
  );
}
