import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, CheckCircle2, Clock, RefreshCw, RotateCcw, ShieldAlert } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import {
  createAgentUpdateCheckOperation,
  createAgentUpdateOperation,
  listAgentLifecycleOperations,
} from "@/api/admin";
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
  Input,
  Loading,
} from "@/components/ui";
import type { AgentLifecycleOperation, AgentLifecycleUpdateRequest } from "@/types";
import { OperationLogTimeline } from "./OperationLogTimeline";

interface AgentUpdatePanelProps {
  agentHostId: number;
}

type BadgeTone = "success" | "warning" | "danger" | "secondary" | "outline";

type UpdateForm = {
  target_version: string;
  release_tag: string;
};

const ACTIVE_STATUSES = new Set(["pending", "claimed", "in_progress"]);
const UPDATE_OPERATION_TYPES = new Set(["agent_update", "agent_update_check"]);
const DEFAULT_FORM: UpdateForm = {
  target_version: "",
  release_tag: "",
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function getString(record: Record<string, unknown>, key: string): string {
  const value = record[key];
  return typeof value === "string" ? value : "";
}

function getNumber(record: Record<string, unknown>, key: string): number | undefined {
  const value = record[key];
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function getBoolean(record: Record<string, unknown>, key: string): boolean {
  return record[key] === true;
}

function getStatusVariant(status: string): BadgeTone {
  switch (status) {
    case "success":
    case "completed":
      return "success";
    case "pending":
    case "claimed":
    case "in_progress":
      return "warning";
    case "failed":
    case "timeout":
    case "cancelled":
    case "unsupported_action":
    case "queue_full":
      return "danger";
    default:
      return "secondary";
  }
}

function isActiveOperation(operation: AgentLifecycleOperation): boolean {
  return ACTIVE_STATUSES.has(operation.status);
}

function getResultPayload(operation: AgentLifecycleOperation | null): Record<string, unknown> {
  return isRecord(operation?.result_payload) ? operation.result_payload : {};
}

function getRequestPayload(operation: AgentLifecycleOperation | null): Record<string, unknown> {
  return isRecord(operation?.request_payload) ? operation.request_payload : {};
}

function buildUpdateRequest(form: UpdateForm): AgentLifecycleUpdateRequest {
  const payload: AgentLifecycleUpdateRequest = {};
  const targetVersion = form.target_version.trim();
  const releaseTag = form.release_tag.trim();
  if (targetVersion) {
    payload.target_version = targetVersion;
  }
  if (releaseTag) {
    payload.release_tag = releaseTag;
  }
  return payload;
}

function formatBoolean(value: boolean, yes: string, no: string): string {
  return value ? yes : no;
}

function DetailItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border border-border bg-muted/20 p-3">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 min-h-5 break-all text-sm font-medium text-foreground">{value || "-"}</div>
    </div>
  );
}

