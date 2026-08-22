import { useMemo, useState } from "react";
import type { ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Save } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { getAgentTrafficPolicy, updateAgentTrafficPolicy } from "@/api/admin";
import { QUERY_KEYS } from "@/lib/constants";
import { formatBytes } from "@/lib/format";
import {
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
} from "@/components/ui";
import type {
  AgentTrafficLimitType,
  AgentTrafficPolicy,
  AgentTrafficPolicyStatus,
  AgentTrafficResetMode,
  AgentTrafficThresholdAction,
  UpdateAgentTrafficPolicyRequest,
} from "@/types";

interface TrafficPolicyFormProps {
  agentHostId: number;
}

type PolicyFormState = {
  enabled: boolean;
  limit_gib: string;
  limit_type: AgentTrafficLimitType;
  threshold_percent: string;
  threshold_action: AgentTrafficThresholdAction;
  reset_mode: AgentTrafficResetMode;
  reset_day: string;
  interval_days: string;
  anchor_at: string;
};

const BYTES_PER_GIB = 1024 ** 3;

const LIMIT_TYPE_OPTIONS: AgentTrafficLimitType[] = ["sum", "upload", "download"];
const THRESHOLD_ACTION_OPTIONS: AgentTrafficThresholdAction[] = [
  "notify_only",
  "subscription_exclude",
  "disable_servers",
  "reset_links",
];
const RESET_MODE_OPTIONS: AgentTrafficResetMode[] = ["off", "fixed_day", "calendar_month", "interval_days"];

function normalizeLimitType(value: string): AgentTrafficLimitType {
  return LIMIT_TYPE_OPTIONS.includes(value as AgentTrafficLimitType) ? value as AgentTrafficLimitType : "sum";
}

function normalizeThresholdAction(value: string): AgentTrafficThresholdAction {
  return THRESHOLD_ACTION_OPTIONS.includes(value as AgentTrafficThresholdAction)
    ? value as AgentTrafficThresholdAction
    : "notify_only";
}

function normalizeResetMode(value: string): AgentTrafficResetMode {
  return RESET_MODE_OPTIONS.includes(value as AgentTrafficResetMode) ? value as AgentTrafficResetMode : "off";
}

function formatGib(bytes: number): string {
  if (!bytes || bytes <= 0) return "";
  const value = bytes / BYTES_PER_GIB;
  return Number.isInteger(value) ? String(value) : value.toFixed(3).replace(/0+$/, "").replace(/\.$/, "");
}

function parseGibToBytes(value: string): number {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) return 0;
  return Math.round(parsed * BYTES_PER_GIB);
}

function formatDateTimeLocal(timestamp: number): string {
  if (!timestamp || timestamp <= 0) return "";
  const date = new Date(timestamp * 1000);
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000);
  return local.toISOString().slice(0, 16);
}

function parseDateTimeLocal(value: string): number {
  if (!value) return 0;
  const parsed = new Date(value).getTime();
  if (!Number.isFinite(parsed)) return 0;
  return Math.floor(parsed / 1000);
}

function parseInteger(value: string): number {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return 0;
  return Math.trunc(parsed);
}

function policyToForm(policy: AgentTrafficPolicy): PolicyFormState {
  return {
    enabled: policy.enabled,
    limit_gib: formatGib(policy.limit_bytes),
    limit_type: normalizeLimitType(policy.limit_type),
    threshold_percent: policy.threshold_percent ? String(policy.threshold_percent) : "",
    threshold_action: normalizeThresholdAction(policy.threshold_action),
    reset_mode: normalizeResetMode(policy.reset_mode),
    reset_day: policy.reset_day ? String(policy.reset_day) : "",
    interval_days: policy.interval_days ? String(policy.interval_days) : "",
    anchor_at: formatDateTimeLocal(policy.anchor_at),
  };
}

function buildPolicyPayload(form: PolicyFormState): UpdateAgentTrafficPolicyRequest {
  const resetMode = form.reset_mode;
  return {
    enabled: form.enabled,
    limit_bytes: parseGibToBytes(form.limit_gib),
    limit_type: form.limit_type,
    threshold_percent: parseInteger(form.threshold_percent),
    threshold_action: form.threshold_action,
    reset_mode: resetMode,
    reset_day: resetMode === "fixed_day" || resetMode === "calendar_month" ? parseInteger(form.reset_day) : 0,
    interval_days: resetMode === "interval_days" ? parseInteger(form.interval_days) : 0,
    anchor_at: resetMode === "interval_days" ? parseDateTimeLocal(form.anchor_at) : 0,
  };
}

