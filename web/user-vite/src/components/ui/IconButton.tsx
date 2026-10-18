import * as React from "react";
import { cn } from "@/lib/utils";

/**
 * 小图标按钮（行内清除、展开/收起箭头、上下移这类微交互）。
 *
 * 存在的意义：这类按钮此前在各页面手写原生 `<button>` + 各自的 padding / hover 底色
 * （`p-0.5`、`px-0.5`、`hover:bg-muted`、`hover:bg-muted-foreground/20`…），尺寸与
 * hover 反馈不一致。这里保留既有视觉，只收敛样式来源。
 */
export type IconButtonSize = "xs" | "sm" | "icon";
export type IconButtonTone = "muted" | "subtle";

const sizeClasses: Record<IconButtonSize, string> = {
  xs: "p-0.5",
  sm: "px-0.5",
  icon: "h-8 w-8",
};

const toneClasses: Record<IconButtonTone, string> = {
  muted: "hover:bg-muted",
  subtle: "hover:bg-muted-foreground/20",
};

interface IconButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  size?: IconButtonSize;
  tone?: IconButtonTone;
}

const IconButton = React.forwardRef<HTMLButtonElement, IconButtonProps>(
  ({ className, size = "xs", tone = "muted", type = "button", ...props }, ref) => (
    <button
      ref={ref}
      type={type}
      className={cn(
        "inline-flex shrink-0 items-center justify-center rounded-none bg-transparent",
        "text-muted-foreground transition-colors hover:text-foreground",
        "focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring",
        "disabled:pointer-events-none disabled:opacity-30",
        sizeClasses[size],
        toneClasses[tone],
        className
      )}
      {...props}
    />
  )
);

IconButton.displayName = "IconButton";

export { IconButton };
