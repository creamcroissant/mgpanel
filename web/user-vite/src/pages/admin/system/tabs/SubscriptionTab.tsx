import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  createSubscriptionSource,
  deleteSubscriptionSource,
  getSubscriptionFilterSummary,
  listSubscriptionFilterReasons,
  listSubscriptionSources,
  syncSubscriptionSource,
  updateSubscriptionSource,
} from "@/api/admin/subscription";
import { fetchSettings, saveSettings } from "@/api/admin/settings";
import { QUERY_KEYS } from "@/lib/constants";
import { formatDateTime } from "@/lib/format";
import type {
  SubscriptionFilterReasonEntry,
  SubscriptionSource,
  SubscriptionSourceType,
  UpsertSubscriptionSourceRequest,
} from "@/types/admin";
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Switch,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Textarea,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui";
import ErrorBanner from "@/components/ui/ErrorBanner";

const CATEGORY = "subscription";
const SOURCE_LIST_LIMIT = 100;
const FILTER_REASON_LIMIT = 8;
const SOURCE_TYPES: SubscriptionSourceType[] = [
  "self_hosted",
  "imported_subscription",
  "custom_node",
];

interface SubscriptionForm {
  subscribeUrls: string;
  subscribePath: string;
  trafficResetMode: string;
  trafficResetDay: string;
  allowChange: boolean;
  injectMeta: boolean;
  injectProtocol: boolean;
}

interface SourceFormState {
  type: SubscriptionSourceType;
  name: string;
  url: string;
  content: string;
  enabled: boolean;
}

type SubscriptionTabContentProps = {
  initialForm: SubscriptionForm;
};

type SourceSaveArgs = {
  id?: number;
  payload: UpsertSubscriptionSourceRequest;
};

function toBool(value?: string) {
  return value === "true" || value === "1";
}

function isKnownSourceType(value: string): value is SubscriptionSourceType {
  return SOURCE_TYPES.includes(value as SubscriptionSourceType);
}

function toSourceType(value: string): SubscriptionSourceType {
  return isKnownSourceType(value) ? value : "imported_subscription";
}

function createEmptySourceForm(): SourceFormState {
  return {
    type: "imported_subscription",
    name: "",
    url: "",
    content: "",
    enabled: true,
  };
}

function sourceToForm(source: SubscriptionSource): SourceFormState {
  return {
    type: toSourceType(source.type),
    name: source.name,
    url: source.url ?? "",
    content: source.content ?? "",
    enabled: source.enabled,
  };
}

function buildSourcePayload(form: SourceFormState): UpsertSubscriptionSourceRequest {
  const type = form.type;
  return {
    type,
    name: form.name.trim(),
    url: type === "imported_subscription" ? form.url.trim() : undefined,
    content: type === "custom_node" ? form.content.trim() : undefined,
    enabled: form.enabled,
  };
}

function maskSensitiveURL(value?: string): string {
  const trimmed = value?.trim();
  if (!trimmed) {
    return "-";
  }
  try {
    const parsed = new URL(trimmed);
    const path = parsed.pathname && parsed.pathname !== "/" ? "/…" : "";
    return `${parsed.protocol}//${parsed.host}${path}`;
  } catch {
    if (trimmed.length <= 10) {
      return "••••";
    }
    return `${trimmed.slice(0, 6)}…${trimmed.slice(-4)}`;
  }
}

function getSourceMaterial(source: SubscriptionSource, t: (key: string) => string): string {
  if (source.type === "imported_subscription") {
    return source.url ? maskSensitiveURL(source.url) : t("admin.system.subscription.sourceNoMaterial");
  }
  if (source.type === "custom_node") {
    const length = source.content?.trim().length ?? 0;
    return length > 0
      ? t("admin.system.subscription.sourceContentSummary").replace("{{count}}", String(length))
      : t("admin.system.subscription.sourceNoMaterial");
  }
  return t("admin.system.subscription.sourceSelfHostedMaterial");
}

