import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { fetchSettings, saveSettings } from "@/api/admin/settings";
import { QUERY_KEYS } from "@/lib/constants";
import {
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Input,
  Loading,
} from "@/components/ui";
import ErrorBanner from "@/components/ui/ErrorBanner";

const CATEGORY = "retention";

interface RetentionForm {
  accessLogDays: string;
  subscriptionLogDays: string;
  loginLogDays: string;
  operationLogDays: string;
  agentOperationLogDays: string;
  trafficReportDedupDays: string;
}

const DEFAULT_VALUES: Record<string, string> = {
  access_log: "7",
  subscription_log: "7",
  login_log: "30",
  operation_log: "90",
  agent_operation_log: "90",
  traffic_report_dedup: "7",
};

type RetentionTabContentProps = {
  initialForm: RetentionForm;
  onSave: (payload: RetentionForm) => void;
  isSaving: boolean;
};

function RetentionTabContent({ initialForm, onSave, isSaving }: RetentionTabContentProps) {
  const { t } = useTranslation();
  const [form, setForm] = useState<RetentionForm>(initialForm);

  const fields = [
    { key: "accessLogDays" as const, settingKey: "access_log", label: t("admin.system.retention.fields.accessLog"), description: t("admin.system.retention.descriptions.accessLog") },
    { key: "subscriptionLogDays" as const, settingKey: "subscription_log", label: t("admin.system.retention.fields.subscriptionLog"), description: t("admin.system.retention.descriptions.subscriptionLog") },
    { key: "loginLogDays" as const, settingKey: "login_log", label: t("admin.system.retention.fields.loginLog"), description: t("admin.system.retention.descriptions.loginLog") },
    { key: "operationLogDays" as const, settingKey: "operation_log", label: t("admin.system.retention.fields.operationLog"), description: t("admin.system.retention.descriptions.operationLog") },
    { key: "agentOperationLogDays" as const, settingKey: "agent_operation_log", label: t("admin.system.retention.fields.agentOperationLog"), description: t("admin.system.retention.descriptions.agentOperationLog") },
    { key: "trafficReportDedupDays" as const, settingKey: "traffic_report_dedup", label: t("admin.system.retention.fields.trafficReportDedup"), description: t("admin.system.retention.descriptions.trafficReportDedup") },
  ];

  return (
    <div className="space-y-6 max-w-2xl">
      <p className="text-sm text-muted-foreground">{t("admin.system.retention.description")}</p>
      {fields.map((field) => (
        <div key={field.key} className="space-y-2">
          <label className="text-sm font-medium">{field.label}</label>
          <p className="text-xs text-muted-foreground">{field.description}</p>
          <Input
            type="number"
            min={1}
            max={3650}
            value={form[field.key]}
            onChange={(e) => setForm((prev) => ({ ...prev, [field.key]: e.target.value }))}
            placeholder={t("admin.system.retention.days")}
          />
        </div>
      ))}
      <Button onClick={() => onSave(form)} disabled={isSaving}>
        {isSaving ? t("common.loading") : t("admin.system.settings.actions.save")}
      </Button>
    </div>
  );
}

export default function DataTab() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();

  const queryKey = useMemo(() => [...QUERY_KEYS.ADMIN_SYSTEM, CATEGORY], []);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey,
    queryFn: () => fetchSettings(CATEGORY),
  });

  // 后端 GetByCategory 返回扁平 SettingsMap（key 如 "access_log.retention_days"）
  const initialForm = useMemo<RetentionForm>(
    () => ({
      accessLogDays: data?.["access_log.retention_days"] ?? DEFAULT_VALUES.access_log,
      subscriptionLogDays: data?.["subscription_log.retention_days"] ?? DEFAULT_VALUES.subscription_log,
      loginLogDays: data?.["login_log.retention_days"] ?? DEFAULT_VALUES.login_log,
      operationLogDays: data?.["operation_log.retention_days"] ?? DEFAULT_VALUES.operation_log,
      agentOperationLogDays: data?.["agent_operation_log.retention_days"] ?? DEFAULT_VALUES.agent_operation_log,
      trafficReportDedupDays: data?.["traffic_report_dedup.retention_days"] ?? DEFAULT_VALUES.traffic_report_dedup,
    }),
    [data]
  );

  const saveMutation = useMutation({
    mutationFn: (payload: RetentionForm) =>
      saveSettings(CATEGORY, {
        "access_log.retention_days": payload.accessLogDays,
        "subscription_log.retention_days": payload.subscriptionLogDays,
        "login_log.retention_days": payload.loginLogDays,
        "operation_log.retention_days": payload.operationLogDays,
        "agent_operation_log.retention_days": payload.agentOperationLogDays,
        "traffic_report_dedup.retention_days": payload.trafficReportDedupDays,
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

  const handleSave = (payload: RetentionForm) => {
    saveMutation.mutate(payload);
  };

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
    <Card>
      <CardHeader>
        <CardTitle>{t("admin.system.retention.title")}</CardTitle>
        <CardDescription>{t("admin.system.retention.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <RetentionTabContent
          initialForm={initialForm}
          onSave={handleSave}
          isSaving={saveMutation.isPending}
        />
      </CardContent>
    </Card>
  );
}