import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/utils";

type UsageTone = "success" | "warning" | "danger" | "primary";

const toneColor: Record<UsageTone, string> = {
  success: "hsl(var(--success))",
  warning: "hsl(var(--warning))",
  danger: "hsl(var(--destructive))",
  primary: "hsl(var(--primary))",
};

interface UsageRingProps extends HTMLAttributes<HTMLDivElement> {
  value: number;
  label: ReactNode;
  detail?: ReactNode;
  tone?: UsageTone;
  size?: "sm" | "md" | "lg";
}

const sizeClasses = {
  sm: "h-24 w-24",
  md: "h-32 w-32",
  lg: "h-40 w-40",
} as const;

export function UsageRing({
  value,
  label,
  detail,
  tone = "primary",
  size = "md",
  className,
  ...props
}: UsageRingProps) {
  const clamped = Math.max(0, Math.min(100, Number.isFinite(value) ? value : 0));
  const rounded = Math.round(clamped);

  return (
    <div
      data-testid="usage-ring"
      role="img"
      aria-label={`${label}: ${rounded}%`}
      className={cn("flex min-w-0 flex-col items-center gap-3 text-center", className)}
      {...props}
    >
      <div
        className={cn("relative grid shrink-0 place-items-center rounded-full", sizeClasses[size])}
        style={{
          background: `conic-gradient(${toneColor[tone]} ${clamped * 3.6}deg, hsl(var(--muted)) 0deg)`,
        }}
      >
        <div className="absolute inset-2 rounded-full border bg-card" />
        <div className="relative space-y-0.5">
          <div className="text-2xl font-semibold tabular-nums text-foreground">{rounded}%</div>
          <div className="text-[11px] font-medium uppercase tracking-[0.08em] text-muted-foreground">{label}</div>
        </div>
      </div>
      {detail && <div className="max-w-[14rem] text-sm leading-6 text-muted-foreground">{detail}</div>}
    </div>
  );
}
