/**
 * ImportSpecDialog — dialog for importing config specs from applied instances.
 * Extracted from SpecEditorDialog for maintainability.
 */
import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Button, Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, Input, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, Switch } from "@/components/ui";
import { importConfigCenterSpecsFromApplied } from "@/api/admin";
import { QUERY_KEYS } from "@/lib/constants";
import type { ConfigCenterCoreType } from "@/types/configCenter";
import type { CoreTypeOption, ImportFormState } from "./configCenterPageTypes";

interface ImportSpecDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  importForm: ImportFormState;
  onImportFormChange: (form: ImportFormState) => void;
  selectedHostId: number | null;
  selectedCoreType: CoreTypeOption;
  fallbackCoreType: string;
}

export function ImportSpecDialog({
  open,
  onOpenChange,
  importForm,
  onImportFormChange,
  selectedHostId,
  selectedCoreType,
  fallbackCoreType,
}: ImportSpecDialogProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();

  const importMutation = useMutation({
    mutationFn: importConfigCenterSpecsFromApplied,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_CONFIG_CENTER_SPECS });
      onOpenChange(false);
      toast.success(t("admin.configCenter.messages.importSuccess"));
    },
    onError: (err: Error) =>
      toast.error(t("admin.configCenter.messages.importFailed"), {
        description: err.message,
      }),
  });

  const handleImport = () => {
    if (!selectedHostId) {
      toast.warning(t("admin.configCenter.messages.selectHostFirst"));
      return;
    }
    importMutation.mutate({
      agent_host_id: selectedHostId,
      core_type: ((selectedCoreType as string) !== "_all" ? selectedCoreType : fallbackCoreType) as ConfigCenterCoreType,
      source: importForm.source,
      filename: importForm.filename.trim() || undefined,
      tag: importForm.tag.trim() || undefined,
      enabled: importForm.enabled,
      overwrite_existing: importForm.overwrite_existing,
      change_note: importForm.change_note.trim() || undefined,
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t("admin.configCenter.import.title")}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.fields.source")}</label>
              <Select
                value={importForm.source}
                onValueChange={(value) =>
                  onImportFormChange({
                    ...importForm,
                    source: value as ImportFormState["source"],
                  })
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="legacy">{t("admin.configCenter.source.legacy")}</SelectItem>
                  <SelectItem value="managed">{t("admin.configCenter.source.managed")}</SelectItem>
                  <SelectItem value="merged">{t("admin.configCenter.source.merged")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.fields.enabled")}</label>
              <div className="flex h-10 items-center rounded-md border border-border px-3">
                <Switch
                  checked={importForm.enabled}
                  onCheckedChange={(checked) =>
                    onImportFormChange({ ...importForm, enabled: checked })
                  }
                />
              </div>
            </div>
          </div>

          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.fields.filename")}</label>
              <Input
                value={importForm.filename}
                onChange={(e) =>
                  onImportFormChange({ ...importForm, filename: e.target.value })
                }
                placeholder={t("admin.configCenter.placeholders.optional")}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.fields.tag")}</label>
              <Input
                value={importForm.tag}
                onChange={(e) =>
                  onImportFormChange({ ...importForm, tag: e.target.value })
                }
                placeholder={t("admin.configCenter.placeholders.optional")}
              />
            </div>
          </div>

          <label className="flex items-center gap-2 text-sm">
            <Switch
              checked={importForm.overwrite_existing}
              onCheckedChange={(checked) =>
                onImportFormChange({ ...importForm, overwrite_existing: checked })
              }
            />
            {t("admin.configCenter.import.overwriteExisting")}
          </label>

          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.configCenter.fields.changeNote")}</label>
            <Input
              value={importForm.change_note}
              onChange={(e) =>
                onImportFormChange({ ...importForm, change_note: e.target.value })
              }
              placeholder={t("admin.configCenter.placeholders.optional")}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button onClick={handleImport} disabled={importMutation.isPending || !selectedHostId}>
            {importMutation.isPending ? t("common.loading") : t("admin.configCenter.actions.import")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
