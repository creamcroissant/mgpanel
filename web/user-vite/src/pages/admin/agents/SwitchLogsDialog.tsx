import { History } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { AgentCoreSwitchLog } from "@/types";
import {
  Badge,
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
import { formatDateTime } from "@/lib/format";
import { FILTER_ALL, getStatusVariant } from "./AgentCorePanelUtils";

interface SwitchLogsDialogProps {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  logsQuery: {
    isLoading: boolean;
    error: unknown;
    refetch: () => void;
    data?: { logs: AgentCoreSwitchLog[]; total?: number } | null;
  };
  logsStatus: string;
  setLogsStatus: React.Dispatch<React.SetStateAction<string>>;
  logsPage: number;
  setLogsPage: React.Dispatch<React.SetStateAction<number>>;
  logsDateRange: { start: string; end: string };
  setLogsDateRange: React.Dispatch<React.SetStateAction<{ start: string; end: string }>>;
}

export function SwitchLogsDialog({
  isOpen,
  onOpenChange,
  logsQuery,
  logsStatus,
  setLogsStatus,
  logsPage,
  setLogsPage,
  logsDateRange,
  setLogsDateRange,
}: SwitchLogsDialogProps) {
  const { t } = useTranslation();

  const handleDateChange = (next: { start?: string; end?: string }) => {
    setLogsDateRange((prev) => ({ ...prev, ...next }));
    setLogsPage(1);
  };

  const handleClose = () => {
    setLogsPage(1);
    setLogsStatus(FILTER_ALL);
    setLogsDateRange({ start: "", end: "" });
    onOpenChange(false);
  };

  const logs = (logsQuery.data as { logs: AgentCoreSwitchLog[]; total?: number } | null)?.logs ?? [];
  const logsTotal = (logsQuery.data as { logs: AgentCoreSwitchLog[]; total?: number } | null)?.total ?? 0;
  const logsTotalPages = Math.ceil(logsTotal / 10);

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
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
                value={logsDateRange.start}
                onChange={(event) => handleDateChange({ start: event.target.value })}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.cores.logsEnd")}</label>
              <Input
                type="date"
                value={logsDateRange.end}
                onChange={(event) => handleDateChange({ end: event.target.value })}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.cores.logsStatus")}</label>
              <Select value={logsStatus} onValueChange={(value) => {
                setLogsStatus(value);
                setLogsPage(1);
              }}>
                <SelectTrigger>
                  <SelectValue placeholder={t("admin.cores.logsStatusPlaceholder")} />
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
            <Button variant="outline" onClick={() => handleDateChange({ start: "", end: "" })}>
              {t("admin.cores.logsClear")}
            </Button>
          </div>

          {logsQuery.isLoading ? (
            <Loading />
          ) : logsQuery.error ? (
            <div className="flex flex-col items-center justify-center gap-3 py-6">
              <p className="text-sm text-destructive">{t("admin.cores.logsLoadError")}</p>
              <Button variant="outline" onClick={() => logsQuery.refetch()}>
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
                  <TableHead>{t("admin.cores.logsStatus")}</TableHead>
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
                      <Badge variant={getStatusVariant(log.status)}>{log.status}</Badge>
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

          <Pagination page={logsPage} totalPages={logsTotalPages} onPageChange={setLogsPage} />
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={handleClose}>
            {t("common.close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
