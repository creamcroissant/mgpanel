import { useTranslation } from "react-i18next";
import { formatDateTime } from "@/lib/format";
import type { SubscriptionFilterReasonEntry } from "@/types/admin";
import { Badge, TableCell, TableRow } from "@/components/ui";

function getFilterReasonLabel(reason: string, t: (key: string) => string): string {
  return t(`admin.system.subscription.filterReasonOptions.${reason}`);
}

function getSourceTypeLabel(type: string, t: (key: string) => string): string {
  return t(`admin.system.subscription.sourceTypeOptions.${type}`);
}

export default function FilterReasonRow({
  reason,
}: { reason: SubscriptionFilterReasonEntry }) {
  const { t } = useTranslation();

  return (
    <TableRow>
      <TableCell className="font-medium">{reason.node_name || "-"}</TableCell>
      <TableCell>
        <Badge variant="warning">{getFilterReasonLabel(reason.reason, t)}</Badge>
      </TableCell>
      <TableCell>
        <div className="space-y-1 text-sm">
          <p>{getSourceTypeLabel(reason.source_type, t)}</p>
          <p className="text-xs text-muted-foreground">
            {t("admin.system.subscription.sourceId").replace(
              "{{id}}",
              String(reason.source_id),
            )}
          </p>
        </div>
      </TableCell>
      <TableCell className="max-w-72 truncate text-muted-foreground">
        {reason.detail || t("admin.system.subscription.noDetail")}
      </TableCell>
      <TableCell>{formatDateTime(reason.created_at)}</TableCell>
    </TableRow>
  );
}
