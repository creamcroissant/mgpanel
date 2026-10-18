import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Activity, AlertTriangle, ListChecks, RefreshCw } from "lucide-react";
import { useTranslation } from "react-i18next";
import { listAgentLifecycleOperations } from "@/api/admin";
import { QUERY_KEYS } from "@/lib/constants";
import { formatDateTime } from "@/lib/format";
import {
  Badge,
  StatusBadge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  EmptyState,
  Loading,
  StatCard,
  ResponsiveGrid,
} from "@/components/ui";
import type { AgentCommandQueueStats, AgentLifecycleOperation } from "@/types";

interface AgentCommandQueuePanelProps {
  agentHostId: number;
}


const ACTIVE_STATUSES = new Set(["pending", "claimed", "in_progress"]);
const REJECTION_STATUSES = new Set(["queue_full", "unsupported_action", "timeout"]);

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function toNumber(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function toStringArray(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }
  return value.filter((item): item is string => typeof item === "string");
}

function normalizeQueueStats(record: Record<string, unknown>): AgentCommandQueueStats | null {
  const capacity = toNumber(record.capacity ?? record.Capacity);
  const queued = toNumber(record.queued ?? record.Queued);
  const inflight = toNumber(record.inflight ?? record.Inflight);
  const workers = toNumber(record.workers ?? record.Workers);
  const available = toNumber(record.available ?? record.Available);
  const updatedAt = toNumber(record.updated_at ?? record.updatedAt ?? record.UpdatedAt);
  if (
    capacity === undefined
    || queued === undefined
    || inflight === undefined
    || workers === undefined
    || available === undefined
  ) {
    return null;
  }
  return {
    capacity,
    queued,
    inflight,
    workers,
    available,
    active_command_ids: toStringArray(record.active_command_ids ?? record.activeCommandIds ?? record.ActiveCommandIDs),
    updated_at: updatedAt ?? 0,
  };
}

function findQueueStats(value: unknown, depth = 0): AgentCommandQueueStats | null {
  if (!isRecord(value) || depth > 3) {
    return null;
  }
  const direct = normalizeQueueStats(value);
  if (direct) {
    return direct;
  }
  for (const key of ["queue_stats", "queueStats", "queue", "stats", "payload"]) {
    const nested = findQueueStats(value[key], depth + 1);
    if (nested) {
      return nested;
    }
  }
  return null;
}

function isActiveOperation(operation: AgentLifecycleOperation): boolean {
  return ACTIVE_STATUSES.has(operation.status);
}

function getLatestQueueStats(operations: AgentLifecycleOperation[]): AgentCommandQueueStats | null {
  for (const operation of operations) {
    const resultStats = findQueueStats(operation.result_payload);
    if (resultStats) {
      return resultStats;
    }
    const requestStats = findQueueStats(operation.request_payload);
    if (requestStats) {
      return requestStats;
    }
  }
  return null;
}

function MetricTile({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded-none border border-border border-border bg-muted/20 p-3">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 text-xl font-semibold tabular-nums text-foreground">{value}</div>
    </div>
  );
}

