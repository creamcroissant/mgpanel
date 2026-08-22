import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { History, RefreshCw } from "lucide-react";
import { useTranslation } from "react-i18next";
import { listOperationLogs } from "@/api/admin";
import { useOperationLogStream } from "@/hooks/useOperationLogStream";
import { QUERY_KEYS } from "@/lib/constants";
import { formatDateTime } from "@/lib/format";
import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  EmptyState,
  Loading,
} from "@/components/ui";
import type { OperationLogEntry, OperationLogLevel, OperationLogScope } from "@/types";

interface OperationLogTimelineProps {
  agentHostId: number;
  targetId?: string;
  scope?: OperationLogScope;
  enabled?: boolean;
}

const LOG_LIMIT = 100;

function getLevelVariant(level: OperationLogLevel): "secondary" | "warning" | "danger" | "outline" {
  switch (level) {
    case "error":
      return "danger";
    case "warn":
      return "warning";
    case "debug":
      return "outline";
    default:
      return "secondary";
  }
}

function formatPayload(payload: unknown): string {
  if (!payload) {
    return "";
  }
  if (typeof payload === "object" && Object.keys(payload).length === 0) {
    return "";
  }
  try {
    return JSON.stringify(payload, null, 2);
  } catch {
    return String(payload);
  }
}

function mergeLogs(historical: OperationLogEntry[], streamed: OperationLogEntry[]): OperationLogEntry[] {
  const byId = new Map<number, OperationLogEntry>();
  for (const entry of historical) {
    byId.set(entry.id, entry);
  }
  for (const entry of streamed) {
    byId.set(entry.id, entry);
  }
  return Array.from(byId.values()).sort((a, b) => a.id - b.id);
}

export function OperationLogTimeline({
  agentHostId,
  targetId,
  scope = "core_operation",
  enabled = true,
}: OperationLogTimelineProps) {
  const { t } = useTranslation();
  const canLoad = enabled && Boolean(targetId);

  const historicalQuery = useQuery({
    queryKey: [...QUERY_KEYS.ADMIN_OPERATION_LOGS, scope, agentHostId, targetId],
    queryFn: () =>
      listOperationLogs({
        scope,
        target_id: targetId ?? "",
        agent_host_id: agentHostId,
        limit: LOG_LIMIT,
      }),
    enabled: canLoad,
  });

  const stream = useOperationLogStream({
    scope,
    targetId,
    agentHostId,
    enabled: canLoad,
    limit: LOG_LIMIT,
    maxEntries: LOG_LIMIT,
  });

  const logs = useMemo(
    () => mergeLogs(historicalQuery.data?.logs ?? [], stream.logs),
    [historicalQuery.data?.logs, stream.logs]
  );

  return (
    <Card className="border border-border shadow-none">
      <CardHeader className="pb-3">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <CardTitle className="text-base">{t("admin.cores.timelineTitle")}</CardTitle>
            <CardDescription>{t("admin.cores.timelineDescription")}</CardDescription>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant={stream.connected ? "success" : "secondary"}>
              {stream.connected ? t("admin.cores.timelineConnected") : t("admin.cores.timelineDisconnected")}
            </Badge>
            <Button
              size="sm"
              variant="outline"
              onClick={() => historicalQuery.refetch()}
              disabled={!canLoad || historicalQuery.isFetching}
            >
              <RefreshCw className="mr-2 h-3.5 w-3.5" />
              {t("common.refresh")}
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-3">
        {!targetId ? (
          <EmptyState
            icon={<History className="h-full w-full" />}
            title={t("admin.cores.timelineEmpty")}
            description={t("admin.cores.timelineEmptyDescription")}
            size="sm"
          />
        ) : historicalQuery.isLoading && logs.length === 0 ? (
          <Loading />
        ) : historicalQuery.error && logs.length === 0 ? (
          <div className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
            {t("admin.cores.timelineLoadError")}
          </div>
        ) : logs.length === 0 ? (
          <EmptyState
            icon={<History className="h-full w-full" />}
            title={t("admin.cores.timelineEmpty")}
            description={t("admin.cores.timelineEmptyDescription")}
            size="sm"
          />
        ) : (
          <div className="space-y-3">
            {stream.error && (
              <div className="rounded-md border border-warning/30 bg-warning/10 p-3 text-xs text-warning-foreground dark:text-warning">
                {t("admin.cores.timelineStreamError", { message: stream.error })}
              </div>
            )}
            {logs.map((entry) => {
              const payload = formatPayload(entry.payload);
              return (
                <div key={entry.id} className="rounded-md border border-border p-3">
                  <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                    <div className="flex flex-wrap items-center gap-2">
                      <Badge variant={getLevelVariant(entry.level)}>{t(`admin.cores.logLevel.${entry.level}`)}</Badge>
                      <span className="font-mono text-xs text-muted-foreground">#{entry.sequence}</span>
                      <span className="text-sm font-medium">{entry.phase || "-"}</span>
                    </div>
                    <span className="whitespace-nowrap text-xs text-muted-foreground">
                      {formatDateTime(entry.reported_at || entry.created_at)}
                    </span>
                  </div>
                  <div className="mt-2 text-sm text-foreground">{entry.message || "-"}</div>
                  {payload && (
                    <pre className="mt-2 max-h-48 overflow-auto rounded-md border border-border bg-muted/40 p-3 text-xs text-muted-foreground">
                      {payload}
                    </pre>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
