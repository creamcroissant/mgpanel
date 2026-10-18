import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import type { ConfigCenterSpec } from "@/types";
import {
  StatusBadge,
  Button,
  Checkbox,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Loading,
} from "@/components/ui";

interface SpecBindDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  spec: ConfigCenterSpec | null;
  hosts: Array<{ id: number; name: string }>;
  /** 当前已绑定的主机 ID（模板 spec 才有意义） */
  boundHostIds: number[];
  isLoading: boolean;
  isPending: boolean;
  onBind: (hostIds: number[]) => void;
  onUnbind: (hostId: number) => void;
}

/**
 * 模板 spec 的主机绑定对话框：勾选要下发该配置的 agent，保存时只提交差异
 * （新增的绑定、取消的解绑），避免把已绑定主机重复提交一遍。
 */
export default function SpecBindDialog({
  open,
  onOpenChange,
  spec,
  hosts,
  boundHostIds,
  isLoading,
  isPending,
  onBind,
  onUnbind,
}: SpecBindDialogProps) {
  const { t } = useTranslation();
  const [selected, setSelected] = useState<number[]>([]);

  useEffect(() => {
    if (open) {
      setSelected(boundHostIds);
    }
  }, [open, boundHostIds]);

  const toggleHost = (hostId: number) => {
    setSelected((prev) =>
      prev.includes(hostId) ? prev.filter((id) => id !== hostId) : [...prev, hostId],
    );
  };

  const allSelected = hosts.length > 0 && selected.length === hosts.length;

  const handleSave = () => {
    const toBind = selected.filter((id) => !boundHostIds.includes(id));
    const toUnbind = boundHostIds.filter((id) => !selected.includes(id));
    if (toBind.length > 0) {
      onBind(toBind);
    }
    toUnbind.forEach((hostId) => onUnbind(hostId));
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("admin.configCenter.bind.title")}</DialogTitle>
          <DialogDescription>
            {t("admin.configCenter.bind.description", { tag: spec?.tag ?? "-" })}
          </DialogDescription>
        </DialogHeader>

        {isLoading ? (
          <Loading />
        ) : hosts.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">
            {t("admin.configCenter.bind.noHosts")}
          </p>
        ) : (
          <div className="flex max-h-[50vh] flex-col gap-3 overflow-y-auto py-1">
            <div className="flex items-center justify-between gap-2">
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={() => setSelected(allSelected ? [] : hosts.map((host) => host.id))}
              >
                {allSelected ? t("admin.configCenter.bind.clearAll") : t("admin.configCenter.bind.selectAll")}
              </Button>
              <span className="text-xs text-muted-foreground">
                {t("admin.configCenter.bind.selectedCount", {
                  selected: selected.length,
                  total: hosts.length,
                })}
              </span>
            </div>

            <div className="space-y-1">
              {hosts.map((host) => {
                const checked = selected.includes(host.id);
                const wasBound = boundHostIds.includes(host.id);
                return (
                  <label
                    key={host.id}
                    className="flex cursor-pointer items-center justify-between gap-3 border border-border px-3 py-2 text-sm hover:bg-muted/50"
                  >
                    <span className="flex min-w-0 items-center gap-3">
                      <Checkbox checked={checked} onCheckedChange={() => toggleHost(host.id)} />
                      <span className="truncate">{host.name}</span>
                    </span>
                    {wasBound ? (
                      <StatusBadge tone="secondary" label={t("admin.configCenter.bind.bound")} />
                    ) : null}
                  </label>
                );
              })}
            </div>
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button onClick={handleSave} disabled={isPending || isLoading || hosts.length === 0}>
            {isPending ? t("common.loading") : t("common.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
