import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  createSubscriptionSource,
  deleteSubscriptionSource,
  listSubscriptionSources,
  syncSubscriptionSource,
  updateSubscriptionSource,
} from "@/api/admin/subscription";
import { QUERY_KEYS } from "@/lib/constants";
import { formatDateTime } from "@/lib/format";
import type {
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
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
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
} from "@/components/ui";
import ErrorBanner from "@/components/ui/ErrorBanner";

const SOURCE_LIST_LIMIT = 100;
const SOURCE_TYPES: SubscriptionSourceType[] = [
  "self_hosted",
  "imported_subscription",
  "custom_node",
];

interface SourceFormState {
  type: SubscriptionSourceType;
  name: string;
  url: string;
  content: string;
  enabled: boolean;
}

type SourceSaveArgs = {
  id?: number;
  payload: UpsertSubscriptionSourceRequest;
};

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

function buildSourcePayload(
  form: SourceFormState,
): UpsertSubscriptionSourceRequest {
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
    const path =
      parsed.pathname && parsed.pathname !== "/" ? "/…" : "";
    return `${parsed.protocol}//${parsed.host}${path}`;
  } catch {
    if (trimmed.length <= 10) {
      return "••••";
    }
    return `${trimmed.slice(0, 6)}…${trimmed.slice(-4)}`;
  }
}

function getSourceMaterial(
  source: SubscriptionSource,
  t: (key: string) => string,
): string {
  if (source.type === "imported_subscription") {
    return source.url
      ? maskSensitiveURL(source.url)
      : t("admin.system.subscription.sourceNoMaterial");
  }
  if (source.type === "custom_node") {
    const length = source.content?.trim().length ?? 0;
    return length > 0
      ? t("admin.system.subscription.sourceContentSummary").replace(
          "{{count}}",
          String(length),
        )
      : t("admin.system.subscription.sourceNoMaterial");
  }
  return t("admin.system.subscription.sourceSelfHostedMaterial");
}

function getSourceTypeLabel(
  type: string,
  t: (key: string) => string,
): string {
  return t(`admin.system.subscription.sourceTypeOptions.${type}`);
}