function getSourceTypeLabel(type: string, t: (key: string) => string): string {
  return t(`admin.system.subscription.sourceTypeOptions.${type}`);
}

function getFilterReasonLabel(reason: string, t: (key: string) => string): string {
  return t(`admin.system.subscription.filterReasonOptions.${reason}`);
}

function MetricTile({ label, value }: { label: string; value: number | string }) {
  return (
    <div className="rounded-md border border-border bg-muted/20 p-3">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="mt-1 text-xl font-semibold tabular-nums text-foreground">{value}</p>
    </div>
  );
}

function SubscriptionTabContent({ initialForm }: SubscriptionTabContentProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [form, setForm] = useState<SubscriptionForm>(initialForm);
  const [pendingPath, setPendingPath] = useState("");
  const [confirmPathOpen, setConfirmPathOpen] = useState(false);

  const queryKey = useMemo(() => [...QUERY_KEYS.ADMIN_SYSTEM, CATEGORY], []);

  const saveMutation = useMutation({
    mutationFn: (payload: SubscriptionForm) =>
      saveSettings(CATEGORY, {
        subscribe_urls: payload.subscribeUrls.trim(),
        subscribe_path: payload.subscribePath.trim(),
        traffic_reset_mode: payload.trafficResetMode,
        traffic_reset_day: payload.trafficResetDay,
        allow_change: payload.allowChange ? "true" : "false",
        inject_meta: payload.injectMeta ? "true" : "false",
        inject_protocol: payload.injectProtocol ? "true" : "false",
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
      toast.success(t("common.success"), {
        description: t("admin.system.settings.messages.saveSuccess"),
      });
    },
    onError: (err: Error) => {
      toast.error(t("common.error"), {
        description: err.message,
      });
    },
  });

  const handleSave = () => {
    saveMutation.mutate(form);
  };

  const handlePathChange = (value: string) => {
    if (value.trim() === form.subscribePath.trim()) {
      return;
    }
    setPendingPath(value);
    setConfirmPathOpen(true);
  };

  const confirmPathChange = () => {
    setForm((prev) => ({ ...prev, subscribePath: pendingPath }));
    setPendingPath("");
    setConfirmPathOpen(false);
  };

  const cancelPathChange = () => {
    setPendingPath("");
    setConfirmPathOpen(false);
  };

  return (
    <>
      <div className="space-y-6 max-w-2xl">
        <div className="space-y-2">
          <label className="text-sm font-medium">
            {t("admin.system.settings.fields.subscribeUrls")}
          </label>
          <Textarea
            value={form.subscribeUrls}
            onChange={(e) => setForm((prev) => ({ ...prev, subscribeUrls: e.target.value }))}
            placeholder={t("admin.system.settings.placeholders.subscribeUrls")}
          />
          <p className="text-xs text-muted-foreground">
            {t("admin.system.settings.tooltips.subscribeUrls")}
          </p>
        </div>

        <div className="space-y-2">
          <label className="text-sm font-medium">
            {t("admin.system.settings.fields.subscribePath")}
          </label>
          <Input
            value={form.subscribePath}
            onChange={(e) => handlePathChange(e.target.value)}
            placeholder={t("admin.system.settings.placeholders.subscribePath")}
          />
          <p className="text-xs text-muted-foreground">
            {t("admin.system.settings.tooltips.subscribePath")}
          </p>
        </div>

        <div className="space-y-2">
          <label className="text-sm font-medium">
            {t("admin.system.subscription.traffic_reset_mode")}
          </label>
          <Select
            value={form.trafficResetMode}
            onValueChange={(value) => setForm((prev) => ({ ...prev, trafficResetMode: value }))}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="global">{t("admin.system.subscription.mode_global")}</SelectItem>
              <SelectItem value="subscription">
                {t("admin.system.subscription.mode_subscription")}
              </SelectItem>
            </SelectContent>
          </Select>
          <p className="text-xs text-muted-foreground">
            {t("admin.system.subscription.tooltips.mode")}
          </p>
        </div>

        {form.trafficResetMode === "global" && (
          <div className="space-y-2">
            <label className="text-sm font-medium">
              {t("admin.system.subscription.traffic_reset_day")}
            </label>
            <Input
              type="number"
              min={1}
              max={28}
              value={form.trafficResetDay}
              onChange={(e) => setForm((prev) => ({ ...prev, trafficResetDay: e.target.value }))}
              placeholder="1-28"
            />
            <p className="text-xs text-muted-foreground">
              {t("admin.system.subscription.tooltips.day")}
            </p>
          </div>
        )}

        <div className="space-y-3">
          <div className="flex items-center justify-between rounded-md border border-border px-3 py-2">
            <div className="space-y-1">
              <p className="text-sm font-medium">
                {t("admin.system.settings.fields.allowChange")}
              </p>
              <p className="text-xs text-muted-foreground">
                {t("admin.system.settings.tooltips.allowChange")}
              </p>
            </div>
            <Switch
              checked={form.allowChange}
              onCheckedChange={(checked) => setForm((prev) => ({ ...prev, allowChange: checked }))}
            />
          </div>

          <div className="flex items-center justify-between rounded-md border border-border px-3 py-2">
            <div className="space-y-1">
              <p className="text-sm font-medium">
                {t("admin.system.settings.fields.injectMeta")}
              </p>
              <p className="text-xs text-muted-foreground">
                {t("admin.system.settings.tooltips.injectMeta")}
              </p>
            </div>
            <Switch
              checked={form.injectMeta}
              onCheckedChange={(checked) => setForm((prev) => ({ ...prev, injectMeta: checked }))}
            />
          </div>

          <div className="flex items-center justify-between rounded-md border border-border px-3 py-2">
            <div className="space-y-1">
              <p className="text-sm font-medium">
                {t("admin.system.settings.fields.injectProtocol")}
              </p>
              <p className="text-xs text-muted-foreground">
                {t("admin.system.settings.tooltips.injectProtocol")}
              </p>
            </div>
            <Switch
              checked={form.injectProtocol}
              onCheckedChange={(checked) => setForm((prev) => ({ ...prev, injectProtocol: checked }))}
            />
          </div>
        </div>

        <Button onClick={handleSave} disabled={saveMutation.isPending}>
          {saveMutation.isPending ? t("common.loading") : t("admin.system.settings.actions.save")}
        </Button>
      </div>

      <Dialog open={confirmPathOpen} onOpenChange={setConfirmPathOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("common.confirm")}</DialogTitle>
            <DialogDescription>{t("admin.system.settings.tooltips.subscribePath")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={cancelPathChange}>
              {t("common.cancel")}
            </Button>
            <Button onClick={confirmPathChange}>{t("common.confirm")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

function SubscriptionSourcesPanel() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [sourceDialogOpen, setSourceDialogOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [editingSource, setEditingSource] = useState<SubscriptionSource | null>(null);
  const [deletingSource, setDeletingSource] = useState<SubscriptionSource | null>(null);
  const [sourceForm, setSourceForm] = useState<SourceFormState>(createEmptySourceForm);

  const sourcesQuery = useQuery({
    queryKey: [...QUERY_KEYS.ADMIN_SUBSCRIPTION_SOURCES, SOURCE_LIST_LIMIT],
    queryFn: () => listSubscriptionSources({ limit: SOURCE_LIST_LIMIT }),
  });

  const invalidateSubscriptionViews = () => {
    queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_SUBSCRIPTION_SOURCES });
    queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_SUBSCRIPTION_FILTER_SUMMARY });
    queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_SUBSCRIPTION_FILTER_REASONS });
  };

  const saveSourceMutation = useMutation({
    mutationFn: ({ id, payload }: SourceSaveArgs) =>
      id ? updateSubscriptionSource(id, payload) : createSubscriptionSource(payload),
    onSuccess: () => {
      invalidateSubscriptionViews();
      setSourceDialogOpen(false);
      setEditingSource(null);
      setSourceForm(createEmptySourceForm());
      toast.success(t("admin.system.subscription.sourceSaveSuccess"));
    },
    onError: (error: Error) => {
      toast.error(t("admin.system.subscription.sourceSaveError"), { description: error.message });
    },
  });

  const deleteSourceMutation = useMutation({
    mutationFn: (id: number) => deleteSubscriptionSource(id),
    onSuccess: () => {
      invalidateSubscriptionViews();
      setDeleteDialogOpen(false);
      setDeletingSource(null);
      toast.success(t("admin.system.subscription.sourceDeleteSuccess"));
    },
    onError: (error: Error) => {
      toast.error(t("admin.system.subscription.sourceDeleteError"), { description: error.message });
    },
  });

  const syncSourceMutation = useMutation({
    mutationFn: (id: number) => syncSubscriptionSource(id),
    onSuccess: (result) => {
      invalidateSubscriptionViews();
      if (result.success) {
        toast.success(t("admin.system.subscription.sourceSyncSuccess"), {
          description: t("admin.system.subscription.sourceSyncNodes").replace(
            "{{count}}",
            String(result.node_count)
          ),
        });
        return;
      }
      toast.error(t("admin.system.subscription.sourceSyncError"), {
        description: result.error || t("admin.system.subscription.sourceSyncUnknownError"),
      });
    },
    onError: (error: Error) => {
      toast.error(t("admin.system.subscription.sourceSyncError"), { description: error.message });
    },
  });

  const openCreateDialog = () => {
    setEditingSource(null);
    setSourceForm(createEmptySourceForm());
    setSourceDialogOpen(true);
  };

  const openEditDialog = (source: SubscriptionSource) => {
    setEditingSource(source);
    setSourceForm(sourceToForm(source));
    setSourceDialogOpen(true);
  };

  const openDeleteDialog = (source: SubscriptionSource) => {
    setDeletingSource(source);
    setDeleteDialogOpen(true);
  };

  const handleSourceSubmit = () => {
    const payload = buildSourcePayload(sourceForm);
    if (!payload.name) {
      toast.error(t("common.error"), {
        description: t("admin.system.subscription.sourceValidationRequired"),
      });
      return;
    }
    if (payload.type === "imported_subscription" && !payload.url) {
      toast.error(t("common.error"), {
        description: t("admin.system.subscription.sourceUrlRequired"),
      });
      return;
    }
    saveSourceMutation.mutate({ id: editingSource?.id, payload });
  };

  const sources = sourcesQuery.data?.sources ?? [];

  return (
    <Card className="border border-border shadow-none">
      <CardHeader className="gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <CardTitle>{t("admin.system.subscription.sourceManagementTitle")}</CardTitle>
          <CardDescription>{t("admin.system.subscription.sourceManagementDescription")}</CardDescription>
        </div>
        <Button size="sm" onClick={openCreateDialog}>
          {t("admin.system.subscription.addSource")}
        </Button>
      </CardHeader>
      <CardContent>
        {sourcesQuery.isLoading ? (
          <Loading />
        ) : sourcesQuery.error ? (
          <ErrorBanner
            message={t("admin.system.subscription.sourceLoadError")}
            onRetry={() => sourcesQuery.refetch()}
          />
        ) : sources.length === 0 ? (
          <EmptyState
            size="sm"
            title={t("admin.system.subscription.sourceEmpty")}
            description={t("admin.system.subscription.sourceEmptyDescription")}
            action={<Button onClick={openCreateDialog}>{t("admin.system.subscription.addSource")}</Button>}
          />
        ) : (
          <div className="overflow-x-auto rounded-md border border-border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("admin.system.subscription.sourceTableName")}</TableHead>
                  <TableHead>{t("admin.system.subscription.sourceType")}</TableHead>
                  <TableHead>{t("admin.system.subscription.sourceMaterial")}</TableHead>
                  <TableHead>{t("admin.system.subscription.sourceStatus")}</TableHead>
                  <TableHead>{t("admin.system.subscription.sourceLastSync")}</TableHead>
                  <TableHead className="text-right">{t("common.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sources.map((source) => (
                  <TableRow key={source.id}>
                    <TableCell className="font-medium">{source.name}</TableCell>
                    <TableCell>{getSourceTypeLabel(source.type, t)}</TableCell>
                    <TableCell>
                      <span className="font-mono text-xs text-muted-foreground">
                        {getSourceMaterial(source, t)}
                      </span>
                    </TableCell>
                    <TableCell>
                      <Badge variant={source.enabled ? "success" : "secondary"}>
                        {source.enabled
                          ? t("admin.system.subscription.sourceEnabled")
                          : t("admin.system.subscription.sourceDisabled")}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {source.last_sync_at
                        ? formatDateTime(source.last_sync_at)
                        : t("admin.system.subscription.sourceNeverSynced")}
                      {source.last_sync_err && (
                        <p className="mt-1 max-w-64 truncate text-xs text-destructive">
                          {source.last_sync_err}
                        </p>
                      )}
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap justify-end gap-2">
                        {source.type === "imported_subscription" && (
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => syncSourceMutation.mutate(source.id)}
                            disabled={syncSourceMutation.isPending}
                          >
                            {t("admin.system.subscription.sourceSync")}
                          </Button>
                        )}
                        <Button variant="outline" size="sm" onClick={() => openEditDialog(source)}>
                          {t("common.edit")}
                        </Button>
                        <Button variant="ghost" size="sm" onClick={() => openDeleteDialog(source)}>
                          {t("common.delete")}
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>

      <Dialog open={sourceDialogOpen} onOpenChange={setSourceDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editingSource
                ? t("admin.system.subscription.editSource")
                : t("admin.system.subscription.addSource")}
            </DialogTitle>
            <DialogDescription>
              {t("admin.system.subscription.sourceDialogDescription")}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm font-medium" htmlFor="subscription-source-name">
                {t("admin.system.subscription.sourceName")}
              </label>
              <Input
                id="subscription-source-name"
                value={sourceForm.name}
                onChange={(event) =>
                  setSourceForm((prev) => ({ ...prev, name: event.target.value }))
                }
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium" htmlFor="subscription-source-type">
                {t("admin.system.subscription.sourceType")}
              </label>
              <Select
                value={sourceForm.type}
                onValueChange={(value) =>
                  setSourceForm((prev) => ({ ...prev, type: toSourceType(value) }))
                }
              >
                <SelectTrigger id="subscription-source-type">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SOURCE_TYPES.map((type) => (
                    <SelectItem key={type} value={type}>
                      {getSourceTypeLabel(type, t)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {sourceForm.type === "imported_subscription" && (
              <div className="space-y-2">
                <label className="text-sm font-medium" htmlFor="subscription-source-url">
                  {t("admin.system.subscription.sourceUrl")}
                </label>
                <Input
                  id="subscription-source-url"
                  value={sourceForm.url}
                  onChange={(event) =>
                    setSourceForm((prev) => ({ ...prev, url: event.target.value }))
                  }
                  placeholder={t("admin.system.subscription.sourceUrlPlaceholder")}
                />
                <p className="text-xs text-muted-foreground">
                  {t("admin.system.subscription.sourceUrlHint")}
                </p>
              </div>
            )}
            {sourceForm.type === "custom_node" && (
              <div className="space-y-2">
                <label className="text-sm font-medium" htmlFor="subscription-source-content">
                  {t("admin.system.subscription.sourceContent")}
                </label>
                <Textarea
                  id="subscription-source-content"
                  value={sourceForm.content}
                  onChange={(event) =>
                    setSourceForm((prev) => ({ ...prev, content: event.target.value }))
                  }
                  placeholder={t("admin.system.subscription.sourceContentPlaceholder")}
                />
              </div>
            )}
            <div className="flex items-center justify-between rounded-md border border-border px-3 py-2">
              <div>
                <p className="text-sm font-medium">{t("admin.system.subscription.sourceEnabled")}</p>
                <p className="text-xs text-muted-foreground">
                  {t("admin.system.subscription.sourceEnabledHint")}
                </p>
              </div>
              <Switch
                checked={sourceForm.enabled}
                onCheckedChange={(checked) =>
                  setSourceForm((prev) => ({ ...prev, enabled: checked }))
                }
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setSourceDialogOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={handleSourceSubmit} disabled={saveSourceMutation.isPending}>
              {saveSourceMutation.isPending ? t("common.loading") : t("common.save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("admin.system.subscription.deleteSourceTitle")}</DialogTitle>
            <DialogDescription>
              {t("admin.system.subscription.deleteSourceDescription").replace(
                "{{name}}",
                deletingSource?.name ?? ""
              )}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteDialogOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={() => deletingSource && deleteSourceMutation.mutate(deletingSource.id)}
              disabled={deleteSourceMutation.isPending || !deletingSource}
            >
              {deleteSourceMutation.isPending ? t("common.loading") : t("common.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}

function SubscriptionFilterDiagnosticsPanel() {
  const { t } = useTranslation();

  const summaryQuery = useQuery({
    queryKey: QUERY_KEYS.ADMIN_SUBSCRIPTION_FILTER_SUMMARY,
    queryFn: () => getSubscriptionFilterSummary(),
  });
  const reasonsQuery = useQuery({
    queryKey: [...QUERY_KEYS.ADMIN_SUBSCRIPTION_FILTER_REASONS, FILTER_REASON_LIMIT],
    queryFn: () => listSubscriptionFilterReasons({ limit: FILTER_REASON_LIMIT }),
  });

  const summary = summaryQuery.data;
  const reasons = reasonsQuery.data?.reasons ?? [];
  const reasonCounts = Object.entries(summary?.reason_counts ?? {})
    .filter(([, count]) => count > 0)
    .sort((a, b) => b[1] - a[1]);

  return (
    <Card className="border border-border shadow-none">
      <CardHeader>
        <CardTitle>{t("admin.system.subscription.filterDiagnosticsTitle")}</CardTitle>
        <CardDescription>{t("admin.system.subscription.filterDiagnosticsDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">
        {summaryQuery.error ? (
          <ErrorBanner
            message={t("admin.system.subscription.filterSummaryLoadError")}
            onRetry={() => summaryQuery.refetch()}
          />
        ) : (
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
            <MetricTile
              label={t("admin.system.subscription.availableNodeCount")}
              value={summary?.available_node_count ?? "-"}
            />
            <MetricTile
              label={t("admin.system.subscription.filteredNodeCount")}
              value={summary?.filtered_node_count ?? "-"}
            />
            <MetricTile
              label={t("admin.system.subscription.totalNodeCount")}
              value={summary?.total_node_count ?? "-"}
            />
            <MetricTile
              label={t("admin.system.subscription.sourceNodeCount")}
              value={summary?.source_node_count ?? "-"}
            />
            <MetricTile
              label={t("admin.system.subscription.enabledSourceCount")}
              value={summary?.enabled_source_count ?? "-"}
            />
            <MetricTile
              label={t("admin.system.subscription.selfHostedCount")}
              value={summary?.self_hosted_count ?? "-"}
            />
          </div>
        )}

        <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
          <div className="rounded-md border border-border p-4">
            <h4 className="text-sm font-medium text-foreground">
              {t("admin.system.subscription.reasonDistribution")}
            </h4>
            {summaryQuery.isLoading ? (
              <p className="mt-3 text-sm text-muted-foreground">{t("common.loading")}</p>
            ) : reasonCounts.length === 0 ? (
              <p className="mt-3 text-sm text-muted-foreground">
                {t("admin.system.subscription.noReasonDistribution")}
              </p>
            ) : (
              <div className="mt-3 space-y-2">
                {reasonCounts.map(([reason, count]) => (
                  <div key={reason} className="flex items-center justify-between gap-3 text-sm">
                    <span className="text-muted-foreground">{getFilterReasonLabel(reason, t)}</span>
                    <Badge variant="outline">{count}</Badge>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="rounded-md border border-border p-4">
            <div className="flex items-start justify-between gap-3">
              <div>
                <h4 className="text-sm font-medium text-foreground">
                  {t("admin.system.subscription.recentFilteredNodes")}
                </h4>
                <p className="mt-1 text-xs text-muted-foreground">
                  {t("admin.system.subscription.recentFilteredNodesHint")}
                </p>
              </div>
              <Button variant="outline" size="sm" onClick={() => reasonsQuery.refetch()}>
                {t("common.refresh")}
              </Button>
            </div>

            {reasonsQuery.isLoading ? (
              <p className="mt-4 text-sm text-muted-foreground">{t("common.loading")}</p>
            ) : reasonsQuery.error ? (
              <div className="mt-4">
                <ErrorBanner
                  message={t("admin.system.subscription.filterReasonsLoadError")}
                  onRetry={() => reasonsQuery.refetch()}
                />
              </div>
            ) : reasons.length === 0 ? (
              <EmptyState
                size="sm"
                title={t("admin.system.subscription.filtersEmpty")}
                description={t("admin.system.subscription.filtersEmptyDescription")}
              />
            ) : (
              <div className="mt-4 overflow-x-auto rounded-md border border-border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("admin.system.subscription.nodeName")}</TableHead>
                      <TableHead>{t("admin.system.subscription.reason")}</TableHead>
                      <TableHead>{t("admin.system.subscription.source")}</TableHead>
                      <TableHead>{t("admin.system.subscription.detail")}</TableHead>
                      <TableHead>{t("admin.system.subscription.createdAt")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {reasons.map((reason) => (
                      <FilterReasonRow key={reason.id} reason={reason} />
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

function FilterReasonRow({ reason }: { reason: SubscriptionFilterReasonEntry }) {
  const { t } = useTranslation();

  return (
    <TableRow>
      <TableCell className="font-medium">{reason.node_name || "-"}</TableCell>
      <TableCell>
        <Badge variant="warning">{getFilterReasonLabel(reason.reason, t)}</Badge>
      </TableCell>
      <TableCell>
        <div className="space-y-1 text-sm">
          <p>{getSourceTypeLabel(reason.source_type, t)}</p>
          <p className="text-xs text-muted-foreground">
            {t("admin.system.subscription.sourceId").replace("{{id}}", String(reason.source_id))}
          </p>
        </div>
      </TableCell>
      <TableCell className="max-w-72 truncate text-muted-foreground">
        {reason.detail || t("admin.system.subscription.noDetail")}
      </TableCell>
      <TableCell>{formatDateTime(reason.created_at)}</TableCell>
    </TableRow>
  );
}

export default function SubscriptionTab() {
  const { t } = useTranslation();

  const queryKey = useMemo(() => [...QUERY_KEYS.ADMIN_SYSTEM, CATEGORY], []);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey,
    queryFn: () => fetchSettings(CATEGORY),
  });

  const initialForm = useMemo<SubscriptionForm>(
    () => ({
      subscribeUrls: data?.subscribe_urls ?? "",
      subscribePath: data?.subscribe_path ?? "",
      trafficResetMode: data?.traffic_reset_mode ?? "global",
      trafficResetDay: data?.traffic_reset_day ?? "1",
      allowChange: toBool(data?.allow_change),
      injectMeta: toBool(data?.inject_meta),
      injectProtocol: toBool(data?.inject_protocol),
    }),
    [data]
  );

  if (isLoading) return <Loading />;

  if (error) {
    return (
      <ErrorBanner
        message={t("admin.system.settings.messages.loadError")}
        onRetry={() => refetch()}
      />
    );
  }

  return (
    <div className="space-y-6">
      <Card className="border border-border shadow-none">
        <CardHeader>
          <CardTitle>{t("admin.system.settings.tabs.subscription")}</CardTitle>
          <CardDescription>{t("admin.system.settings.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <SubscriptionTabContent initialForm={initialForm} />
        </CardContent>
      </Card>
      <SubscriptionSourcesPanel />
      <SubscriptionFilterDiagnosticsPanel />
    </div>
  );
}