function buildPolicyKey(status: AgentTrafficPolicyStatus): string {
  const policy = status.policy;
  return [
    policy.updated_at,
    policy.enabled,
    policy.limit_bytes,
    policy.limit_type,
    policy.threshold_percent,
    policy.threshold_action,
    policy.reset_mode,
    policy.reset_day,
    policy.interval_days,
    policy.anchor_at,
  ].join(":");
}

function FieldLabel({ htmlFor, children }: { htmlFor?: string; children: ReactNode }) {
  return <label htmlFor={htmlFor} className="text-sm font-medium text-foreground">{children}</label>;
}

function TrafficPolicyEditor({ agentHostId, status }: { agentHostId: number; status: AgentTrafficPolicyStatus }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [form, setForm] = useState<PolicyFormState>(() => policyToForm(status.policy));

  const limitBytes = useMemo(() => parseGibToBytes(form.limit_gib), [form.limit_gib]);
  const showResetDay = form.reset_mode === "fixed_day" || form.reset_mode === "calendar_month";
  const showInterval = form.reset_mode === "interval_days";

  const updateMutation = useMutation({
    mutationFn: (payload: UpdateAgentTrafficPolicyRequest) => updateAgentTrafficPolicy(agentHostId, payload),
    onSuccess: (result) => {
      queryClient.setQueryData([...QUERY_KEYS.ADMIN_AGENT_TRAFFIC_POLICY, agentHostId], result);
      queryClient.setQueryData([...QUERY_KEYS.ADMIN_AGENT_TRAFFIC_STATUS, agentHostId], result);
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_AGENT_TRAFFIC_POLICY });
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_AGENT_TRAFFIC_STATUS });
      toast.success(t("admin.cores.trafficPolicySaveSuccess"));
    },
    onError: (error: Error) => {
      toast.error(t("admin.cores.trafficPolicySaveError"), { description: error.message });
    },
  });

  const validateAndSubmit = () => {
    const payload = buildPolicyPayload(form);
    if (payload.enabled && payload.limit_bytes <= 0) {
      toast.warning(t("admin.cores.validationError"), { description: t("admin.cores.trafficLimitRequired") });
      return;
    }
    if (payload.threshold_percent < 0 || payload.threshold_percent > 100) {
      toast.warning(t("admin.cores.validationError"), { description: t("admin.cores.trafficThresholdInvalid") });
      return;
    }
    if (payload.reset_mode === "fixed_day" && (payload.reset_day < 1 || payload.reset_day > 31)) {
      toast.warning(t("admin.cores.validationError"), { description: t("admin.cores.trafficResetDayInvalid") });
      return;
    }
    if (payload.reset_mode === "interval_days" && payload.interval_days <= 0) {
      toast.warning(t("admin.cores.validationError"), { description: t("admin.cores.trafficIntervalInvalid") });
      return;
    }
    updateMutation.mutate(payload);
  };

  return (
    <>
      <CardHeader className="pb-3">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <CardTitle className="text-sm">{t("admin.cores.trafficPolicyTitle")}</CardTitle>
            <CardDescription>{t("admin.cores.trafficPolicyDescription")}</CardDescription>
          </div>
          <div className="flex items-center gap-3 rounded-md border border-border px-3 py-2">
            <span className="text-sm text-muted-foreground">{t("admin.cores.trafficPolicyEnabled")}</span>
            <Switch
              checked={form.enabled}
              onCheckedChange={(checked) => setForm((current) => ({ ...current, enabled: checked }))}
              aria-label={t("admin.cores.trafficPolicyEnabled")}
            />
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-4 lg:grid-cols-3">
          <div className="space-y-2">
            <FieldLabel htmlFor="traffic-limit-gib">{t("admin.cores.trafficLimitGib")}</FieldLabel>
            <Input
              id="traffic-limit-gib"
              type="number"
              min="0"
              step="0.1"
              value={form.limit_gib}
              onChange={(event) => setForm((current) => ({ ...current, limit_gib: event.target.value }))}
              placeholder="100"
            />
            <p className="text-xs text-muted-foreground">
              {t("admin.cores.trafficLimitBytesHint", { value: formatBytes(limitBytes) })}
            </p>
          </div>

          <div className="space-y-2">
            <FieldLabel>{t("admin.cores.trafficLimitType")}</FieldLabel>
            <Select
              value={form.limit_type}
              onValueChange={(value) => setForm((current) => ({ ...current, limit_type: normalizeLimitType(value) }))}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {LIMIT_TYPE_OPTIONS.map((option) => (
                  <SelectItem key={option} value={option}>{t(`admin.cores.trafficLimitTypeOptions.${option}`)}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <FieldLabel htmlFor="traffic-threshold-percent">{t("admin.cores.trafficThresholdPercent")}</FieldLabel>
            <Input
              id="traffic-threshold-percent"
              type="number"
              min="0"
              max="100"
              value={form.threshold_percent}
              onChange={(event) => setForm((current) => ({ ...current, threshold_percent: event.target.value }))}
              placeholder="80"
            />
            <p className="text-xs text-muted-foreground">{t("admin.cores.trafficThresholdHint")}</p>
          </div>
        </div>

        <div className="grid gap-4 lg:grid-cols-3">
          <div className="space-y-2">
            <FieldLabel>{t("admin.cores.trafficThresholdAction")}</FieldLabel>
            <Select
              value={form.threshold_action}
              onValueChange={(value) => setForm((current) => ({ ...current, threshold_action: normalizeThresholdAction(value) }))}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {THRESHOLD_ACTION_OPTIONS.map((option) => (
                  <SelectItem key={option} value={option}>{t(`admin.cores.trafficThresholdActionOptions.${option}`)}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <FieldLabel>{t("admin.cores.trafficResetMode")}</FieldLabel>
            <Select
              value={form.reset_mode}
              onValueChange={(value) => setForm((current) => ({ ...current, reset_mode: normalizeResetMode(value) }))}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {RESET_MODE_OPTIONS.map((option) => (
                  <SelectItem key={option} value={option}>{t(`admin.cores.trafficResetModeOptions.${option}`)}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {showResetDay && (
            <div className="space-y-2">
              <FieldLabel htmlFor="traffic-reset-day">{t("admin.cores.trafficResetDay")}</FieldLabel>
              <Input
                id="traffic-reset-day"
                type="number"
                min="1"
                max="31"
                value={form.reset_day}
                onChange={(event) => setForm((current) => ({ ...current, reset_day: event.target.value }))}
                placeholder="1"
              />
            </div>
          )}

          {showInterval && (
            <>
              <div className="space-y-2">
                <FieldLabel htmlFor="traffic-interval-days">{t("admin.cores.trafficIntervalDays")}</FieldLabel>
                <Input
                  id="traffic-interval-days"
                  type="number"
                  min="1"
                  value={form.interval_days}
                  onChange={(event) => setForm((current) => ({ ...current, interval_days: event.target.value }))}
                  placeholder="30"
                />
              </div>
              <div className="space-y-2">
                <FieldLabel htmlFor="traffic-anchor-at">{t("admin.cores.trafficAnchorAt")}</FieldLabel>
                <Input
                  id="traffic-anchor-at"
                  type="datetime-local"
                  value={form.anchor_at}
                  onChange={(event) => setForm((current) => ({ ...current, anchor_at: event.target.value }))}
                />
              </div>
            </>
          )}
        </div>

        <div className="flex flex-col gap-3 rounded-md border border-border bg-muted/20 p-3 text-sm text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
          <div>
            {t("admin.cores.trafficCurrentPolicySummary", {
              limit: formatBytes(status.policy.limit_bytes),
              threshold: status.policy.threshold_percent || 0,
            })}
          </div>
          <Button onClick={validateAndSubmit} disabled={updateMutation.isPending}>
            <Save className="mr-2 h-4 w-4" />
            {updateMutation.isPending ? t("common.loading") : t("common.save")}
          </Button>
        </div>
      </CardContent>
    </>
  );
}

export function TrafficPolicyForm({ agentHostId }: TrafficPolicyFormProps) {
  const { t } = useTranslation();
  const policyQuery = useQuery({
    queryKey: [...QUERY_KEYS.ADMIN_AGENT_TRAFFIC_POLICY, agentHostId],
    queryFn: () => getAgentTrafficPolicy(agentHostId),
  });

  return (
    <Card className="border border-border shadow-none" data-testid="agent-traffic-policy-form">
      {policyQuery.isLoading ? (
        <CardContent className="p-4"><Loading /></CardContent>
      ) : policyQuery.error ? (
        <CardContent className="flex flex-col items-center justify-center gap-3 py-6">
          <p className="text-sm text-destructive">{t("admin.cores.trafficPolicyLoadError")}</p>
          <Button variant="outline" onClick={() => policyQuery.refetch()}>{t("common.retry")}</Button>
        </CardContent>
      ) : policyQuery.data ? (
        <TrafficPolicyEditor key={buildPolicyKey(policyQuery.data)} agentHostId={agentHostId} status={policyQuery.data} />
      ) : null}
    </Card>
  );
}
