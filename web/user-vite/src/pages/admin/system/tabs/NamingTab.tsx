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

const CATEGORY = "naming";

interface NamingForm {
  enabled: string;
  template: string;
}

const DEFAULT_VALUES: NamingForm = {
  enabled: "1",
  template: "{flag}{region}_{agent_name}{serial}",
};

const PREVIEW_EXAMPLES = [
  "🇭🇰HK_datawave-hk",
  "🇯🇵JP_datawave-jps-02",
  "🇸🇬SG_datawave-sg",
];

type NamingTabContentProps = {
  initialForm: NamingForm;
  onSave: (payload: NamingForm) => void;
  isSaving: boolean;
};

function NamingTabContent({ initialForm, onSave, isSaving }: NamingTabContentProps) {
  const { t } = useTranslation();
  const [form, setForm] = useState<NamingForm>(initialForm);

  return (
    <div className="space-y-6 max-w-2xl">
      <div className="flex items-center justify-between">
        <div>
          <label className="text-sm font-medium">{t("admin.system.naming.fields.enabled")}</label>
        </div>
        <Switch
          checked={form.enabled === "1"}
          onCheckedChange={(checked) =>
            setForm((prev) => ({ ...prev, enabled: checked ? "1" : "0" }))
          }
        />
      </div>

      <div className="space-y-2">
        <label className="text-sm font-medium">{t("admin.system.naming.fields.template")}</label>
        <p className="text-xs text-muted-foreground">{t("admin.system.naming.descriptions.template")}</p>
        <Input
          value={form.template}
          onChange={(e) => setForm((prev) => ({ ...prev, template: e.target.value }))}
          placeholder="{flag}{region}_{agent_name}{serial}"
        />
      </div>

      <div className="space-y-2">
        <label className="text-sm font-medium">{t("admin.system.naming.preview.title")}</label>
        <p className="text-xs text-muted-foreground">{t("admin.system.naming.preview.hint")}</p>
        <div className="rounded-md border bg-muted/30 p-3 space-y-1">
          {PREVIEW_EXAMPLES.map((example) => (
            <div key={example} className="text-sm font-mono">
              {form.enabled === "1"
                ? form.template
                    .replace("{flag}", example.match(/[^\x00-\x7F]/)?.[0] ?? "🏳")
                    .replace("{region}", example.match(/[A-Z]{2}/)?.[0] ?? "XX")
                    .replace("{agent_name}", example.split("_")[1]?.split("-")[0] ?? "agent")
                    .replace("{serial}", example.includes("-02") ? "-02" : "")
                    .replace("{type}", "vless")
                : example}
            </div>
          ))}
        </div>
      </div>

      <Button onClick={() => onSave(form)} disabled={isSaving}>
        {isSaving ? t("common.loading") : t("admin.system.settings.actions.save")}
      </Button>
    </div>
  );
}

export default function NamingTab() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();

  const queryKey = useMemo(() => [...QUERY_KEYS.ADMIN_SYSTEM, CATEGORY], []);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey,
    queryFn: () => fetchSettings(CATEGORY),
  });

  const initialForm = useMemo<NamingForm>(
    () => ({
      enabled: data?.["node_naming_enabled"] ?? DEFAULT_VALUES.enabled,
      template: data?.["node_naming_template"] ?? DEFAULT_VALUES.template,
    }),
    [data]
  );

  const saveMutation = useMutation({
    mutationFn: (payload: NamingForm) =>
      saveSettings(CATEGORY, {
        node_naming_enabled: payload.enabled,
        node_naming_template: payload.template,
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

  const handleSave = (payload: NamingForm) => {
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
        <CardTitle>{t("admin.system.naming.title")}</CardTitle>
        <CardDescription>{t("admin.system.naming.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <NamingTabContent
          initialForm={initialForm}
          onSave={handleSave}
          isSaving={saveMutation.isPending}
        />
      </CardContent>
    </Card>
  );
}