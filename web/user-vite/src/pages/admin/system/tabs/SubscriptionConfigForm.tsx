import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { saveSettings } from "@/api/admin/settings";
import { QUERY_KEYS } from "@/lib/constants";
import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Switch,
  Textarea,
} from "@/components/ui";

const CATEGORY = "subscription";

interface SubscriptionForm {
  subscribeUrls: string;
  subscribePath: string;
  trafficResetMode: string;
  trafficResetDay: string;
  allowChange: boolean;
  injectMeta: boolean;
  injectProtocol: boolean;
}

interface SubscriptionConfigFormProps {
  initialForm: SubscriptionForm;
}

export default function SubscriptionConfigForm({
  initialForm,
}: SubscriptionConfigFormProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [form, setForm] = useState<SubscriptionForm>(initialForm);
  const [pendingPath, setPendingPath] = useState("");
  const [confirmPathOpen, setConfirmPathOpen] = useState(false);

  const queryKey = useMemo(
    () => [...QUERY_KEYS.ADMIN_SYSTEM, CATEGORY],
    [],
  );

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
            onChange={(e) =>
              setForm((prev) => ({
                ...prev,
                subscribeUrls: e.target.value,
              }))
            }
            placeholder={t(
              "admin.system.settings.placeholders.subscribeUrls",
            )}
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
            placeholder={t(
              "admin.system.settings.placeholders.subscribePath",
            )}
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
            onValueChange={(value) =>
              setForm((prev) => ({ ...prev, trafficResetMode: value }))
            }
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="global">
                {t("admin.system.subscription.mode_global")}
              </SelectItem>
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
              onChange={(e) =>
                setForm((prev) => ({
                  ...prev,
                  trafficResetDay: e.target.value,
                }))
              }
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
              onCheckedChange={(checked) =>
                setForm((prev) => ({ ...prev, allowChange: checked }))
              }
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
              onCheckedChange={(checked) =>
                setForm((prev) => ({ ...prev, injectMeta: checked }))
              }
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
              onCheckedChange={(checked) =>
                setForm((prev) => ({ ...prev, injectProtocol: checked }))
              }
            />
          </div>
        </div>

        <Button onClick={handleSave} disabled={saveMutation.isPending}>
          {saveMutation.isPending
            ? t("common.loading")
            : t("admin.system.settings.actions.save")}
        </Button>
      </div>

      <Dialog open={confirmPathOpen} onOpenChange={setConfirmPathOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("common.confirm")}</DialogTitle>
            <DialogDescription>
              {t("admin.system.settings.tooltips.subscribePath")}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={cancelPathChange}>
              {t("common.cancel")}
            </Button>
            <Button onClick={confirmPathChange}>
              {t("common.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
