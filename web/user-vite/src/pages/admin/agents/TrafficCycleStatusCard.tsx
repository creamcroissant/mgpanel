import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, Copy, Gauge, RefreshCw, RotateCcw } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { getAgentTrafficStatus, resetAgentTrafficCycle } from "@/api/admin";
import { QUERY_KEYS } from "@/lib/constants";
import { formatBytes, formatDateTime } from "@/lib/format";
import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Loading,
} from "@/components/ui";
import type { AgentTrafficPolicyStatus, AgentTrafficResetResult } from "@/types";

interface TrafficCycleStatusCardProps {
  agentHostId: number;
}

async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
}

function hasTrustedCounters(status: AgentTrafficPolicyStatus | undefined): boolean {
  return status?.state?.counter_seen === true;
}

function formatTrustedBytes(value: number, trusted: boolean, t: (key: string) => string): string {
  return trusted ? formatBytes(value) : t("admin.cores.trafficUnknown");
}

function getUsagePercent(status: AgentTrafficPolicyStatus | undefined): number {
  if (!status || status.threshold_bytes <= 0) {
    return 0;
  }
  return Math.min(100, Math.max(0, (status.usage_bytes / status.threshold_bytes) * 100));
}

function MetricTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border border-border bg-muted/20 p-3">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 text-sm font-semibold tabular-nums text-foreground">{value}</div>
    </div>
  );
}

function ResetResultSummary({ result }: { result: AgentTrafficResetResult }) {
  const { t } = useTranslation();
  return (
    <div className="rounded-md border border-success/30 bg-success/10 p-3 text-sm text-success-foreground dark:text-success">
      <div className="font-medium">{t("admin.cores.trafficResetResult")}</div>
      <div className="mt-2 grid gap-2 text-xs sm:grid-cols-2 lg:grid-cols-4">
        <span>{t("admin.cores.trafficResetAt")}: {formatDateTime(result.reset_at)}</span>
        <span>{t("admin.cores.trafficCycleKey")}: {result.cycle_key || "-"}</span>
        <span>{t("admin.cores.trafficRestoredServers")}: {result.restored_servers}</span>
        <span>{t("admin.cores.trafficFilterReasonsCleared")}: {result.cleared_filter_reasons ? t("common.success") : "-"}</span>
      </div>
    </div>
  );
}

