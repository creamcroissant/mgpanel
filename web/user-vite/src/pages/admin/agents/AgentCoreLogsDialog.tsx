import { useTranslation } from "react-i18next";
import { History } from "lucide-react";
import { formatDateTime } from "@/lib/format";
import {
  StatusBadge,
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  Input,
  Loading,
  Pagination,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui";
import type { AgentCoreSwitchLog } from "@/types";
import { FILTER_ALL } from "./agentCorePanelUtils";

interface AgentCoreLogsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  logs: AgentCoreSwitchLog[];
  isLoading: boolean;
  error: Error | null;
  onRetry: () => void;
  status: string;
  onStatusChange: (status: string) => void;
  dateRange: { start: string; end: string };
  onDateRangeChange: (next: { start?: string; end?: string }) => void;
  page: number;
  totalPages: number;
  onPageChange: (page: number) => void;
}

export function AgentCoreLogsDialog({
  open,
  onOpenChange,
  logs,
  isLoading,
  error,
  onRetry,
  status,
  onStatusChange,
  dateRange,
  onDateRangeChange,
  page,
  totalPages,
  onPageChange,
}: AgentCoreLogsDialogProps) {
  const { t } = useTranslation();
  return (
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="sm:max-w-5xl">
          <DialogHeader>
            <DialogTitle>{t("admin.cores.logsTitle")}</DialogTitle>
          </DialogHeader>
          <div className="flex max-h-[calc(100dvh-8rem)] flex-col gap-4 overflow-y-auto py-2 pr-1">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.cores.logsStart")}</label>
                <Input
                  type="date"
                  value={dateRange.start}
                  onChange={(event) => onDateRangeChange({ start: event.target.value })}
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.cores.logsEnd")}</label>
                <Input
                  type="date"
                  value={dateRange.end}
                  onChange={(event) => onDateRangeChange({ end: event.target.value })}
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.cores.status")}</label>
                <Select value={status} onValueChange={(value) => {
                  onStatusChange(value);
                  onPageChange(1);
                }}>
                  <SelectTrigger>
                    <SelectValue placeholder={t("admin.cores.statusPlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={FILTER_ALL}>{t("common.all")}</SelectItem>
                    <SelectItem value="pending">{t("admin.cores.status.pending")}</SelectItem>
                    <SelectItem value="claimed">{t("admin.cores.status.claimed")}</SelectItem>
                    <SelectItem value="in_progress">{t("admin.cores.status.in_progress")}</SelectItem>
                    <SelectItem value="completed">{t("admin.cores.status.completed")}</SelectItem>
                    <SelectItem value="failed">{t("admin.cores.status.failed")}</SelectItem>
                    <SelectItem value="rolled_back">{t("admin.cores.status.rolled_back")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <Button variant="outline" onClick={() => onDateRangeChange({ start: "", end: "" })}>
                {t("admin.cores.logsClear")}
              </Button>
            </div>

            {isLoading ? (
              <Loading />
            ) : error ? (
              <div className="flex flex-col items-center justify-center gap-3 py-6">
                <p className="text-sm text-destructive">{t("admin.cores.logsLoadError")}</p>
                <Button variant="outline" onClick={() => onRetry()}>
                  {t("common.retry")}
                </Button>
              </div>
            ) : logs.length === 0 ? (
              <EmptyState
                icon={<History className="h-full w-full" />}
                title={t("admin.cores.logsEmpty")}
                description={t("admin.cores.logsEmptyDescription")}
                size="sm"
              />
            ) : (
              <Table aria-label={t("admin.cores.logsTitle")}>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("admin.cores.logsTime")}</TableHead>
                    <TableHead>{t("admin.cores.logsFrom")}</TableHead>
                    <TableHead>{t("admin.cores.logsTo")}</TableHead>
                    <TableHead>{t("admin.cores.status")}</TableHead>
                    <TableHead>{t("admin.cores.logsOperator")}</TableHead>
                    <TableHead>{t("admin.cores.logsDetail")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {logs.map((log: AgentCoreSwitchLog) => (
                    <TableRow key={log.id}>
                      <TableCell className="whitespace-nowrap">{formatDateTime(log.created_at)}</TableCell>
                      <TableCell className="text-xs text-muted-foreground">
                        {log.from_core_type || "-"}
                        <div className="font-mono">{log.from_instance_id || "-"}</div>
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground">
                        {log.to_core_type}
                        <div className="font-mono">{log.to_instance_id}</div>
                      </TableCell>
                      <TableCell>
                        <StatusBadge status={log.status} label={log.status} />
                      </TableCell>
                      <TableCell>{log.operator_id ?? "-"}</TableCell>
                      <TableCell>
                        <div className="max-w-xl truncate text-xs text-muted-foreground">{log.detail || "-"}</div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}

            <Pagination page={page} totalPages={totalPages} onPageChange={onPageChange} />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => onOpenChange(false)}>
              {t("common.close")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
  );
}
