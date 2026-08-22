import { useTranslation } from "react-i18next";

interface ResourceGaugeProps {
  label: string;
  used: number;
  total: number;
  unit?: string;
  showPercentage?: boolean;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB", "PB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

function getUsageColor(percentage: number): "success" | "warning" | "danger" {
  if (percentage >= 80) return "danger";
  if (percentage >= 60) return "warning";
  return "success";
}

const colorClasses: Record<"success" | "warning" | "danger", string> = {
  success: "bg-emerald-500",
  warning: "bg-amber-500",
  danger: "bg-red-500",
};

export default function ResourceGauge({
  label,
  used,
  total,
  unit,
  showPercentage = true,
}: ResourceGaugeProps) {
  const { t } = useTranslation();

  const percentage = total > 0 ? Math.min((used / total) * 100, 100) : 0;
  const color = getUsageColor(percentage);

  const formattedUsed = unit === "bytes" ? formatBytes(used) : `${used.toFixed(1)}${unit || ""}`;
  const formattedTotal = unit === "bytes" ? formatBytes(total) : `${total.toFixed(1)}${unit || ""}`;

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-3 text-xs text-muted-foreground">
        <span className="font-medium text-foreground">{label}</span>
        <span className="text-right">
          {formattedUsed} / {formattedTotal}
          {showPercentage && ` (${percentage.toFixed(0)}%)`}
        </span>
      </div>
      <div
        role="progressbar"
        aria-label={`${label} ${t("admin.agents.usage")}`}
        aria-valuenow={Math.round(percentage)}
        aria-valuemin={0}
        aria-valuemax={100}
        className="h-2.5 w-full overflow-hidden rounded-full bg-muted/80"
      >
        <div className={`h-full rounded-full transition-all ${colorClasses[color]}`} style={{ width: `${percentage}%` }} />
      </div>
    </div>
  );
}
