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
  Loading,
  Switch,
} from "@/components/ui";
import ErrorBanner from "@/components/ui/ErrorBanner";

const CATEGORY = "mcp";

interface MCPForm {
  enabled: boolean;
}

function toBool(value?: string) {
  return value === "true" || value === "1";
}

type MCPTabContentProps = {
  initialForm: MCPForm;
  onSave: (payload: MCPForm) => void;
  isSaving: boolean;
};

function MCPTabContent({ initialForm, onSave, isSaving }: MCPTabContentProps) {
  const { t } = useTranslation();
  const [enabled, setEnabled] = useState(initialForm.enabled);

  const handleSave = () => {
    onSave({ enabled });
  };

  return (
    <div className="space-y-6 max-w-2xl">
      <div className="flex items-center justify-between rounded-lg border p-4">
        <div className="space-y-0.5">
          <label className="text-sm font-medium" htmlFor="mcp-toggle">
            {t("admin.system.settings.fields.mcpEnabled")}
          </label>
          <p className="text-xs text-muted-foreground">
            {t("admin.system.settings.tooltips.mcpEnabled")}
          </p>
        </div>
        <Switch
          id="mcp-toggle"
          checked={enabled}
          onCheckedChange={setEnabled}
        />
      </div>

      <Button onClick={handleSave} disabled={isSaving}>
        {isSaving ? t("common.loading") : t("admin.system.settings.actions.save")}
      </Button>

      {enabled !== initialForm.enabled && (
        <p className="text-xs text-amber-500">
          {t("admin.system.settings.messages.mcpRestartHint")}
        </p>
      )}
    </div>
  );
}

export default function MCPTab() {
  const { t } = useTranslation();

  const settingsQueryKey = useMemo(() => [...QUERY_KEYS.ADMIN_SYSTEM, CATEGORY], []);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: settingsQueryKey,
    queryFn: () => fetchSettings(CATEGORY),
  });

  const queryClient = useQueryClient();

  const saveMutation = useMutation({
    mutationFn: (form: MCPForm) =>
      saveSettings(CATEGORY, {
        enabled: form.enabled ? "true" : "false",
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

  const initialForm = useMemo<MCPForm>(
    () => ({
      enabled: toBool(data?.enabled),
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
    <Card>
      <CardHeader>
        <CardTitle>{t("admin.system.settings.tabs.mcp")}</CardTitle>
        <CardDescription>{t("admin.system.settings.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <MCPTabContent
          initialForm={initialForm}
          onSave={(form) => saveMutation.mutate(form)}
          isSaving={saveMutation.isPending}
        />
      </CardContent>
    </Card>
  );
}