export default function SubscriptionSourceManager() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [sourceDialogOpen, setSourceDialogOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [editingSource, setEditingSource] =
    useState<SubscriptionSource | null>(null);
  const [deletingSource, setDeletingSource] =
    useState<SubscriptionSource | null>(null);
  const [sourceForm, setSourceForm] = useState<SourceFormState>(
    createEmptySourceForm,
  );

  const sourcesQuery = useQuery({
    queryKey: [...QUERY_KEYS.ADMIN_SUBSCRIPTION_SOURCES, SOURCE_LIST_LIMIT],
    queryFn: () => listSubscriptionSources({ limit: SOURCE_LIST_LIMIT }),
  });

  const invalidateSubscriptionViews = () => {
    queryClient.invalidateQueries({
      queryKey: QUERY_KEYS.ADMIN_SUBSCRIPTION_SOURCES,
    });
    queryClient.invalidateQueries({
      queryKey: QUERY_KEYS.ADMIN_SUBSCRIPTION_FILTER_SUMMARY,
    });
    queryClient.invalidateQueries({
      queryKey: QUERY_KEYS.ADMIN_SUBSCRIPTION_FILTER_REASONS,
    });
  };

  const saveSourceMutation = useMutation({
    mutationFn: ({ id, payload }: SourceSaveArgs) =>
      id
        ? updateSubscriptionSource(id, payload)
        : createSubscriptionSource(payload),
    onSuccess: () => {
      invalidateSubscriptionViews();
      setSourceDialogOpen(false);
      setEditingSource(null);
      setSourceForm(createEmptySourceForm());
      toast.success(t("admin.system.subscription.sourceSaveSuccess"));
    },
    onError: (error: Error) => {
      toast.error(t("admin.system.subscription.sourceSaveError"), {
        description: error.message,
      });
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
      toast.error(t("admin.system.subscription.sourceDeleteError"), {
        description: error.message,
      });
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
            String(result.node_count),
          ),
        });
        return;
      }
      toast.error(t("admin.system.subscription.sourceSyncError"), {
        description:
          result.error ||
          t("admin.system.subscription.sourceSyncUnknownError"),
      });
    },
    onError: (error: Error) => {
      toast.error(t("admin.system.subscription.sourceSyncError"), {
        description: error.message,
      });
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
          <CardTitle>
            {t("admin.system.subscription.sourceManagementTitle")}
          </CardTitle>
          <CardDescription>
            {t("admin.system.subscription.sourceManagementDescription")}
          </CardDescription>
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
            action={
              <Button onClick={openCreateDialog}>
                {t("admin.system.subscription.addSource")}
              </Button>
            }
          />
        ) : (
          <div className="overflow-x-auto rounded-md border border-border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>
                    {t("admin.system.subscription.sourceTableName")}
                  </TableHead>
                  <TableHead>
                    {t("admin.system.subscription.sourceType")}
                  </TableHead>
                  <TableHead>
                    {t("admin.system.subscription.sourceMaterial")}
                  </TableHead>
                  <TableHead>
                    {t("admin.system.subscription.sourceStatus")}
                  </TableHead>
                  <TableHead>
                    {t("admin.system.subscription.sourceLastSync")}
                  </TableHead>
                  <TableHead className="text-right">
                    {t("common.actions")}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sources.map((source) => (
                  <TableRow key={source.id}>
                    <TableCell className="font-medium">
                      {source.name}
                    </TableCell>
                    <TableCell>
                      {getSourceTypeLabel(source.type, t)}
                    </TableCell>
                    <TableCell>
                      <span className="font-mono text-xs text-muted-foreground">
                        {getSourceMaterial(source, t)}
                      </span>
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={source.enabled ? "success" : "secondary"}
                      >
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
                            onClick={() =>
                              syncSourceMutation.mutate(source.id)
                            }
                            disabled={syncSourceMutation.isPending}
                          >
                            {t("admin.system.subscription.sourceSync")}
                          </Button>
                        )}
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => openEditDialog(source)}
                        >
                          {t("common.edit")}
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => openDeleteDialog(source)}
                        >
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
              <label
                className="text-sm font-medium"
                htmlFor="subscription-source-name"
              >
                {t("admin.system.subscription.sourceName")}
              </label>
              <Input
                id="subscription-source-name"
                value={sourceForm.name}
                onChange={(event) =>
                  setSourceForm((prev) => ({
                    ...prev,
                    name: event.target.value,
                  }))
                }
              />
            </div>
            <div className="space-y-2">
              <label
                className="text-sm font-medium"
                htmlFor="subscription-source-type"
              >
                {t("admin.system.subscription.sourceType")}
              </label>
              <Select
                value={sourceForm.type}
                onValueChange={(value) =>
                  setSourceForm((prev) => ({
                    ...prev,
                    type: toSourceType(value),
                  }))
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
                <label
                  className="text-sm font-medium"
                  htmlFor="subscription-source-url"
                >
                  {t("admin.system.subscription.sourceUrl")}
                </label>
                <Input
                  id="subscription-source-url"
                  value={sourceForm.url}
                  onChange={(event) =>
                    setSourceForm((prev) => ({
                      ...prev,
                      url: event.target.value,
                    }))
                  }
                  placeholder={t(
                    "admin.system.subscription.sourceUrlPlaceholder",
                  )}
                />
                <p className="text-xs text-muted-foreground">
                  {t("admin.system.subscription.sourceUrlHint")}
                </p>
              </div>
            )}
            {sourceForm.type === "custom_node" && (
              <div className="space-y-2">
                <label
                  className="text-sm font-medium"
                  htmlFor="subscription-source-content"
                >
                  {t("admin.system.subscription.sourceContent")}
                </label>
                <Textarea
                  id="subscription-source-content"
                  value={sourceForm.content}
                  onChange={(event) =>
                    setSourceForm((prev) => ({
                      ...prev,
                      content: event.target.value,
                    }))
                  }
                  placeholder={t(
                    "admin.system.subscription.sourceContentPlaceholder",
                  )}
                />
              </div>
            )}
            <div className="flex items-center justify-between rounded-md border border-border px-3 py-2">
              <div>
                <p className="text-sm font-medium">
                  {t("admin.system.subscription.sourceEnabled")}
                </p>
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
            <Button
              variant="outline"
              onClick={() => setSourceDialogOpen(false)}
            >
              {t("common.cancel")}
            </Button>
            <Button
              onClick={handleSourceSubmit}
              disabled={saveSourceMutation.isPending}
            >
              {saveSourceMutation.isPending
                ? t("common.loading")
                : t("common.save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t("admin.system.subscription.deleteSourceTitle")}
            </DialogTitle>
            <DialogDescription>
              {t("admin.system.subscription.deleteSourceDescription").replace(
                "{{name}}",
                deletingSource?.name ?? "",
              )}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setDeleteDialogOpen(false)}
            >
              {t("common.cancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={() =>
                deletingSource &&
                deleteSourceMutation.mutate(deletingSource.id)
              }
              disabled={deleteSourceMutation.isPending || !deletingSource}
            >
              {deleteSourceMutation.isPending
                ? t("common.loading")
                : t("common.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
