import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { Activity, Gauge, Wifi } from "lucide-react";
import { fetchProbeReport, type ProbeReportItem } from "@/api/admin/cdn";
import { Loading, Card, CardContent, ResponsiveGrid } from "@/components/ui";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";

interface LatencyProbePanelProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function latencyColor(ms: number): string {
  if (ms <= 0) return "text-muted-foreground";
  if (ms < 50) return "text-emerald-500";
  if (ms < 150) return "text-amber-500";
  return "text-red-500";
}

function latencyBgColor(ms: number): string {
  if (ms <= 0) return "";
  if (ms < 50) return "bg-emerald-500/10";
  if (ms < 150) return "bg-amber-500/10";
  return "bg-red-500/10";
}

function packetLossColor(loss: number): string {
  if (loss <= 0) return "text-emerald-500";
  if (loss < 0.05) return "text-amber-500";
  return "text-red-500";
}

function latencyLabel(ms: number): string {
  if (ms <= 0) return "-";
  if (ms >= 1000) return `${(ms / 1000).toFixed(1)}s`;
  return `${Math.round(ms)}ms`;
}

function ispLabel(isp: string): string {
  switch (isp) {
    case "mobile": return "移动";
    case "unicom": return "联通";
    case "telecom": return "电信";
    default: return isp;
  }
}

export default function LatencyProbePanel({ open, onOpenChange }: LatencyProbePanelProps) {
  const { t } = useTranslation();

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["cdn", "probe-report"],
    queryFn: fetchProbeReport,
    refetchInterval: open ? 30000 : false,
    enabled: open,
  });

  const ispOrder = ["mobile", "unicom", "telecom"];
  const ispIcons: Record<string, typeof Wifi> = {
    mobile: Wifi,
    unicom: Wifi,
    telecom: Wifi,
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-5xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Activity className="h-5 w-5" />
            {t("admin.probe.title")}
          </DialogTitle>
          <DialogDescription>{t("admin.probe.description")}</DialogDescription>
        </DialogHeader>

        {isLoading && <Loading />}

        {error && (
          <div className="flex flex-col items-center gap-3 py-10 text-sm text-destructive">
            <p>{t("admin.probe.loadError")}</p>
            <button
              className="text-sm underline underline-offset-2 hover:text-destructive/80"
              onClick={() => refetch()}
            >
              {t("common.retry")}
            </button>
          </div>
        )}

        {data && !data.enabled && (
          <div className="py-10 text-center text-sm text-muted-foreground">
            <Activity className="mx-auto mb-3 h-10 w-10 opacity-40" />
            <p>{t("admin.probe.notEnabled")}</p>
          </div>
        )}

        {data?.enabled && (
          <div className="space-y-5">
            {/* ISP summary cards */}
            <ResponsiveGrid minColWidth={160} gap={12}>
              {ispOrder.map((isp) => {
                const lat = data.isp_summary?.[isp];
                if (!lat) return null;
                const Icon = ispIcons[isp] || Wifi;
                return (
                  <Card key={isp} className={`shadow-none ${latencyBgColor(lat)}`}>
                    <CardContent className="p-4">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2 text-sm text-muted-foreground">
                          <Icon className="h-4 w-4" />
                          {ispLabel(isp)}
                        </div>
                        <span className={`text-lg font-semibold tracking-tight ${latencyColor(lat)}`}>
                          {latencyLabel(lat)}
                        </span>
                      </div>
                    </CardContent>
                  </Card>
                );
              })}
            </ResponsiveGrid>

            {/* Probe target table */}
            <div className="overflow-x-auto rounded-lg border">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b bg-muted/50 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
                    <th className="px-4 py-3">{t("admin.probe.label")}</th>
                    <th className="px-4 py-3">{t("admin.probe.target")}</th>
                    <th className="px-4 py-3">{t("admin.probe.province")}</th>
                    <th className="px-4 py-3">{t("admin.probe.isp")}</th>
                    <th className="px-4 py-3 text-right">{t("admin.probe.latency")}</th>
                    <th className="px-4 py-3 text-right">{t("admin.probe.packetLoss")}</th>
                    <th className="px-4 py-3 text-right">{t("admin.probe.probes")}</th>
                  </tr>
                </thead>
                <tbody>
                  {(data.results ?? []).map((item: ProbeReportItem, i: number) => (
                    <tr
                      key={`${item.target}-${i}`}
                      className="border-b last:border-0 hover:bg-muted/30"
                    >
                      <td className="max-w-[160px] truncate px-4 py-3 font-medium">
                        {item.label || "-"}
                      </td>
                      <td className="max-w-[200px] truncate px-4 py-3 text-muted-foreground font-mono text-xs">
                        {item.target}
                      </td>
                      <td className="px-4 py-3 text-muted-foreground">
                        {item.province || "-"}
                      </td>
                      <td className="px-4 py-3">
                        {item.isp ? (
                          <Badge variant="outline" className="text-xs font-normal">
                            {ispLabel(item.isp)}
                          </Badge>
                        ) : "-"}
                      </td>
                      <td className={`px-4 py-3 text-right font-mono text-sm font-medium ${latencyColor(item.avg_latency_ms)}`}>
                        <span className={`inline-block rounded px-1.5 py-0.5 ${latencyBgColor(item.avg_latency_ms)}`}>
                          {latencyLabel(item.avg_latency_ms)}
                        </span>
                      </td>
                      <td className={`px-4 py-3 text-right font-mono text-sm ${packetLossColor(item.packet_loss)}`}>
                        {item.packet_loss > 0
                          ? `${(item.packet_loss * 100).toFixed(1)}%`
                          : item.packet_loss === 0 && item.total_probes > 0
                            ? "0%"
                            : "-"}
                      </td>
                      <td className="px-4 py-3 text-right text-muted-foreground font-mono text-xs">
                        {item.total_probes}
                      </td>
                    </tr>
                  ))}

                  {(!data.results || data.results.length === 0) && (
                    <tr>
                      <td colSpan={7} className="py-10 text-center text-sm text-muted-foreground">
                        <Gauge className="mx-auto mb-2 h-6 w-6 opacity-40" />
                        {t("admin.probe.noData")}
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
