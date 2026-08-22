import type { ReactNode } from "react";
import { TrendingUp, TrendingDown, Minus } from "lucide-react";
import { Card, CardContent } from "@/components/ui";
import { cn } from "@/lib/utils";

interface StatCardProps {
  title: string;
  value: string | number;
  hint?: string;
  icon?: ReactNode;
  children?: ReactNode;
  className?: string;
  trend?: {
    value: number;
    label?: string;
  };
  variant?: "default" | "primary" | "success" | "warning" | "danger";
}

export default function StatCard({
  title,
  value,
  hint,
  icon,
  children,
  className = "",
  trend,
  variant = "default",
}: StatCardProps) {
  const variantStyles = {
    default: {
      iconWrap: "bg-muted text-muted-foreground",
    },
    primary: {
      iconWrap: "bg-primary/10 text-primary",
    },
    success: {
      iconWrap: "bg-emerald-500/10 text-emerald-600",
    },
    warning: {
      iconWrap: "bg-amber-500/10 text-amber-600",
    },
    danger: {
      iconWrap: "bg-red-500/10 text-red-600",
    },
  };

  const styles = variantStyles[variant];

  const getTrendIcon = () => {
    if (!trend) return null;
    if (trend.value > 0) {
      return <TrendingUp className="h-3 w-3" />;
    }
    if (trend.value < 0) {
      return <TrendingDown className="h-3 w-3" />;
    }
    return <Minus className="h-3 w-3" />;
  };

  const getTrendColor = () => {
    if (!trend) return "";
    if (trend.value > 0) return "text-emerald-600";
    if (trend.value < 0) return "text-red-600";
    return "text-muted-foreground";
  };

  return (
    <Card className={className}>
      <CardContent className="p-5 sm:p-6">
        <div className="flex items-start gap-4">
          {icon && (
            <div className={cn("flex h-10 w-10 shrink-0 items-center justify-center rounded-md", styles.iconWrap)}>
              {icon}
            </div>
          )}
          <div className="min-w-0 flex-1">
            <p className="text-sm font-medium text-muted-foreground">{title}</p>
            <div className="mt-2 flex flex-wrap items-end gap-2">
              <p className="truncate text-2xl font-semibold tracking-tight text-foreground">{value}</p>
              {trend && (
                <div className={cn("flex items-center gap-1 text-xs font-medium", getTrendColor())}>
                  {getTrendIcon()}
                  <span>{Math.abs(trend.value)}%</span>
                  {trend.label && <span className="text-muted-foreground">{trend.label}</span>}
                </div>
              )}
            </div>
            {hint && <p className="mt-2 text-sm text-muted-foreground">{hint}</p>}
            {children ? <div className="mt-4">{children}</div> : null}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
