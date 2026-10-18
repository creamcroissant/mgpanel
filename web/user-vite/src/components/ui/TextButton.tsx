import * as React from "react";
import { cn } from "@/lib/utils";

/**
 * 纯文字按钮（"显示高级选项"、"清空"这类行内操作）。
 *
 * 存在的意义：这类按钮此前在各页面手写原生 `<button>` + 各不相同的 hover / 下划线样式，
 * 外观与可访问性（focus 环、disabled 态）都不统一。这里保留既有视觉（灰色文字、
 * hover 变亮），只把样式来源收敛到一处。
 */
export type TextButtonVariant = "muted" | "underline" | "danger";
export type TextButtonSize = "xs" | "sm";

const variantClasses: Record<TextButtonVariant, string> = {
  muted: "text-muted-foreground hover:text-foreground",
  underline: "text-muted-foreground underline underline-offset-2 hover:text-foreground",
  danger: "text-muted-foreground hover:text-destructive",
};

const sizeClasses: Record<TextButtonSize, string> = {
  xs: "text-xs",
  sm: "text-sm",
};

interface TextButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: TextButtonVariant;
  size?: TextButtonSize;
}

const TextButton = React.forwardRef<HTMLButtonElement, TextButtonProps>(
  ({ className, variant = "muted", size = "xs", type = "button", ...props }, ref) => (
    <button
      ref={ref}
      type={type}
      className={cn(
        "inline-flex items-center rounded-none bg-transparent p-0 transition-colors",
        "focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring",
        "disabled:pointer-events-none disabled:opacity-50",
        variantClasses[variant],
        sizeClasses[size],
        className
      )}
      {...props}
    />
  )
);

TextButton.displayName = "TextButton";

export { TextButton };
