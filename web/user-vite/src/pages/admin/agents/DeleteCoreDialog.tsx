import { useTranslation } from "react-i18next";
import type { AgentCoreInstance } from "@/types";
import {
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui";

interface DeleteCoreDialogProps {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  deleteTarget: AgentCoreInstance | null;
  onConfirm: () => void;
  isPending: boolean;
}

export function DeleteCoreDialog({
  isOpen,
  onOpenChange,
  deleteTarget,
  onConfirm,
  isPending,
}: DeleteCoreDialogProps) {
  const { t } = useTranslation();
  return (
    <Dialog open={isOpen} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("admin.cores.deleteTitle")}</DialogTitle>
        </DialogHeader>
        <div className="py-2 text-sm text-muted-foreground">
          {t("admin.cores.deleteConfirm", {
            instanceId: deleteTarget?.instance_id,
            coreType: deleteTarget?.core_type,
          })}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button variant="destructive" onClick={onConfirm} disabled={isPending}>
            {isPending ? t("common.loading") : t("common.delete")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