export function TrafficCycleStatusCard({ agentHostId }: TrafficCycleStatusCardProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [isResetOpen, setIsResetOpen] = useState(false);
  const [lastResetResult, setLastResetResult] = useState<AgentTrafficResetResult | null>(null);

  const statusQuery = useQuery({
    queryKey: [...QUERY_KEYS.ADMIN_AGENT_TRAFFIC_STATUS, agentHostId],
    queryFn: () => getAgentTrafficStatus(agentHostId),
  });

  const resetMutation = useMutation({
    mutationFn: () => resetAgentTrafficCycle(agentHostId, { source: "admin" }),
    onSuccess: (result) => {
      setIsResetOpen(false);
      setLastResetResult(result);
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_AGENT_TRAFFIC_STATUS });
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_AGENT_TRAFFIC_POLICY });
      toast.success(t("admin.cores.trafficResetSuccess"));
    },
    onError: (error: Error) => {
      toast.error(t("admin.cores.trafficResetError"), { description: error.message });
    },
  });

  const status = statusQuery.data;
  const trusted = hasTrustedCounters(status);
  const usagePercent = getUsagePercent(status);
  const nextResetAt = status?.next_reset_at ? formatDateTime(status.next_reset_at) : t("admin.cores.trafficNoNextReset");

  return (
    <Card className="border border-border shadow-none" data-testid="agent-traffic-cycle-card">
      <CardHeader className="pb-3">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0 shrink">
            <CardTitle className="text-sm">{t("admin.cores.trafficCycleTitle")}</CardTitle>
            <CardDescription>{t("admin.cores.trafficCycleDescription")}</CardDescription>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <Button size="sm" variant="outline" onClick={() => statusQuery.refetch()} disabled={statusQuery.isFetching}>
              <RefreshCw className="mr-2 h-3.5 w-3.5" />
              {t("common.refresh")}
            </Button>
            <Button size="sm" variant="outline" onClick={() => setIsResetOpen(true)} disabled={resetMutation.isPending}>
              <RotateCcw className="mr-2 h-3.5 w-3.5" />
              {t("admin.cores.trafficManualReset")}
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {statusQuery.isLoading ? (
          <Loading />
        ) : statusQuery.error ? (
          <div className="flex flex-col items-center justify-center gap-3 py-6">
            <p className="text-sm text-destructive">{t("admin.cores.trafficStatusLoadError")}</p>
            <Button variant="outline" onClick={() => statusQuery.refetch()}>{t("common.retry")}</Button>
          </div>
        ) : status ? (
          <>
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant={status.policy.enabled ? "success" : "secondary"}>
                {status.policy.enabled ? t("admin.cores.trafficPolicyEnabled") : t("admin.cores.trafficPolicyDisabled")}
              </Badge>
              <Badge variant={status.threshold_reached ? "danger" : "outline"}>
                {status.threshold_reached ? t("admin.cores.trafficThresholdReached") : t("admin.cores.trafficThresholdNormal")}
              </Badge>
              <Badge variant={trusted ? "success" : "warning"}>
                {trusted ? t("admin.cores.trafficCountersTrusted") : t("admin.cores.trafficCountersUntrusted")}
              </Badge>
            </div>

            {!trusted && (
              <div className="flex items-start gap-2 rounded-md border border-warning/30 bg-warning/10 p-3 text-sm text-warning-foreground dark:text-warning">
                <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                <span>{t("admin.cores.trafficCountersUntrustedHint")}</span>
              </div>
            )}

            <div className="rounded-md border border-border p-3">
              <div className="flex items-center justify-between gap-3">
                <div className="flex items-center gap-2 text-sm font-medium">
                  <Gauge className="h-4 w-4 text-primary" />
                  {t("admin.cores.trafficUsageProgress")}
                </div>
                <span className="text-xs text-muted-foreground">{usagePercent.toFixed(1)}%</span>
              </div>
              <div className="mt-3 h-2 rounded-full bg-muted">
                <div className="h-2 rounded-full bg-primary" style={{ width: `${usagePercent}%` }} />
              </div>
            </div>

            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
              <MetricTile label={t("admin.cores.trafficUsage")} value={formatTrustedBytes(status.usage_bytes, trusted, t)} />
              <MetricTile label={t("admin.cores.trafficThresholdBytes")} value={formatBytes(status.threshold_bytes)} />
              <MetricTile label={t("admin.cores.trafficCycleUpload")} value={formatTrustedBytes(status.cycle_upload_bytes, trusted, t)} />
              <MetricTile label={t("admin.cores.trafficCycleDownload")} value={formatTrustedBytes(status.cycle_download_bytes, trusted, t)} />
              <MetricTile label={t("admin.cores.trafficCycleTotal")} value={formatTrustedBytes(status.cycle_total_bytes, trusted, t)} />
              <MetricTile label={t("admin.cores.trafficNextResetAt")} value={nextResetAt} />
              <MetricTile label={t("admin.cores.trafficLastResetAt")} value={status.policy.last_reset_at ? formatDateTime(status.policy.last_reset_at) : "-"} />
              <div className="rounded-md border border-border bg-muted/20 p-3">
                <div className="text-xs text-muted-foreground">{t("admin.cores.trafficBootId")}</div>
                {trusted && status.state?.boot_id ? (
                  <div className="mt-1 flex items-center gap-2">
                    <span className="min-w-0 truncate text-sm font-mono text-foreground" title={status.state.boot_id}>
                      {status.state.boot_id}
                    </span>
                    <button
                      type="button"
                      className="shrink-0 text-muted-foreground hover:text-foreground"
                      aria-label={t("common.copy")}
                      onClick={async () => {
                        const copied = await copyText(status.state!.boot_id);
                        if (copied) toast.success(t("common.success"));
                      }}
                    >
                      <Copy className="h-3.5 w-3.5" />
                    </button>
                  </div>
                ) : (
                  <div className="mt-1 text-lg font-semibold tabular-nums text-foreground">{t("admin.cores.trafficUnknown")}</div>
                )}
              </div>
            </div>
          </>
        ) : null}

        {lastResetResult && <ResetResultSummary result={lastResetResult} />}
      </CardContent>

      <Dialog open={isResetOpen} onOpenChange={setIsResetOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{t("admin.cores.trafficResetTitle")}</DialogTitle>
          </DialogHeader>
          <div className="py-2 text-sm text-muted-foreground">{t("admin.cores.trafficResetConfirm")}</div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsResetOpen(false)}>{t("common.cancel")}</Button>
            <Button onClick={() => resetMutation.mutate()} disabled={resetMutation.isPending}>
              {resetMutation.isPending ? t("common.loading") : t("admin.cores.trafficManualReset")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
