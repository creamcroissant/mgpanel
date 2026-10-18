import { useTranslation } from "react-i18next";
import { History } from "lucide-react";
import { formatDateTime } from "@/lib/format";
import {
  StatusBadge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
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
import type { AgentCoreOperation } from "@/types";
import { AgentCommandQueuePanel } from "./AgentCommandQueuePanel";
import { OperationLogTimeline } from "./OperationLogTimeline";
import { buildOperationTarget, describeOperationPayload, FILTER_ALL } from "./agentCorePanelUtils";

interface AgentCoreOperationsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  agentHostId: number;
  operations: AgentCoreOperation[];
  recentOperations: AgentCoreOperation[];
  selectedOperation: AgentCoreOperation | null;
  onSelectOperation: (id: string) => void;
  hasActiveOperations: boolean;
  status: string;
  onStatusChange: (status: string) => void;
  operationType: string;
  onTypeChange: (type: string) => void;
  dateRange: { start: string; end: string };
  onDateRangeChange: (next: { start?: string; end?: string }) => void;
  page: number;
  totalPages: number;
  onPageChange: (page: number) => void;
  isLoading: boolean;
}

export function AgentCoreOperationsDialog({
  open,
  onOpenChange,
  agentHostId,
  operations,
  recentOperations,
  selectedOperation,
  onSelectOperation,
  hasActiveOperations,
  status,
  onStatusChange,
  operationType,
  onTypeChange,
  dateRange,
  onDateRangeChange,
  page,
  totalPages,
  onPageChange,
  isLoading,
}: AgentCoreOperationsDialogProps) {
  const { t } = useTranslation();
  return (
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="sm:max-w-4xl">
          <DialogHeader>
            <DialogTitle>{t("admin.cores.operationsTitle")}</DialogTitle>
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
                <label className="text-sm font-medium">{t("admin.cores.logsStatus")}</label>
                <Select value={status} onValueChange={(value) => {
                  onStatusChange(value);
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
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.cores.operationType")}</label>
                <Select value={operationType} onValueChange={(value) => {
                  onTypeChange(value);
                }}>
                  <SelectTrigger>
                    <SelectValue placeholder={t("admin.cores.operationTypePlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={FILTER_ALL}>{t("common.all")}</SelectItem>
                    <SelectItem value="create">{t("admin.cores.operationType.create")}</SelectItem>
                    <SelectItem value="switch">{t("admin.cores.operationType.switch")}</SelectItem>
                    <SelectItem value="install">{t("admin.cores.operationType.install")}</SelectItem>
                    <SelectItem value="ensure">{t("admin.cores.operationType.ensure")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            <AgentCommandQueuePanel agentHostId={agentHostId} />

            <Card className="border border-border">
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">{t("admin.cores.operationsSummaryTitle")}</CardTitle>
                <CardDescription>
                  {hasActiveOperations ? t("admin.cores.operationsPolling") : t("admin.cores.operationsIdle")}
                </CardDescription>
              </CardHeader>
              <CardContent>
                {recentOperations.length === 0 ? (
                  <EmptyState
                    icon={<History className="h-full w-full" />}
                    title={t("admin.cores.operationsEmpty")}
                    description={t("admin.cores.operationsEmptyDescription")}
                    size="sm"
                  />
                ) : (
                  <div className="space-y-2">
                    {recentOperations.map((operation) => {
                      const selected = selectedOperation?.id === operation.id;
                      return (
                        <button
                          type="button"
                          key={operation.id}
                          className={`flex w-full flex-col gap-2 rounded-none border border-border p-3 text-left transition-colors sm:flex-row sm:items-center sm:justify-between ${
                            selected ? "border-primary bg-primary/5" : "border-border hover:bg-muted/50"
                          }`}
                          onClick={() => onSelectOperation(operation.id)}
                        >
                          <div className="space-y-1">
                            <div className="flex flex-wrap items-center gap-2">
                              <StatusBadge status={operation.status} label={operation.status} />
                              <span className="text-sm font-medium">
                                {t(`admin.cores.operationType.${operation.operation_type}`)}
                              </span>
                            </div>
                            <div className="text-xs text-muted-foreground">{buildOperationTarget(operation)}</div>
                            <div className="text-xs text-muted-foreground">{formatDateTime(operation.created_at)}</div>
                          </div>
                          <div className="max-w-[360px] truncate text-xs text-muted-foreground">
                            {describeOperationPayload(operation)}
                          </div>
                        </button>
                      );
                    })}
                  </div>
                )}
              </CardContent>
            </Card>

            <OperationLogTimeline
              agentHostId={agentHostId}
              targetId={selectedOperation?.id}
              enabled={Boolean(selectedOperation?.id)}
            />

            {isLoading ? (
              <Loading />
            ) : operations.length === 0 ? (
              <EmptyState
                icon={<History className="h-full w-full" />}
                title={t("admin.cores.operationsEmpty")}
                description={t("admin.cores.operationsEmptyDescription")}
                size="sm"
              />
            ) : (
              <Table aria-label={t("admin.cores.operationsTitle")}>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("admin.cores.logsTime")}</TableHead>
                    <TableHead>{t("admin.cores.operationType")}</TableHead>
                    <TableHead>{t("admin.cores.operationsTarget")}</TableHead>
                    <TableHead>{t("admin.cores.logsStatus")}</TableHead>
                    <TableHead>{t("admin.cores.logsDetail")}</TableHead>
                    <TableHead>{t("admin.cores.operationsFinishedAt")}</TableHead>
                    <TableHead>{t("common.actions")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {operations.map((operation) => (
                    <TableRow
                      key={operation.id}
                      className={selectedOperation?.id === operation.id ? "bg-primary/5" : undefined}
                    >
                      <TableCell>{formatDateTime(operation.created_at)}</TableCell>
                      <TableCell>{t(`admin.cores.operationType.${operation.operation_type}`)}</TableCell>
                      <TableCell>{buildOperationTarget(operation)}</TableCell>
                      <TableCell>
                        <StatusBadge status={operation.status} label={operation.status} />
                      </TableCell>
                      <TableCell>
                        <div className="max-w-[320px] truncate text-xs text-muted-foreground">
                          {describeOperationPayload(operation)}
                        </div>
                      </TableCell>
                      <TableCell>{formatDateTime(operation.finished_at ?? 0)}</TableCell>
                      <TableCell>
                        <Button size="sm" variant="outline" onClick={() => onSelectOperation(operation.id)}>
                          {t("admin.cores.viewTimeline")}
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}

            <Pagination page={page} totalPages={totalPages} onPageChange={onPageChange} />

            <details className="rounded-none border border-border border-border">
              <summary className="cursor-pointer select-none p-3 text-sm font-medium text-muted-foreground hover:text-foreground">
                {t("admin.cores.viewTimeline")}
              </summary>
              <div className="border-t border-border p-3">
                <OperationLogTimeline
                  agentHostId={agentHostId}
                  targetId={selectedOperation?.id}
                  enabled={open && Boolean(selectedOperation?.id)}
                />
              </div>
            </details>
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