export function AgentUpdatePanel({ agentHostId }: AgentUpdatePanelProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [form, setForm] = useState<UpdateForm>(DEFAULT_FORM);
  const [selectedOperationId, setSelectedOperationId] = useState<string | null>(null);

  const operationsQuery = useQuery({
    queryKey: [...QUERY_KEYS.ADMIN_AGENT_LIFECYCLE_OPERATIONS, agentHostId, "agent-update"],
    queryFn: () => listAgentLifecycleOperations(agentHostId, { limit: 10 }),
    refetchInterval: (query) => {
      const operations = query.state.data?.operations ?? [];
      return operations.some((operation) => UPDATE_OPERATION_TYPES.has(operation.operation_type) && isActiveOperation(operation))
        ? 3000
        : false;
    },
  });

  const updateOperations = useMemo(
    () => (operationsQuery.data?.operations ?? []).filter((operation) => UPDATE_OPERATION_TYPES.has(operation.operation_type)),
    [operationsQuery.data?.operations]
  );
  const selectedOperation =
    updateOperations.find((operation) => operation.id === selectedOperationId) ?? updateOperations[0] ?? null;
  const latestPayload = getResultPayload(selectedOperation);
  const latestRequest = getRequestPayload(selectedOperation);
  const hasActiveUpdate = updateOperations.some(isActiveOperation);

  const currentVersion = getString(latestPayload, "current_version") || getString(latestRequest, "current_version");
  const targetVersion = getString(latestPayload, "target_version") || getString(latestRequest, "target_version");
  const previousVersion = getString(latestPayload, "previous_version");
  const releaseTag = getString(latestPayload, "release_tag") || getString(latestRequest, "release_tag");
  const healthDeadlineAt = getNumber(latestPayload, "health_deadline_at");
  const rollbackAvailable = getBoolean(latestPayload, "rollback_available");
  const rolledBack = getBoolean(latestPayload, "rolled_back");
  const lockedBadVersion = getString(latestPayload, "locked_bad_version");
  const crashCount = getNumber(latestPayload, "crash_count") ?? 0;
  const compatible = getBoolean(latestPayload, "compatible");
  const upToDate = getBoolean(latestPayload, "up_to_date");
  const locked = getBoolean(latestPayload, "locked");

  const invalidateLifecycle = () => {
    queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_AGENT_LIFECYCLE_OPERATIONS });
    queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_OPERATION_LOGS });
  };

  const updateCheckMutation = useMutation({
    mutationFn: () => createAgentUpdateCheckOperation(agentHostId, buildUpdateRequest(form)),
    onSuccess: (operation) => {
      setSelectedOperationId(operation.id);
      invalidateLifecycle();
      toast.success(t("admin.cores.updateCheckSubmitted"));
    },
    onError: (error: Error) => {
      toast.error(t("admin.cores.updateCheckError"), { description: error.message });
    },
  });

  const updateMutation = useMutation({
    mutationFn: () => createAgentUpdateOperation(agentHostId, buildUpdateRequest(form)),
    onSuccess: (operation) => {
      setSelectedOperationId(operation.id);
      invalidateLifecycle();
      toast.success(t("admin.cores.updateSubmitted"));
    },
    onError: (error: Error) => {
      toast.error(t("admin.cores.updateError"), { description: error.message });
    },
  });

  return (
    <Card className="border border-border shadow-none" data-testid="agent-update-panel">
      <CardHeader className="pb-3">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <CardTitle className="text-sm">{t("admin.cores.updatePanelTitle")}</CardTitle>
            <CardDescription>{t("admin.cores.updatePanelDescription")}</CardDescription>
          </div>
          <Button size="sm" variant="outline" onClick={() => operationsQuery.refetch()} disabled={operationsQuery.isFetching}>
            <RefreshCw className="mr-2 h-3.5 w-3.5" />
            {t("common.refresh")}
          </Button>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {hasActiveUpdate && (
          <div className="rounded-md border border-warning/30 bg-warning/10 p-3 text-sm text-warning-foreground dark:text-warning">
            {t("admin.cores.updateActiveDescription")}
          </div>
        )}

        {operationsQuery.isLoading ? (
          <Loading />
        ) : updateOperations.length === 0 ? (
          <EmptyState
            icon={<RefreshCw className="h-full w-full" />}
            title={t("admin.cores.updateEmpty")}
            description={t("admin.cores.updateEmptyDescription")}
            size="sm"
          />
        ) : (
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <DetailItem label={t("admin.cores.updateCurrentVersion")} value={currentVersion} />
            <DetailItem label={t("admin.cores.updateTargetVersion")} value={targetVersion} />
            <DetailItem label={t("admin.cores.updatePreviousVersion")} value={previousVersion} />
            <DetailItem label={t("admin.cores.updateReleaseTag")} value={releaseTag} />
          </div>
        )}

        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          <div className="rounded-md border border-border p-3">
            <div className="flex items-center gap-2 text-sm font-medium">
              <CheckCircle2 className="h-4 w-4 text-success" />
              {t("admin.cores.updateCheckResult")}
            </div>
            <div className="mt-2 flex flex-wrap gap-2">
              <Badge variant={compatible ? "success" : "secondary"}>
                {formatBoolean(compatible, t("admin.cores.updateCompatible"), t("admin.cores.updateCompatibilityUnknown"))}
              </Badge>
              <Badge variant={upToDate ? "success" : "warning"}>
                {formatBoolean(upToDate, t("admin.cores.updateUpToDate"), t("admin.cores.updateCanUpgrade"))}
              </Badge>
              {locked && <Badge variant="danger">{t("admin.cores.updateLocked")}</Badge>}
            </div>
          </div>

          <div className="rounded-md border border-border p-3">
            <div className="flex items-center gap-2 text-sm font-medium">
              <Clock className="h-4 w-4 text-primary" />
              {t("admin.cores.updateHealthConfirmation")}
            </div>
            <div className="mt-2 text-sm text-muted-foreground">
              {healthDeadlineAt ? formatDateTime(healthDeadlineAt) : t("admin.cores.updateHealthIdle")}
            </div>
          </div>

          <div className="rounded-md border border-border p-3">
            <div className="flex items-center gap-2 text-sm font-medium">
              <RotateCcw className="h-4 w-4 text-warning" />
              {t("admin.cores.updateRollback")}
            </div>
            <div className="mt-2 flex flex-wrap gap-2">
              <Badge variant={rollbackAvailable ? "warning" : "secondary"}>
                {rollbackAvailable ? t("admin.cores.updateRollbackAvailable") : t("admin.cores.updateRollbackUnavailable")}
              </Badge>
              {rolledBack && <Badge variant="danger">{t("admin.cores.updateRolledBack")}</Badge>}
            </div>
          </div>

          <div className="rounded-md border border-border p-3">
            <div className="flex items-center gap-2 text-sm font-medium">
              <ShieldAlert className="h-4 w-4 text-destructive" />
              {t("admin.cores.updateSafetyState")}
            </div>
            <div className="mt-2 space-y-1 text-sm text-muted-foreground">
              <div>{t("admin.cores.updateCrashCount", { count: crashCount })}</div>
              {lockedBadVersion ? (
                <Badge variant="danger">{t("admin.cores.updateLockedBadVersion", { version: lockedBadVersion })}</Badge>
              ) : (
                <span>{t("admin.cores.updateNoLockedBadVersion")}</span>
              )}
            </div>
          </div>
        </div>

        <div className="rounded-md border border-border p-3">
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.cores.updateTargetVersion")}</label>
              <Input
                value={form.target_version}
                onChange={(event) => setForm((current) => ({ ...current, target_version: event.target.value }))}
                placeholder={t("admin.cores.updateTargetPlaceholder")}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.cores.updateReleaseTag")}</label>
              <Input
                value={form.release_tag}
                onChange={(event) => setForm((current) => ({ ...current, release_tag: event.target.value }))}
                placeholder={t("admin.cores.updateReleasePlaceholder")}
              />
            </div>
          </div>
          <div className="mt-3 flex flex-wrap gap-2">
            <Button
              size="sm"
              variant="outline"
              disabled={hasActiveUpdate || updateCheckMutation.isPending || updateMutation.isPending}
              onClick={() => updateCheckMutation.mutate()}
            >
              {updateCheckMutation.isPending ? t("common.loading") : t("admin.cores.updateCheckAction")}
            </Button>
            <Button
              size="sm"
              disabled={hasActiveUpdate || updateCheckMutation.isPending || updateMutation.isPending}
              onClick={() => updateMutation.mutate()}
            >
              {updateMutation.isPending ? t("common.loading") : t("admin.cores.updateAction")}
            </Button>
          </div>
        </div>

        {updateOperations.length > 0 && (
          <div className="space-y-2">
            <div className="text-sm font-medium">{t("admin.cores.updateRecentOperations")}</div>
            {updateOperations.slice(0, 5).map((operation) => (
              <button
                key={operation.id}
                type="button"
                className={`flex w-full flex-col gap-2 rounded-md border p-3 text-left transition-colors sm:flex-row sm:items-center sm:justify-between ${
                  selectedOperation?.id === operation.id ? "border-primary bg-primary/5" : "border-border hover:bg-muted/50"
                }`}
                onClick={() => setSelectedOperationId(operation.id)}
              >
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant={getStatusVariant(operation.status)}>
                    {t(`admin.cores.status.${operation.status}`, { defaultValue: operation.status })}
                  </Badge>
                  <span className="text-sm font-medium">
                    {t(`admin.cores.operationType.${operation.operation_type}`, { defaultValue: operation.operation_type })}
                  </span>
                </div>
                <div className="text-xs text-muted-foreground">
                  {operation.error_message || formatDateTime(operation.created_at)}
                </div>
              </button>
            ))}
          </div>
        )}

        {selectedOperation?.error_message && (
          <div className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
            <div className="flex items-center gap-2 font-medium">
              <AlertTriangle className="h-4 w-4" />
              {t("admin.cores.updateErrorMessage")}
            </div>
            <div className="mt-1 break-all">{selectedOperation.error_message}</div>
          </div>
        )}

        <OperationLogTimeline
          agentHostId={agentHostId}
          targetId={selectedOperation?.id}
          scope="agent_operation"
          enabled={Boolean(selectedOperation?.id)}
        />
      </CardContent>
    </Card>
  );
}