export function AgentCommandQueuePanel({ agentHostId }: AgentCommandQueuePanelProps) {
  const { t } = useTranslation();
  const operationsQuery = useQuery({
    queryKey: [...QUERY_KEYS.ADMIN_AGENT_LIFECYCLE_OPERATIONS, agentHostId, "command-queue"],
    queryFn: () => listAgentLifecycleOperations(agentHostId, { limit: 20 }),
    refetchInterval: (query) => {
      const operations = query.state.data?.operations ?? [];
      return operations.some(isActiveOperation) ? 5000 : false;
    },
  });

  const operations = useMemo(() => operationsQuery.data?.operations ?? [], [operationsQuery.data?.operations]);
  const queueStats = useMemo(() => getLatestQueueStats(operations), [operations]);
  const rejectionOperations = useMemo(
    () => operations.filter((operation) => REJECTION_STATUSES.has(operation.status)),
    [operations]
  );
  const activeOperations = useMemo(() => operations.filter(isActiveOperation), [operations]);

  return (
    <Card className="border border-border" data-testid="agent-command-queue-panel">
      <CardHeader className="pb-3">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <CardTitle className="text-sm">{t("admin.cores.queuePanelTitle")}</CardTitle>
            <CardDescription>{t("admin.cores.queuePanelDescription")}</CardDescription>
          </div>
          <Button size="sm" variant="outline" onClick={() => operationsQuery.refetch()} disabled={operationsQuery.isFetching}>
            <RefreshCw className="mr-2 h-3.5 w-3.5" />
            {t("common.refresh")}
          </Button>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {operationsQuery.isLoading ? (
          <Loading />
        ) : queueStats ? (
          <>
            <ResponsiveGrid minColWidth={180} gap={12}>
              <MetricTile label={t("admin.cores.queueCapacity")} value={queueStats.capacity} />
              <MetricTile label={t("admin.cores.queueQueued")} value={queueStats.queued} />
              <MetricTile label={t("admin.cores.queueInflight")} value={queueStats.inflight} />
              <MetricTile label={t("admin.cores.queueWorkers")} value={queueStats.workers} />
              <MetricTile label={t("admin.cores.queueAvailable")} value={queueStats.available} />
            </ResponsiveGrid>
            <div className="rounded-none border border-border border-border p-3 text-sm text-muted-foreground">
              <div className="flex items-center gap-2 text-foreground">
                <Activity className="h-4 w-4 text-primary" />
                <span className="font-medium">{t("admin.cores.queueUpdatedAt")}</span>
                <span>{queueStats.updated_at ? formatDateTime(queueStats.updated_at) : "-"}</span>
              </div>
              {queueStats.active_command_ids && queueStats.active_command_ids.length > 0 && (
                <div className="mt-2 flex flex-wrap gap-2">
                  {queueStats.active_command_ids.map((id) => (
                    <Badge key={id} variant="outline" className="font-mono">
                      {id}
                    </Badge>
                  ))}
                </div>
              )}
            </div>
          </>
        ) : (
          <EmptyState
            icon={<ListChecks className="h-full w-full" />}
            title={t("admin.cores.queueStatsEmpty")}
            description={t("admin.cores.queueStatsEmptyDescription")}
            size="sm"
          />
        )}

        <ResponsiveGrid minColWidth={220} gap={12}>
          <StatCard
            title={t("admin.cores.queueActiveOperations")}
            value={activeOperations.length}
            icon={<ListChecks className="h-4 w-4" />}
            variant="primary"
          />
          <StatCard
            title={t("admin.cores.queueRejections")}
            value={rejectionOperations.length}
            icon={<AlertTriangle className="h-4 w-4" />}
            variant="warning"
          />
        </ResponsiveGrid>

        {rejectionOperations.length === 0 ? (
          <div className="rounded-none border border-border border-border bg-muted/20 p-3 text-sm text-muted-foreground">
            {t("admin.cores.queueNoRejections")}
          </div>
        ) : (
          <div className="space-y-2">
            <div className="text-sm font-medium">{t("admin.cores.queueRecentRejections")}</div>
            {rejectionOperations.slice(0, 5).map((operation) => (
              <div key={operation.id} className="rounded-none border border-border border-border p-3">
                <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                  <div className="flex flex-wrap items-center gap-2">
                    <StatusBadge status={operation.status} label={t(`admin.cores.status.${operation.status}`, { defaultValue: operation.status })} />
                    <span className="text-sm font-medium">
                      {t(`admin.cores.operationType.${operation.operation_type}`, { defaultValue: operation.operation_type })}
                    </span>
                  </div>
                  <span className="text-xs text-muted-foreground">{formatDateTime(operation.created_at)}</span>
                </div>
                <div className="mt-2 break-all text-xs text-muted-foreground">
                  {operation.error_message || operation.id}
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
