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
  Switch,
} from "@/components/ui";
import ErrorBanner from "@/components/ui/ErrorBanner";

const CATEGORY = "general";

interface GeneralSettingsForm {
  siteName: string;
  siteDesc: string;
  siteUrl: string;
  siteLogo: string;
  forceHttps: boolean;
}

function toBool(value?: string) {
  return value === "true" || value === "1";
}

function normalizeUrl(value: string) {
  return value.trim().replace(/\/+$/u, "");
}

type GeneralTabContentProps = {
  initialForm: GeneralSettingsForm;
  onSave: (payload: GeneralSettingsForm) => void;
  isSaving: boolean;
};

function GeneralTabContent({ initialForm, onSave, isSaving }: GeneralTabContentProps) {
  const { t } = useTranslation();
  const [form, setForm] = useState<GeneralSettingsForm>(initialForm);

  return (
    <div className="space-y-6 max-w-2xl">
      <div className="space-y-2">
        <label className="text-sm font-medium">{t("admin.system.settings.fields.siteName")}</label>
        <Input
          value={form.siteName}
          onChange={(e) => setForm((prev) => ({ ...prev, siteName: e.target.value }))}
          placeholder={t("admin.system.settings.placeholders.siteName")}
        />
      </div>

      <div className="space-y-2">
        <label className="text-sm font-medium">{t("admin.system.settings.fields.siteDesc")}</label>
        <Input
          value={form.siteDesc}
          onChange={(e) => setForm((prev) => ({ ...prev, siteDesc: e.target.value }))}
          placeholder={t("admin.system.settings.placeholders.siteDesc")}
        />
      </div>

      <div className="space-y-2">
        <label className="text-sm font-medium">{t("admin.system.settings.fields.siteUrl")}</label>
        <Input
          value={form.siteUrl}
          onChange={(e) => setForm((prev) => ({ ...prev, siteUrl: e.target.value }))}
          placeholder={t("admin.system.settings.placeholders.siteUrl")}
        />
      </div>

      <div className="space-y-2">
        <label className="text-sm font-medium">{t("admin.system.settings.fields.siteLogo")}</label>
        <Input
          value={form.siteLogo}
          onChange={(e) => setForm((prev) => ({ ...prev, siteLogo: e.target.value }))}
          placeholder={t("admin.system.settings.placeholders.siteLogo")}
        />
      </div>

      <div className="flex items-center justify-between rounded-md border border-border px-3 py-2">
        <div className="space-y-1">
          <p className="text-sm font-medium">{t("admin.system.settings.fields.forceHttps")}</p>
          <p className="text-xs text-muted-foreground">{t("admin.system.settings.tooltips.forceHttps")}</p>
        </div>
        <Switch
          checked={form.forceHttps}
          onCheckedChange={(checked) => setForm((prev) => ({ ...prev, forceHttps: checked }))}
        />
      </div>

      <Button onClick={() => onSave(form)} disabled={isSaving}>
        {isSaving ? t("common.loading") : t("admin.system.settings.actions.save")}
      </Button>
    </div>
  );
}

export default function GeneralTab() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();

  const queryKey = useMemo(() => [...QUERY_KEYS.ADMIN_SYSTEM, CATEGORY], []);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey,
    queryFn: () => fetchSettings(CATEGORY),
  });

  const initialForm = useMemo<GeneralSettingsForm>(
    () => ({
      siteName: data?.app_name ?? "",
      siteDesc: data?.app_description ?? "",
      siteUrl: data?.app_url ?? "",
      siteLogo: data?.logo ?? "",
      forceHttps: toBool(data?.force_https),
    }),
    [data]
  );

  const saveMutation = useMutation({
    mutationFn: (payload: GeneralSettingsForm) =>
      saveSettings(CATEGORY, {
        app_name: payload.siteName.trim(),
        app_description: payload.siteDesc.trim(),
        app_url: normalizeUrl(payload.siteUrl),
        logo: payload.siteLogo.trim(),
        force_https: payload.forceHttps ? "true" : "false",
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

  const handleSave = (payload: GeneralSettingsForm) => {
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
        <CardTitle>{t("admin.system.settings.tabs.general")}</CardTitle>
        <CardDescription>{t("admin.system.settings.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <GeneralTabContent
          initialForm={initialForm}
          onSave={handleSave}
          isSaving={saveMutation.isPending}
        />
      </CardContent>
    </Card>
  );
}
