import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { fetchSettings, saveSettings, testSMTP } from "@/api/admin/settings";
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Switch,
} from "@/components/ui";
import ErrorBanner from "@/components/ui/ErrorBanner";

const CATEGORY = "smtp";
const ALERT_CATEGORY = "smtp_alerts";

interface EmailForm {
  smtpHost: string;
  smtpPort: string;
  smtpEncryption: string;
  smtpUsername: string;
  smtpPassword: string;
  smtpFrom: string;
  smtpTo: string;
  alertTrafficEnabled: boolean;
  alertTrafficThreshold: string;
  alertExpireEnabled: boolean;
  alertExpireThreshold: string;
}

function toBool(value?: string) {
  return value === "true" || value === "1";
}

function isEmail(value: string) {
  if (!value) return false;
  return /^\S+@\S+\.\S+$/u.test(value);
}

type EmailTabContentProps = {
  initialForm: EmailForm;
};

function EmailTabContent({ initialForm }: EmailTabContentProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [form, setForm] = useState<EmailForm>(initialForm);

  const smtpQueryKey = useMemo(() => [...QUERY_KEYS.ADMIN_SYSTEM, CATEGORY], []);
  const alertQueryKey = useMemo(() => [...QUERY_KEYS.ADMIN_SYSTEM, ALERT_CATEGORY], []);

  const saveSMTPMutation = useMutation({
    mutationFn: (payload: EmailForm) =>
      saveSettings(CATEGORY, {
        smtp_host: payload.smtpHost.trim(),
        smtp_port: payload.smtpPort.trim(),
        smtp_encryption: payload.smtpEncryption,
        smtp_username: payload.smtpUsername.trim(),
        ...(payload.smtpPassword.trim()
          ? { smtp_password: payload.smtpPassword.trim() }
          : {}),
        smtp_from: payload.smtpFrom.trim(),
        smtp_to: payload.smtpTo.trim(),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: smtpQueryKey });
      setForm((prev) => ({ ...prev, smtpPassword: "" }));
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

  const saveAlertMutation = useMutation({
    mutationFn: (payload: EmailForm) =>
      saveSettings(ALERT_CATEGORY, {
        alert_traffic_enabled: payload.alertTrafficEnabled ? "true" : "false",
        alert_traffic_threshold: payload.alertTrafficThreshold.trim(),
        alert_expire_enabled: payload.alertExpireEnabled ? "true" : "false",
        alert_expire_days: payload.alertExpireThreshold.trim(),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: alertQueryKey });
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

  const testMutation = useMutation({
    mutationFn: (payload: EmailForm) =>
      testSMTP({
        host: payload.smtpHost.trim(),
        port: Number(payload.smtpPort),
        encryption: payload.smtpEncryption,
        username: payload.smtpUsername.trim(),
        password: payload.smtpPassword.trim(),
        from_address: payload.smtpFrom.trim(),
        to_address: payload.smtpTo.trim(),
      }),
    onSuccess: () => {
      toast.success(t("common.success"), {
        description: t("admin.system.settings.messages.testSuccess"),
      });
    },
    onError: (err: Error) => {
      toast.error(t("common.error"), {
        description: err.message,
      });
    },
  });

  const handleSaveSMTP = () => {
    if (!isEmail(form.smtpFrom) || (form.smtpTo && !isEmail(form.smtpTo))) {
      toast.warning(t("common.error"), {
        description: t("admin.system.settings.tooltips.smtpFrom"),
      });
      return;
    }
    saveSMTPMutation.mutate(form);
  };

  const handleSaveAlerts = () => {
    saveAlertMutation.mutate(form);
  };

  const handleTest = () => {
    if (!isEmail(form.smtpFrom) || !isEmail(form.smtpTo)) {
      toast.warning(t("common.error"), {
        description: t("admin.system.settings.tooltips.smtpFrom"),
      });
      return;
    }
    testMutation.mutate(form);
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>{t("admin.system.settings.tabs.email")}</CardTitle>
          <CardDescription>{t("admin.system.settings.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-6 max-w-2xl">
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <label className="text-sm font-medium">
                  {t("admin.system.settings.fields.smtpHost")}
                </label>
                <Input
                  value={form.smtpHost}
                  onChange={(e) => setForm((prev) => ({ ...prev, smtpHost: e.target.value }))}
                  placeholder={t("admin.system.settings.placeholders.smtpHost")}
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">
                  {t("admin.system.settings.fields.smtpPort")}
                </label>
                <Input
                  type="number"
                  min={1}
                  value={form.smtpPort}
                  onChange={(e) => setForm((prev) => ({ ...prev, smtpPort: e.target.value }))}
                  placeholder={t("admin.system.settings.placeholders.smtpPort")}
                />
              </div>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">
                {t("admin.system.settings.fields.smtpEncryption")}
              </label>
              <Select
                value={form.smtpEncryption}
                onValueChange={(value) => setForm((prev) => ({ ...prev, smtpEncryption: value }))}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t("admin.system.settings.fields.smtpEncryption")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">{t("admin.system.settings.options.smtpEncryption.none")}</SelectItem>
                  <SelectItem value="starttls">{t("admin.system.settings.options.smtpEncryption.starttls")}</SelectItem>
                  <SelectItem value="ssl">{t("admin.system.settings.options.smtpEncryption.ssl")}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <label className="text-sm font-medium">
                  {t("admin.system.settings.fields.smtpUsername")}
                </label>
                <Input
                  value={form.smtpUsername}
                  onChange={(e) => setForm((prev) => ({ ...prev, smtpUsername: e.target.value }))}
                  placeholder={t("admin.system.settings.placeholders.smtpUsername")}
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">
                  {t("admin.system.settings.fields.smtpPassword")}
                </label>
                <Input
                  type="password"
                  value={form.smtpPassword}
                  onChange={(e) => setForm((prev) => ({ ...prev, smtpPassword: e.target.value }))}
                  placeholder={t("admin.system.settings.placeholders.smtpPassword")}
                />
              </div>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <label className="text-sm font-medium">
                  {t("admin.system.settings.fields.smtpFrom")}
                </label>
                <Input
                  value={form.smtpFrom}
                  onChange={(e) => setForm((prev) => ({ ...prev, smtpFrom: e.target.value }))}
                  placeholder={t("admin.system.settings.placeholders.smtpFrom")}
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">
                  {t("admin.system.settings.fields.smtpTo")}
                </label>
                <Input
                  value={form.smtpTo}
                  onChange={(e) => setForm((prev) => ({ ...prev, smtpTo: e.target.value }))}
                  placeholder={t("admin.system.settings.placeholders.smtpTo")}
                />
              </div>
            </div>

            <div className="flex flex-wrap gap-2">
              <Button onClick={handleSaveSMTP} disabled={saveSMTPMutation.isPending}>
                {saveSMTPMutation.isPending ? t("common.loading") : t("admin.system.settings.actions.save")}
              </Button>
              <Button variant="outline" onClick={handleTest} disabled={testMutation.isPending}>
                {testMutation.isPending ? t("common.loading") : t("admin.system.settings.actions.test")}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("admin.system.settings.fields.alertTrafficEnabled")}</CardTitle>
          <CardDescription>{t("admin.system.settings.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-6 max-w-2xl">
            <div className="flex items-center justify-between rounded-md border border-border px-3 py-2">
              <div className="space-y-1">
                <p className="text-sm font-medium">
                  {t("admin.system.settings.fields.alertTrafficEnabled")}
                </p>
                <p className="text-xs text-muted-foreground">
                  {t("admin.system.settings.tooltips.alertTrafficEnabled")}
                </p>
              </div>
              <Switch
                checked={form.alertTrafficEnabled}
                onCheckedChange={(checked) =>
                  setForm((prev) => ({ ...prev, alertTrafficEnabled: checked }))
                }
              />
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">
                {t("admin.system.settings.fields.alertTrafficThreshold")}
              </label>
              <Input
                type="number"
                min={0}
                value={form.alertTrafficThreshold}
                onChange={(e) =>
                  setForm((prev) => ({ ...prev, alertTrafficThreshold: e.target.value }))
                }
                placeholder={t("admin.system.settings.placeholders.alertTrafficThreshold")}
              />
            </div>

            <div className="flex items-center justify-between rounded-md border border-border px-3 py-2">
              <div className="space-y-1">
                <p className="text-sm font-medium">
                  {t("admin.system.settings.fields.alertExpireEnabled")}
                </p>
                <p className="text-xs text-muted-foreground">
                  {t("admin.system.settings.tooltips.alertExpireEnabled")}
                </p>
              </div>
              <Switch
                checked={form.alertExpireEnabled}
                onCheckedChange={(checked) =>
                  setForm((prev) => ({ ...prev, alertExpireEnabled: checked }))
                }
              />
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">
                {t("admin.system.settings.fields.alertExpireThreshold")}
              </label>
              <Input
                type="number"
                min={0}
                value={form.alertExpireThreshold}
                onChange={(e) =>
                  setForm((prev) => ({ ...prev, alertExpireThreshold: e.target.value }))
                }
                placeholder={t("admin.system.settings.placeholders.alertExpireThreshold")}
              />
            </div>

            <Button onClick={handleSaveAlerts} disabled={saveAlertMutation.isPending}>
              {saveAlertMutation.isPending ? t("common.loading") : t("admin.system.settings.actions.save")}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

export default function EmailTab() {
  const { t } = useTranslation();

  const smtpQueryKey = useMemo(() => [...QUERY_KEYS.ADMIN_SYSTEM, CATEGORY], []);
  const alertQueryKey = useMemo(() => [...QUERY_KEYS.ADMIN_SYSTEM, ALERT_CATEGORY], []);

  const smtpQuery = useQuery({
    queryKey: smtpQueryKey,
    queryFn: () => fetchSettings(CATEGORY),
  });

  const alertQuery = useQuery({
    queryKey: alertQueryKey,
    queryFn: () => fetchSettings(ALERT_CATEGORY),
  });

  const initialForm = useMemo<EmailForm>(
    () => ({
      smtpHost: smtpQuery.data?.smtp_host ?? "",
      smtpPort: smtpQuery.data?.smtp_port ?? "",
      smtpEncryption: smtpQuery.data?.smtp_encryption ?? "none",
      smtpUsername: smtpQuery.data?.smtp_username ?? "",
      smtpPassword: "",
      smtpFrom: smtpQuery.data?.smtp_from ?? "",
      smtpTo: smtpQuery.data?.smtp_to ?? "",
      alertTrafficEnabled: toBool(alertQuery.data?.alert_traffic_enabled),
      alertTrafficThreshold: alertQuery.data?.alert_traffic_threshold ?? "",
      alertExpireEnabled: toBool(alertQuery.data?.alert_expire_enabled),
      alertExpireThreshold: alertQuery.data?.alert_expire_days ?? "",
    }),
    [alertQuery.data, smtpQuery.data]
  );

  if (smtpQuery.isLoading || alertQuery.isLoading) return <Loading />;

  if (smtpQuery.error || alertQuery.error) {
    return (
      <ErrorBanner
        message={t("admin.system.settings.messages.loadError")}
        onRetry={() => {
          smtpQuery.refetch();
          alertQuery.refetch();
        }}
      />
    );
  }

  return <EmailTabContent initialForm={initialForm} />;
}
