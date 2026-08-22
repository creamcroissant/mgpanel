import { useMemo, useRef, useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { CheckCircle2, ChevronDown, ChevronRight, Diff, GitCompare, RefreshCw, ShieldAlert } from "lucide-react";
import { QUERY_KEYS } from "@/lib/constants";
import {
  getConfigCenterApplyRunDetail,
  listConfigCenterApplyRuns,
  listConfigCenterDriftStates,
  listConfigCenterAppliedSnapshot,
  listConfigCenterRecoveryStates,
} from "@/api/admin";
import { formatDateTime } from "@/lib/format";
import type {
  ConfigCenterApplyRunDetail,
  ConfigCenterApplyRunStatus,
  ConfigCenterInboundIndex,
  ConfigCenterInventory,
  ConfigCenterSemanticDiffItem,
  ConfigCenterSnapshot,
  ConfigCenterSource,
} from "@/types/configCenter";
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui";
import {
  formatApplyStatusVariant,
  formatDriftVariant,
  formatQueryErrorMessage,
  isApplyRunActive,
  isApplyRunTerminal,
  formatApplyRunTerminalMessageKey,
} from "./configCenterPageUtils";
import type { CoreTypeOption } from "./configCenterPageTypes";

interface DeploymentsTabProps {
  selectedHostId: number | null;
  selectedCoreType: CoreTypeOption;
}

export default function DeploymentsTab({ selectedHostId, selectedCoreType }: DeploymentsTabProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [expandedRunId, setExpandedRunId] = useState<string | null>(null);
  const [trackedApplyRunIds, setTrackedApplyRunIds] = useState<string[]>([]);
  const previousApplyStatusesRef = useRef<Record<string, ConfigCenterApplyRunStatus>>({});

  // ---- Queries ----

  const applyRunsQuery = useQuery({
    queryKey: [...QUERY_KEYS.ADMIN_CONFIG_CENTER_APPLY_RUNS, selectedHostId, selectedCoreType],
    queryFn: () =>
      listConfigCenterApplyRuns({
        agent_host_id: selectedHostId ?? 0,
        core_type: selectedCoreType,
        limit: 20,
        offset: 0,
      }),
    enabled: selectedHostId !== null,
    refetchInterval: (query) => {
      const runs = query.state.data?.data ?? [];
      return runs.some((run) => isApplyRunActive(run.status)) ? 3000 : false;
    },
  });

  const driftQuery = useQuery({
    queryKey: [...QUERY_KEYS.ADMIN_CONFIG_CENTER_DRIFT, selectedHostId, selectedCoreType],
    queryFn: () =>
      listConfigCenterDriftStates({ agent_host_id: selectedHostId ?? 0, core_type: selectedCoreType, limit: 200 }),
    enabled: selectedHostId !== null,
  });

  const recoveryQuery = useQuery({
    queryKey: [...QUERY_KEYS.ADMIN_CONFIG_CENTER_RECOVER, selectedHostId, selectedCoreType],
    queryFn: () =>
      listConfigCenterRecoveryStates({ agent_host_id: selectedHostId ?? 0, core_type: selectedCoreType, limit: 200 }),
    enabled: selectedHostId !== null,
  });

  const snapshotQuery = useQuery({
    queryKey: [...QUERY_KEYS.ADMIN_CONFIG_CENTER_SNAPSHOT, selectedHostId, selectedCoreType],
    queryFn: () =>
      listConfigCenterAppliedSnapshot({ agent_host_id: selectedHostId ?? 0, core_type: selectedCoreType, limit: 200 }),
    enabled: selectedHostId !== null,
  });

  const applyDetailQuery = useQuery({
    queryKey: [...QUERY_KEYS.ADMIN_CONFIG_CENTER_APPLY_RUNS, "detail", expandedRunId],
    queryFn: () => getConfigCenterApplyRunDetail(expandedRunId as string, {}),
    enabled: selectedHostId !== null && expandedRunId !== null && expandedRunId.length > 0,
  });

  // ---- Derived ----

  const allErrors = [applyRunsQuery.error, driftQuery.error, recoveryQuery.error, snapshotQuery.error].filter(Boolean);

  const applyRuns = useMemo(() => applyRunsQuery.data?.data ?? [], [applyRunsQuery.data?.data]);
  const driftStates = useMemo(() => driftQuery.data?.data ?? [], [driftQuery.data?.data]);
  const recoveryStates = useMemo(() => recoveryQuery.data?.data ?? [], [recoveryQuery.data?.data]);
  const snapshot = snapshotQuery.data;
  const snapshotInventories = useMemo(
    () => (Array.isArray((snapshot as ConfigCenterSnapshot)?.inventories) ? (snapshot as ConfigCenterSnapshot).inventories : []) as ConfigCenterInventory[],
    [snapshot],
  );
  const snapshotInboundIndexes = useMemo(
    () => (Array.isArray((snapshot as ConfigCenterSnapshot)?.inbound_indexes) ? (snapshot as ConfigCenterSnapshot).inbound_indexes : []) as ConfigCenterInboundIndex[],
    [snapshot],
  );

  const lastRun = useMemo(() => (applyRuns.length > 0 ? applyRuns[0] : null), [applyRuns]);
  const lastRunRevision = lastRun?.target_revision;
  const activeDriftCount = driftStates.length;

  // ---- Apply tracking effect ----
  useEffect(() => {
    const previousStatuses = previousApplyStatusesRef.current;
    const nextStatuses: Record<string, ConfigCenterApplyRunStatus> = {};
    applyRuns.forEach((run) => {
      nextStatuses[run.run_id] = run.status;
      const wasTracked = trackedApplyRunIds.includes(run.run_id);
      const previousStatus = previousStatuses[run.run_id];
      if (!wasTracked || previousStatus === run.status) return;
      const enteredTerminal =
        isApplyRunTerminal(run.status) && (!previousStatus || isApplyRunActive(previousStatus));
      if (!enteredTerminal) return;
      if (run.status === "success") {
        toast.success(t(formatApplyRunTerminalMessageKey(run.status)), { description: run.run_id });
      } else {
        toast.error(t(formatApplyRunTerminalMessageKey(run.status)), {
          description: run.error_message || run.run_id,
        });
      }
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_CONFIG_CENTER_SNAPSHOT });
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_CONFIG_CENTER_DRIFT });
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_CONFIG_CENTER_RECOVER });
      setTrackedApplyRunIds((current) => current.filter((id) => id !== run.run_id));
    });
    previousApplyStatusesRef.current = nextStatuses;
  }, [applyRuns, queryClient, t, trackedApplyRunIds]);

  // ---- Render helpers ----

  const renderFieldDiffs = (item: ConfigCenterSemanticDiffItem) =>
    item.field_diffs && item.field_diffs.length > 0 ? (
      <div className="space-y-1 text-xs text-muted-foreground">
        {item.field_diffs.map((fd, fi) => (
          <div key={`${fd.field}-${fi}`} className="break-all">
            <span className="font-medium text-foreground">{fd.field}</span>
            <span className="mx-1">:</span>
            <span>{fd.desired}</span>
            <span className="mx-1">&rarr;</span>
            <span>{fd.applied}</span>
          </div>
        ))}
      </div>
    ) : (
      <span className="text-xs text-muted-foreground">-</span>
    );

  // ---- Main Render ----

  const isLoading = applyRunsQuery.isLoading || driftQuery.isLoading || snapshotQuery.isLoading;

  return (
    <div className="space-y-4">
      {/* Status Bar */}
      {selectedHostId && !isLoading && (
        <Card>
          <CardContent className="p-4">
            <div className="flex flex-wrap items-center gap-4 text-sm">
              {lastRunRevision != null && (
                <span>
                  {t("admin.configCenter.deployments.current")}: <strong className="text-foreground">r{lastRunRevision}</strong>
                </span>
              )}
              <span className="flex items-center gap-1">
                <ShieldAlert className={`h-3.5 w-3.5 ${activeDriftCount > 0 ? "text-amber-500" : "text-emerald-500"}`} />
                {t("admin.configCenter.deployments.driftCount")}: <strong className={activeDriftCount > 0 ? "text-amber-500" : "text-emerald-500"}>{activeDriftCount}</strong>
              </span>
              {lastRun && (
                <span className="flex items-center gap-1">
                  {t("admin.configCenter.deployments.lastApply")}:
                  <Badge variant={formatApplyStatusVariant(lastRun.status)} className="ml-1">{lastRun.status}</Badge>
                  <span className="text-muted-foreground">· {formatDateTime(lastRun.finished_at || lastRun.started_at)}</span>
                </span>
              )}
              {recoveryStates.length > 0 && (
                <span className="text-muted-foreground">
                  · {t("admin.configCenter.deployments.recovered")}: {recoveryStates.length}
                </span>
              )}
              <Button variant="ghost" size="sm" className="ml-auto" onClick={() => applyRunsQuery.refetch()}>
                <RefreshCw className={`mr-1 h-3.5 w-3.5 ${applyRunsQuery.isFetching ? "animate-spin" : ""}`} />
                {t("common.refresh")}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {allErrors.length > 0 && (
        <div className="bg-red-50 border border-red-200 rounded-md p-4 mb-4">
          <p className="text-red-800 text-sm font-medium">{t("admin.configCenter.messages.loadFailed") ?? "数据加载失败"}</p>
          {allErrors.map((err, i) => (
            <p key={i} className="text-red-600 text-xs mt-1">{(err as Error)?.message || "未知错误"}</p>
          ))}
          <button className="text-sm text-blue-600 mt-2 hover:underline" onClick={() => {
            applyRunsQuery.refetch();
            driftQuery.refetch();
            recoveryQuery.refetch();
            snapshotQuery.refetch();
          }}>{t("common.retry") ?? "重试"}</button>
        </div>
      )}

      {isLoading ? (
        <Loading />
      ) : !selectedHostId ? (
        <div className="rounded-md border border-dashed border-border p-10 text-center text-sm text-muted-foreground">
          {t("admin.configCenter.placeholders.selectHost")}
        </div>
      ) : (
        <>
          {/* Apply Timeline */}
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base">{t("admin.configCenter.deployments.timeline")}</CardTitle>
              <CardDescription>
                {applyRuns.length > 0
                  ? t("admin.configCenter.deployments.lastRuns", { count: applyRuns.length })
                  : t("admin.configCenter.deployments.noRuns")}
              </CardDescription>
            </CardHeader>
            <CardContent>
              {applyRuns.length === 0 ? (
                <EmptyState
                  icon={<CheckCircle2 className="h-full w-full" />}
                  title={t("admin.configCenter.empty.noApplyRunTitle")}
                  description={t("admin.configCenter.empty.noApplyRunDescription")}
                  size="sm"
                />
              ) : (
                <div className="space-y-2">
                  {applyRuns.map((run) => {
                    const isExpanded = expandedRunId === run.run_id;
                    return (
                      <div key={run.run_id} className="rounded-md border border-border">
                        {/* Run header (always visible) */}
                        <div
                          className="flex cursor-pointer items-center gap-3 p-3 transition-colors hover:bg-muted/30"
                          onClick={() => setExpandedRunId(isExpanded ? null : run.run_id)}
                          role="button"
                          tabIndex={0}
                        >
                          <div className="shrink-0">
                            {isExpanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                          </div>
                          <span className="min-w-[3rem] font-medium">r{run.target_revision}</span>
                          <Badge variant={formatApplyStatusVariant(run.status)} className="shrink-0">{run.status}</Badge>
                          <span className="text-xs text-muted-foreground">{run.run_id}</span>
                          {run.previous_revision > 0 && (
                            <span className="text-xs text-muted-foreground">
                              r{run.previous_revision} &rarr; r{run.target_revision}
                            </span>
                          )}
                          <span className="ml-auto text-xs text-muted-foreground">
                            {formatDateTime(run.finished_at || run.started_at)}
                          </span>
                        </div>

                        {/* Expanded detail */}
                        {isExpanded && (
                          <div className="border-t border-border p-3">
                            {run.error_message && (
                              <div className="mb-3 rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
                                {run.error_message}
                              </div>
                            )}

                            {applyDetailQuery.isLoading && expandedRunId === run.run_id ? (
                              <Loading />
                            ) : applyDetailQuery.error && expandedRunId === run.run_id ? (
                              <p className="text-sm text-destructive">
                                {t("admin.configCenter.messages.applyDetailLoadFailed")}
                                <span className="ml-2 text-xs text-muted-foreground">
                                  {formatQueryErrorMessage(applyDetailQuery.error)}
                                </span>
                              </p>
                            ) : null}

                            {applyDetailQuery.data && expandedRunId === run.run_id && (() => {
                              const detail = applyDetailQuery.data as ConfigCenterApplyRunDetail;
                              return (
                                <div className="space-y-4">
                                {/* Issues */}
                                {detail.issues && detail.issues.length > 0 && (
                                    <div className="space-y-2">
                                      {detail.issues.map((item, idx) => (
                                        <div key={`${item.code}-${idx}`} className="rounded-md border border-amber-500/30 bg-amber-500/5 p-3">
                                          <Badge variant="warning">{item.code}</Badge>
                                          <p className="mt-1 text-sm text-muted-foreground">{item.message}</p>
                                        </div>
                                      ))}
                                    </div>
                                  )}

                                {/* Semantic diff */}
                                <div>
                                  <h4 className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                                    {t("admin.configCenter.applyRuns.semanticTitle")}
                                  </h4>
                                  {(() => {
                                    const items = Array.isArray(
                                      (applyDetailQuery.data as ConfigCenterApplyRunDetail).semantic_diff?.items
                                    )
                                      ? (applyDetailQuery.data as ConfigCenterApplyRunDetail).semantic_diff!.items
                                      : [];
                                    return items.length === 0 ? (
                                      <p className="text-sm text-muted-foreground">{t("admin.configCenter.empty.noApplySemanticDiff")}</p>
                                    ) : (
                                      <Table>
                                        <TableHeader>
                                          <TableRow>
                                            <TableHead>{t("admin.configCenter.fields.tag")}</TableHead>
                                            <TableHead>{t("admin.configCenter.fields.driftType")}</TableHead>
                                            <TableHead>{t("admin.configCenter.fields.fieldDiffs")}</TableHead>
                                          </TableRow>
                                        </TableHeader>
                                        <TableBody>
                                          {items.map((item, idx) => (
                                            <TableRow key={`${item.tag}-${idx}`}>
                                              <TableCell>{item.tag}</TableCell>
                                              <TableCell>
                                                <Badge variant={formatDriftVariant(item.drift_type)}>{item.drift_type}</Badge>
                                              </TableCell>
                                              <TableCell>{renderFieldDiffs(item)}</TableCell>
                                            </TableRow>
                                          ))}
                                        </TableBody>
                                      </Table>
                                    );
                                  })()}
                                </div>

                                {/* Text diff */}
                                <div>
                                  <h4 className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                                    {t("admin.configCenter.applyRuns.textTitle")}
                                  </h4>
                                  {(applyDetailQuery.data as ConfigCenterApplyRunDetail).text_diff ? (
                                    <div className="space-y-2">
                                      <div className="flex flex-wrap gap-2">
                                        {(applyDetailQuery.data as ConfigCenterApplyRunDetail).text_diff!.filename && (
                                          <Badge variant="secondary">{(applyDetailQuery.data as ConfigCenterApplyRunDetail).text_diff!.filename}</Badge>
                                        )}
                                        {(applyDetailQuery.data as ConfigCenterApplyRunDetail).text_diff!.tag && (
                                          <Badge variant="secondary">{(applyDetailQuery.data as ConfigCenterApplyRunDetail).text_diff!.tag}</Badge>
                                        )}
                                      </div>
                                      <pre className="max-h-60 overflow-auto whitespace-pre-wrap rounded-md border border-border p-3 text-xs">
                                        {(applyDetailQuery.data as ConfigCenterApplyRunDetail).text_diff!.unified_diff || "-"}
                                      </pre>
                                    </div>
                                  ) : (
                                    <p className="text-sm text-muted-foreground">{t("admin.configCenter.empty.noApplyTextDiff")}</p>
                                  )}
                                </div>
                              </div>
                              );
                            })()}
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              )}
            </CardContent>
          </Card>

          {/* Applied Snapshot */}
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base">{t("admin.configCenter.snapshot.title")}</CardTitle>
              <CardDescription>{t("admin.configCenter.snapshot.description")}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              {/* Files */}
              <div>
                <h4 className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                  {t("admin.configCenter.snapshot.inventoryTitle")}
                </h4>
                {snapshotInventories.length === 0 ? (
                  <EmptyState
                    icon={<Diff className="h-full w-full" />}
                    title={t("admin.configCenter.empty.noInventoryTitle")}
                    description={t("admin.configCenter.empty.noInventoryDescription")}
                    size="sm"
                  />
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t("admin.configCenter.fields.source")}</TableHead>
                        <TableHead>{t("admin.configCenter.fields.filename")}</TableHead>
                        <TableHead>{t("admin.configCenter.fields.parseStatus")}</TableHead>
                        <TableHead>{t("admin.configCenter.fields.lastSeenAt")}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {snapshotInventories.map((item) => (
                        <TableRow key={item.id}>
                          <TableCell>
                            {/* TODO: define proper typing for dynamic translation keys */}
                            <Badge variant="secondary">{t(`admin.configCenter.source.${item.source}` as `admin.configCenter.source.${ConfigCenterSource}`)}</Badge>
                          </TableCell>
                          <TableCell className="font-mono text-xs">{item.filename}</TableCell>
                          <TableCell>
                            <Badge variant={item.parse_status === "ok" ? "success" : "warning"}>{item.parse_status}</Badge>
                          </TableCell>
                          <TableCell className="text-xs text-muted-foreground">{formatDateTime(item.last_seen_at)}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </div>

              {/* Inbound Indexes */}
              <div>
                <h4 className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                  {t("admin.configCenter.snapshot.inboundTitle")}
                </h4>
                {snapshotInboundIndexes.length === 0 ? (
                  <EmptyState
                    icon={<GitCompare className="h-full w-full" />}
                    title={t("admin.configCenter.empty.noInboundIndexTitle")}
                    description={t("admin.configCenter.empty.noInboundIndexDescription")}
                    size="sm"
                  />
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t("admin.configCenter.fields.source")}</TableHead>
                        <TableHead>{t("admin.configCenter.fields.tag")}</TableHead>
                        <TableHead>{t("admin.configCenter.fields.protocol")}</TableHead>
                        <TableHead>{t("admin.configCenter.fields.listen")}</TableHead>
                        <TableHead>{t("admin.configCenter.fields.port")}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {snapshotInboundIndexes.map((item) => (
                        <TableRow key={item.id}>
                          <TableCell>
                            <Badge variant="secondary">{t(`admin.configCenter.source.${item.source}` as `admin.configCenter.source.${ConfigCenterSource}`)}</Badge>
                          </TableCell>
                          <TableCell className="font-mono text-xs">{item.tag}</TableCell>
                          <TableCell>{item.protocol || "-"}</TableCell>
                          <TableCell>{item.listen || "-"}</TableCell>
                          <TableCell>{item.port ?? "-"}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </div>
            </CardContent>
          </Card>
        </>
      )}
    </div>
  );
}
