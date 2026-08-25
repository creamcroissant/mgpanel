import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { Activity, Gauge, Wifi } from "lucide-react";
import { fetchProbeReport, type ProbeReportItem } from "@/api/admin/cdn";
import { Loading, Card, CardContent, ResponsiveGrid, Table, TableHeader, TableRow, TableHead, TableBody, TableCell, EmptyState } from "@/components/ui";
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
  if (ms < 50) return "text-success";
  if (ms < 150) return "text-warning";
  return "text-destructive";
}

function latencyBgColor(ms: number): string {
  if (ms <= 0) return "";
  if (ms < 50) return "bg-success/10";
  if (ms < 150) return "bg-warning/10";
  return "bg-destructive/10";
}

function packetLossColor(loss: number): string {
  if (loss <= 0) return "text-success";
  if (loss < 0.05) return "text-warning";
  return "text-destructive";
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
                  <Card key={isp} className={` ${latencyBgColor(lat)}`}>
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
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("admin.probe.label")}</TableHead>
                  <TableHead>{t("admin.probe.target")}</TableHead>
                  <TableHead>{t("admin.probe.province")}</TableHead>
                  <TableHead>{t("admin.probe.isp")}</TableHead>
                  <TableHead className="text-right">{t("admin.probe.latency")}</TableHead>
                  <TableHead className="text-right">{t("admin.probe.packetLoss")}</TableHead>
                  <TableHead className="text-right">{t("admin.probe.probes")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                  {(data.results ?? []).map((item: ProbeReportItem, i: number) => (
                    <TableRow key={`${item.target}-${i}`}>
                      <TableCell className="max-w-[160px] truncate font-medium">
                        {item.label || "-"}
                      </TableCell>
                      <TableCell className="max-w-[200px] truncate font-mono text-xs text-muted-foreground">
                        {item.target}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {item.province || "-"}
                      </TableCell>
                      <TableCell>
                        {item.isp ? (
                          <Badge variant="outline" className="text-xs font-normal">
                            {ispLabel(item.isp)}
                          </Badge>
                        ) : "-"}
                      </TableCell>
                      <TableCell className={`text-right font-mono font-medium ${latencyColor(item.avg_latency_ms)}`}>
                        <span className={`inline-block rounded px-1.5 py-0.5 ${latencyBgColor(item.avg_latency_ms)}`}>
                          {latencyLabel(item.avg_latency_ms)}
                        </span>
                      </TableCell>
                      <TableCell className={`text-right font-mono ${packetLossColor(item.packet_loss)}`}>
                        {item.packet_loss > 0
                          ? `${(item.packet_loss * 100).toFixed(1)}%`
                          : item.packet_loss === 0 && item.total_probes > 0
                            ? "0%"
                            : "-"}
                      </TableCell>
                      <TableCell className="text-right font-mono text-xs text-muted-foreground">
                        {item.total_probes}
                      </TableCell>
                    </TableRow>
                  ))}

                  {(!data.results || data.results.length === 0) && (
                    <TableRow>
                      <TableCell colSpan={7}>
                        <EmptyState
                          icon={<Gauge className="h-full w-full" />}
                          title={t("admin.probe.noData")}
                          size="sm"
                        />
                      </TableCell>
                    </TableRow>
                  )}
              </TableBody>
            </Table>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
