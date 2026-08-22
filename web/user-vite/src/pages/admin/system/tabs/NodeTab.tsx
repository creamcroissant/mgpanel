import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Eye } from "lucide-react";
import { toast } from "sonner";
import {
  fetchSettings,
  getKey,
  resetKey,
  revealKey,
  saveSettings,
} from "@/api/admin/settings";
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
  Input,
  Loading,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Switch,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui";
import ErrorBanner from "@/components/ui/ErrorBanner";

const CATEGORY = "node";
const FULLY_MASKED_KEY = "••••••••••••••••";
const TEMPLATE_INTERVAL_KEY = {
  pull: "server_pull_interval",
  push: "server_push_interval",
};

interface NodeForm {
  communicationKey: string;
  masked: boolean;
  hasValue: boolean;
  lastModified?: number;
  agentGrpcAddress: string;
  grpcTlsEnabled: boolean;
  pullInterval: string;
  pushInterval: string;
  deviceLimitMode: string;
}

type NodeTabContentProps = {
  initialForm: NodeForm;
};

function NodeTabContent({ initialForm }: NodeTabContentProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [form, setForm] = useState<NodeForm>(initialForm);
  const [resetOpen, setResetOpen] = useState(false);

  const settingsQueryKey = useMemo(() => [...QUERY_KEYS.ADMIN_SYSTEM, CATEGORY], []);
  const keyQueryKey = useMemo(() => [...QUERY_KEYS.ADMIN_SYSTEM, "key"], []);

  const saveMutation = useMutation({
    mutationFn: (payload: NodeForm) =>
      saveSettings(CATEGORY, {
        agent_grpc_address: payload.agentGrpcAddress.trim(),
        grpc_tls_enabled: payload.grpcTlsEnabled ? "true" : "false",
        [TEMPLATE_INTERVAL_KEY.pull]: payload.pullInterval.trim(),
        [TEMPLATE_INTERVAL_KEY.push]: payload.pushInterval.trim(),
        device_limit_mode: payload.deviceLimitMode.trim(),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: settingsQueryKey });
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

  const revealMutation = useMutation({
    mutationFn: revealKey,
    onSuccess: (payload) => {
      queryClient.invalidateQueries({ queryKey: keyQueryKey });
      setForm((prev) => ({
        ...prev,
        communicationKey: payload.key,
        masked: payload.masked,
        hasValue: payload.has_value,
        lastModified: payload.last_modified ?? prev.lastModified,
      }));
      toast.success(t("common.success"), {
        description: t("admin.system.settings.messages.revealSuccess"),
      });
    },
    onError: (err: Error) => {
      toast.error(t("common.error"), {
        description: err.message,
      });
    },
  });

  const resetMutation = useMutation({
    mutationFn: resetKey,
    onSuccess: (payload) => {
      setResetOpen(false);
      queryClient.invalidateQueries({ queryKey: keyQueryKey });
      setForm((prev) => ({
        ...prev,
        communicationKey: payload.key,
        masked: payload.masked,
        hasValue: payload.has_value,
        lastModified: payload.last_modified ?? prev.lastModified,
      }));
      toast.success(t("common.success"), {
        description: t("admin.system.settings.messages.resetSuccess"),
      });
    },
    onError: (err: Error) => {
      toast.error(t("common.error"), {
        description: err.message,
      });
    },
  });

  const handleSave = () => {
    const pull = Number(form.pullInterval);
    const push = Number(form.pushInterval);
    if (!Number.isFinite(pull) || pull <= 0 || !Number.isFinite(push) || push <= 0) {
      toast.warning(t("common.error"), {
        description: t("admin.system.settings.tooltips.pullInterval"),
      });
      return;
    }
    saveMutation.mutate(form);
  };

  const displayedCommunicationKey = form.masked && form.hasValue ? FULLY_MASKED_KEY : form.communicationKey;

  return (
    <>
      <div className="space-y-6 max-w-2xl">
        <div className="space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <label className="text-sm font-medium">{t("admin.system.settings.fields.communicationKey")}</label>
            <Badge variant={form.masked ? "secondary" : "warning"}>
              {form.masked
                ? t("admin.system.settings.keyState.masked")
                : t("admin.system.settings.keyState.revealed")}
            </Badge>
            {!form.hasValue && <Badge variant="danger">{t("admin.system.settings.keyState.empty")}</Badge>}
          </div>
          <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
            <div className="relative min-w-0">
              <Input
                value={displayedCommunicationKey}
                readOnly
                className="pr-11 font-mono text-xs"
                aria-label={t("admin.system.settings.fields.communicationKey")}
                data-testid="system-settings-node-key"
              />
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="absolute right-0 top-0 h-10 w-10 text-muted-foreground hover:text-foreground"
                onClick={() => revealMutation.mutate()}
                disabled={!form.hasValue || revealMutation.isPending}
                aria-label={t("admin.system.settings.actions.reveal")}
                title={t("admin.system.settings.actions.reveal")}
                data-testid="system-settings-reveal-key-button"
              >
                <Eye className="h-4 w-4" aria-hidden="true" />
              </Button>
            </div>
            <Button type="button" variant="destructive" onClick={() => setResetOpen(true)} data-testid="system-settings-reset-key-button">
              {t("admin.system.settings.actions.reset")}
            </Button>
          </div>
          <p className="text-xs leading-5 text-muted-foreground">{t("admin.system.settings.tooltips.communicationKey")}</p>
          <p className="text-xs leading-5 text-muted-foreground">{t("admin.system.settings.tooltips.communicationKeyRegistration")}</p>
          <div className="grid gap-2 text-xs text-muted-foreground sm:grid-cols-2">
            <div>
              <span className="font-medium text-foreground">{t("admin.system.settings.keyMeta.hasValue")}: </span>
              {form.hasValue
                ? t("admin.system.settings.keyMeta.available")
                : t("admin.system.settings.keyMeta.unavailable")}
            </div>
            <div>
              <span className="font-medium text-foreground">{t("admin.system.settings.keyMeta.lastModified")}: </span>
              {formatDateTime(form.lastModified ?? 0)}
            </div>
          </div>
        </div>

        <div className="space-y-2">
          <label className="text-sm font-medium">{t("admin.system.settings.fields.agentGrpcAddress")}</label>
          <Input
            value={form.agentGrpcAddress}
            onChange={(e) => setForm((prev) => ({ ...prev, agentGrpcAddress: e.target.value }))}
            placeholder={t("admin.system.settings.placeholders.agentGrpcAddress")}
          />
          <p className="text-xs text-muted-foreground">{t("admin.system.settings.tooltips.agentGrpcAddress")}</p>
        </div>

        <div className="flex items-center justify-between rounded-lg border p-4">
          <label className="text-sm font-medium" htmlFor="grpc-tls-toggle">
            {t("admin.system.settings.fields.grpcTlsEnabled")}
          </label>
          <Switch
            id="grpc-tls-toggle"
            checked={form.grpcTlsEnabled}
            onCheckedChange={(v) => setForm((prev) => ({ ...prev, grpcTlsEnabled: v }))}
          />
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.system.settings.fields.pullInterval")}</label>
            <Input
              type="number"
              min={1}
              value={form.pullInterval}
              onChange={(e) => setForm((prev) => ({ ...prev, pullInterval: e.target.value }))}
              placeholder={t("admin.system.settings.placeholders.pullInterval")}
            />
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.system.settings.fields.pushInterval")}</label>
            <Input
              type="number"
              min={1}
              value={form.pushInterval}
              onChange={(e) => setForm((prev) => ({ ...prev, pushInterval: e.target.value }))}
              placeholder={t("admin.system.settings.placeholders.pushInterval")}
            />
          </div>
        </div>

        <div className="space-y-2">
          <label className="text-sm font-medium">{t("admin.system.settings.fields.deviceLimitMode")}</label>
          <Select value={form.deviceLimitMode} onValueChange={(value) => setForm((prev) => ({ ...prev, deviceLimitMode: value }))}>
            <SelectTrigger>
              <SelectValue placeholder={t("admin.system.settings.fields.deviceLimitMode")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="0">{t("admin.system.settings.tooltips.deviceLimitMode")}</SelectItem>
              <SelectItem value="1">{t("admin.system.settings.options.deviceLimitMode.ipLimit")}</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <Button onClick={handleSave} disabled={saveMutation.isPending}>
          {saveMutation.isPending ? t("common.loading") : t("admin.system.settings.actions.save")}
        </Button>
      </div>

      <Dialog open={resetOpen} onOpenChange={setResetOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("common.confirm")}</DialogTitle>
            <DialogDescription>{t("admin.system.settings.messages.resetConfirm")}</DialogDescription>
          </DialogHeader>
          <div className="rounded-md border border-warning/30 bg-warning/10 p-3 text-sm text-warning-foreground dark:text-warning">
            {t("admin.system.settings.messages.resetImpact")}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setResetOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={() => resetMutation.mutate()}
              disabled={resetMutation.isPending}
            >
              {resetMutation.isPending ? t("common.loading") : t("admin.system.settings.actions.reset")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

export default function NodeTab() {
  const { t } = useTranslation();

  const settingsQueryKey = useMemo(() => [...QUERY_KEYS.ADMIN_SYSTEM, CATEGORY], []);
  const keyQueryKey = useMemo(() => [...QUERY_KEYS.ADMIN_SYSTEM, "key"], []);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: settingsQueryKey,
    queryFn: () => fetchSettings(CATEGORY),
  });

  const keyQuery = useQuery({
    queryKey: keyQueryKey,
    queryFn: () => getKey(),
  });

  const initialForm = useMemo<NodeForm>(
    () => ({
      communicationKey: keyQuery.data?.key ?? "",
      masked: keyQuery.data?.masked ?? true,
      hasValue: keyQuery.data?.has_value ?? false,
      lastModified: keyQuery.data?.last_modified,
      agentGrpcAddress: (data?.agent_grpc_address ?? "").trim(),
      grpcTlsEnabled: (data?.grpc_tls_enabled ?? "") === "true",
      pullInterval: data?.[TEMPLATE_INTERVAL_KEY.pull] ?? "60",
      pushInterval: data?.[TEMPLATE_INTERVAL_KEY.push] ?? "60",
      deviceLimitMode: data?.device_limit_mode ?? "0",
    }),
    [data, keyQuery.data]
  );

  if (isLoading || keyQuery.isLoading) return <Loading />;

  if (error || keyQuery.error) {
    return (
      <ErrorBanner
        message={t("admin.system.settings.messages.loadError")}
        onRetry={() => {
          refetch();
          keyQuery.refetch();
        }}
      />
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("admin.system.settings.tabs.node")}</CardTitle>
        <CardDescription>{t("admin.system.settings.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <NodeTabContent initialForm={initialForm} />
      </CardContent>
    </Card>
  );
}
