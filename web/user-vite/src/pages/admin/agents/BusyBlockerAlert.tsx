import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui";
import { formatDateTime } from "@/lib/format";
import type { AgentOperationBlocker } from "@/types";

interface BusyBlockerAlertProps {
  blocker: AgentOperationBlocker;
  onDismiss: () => void;
}

export function BusyBlockerAlert({ blocker, onDismiss }: BusyBlockerAlertProps) {
  const { t } = useTranslation();
  return (
    <div className="rounded-md border border-warning/30 bg-warning/10 p-4 text-sm">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="space-y-2">
          <div className="font-medium text-warning-foreground dark:text-warning">{t("admin.cores.busyTitle")}</div>
          <div className="text-muted-foreground">{t("admin.cores.busyDescription")}</div>
          <div className="grid gap-2 text-xs text-muted-foreground sm:grid-cols-2 lg:grid-cols-4">
            <div>
              <span className="font-medium text-foreground">{t("admin.cores.busyScope")}: </span>
              {blocker.scope}
            </div>
            <div>
              <span className="font-medium text-foreground">{t("admin.cores.busyOperation")}: </span>
              {blocker.operation_type}
            </div>
            <div>
              <span className="font-medium text-foreground">{t("admin.cores.busyStatus")}: </span>
              {blocker.status}
            </div>
            <div>
              <span className="font-medium text-foreground">{t("admin.cores.busyStartedAt")}: </span>
              {formatDateTime(blocker.created_at)}
            </div>
          </div>
          <div className="break-all font-mono text-xs text-muted-foreground">{blocker.id}</div>
        </div>
        <Button size="sm" variant="outline" onClick={onDismiss}>
          {t("common.close")}
        </Button>
      </div>
    </div>
  );
}
