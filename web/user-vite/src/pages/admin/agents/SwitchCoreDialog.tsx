import { useTranslation } from "react-i18next";
import type { CoreSwitchForm } from "./AgentCorePanelUtils";
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

interface SwitchCoreDialogProps {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  switchForm: CoreSwitchForm;
  setSwitchForm: React.Dispatch<React.SetStateAction<CoreSwitchForm>>;
  instanceOptions: { value: string; label: string }[];
  coreOptions: { value: string; label: string }[];
  onSubmit: () => void;
  isPending: boolean;
}

export function SwitchCoreDialog({
  isOpen,
  onOpenChange,
  switchForm,
  setSwitchForm,
  instanceOptions,
  coreOptions,
  onSubmit,
  isPending,
}: SwitchCoreDialogProps) {
  const { t } = useTranslation();
  return (
    <Dialog open={isOpen} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{t("admin.cores.switchTitle")}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.cores.fields.fromInstance")}</label>
            <Select
              value={switchForm.from_instance_id}
              onValueChange={(value) => setSwitchForm((prev) => ({ ...prev, from_instance_id: value }))}
            >
              <SelectTrigger>
                <SelectValue placeholder={t("admin.cores.placeholders.fromInstance")} />
              </SelectTrigger>
              <SelectContent>
                {instanceOptions.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.cores.fields.coreType")}</label>
            <Select
              value={switchForm.to_core_type}
              onValueChange={(value) => setSwitchForm((prev) => ({ ...prev, to_core_type: value }))}
            >
              <SelectTrigger>
                <SelectValue placeholder={t("admin.cores.placeholders.coreType")} />
              </SelectTrigger>
              <SelectContent>
                {coreOptions.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.cores.fields.template")}</label>
            <Input
              value={switchForm.config_template_id}
              onChange={(event) => setSwitchForm((prev) => ({ ...prev, config_template_id: event.target.value }))}
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
            {isPending ? t("common.loading") : t("admin.cores.switch")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
