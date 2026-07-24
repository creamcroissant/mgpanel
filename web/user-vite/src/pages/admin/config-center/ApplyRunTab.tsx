import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { CheckCircle2, RefreshCw } from "lucide-react";
import { QUERY_KEYS } from "@/lib/constants";
import { getConfigCenterApplyRunDetail, listConfigCenterApplyRuns } from "@/api/admin";
import { formatDateTime } from "@/lib/format";
import type {
  ConfigCenterApplyRun,
  ConfigCenterApplyRunDetail,
  ConfigCenterApplyRunStatus,
  ConfigCenterCoreType,
  ConfigCenterSemanticDiffItem,
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
  ResponsiveList,
  ResponsiveListField,
  ResponsiveListItem,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
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
import { useIsMobileViewport } from "@/hooks";

export type CoreTypeOption = ConfigCenterCoreType;

interface ApplyRunTabProps {
  selectedHostId: number | null;
  selectedCoreType: CoreTypeOption;
  diffFilename: string;
  diffTag: string;
}



export function ApplyRunTab({
  selectedHostId,
  selectedCoreType,
  diffFilename,
  diffTag,
}: ApplyRunTabProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const isMobileViewport = useIsMobileViewport();
  const [selectedApplyRunId, setSelectedApplyRunId] = useState("");
  const [trackedApplyRunIds, setTrackedApplyRunIds] = useState<string[]>([]);
  const previousApplyStatusesRef = useRef<Record<string, ConfigCenterApplyRunStatus>>({});

  /* ---------- queries ---------- */

  const applyRunsQuery = useQuery({
    queryKey: [
      ...QUERY_KEYS.ADMIN_CONFIG_CENTER_APPLY_RUNS,
      selectedHostId,
      selectedCoreType,
    ],
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

  const applyRuns = useMemo(
    () => applyRunsQuery.data?.data ?? [],
    [applyRunsQuery.data?.data]
  );

  /* ---------- derived state ---------- */

  const resolvedSelectedApplyRunId = useMemo(() => {
    if (applyRuns.length === 0) return "";
    if (!selectedApplyRunId) return applyRuns[0].run_id;
    if (applyRuns.some((item) => item.run_id === selectedApplyRunId)) {
      return selectedApplyRunId;
    }
    return applyRuns[0].run_id;
  }, [applyRuns, selectedApplyRunId]);

  const selectedApplyRun = useMemo(
    () => applyRuns.find((item) => item.run_id === resolvedSelectedApplyRunId) ?? null,
    [applyRuns, resolvedSelectedApplyRunId]
  );

  const applyDetailQuery = useQuery({
    queryKey: [
      ...QUERY_KEYS.ADMIN_CONFIG_CENTER_APPLY_RUNS,
      "detail",
      resolvedSelectedApplyRunId,
      diffTag,
      diffFilename,
    ],
    queryFn: () =>
      getConfigCenterApplyRunDetail(resolvedSelectedApplyRunId, {
        include_text: Boolean((diffTag || "").trim() || (diffFilename || "").trim()),
        text_tag: (diffTag || "").trim() || undefined,
        text_file: (diffFilename || "").trim() || undefined,
      }),
    enabled:
      selectedHostId !== null && (resolvedSelectedApplyRunId || "").trim().length > 0,
  });

  const applyDetail = applyDetailQuery.data as ConfigCenterApplyRunDetail | undefined;

  const applySemanticItems = useMemo(
    () =>
      Array.isArray(applyDetail?.semantic_diff?.items)
        ? applyDetail.semantic_diff.items
        : [],
    [applyDetail]
  );

  /* ---------- status tracking effect ---------- */

  useEffect(() => {
    const previousStatuses = previousApplyStatusesRef.current;
    const nextStatuses: Record<string, ConfigCenterApplyRunStatus> = {};

    applyRuns.forEach((run) => {
      nextStatuses[run.run_id] = run.status;
      const wasTracked = trackedApplyRunIds.includes(run.run_id);
      const previousStatus = previousStatuses[run.run_id];
      if (!wasTracked || previousStatus === run.status) return;

      const enteredTerminal =
        isApplyRunTerminal(run.status) &&
        (!previousStatus || isApplyRunActive(previousStatus));
      if (!enteredTerminal) return;

      const messageKey = formatApplyRunTerminalMessageKey(run.status);
      if (run.status === "success") {
        toast.success(t(messageKey), { description: run.run_id });
      } else {
        toast.error(t(messageKey), {
          description: run.error_message || run.run_id,
        });
      }

      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.ADMIN_CONFIG_CENTER_APPLY_RUNS,
      });
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.ADMIN_CONFIG_CENTER_SNAPSHOT,
      });
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.ADMIN_CONFIG_CENTER_DRIFT,
      });
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.ADMIN_CONFIG_CENTER_RECOVER,
      });
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.ADMIN_CONFIG_CENTER_DIFF_SEMANTIC,
      });
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.ADMIN_CONFIG_CENTER_DIFF_TEXT,
      });
      setTrackedApplyRunIds((current) => current.filter((id) => id !== run.run_id));
    });

    previousApplyStatusesRef.current = nextStatuses;
  }, [applyRuns, queryClient, t, trackedApplyRunIds]);

  /* ---------- render helpers ---------- */

  const renderApplyRunStatusBadge = (status: ConfigCenterApplyRunStatus) => (
    <Badge variant={formatApplyStatusVariant(status)}>{status}</Badge>
  );

  const renderApplyRunActions = (run: ConfigCenterApplyRun, layout: "desktop" | "mobile") => (
    <Button
      className={layout === "mobile" ? "mt-4 w-full" : undefined}
      size={layout === "mobile" ? "default" : "sm"}
      variant="ghost"
      onClick={() => {
        setSelectedApplyRunId(run.run_id);
      }}
    >
      {t("common.view")}
    </Button>
  );

  const renderFieldDiffs = (item: ConfigCenterSemanticDiffItem) =>
    item.field_diffs && item.field_diffs.length > 0 ? (
      <div className="space-y-1 text-xs text-muted-foreground">
        {item.field_diffs.map((fieldDiff, fdIndex) => (
          <div key={`${fieldDiff.field}-${fdIndex}`} className="break-all">
            <span className="font-medium text-foreground">{fieldDiff.field}</span>
            <span className="mx-1">:</span>
            <span>{fieldDiff.desired}</span>
            <span className="mx-1">&rarr;</span>
            <span>{fieldDiff.applied}</span>
          </div>
        ))}
      </div>
    ) : (
      <span className="text-xs text-muted-foreground">-</span>
    );

  const renderSemanticDiffMobileList = (
    items: ConfigCenterSemanticDiffItem[],
    label: string,
    showFieldDiffs: boolean
  ) => (
    <ResponsiveList label={label}>
      {items.map((item, index) => (
        <ResponsiveListItem key={`${item.tag}-${index}`}>
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0 font-medium text-foreground break-all">
              {item.tag}
            </div>
            <Badge variant={formatDriftVariant(item.drift_type)}>
              {item.drift_type}
            </Badge>
          </div>
          <dl className="mt-4 grid grid-cols-2 gap-3">
            <ResponsiveListField
              label={t("admin.configCenter.fields.tag")}
              className="col-span-2"
            >
              <span className="break-all">{item.tag}</span>
            </ResponsiveListField>
            <ResponsiveListField label={t("admin.configCenter.fields.driftType")}>
              <Badge variant={formatDriftVariant(item.drift_type)}>
                {item.drift_type}
              </Badge>
            </ResponsiveListField>
            {showFieldDiffs ? (
              <ResponsiveListField
                label={t("admin.configCenter.fields.fieldDiffs")}
                className="col-span-2"
              >
                {renderFieldDiffs(item)}
              </ResponsiveListField>
            ) : null}
          </dl>
        </ResponsiveListItem>
      ))}
    </ResponsiveList>
  );

  /* ---------- main render ---------- */

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>{t("admin.configCenter.applyRuns.title")}</CardTitle>
          <CardDescription>
            {t("admin.configCenter.applyRuns.description")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-end">
            <div className="space-y-2">
              <label className="text-sm font-medium">
                {t("admin.configCenter.fields.applyRun")}
              </label>
              <Select
                value={resolvedSelectedApplyRunId || undefined}
                onValueChange={setSelectedApplyRunId}
              >
                <SelectTrigger>
                  <SelectValue
                    placeholder={t("admin.configCenter.placeholders.selectApplyRun")}
                  />
                </SelectTrigger>
                <SelectContent>
                  {applyRuns.map((run) => (
                    <SelectItem key={run.run_id} value={run.run_id}>
                      {`${run.run_id} · ${run.status} · r${run.target_revision}`}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <Button
              variant="outline"
              onClick={() => {
                applyRunsQuery.refetch();
                if (resolvedSelectedApplyRunId) {
                  applyDetailQuery.refetch();
                }
              }}
            >
              <RefreshCw className="mr-2 h-4 w-4" />
              {t("common.refresh")}
            </Button>
          </div>

          {applyRunsQuery.isLoading ? (
            <Loading />
          ) : applyRunsQuery.error ? (
            <div className="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
              {t("admin.configCenter.messages.applyRunsLoadFailed")}
              <div className="mt-1 text-xs opacity-80">
                {formatQueryErrorMessage(applyRunsQuery.error)}
              </div>
            </div>
          ) : applyRuns.length === 0 ? (
            <EmptyState
              icon={<CheckCircle2 className="h-full w-full" />}
              title={t("admin.configCenter.empty.noApplyRunTitle")}
              description={t("admin.configCenter.empty.noApplyRunDescription")}
              size="sm"
            />
          ) : (
            <>
              {isMobileViewport ? (
                <ResponsiveList label={t("admin.configCenter.applyRuns.title")}>
                  {applyRuns.map((run) => (
                    <ResponsiveListItem key={run.run_id}>
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0 space-y-1">
                          <div className="break-all font-medium text-foreground">
                            {run.run_id}
                          </div>
                          <div className="text-sm text-muted-foreground">
                            r{run.target_revision}
                          </div>
                        </div>
                        {renderApplyRunStatusBadge(run.status)}
                      </div>
                      <dl className="mt-4 grid grid-cols-2 gap-3">
                        <ResponsiveListField
                          label={t("admin.configCenter.fields.runId")}
                          className="col-span-2"
                        >
                          <span className="break-all">{run.run_id}</span>
                        </ResponsiveListField>
                        <ResponsiveListField
                          label={t("admin.configCenter.fields.status")}
                        >
                          {renderApplyRunStatusBadge(run.status)}
                        </ResponsiveListField>
                        <ResponsiveListField
                          label={t("admin.configCenter.fields.revision")}
                        >
                          {run.target_revision}
                        </ResponsiveListField>
                        <ResponsiveListField
                          label={t("admin.configCenter.fields.updatedAt")}
                          className="col-span-2"
                        >
                          {formatDateTime(run.finished_at || run.started_at)}
                        </ResponsiveListField>
                      </dl>
                      {renderApplyRunActions(run, "mobile")}
                    </ResponsiveListItem>
                  ))}
                </ResponsiveList>
              ) : (
                <Table
                  aria-label={
                    t("admin.configCenter.applyRuns.title") as string
                  }
                >
                  <TableHeader>
                    <TableRow>
                      <TableHead>
                        {t("admin.configCenter.fields.runId")}
                      </TableHead>
                      <TableHead>
                        {t("admin.configCenter.fields.status")}
                      </TableHead>
                      <TableHead>
                        {t("admin.configCenter.fields.revision")}
                      </TableHead>
                      <TableHead>
                        {t("admin.configCenter.fields.updatedAt")}
                      </TableHead>
                      <TableHead>{t("common.actions")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {applyRuns.map((run) => (
                      <TableRow key={run.run_id}>
                        <TableCell className="font-medium">
                          {run.run_id}
                        </TableCell>
                        <TableCell>
                          {renderApplyRunStatusBadge(run.status)}
                        </TableCell>
                        <TableCell>{run.target_revision}</TableCell>
                        <TableCell>
                          {formatDateTime(run.finished_at || run.started_at)}
                        </TableCell>
                        <TableCell>
                          {renderApplyRunActions(run, "desktop")}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}

              {selectedApplyRun ? (
                <div className="space-y-4 rounded-md border border-border p-4">
                  <div className="flex flex-wrap items-center gap-2">
                    <Badge
                      variant={formatApplyStatusVariant(selectedApplyRun.status)}
                    >
                      {selectedApplyRun.status}
                    </Badge>
                    <span className="text-sm text-muted-foreground">
                      {selectedApplyRun.run_id}
                    </span>
                    <span className="text-sm text-muted-foreground">
                      r{selectedApplyRun.previous_revision || 0} &rarr; r
                      {selectedApplyRun.target_revision}
                    </span>
                  </div>

                  {selectedApplyRun.error_message ? (
                    <div className="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
                      {selectedApplyRun.error_message}
                    </div>
                  ) : null}

                  {applyDetailQuery.isLoading ? (
                    <Loading />
                  ) : applyDetailQuery.error ? (
                    <div className="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
                      {t("admin.configCenter.messages.applyDetailLoadFailed")}
                      <div className="mt-1 text-xs opacity-80">
                        {formatQueryErrorMessage(applyDetailQuery.error)}
                      </div>
                    </div>
                  ) : applyDetail ? (
                    <div className="space-y-4">
                      {applyDetail.issues &&
                      applyDetail.issues.length > 0 ? (
                        <div className="space-y-2">
                          <p className="text-sm font-medium">
                            {t(
                              "admin.configCenter.applyRuns.issuesTitle"
                            )}
                          </p>
                          <div className="space-y-2">
                            {applyDetail.issues.map((item, index) => (
                              <div
                                key={`${item.code}-${index}`}
                                className="rounded-md border border-amber-500/30 bg-amber-500/5 p-3"
                              >
                                <div className="flex flex-wrap items-center gap-2">
                                  <Badge variant="warning">
                                    {item.code}
                                  </Badge>
                                </div>
                                <p className="mt-2 text-sm text-muted-foreground">
                                  {item.message}
                                </p>
                              </div>
                            ))}
                          </div>
                        </div>
                      ) : null}

                      <div className="grid gap-4 lg:grid-cols-2">
                        <Card>
                          <CardHeader>
                            <CardTitle className="text-base">
                              {t(
                                "admin.configCenter.applyRuns.semanticTitle"
                              )}
                            </CardTitle>
                            <CardDescription>
                              {t(
                                "admin.configCenter.applyRuns.semanticDescription"
                              )}
                            </CardDescription>
                          </CardHeader>
                          <CardContent>
                            {applySemanticItems.length > 0 ? (
                              isMobileViewport ? (
                                renderSemanticDiffMobileList(
                                  applySemanticItems,
                                  t(
                                    "admin.configCenter.applyRuns.semanticTitle"
                                  ),
                                  false
                                )
                              ) : (
                                <Table>
                                  <TableHeader>
                                    <TableRow>
                                      <TableHead>
                                        {t(
                                          "admin.configCenter.fields.tag"
                                        )}
                                      </TableHead>
                                      <TableHead>
                                        {t(
                                          "admin.configCenter.fields.driftType"
                                        )}
                                      </TableHead>
                                    </TableRow>
                                  </TableHeader>
                                  <TableBody>
                                    {applySemanticItems.map(
                                      (item, index) => (
                                        <TableRow
                                          key={`${item.tag}-${index}`}
                                        >
                                          <TableCell>
                                            {item.tag}
                                          </TableCell>
                                          <TableCell>
                                            <Badge
                                              variant={formatDriftVariant(
                                                item.drift_type
                                              )}
                                            >
                                              {item.drift_type}
                                            </Badge>
                                          </TableCell>
                                        </TableRow>
                                      )
                                    )}
                                  </TableBody>
                                </Table>
                              )
                            ) : (
                              <p className="text-sm text-muted-foreground">
                                {t(
                                  "admin.configCenter.empty.noApplySemanticDiff"
                                )}
                              </p>
                            )}
                          </CardContent>
                        </Card>

                        <Card>
                          <CardHeader>
                            <CardTitle className="text-base">
                              {t(
                                "admin.configCenter.applyRuns.textTitle"
                              )}
                            </CardTitle>
                            <CardDescription>
                              {t(
                                "admin.configCenter.applyRuns.textDescription"
                              )}
                            </CardDescription>
                          </CardHeader>
                          <CardContent>
                            {applyDetail.text_diff ? (
                              <div className="space-y-3">
                                <div className="flex flex-wrap gap-2">
                                  <Badge variant="secondary">
                                    {applyDetail.text_diff.filename ||
                                      "-"}
                                  </Badge>
                                  <Badge variant="secondary">
                                    {applyDetail.text_diff.tag || "-"}
                                  </Badge>
                                </div>
                                <pre className="max-h-80 overflow-auto whitespace-pre-wrap rounded-md border border-border p-3 text-xs">
                                  {applyDetail.text_diff.unified_diff ||
                                    "-"}
                                </pre>
                              </div>
                            ) : (
                              <p className="text-sm text-muted-foreground">
                                {(diffFilename || "").trim() ||
                                (diffTag || "").trim()
                                  ? t(
                                      "admin.configCenter.empty.noApplyTextDiff"
                                    )
                                  : t(
                                      "admin.configCenter.empty.selectApplyTextDiff"
                                    )}
                              </p>
                            )}
                          </CardContent>
                        </Card>
                      </div>
                    </div>
                  ) : null}
                </div>
              ) : null}
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

export default ApplyRunTab;
