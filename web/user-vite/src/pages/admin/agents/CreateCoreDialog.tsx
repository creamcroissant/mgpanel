import { useTranslation } from "react-i18next";
import type { CoreInstanceForm } from "./AgentCorePanelUtils";
import {
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui";

interface CreateCoreDialogProps {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  createForm: CoreInstanceForm;
  setCreateForm: React.Dispatch<React.SetStateAction<CoreInstanceForm>>;
  availableCreateOptions: { value: string; label: string }[];
  onSubmit: () => void;
  isPending: boolean;
}

export function CreateCoreDialog({
  isOpen,
  onOpenChange,
  createForm,
  setCreateForm,
  availableCreateOptions,
  onSubmit,
  isPending,
}: CreateCoreDialogProps) {
  const { t } = useTranslation();
  return (
    <Dialog open={isOpen} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{t("admin.cores.createTitle")}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.cores.fields.coreType")}</label>
            <Select
              value={createForm.core_type}
              onValueChange={(value) => setCreateForm((prev) => ({ ...prev, core_type: value }))}
            >
              <SelectTrigger>
                <SelectValue placeholder={t("admin.cores.placeholders.coreType")} />
              </SelectTrigger>
              <SelectContent>
                {availableCreateOptions.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.cores.fields.instanceId")}</label>
            <Input
              value={createForm.instance_id}
              onChange={(event) => setCreateForm((prev) => ({ ...prev, instance_id: event.target.value }))}
              placeholder={t("admin.cores.placeholders.instanceId")}
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.cores.fields.template")}</label>
            <Input
              value={createForm.config_template_id}
              onChange={(event) => setCreateForm((prev) => ({ ...prev, config_template_id: event.target.value }))}
              placeholder={t("admin.cores.placeholders.templateId")}
            />
            <p className="text-xs text-muted-foreground">{t("admin.cores.templateHint")}</p>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button onClick={onSubmit} disabled={isPending}>
            {isPending ? t("common.loading") : t("common.create")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
