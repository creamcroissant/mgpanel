import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { Calendar, ClipboardList, Filter, RefreshCw, Search, Trash2 } from "lucide-react";
import { QUERY_KEYS } from "@/lib/constants";
import { fetchAccessLogs, getAccessLogStats, cleanupAccessLogs } from "@/api/admin";
import { getAgentHosts } from "@/api/admin/agentHost";
import { formatDateTime } from "@/lib/format";
import type { AccessLogEntry, AgentHost } from "@/types";
import { AdminPageShell, formatBytes } from "@/components/admin";
import {
  Badge,
  Button,
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

const PAGE_SIZE = 20;

function toTimestamp(dateValue: string, endOfDay = false): number | undefined {
  if (!dateValue) return undefined;
  const timeValue = endOfDay ? "T23:59:59" : "T00:00:00";
  const timestamp = new Date(`${dateValue}${timeValue}`).getTime();
  if (Number.isNaN(timestamp)) return undefined;
  return Math.floor(timestamp / 1000);
}

function resolveAgentName(agentHosts: AgentHost[], agentHostId: number): string {
  const agent = agentHosts.find((item) => item.id === agentHostId);
  return agent ? `${agent.name} (${agent.host})` : String(agentHostId);
}

export default function AccessLogList() {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const [filters, setFilters] = useState({
    agent_host_id: "",
    user_id: "",
    target_domain: "",
    source_ip: "",
    protocol: "",
    start_at: "",
    end_at: "",
  });

  const agentHostsQuery = useQuery({
    queryKey: QUERY_KEYS.ADMIN_AGENTS,
    queryFn: () => getAgentHosts({ page: 1, page_size: 100 }),
  });

  const agentHosts = agentHostsQuery.data?.data || [];

  const statsQuery = useQuery({
    queryKey: [
      ...QUERY_KEYS.ADMIN_ACCESS_LOG_STATS,
      filters.agent_host_id,
      filters.user_id,
      filters.start_at,
      filters.end_at,
    ],
    queryFn: () =>
      getAccessLogStats({
        agent_host_id: filters.agent_host_id ? Number(filters.agent_host_id) : undefined,
        user_id: filters.user_id ? Number(filters.user_id) : undefined,
        start_at: toTimestamp(filters.start_at),
        end_at: toTimestamp(filters.end_at, true),
      }),
  });

  const logsQuery = useQuery({
    queryKey: [
      ...QUERY_KEYS.ADMIN_ACCESS_LOGS,
      page,
      filters.agent_host_id,
      filters.user_id,
      filters.target_domain,
      filters.source_ip,
      filters.protocol,
      filters.start_at,
      filters.end_at,
    ],
    queryFn: () =>
      fetchAccessLogs({
        limit: PAGE_SIZE,
        offset: (page - 1) * PAGE_SIZE,
        agent_host_id: filters.agent_host_id ? Number(filters.agent_host_id) : undefined,
        user_id: filters.user_id ? Number(filters.user_id) : undefined,
        target_domain: filters.target_domain || undefined,
        source_ip: filters.source_ip || undefined,
        protocol: filters.protocol || undefined,
        start_at: toTimestamp(filters.start_at),
        end_at: toTimestamp(filters.end_at, true),
      }),
  });

  const cleanupMutation = useMutation({
    mutationFn: cleanupAccessLogs,
    onSuccess: (count) => {
      toast.success(t("admin.accessLogs.cleanupSuccess", { count }));
      logsQuery.refetch();
      statsQuery.refetch();
    },
    onError: (err: Error) => {
      toast.error(t("admin.accessLogs.cleanupError"), { description: err.message });
    },
  });

  const logs: AccessLogEntry[] = logsQuery.data?.logs || [];
  const total = logsQuery.data?.total || 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const stats = statsQuery.data;

  const protocolOptions = useMemo(() => ["tcp", "udp", "ws", "grpc", "http", "https"], []);

  const handleFilterChange = (key: keyof typeof filters, value: string) => {
    setFilters((prev) => ({ ...prev, [key]: value }));
    setPage(1);
  };

  const handleClearFilters = () => {
    setFilters({
      agent_host_id: "",
      user_id: "",
      target_domain: "",
      source_ip: "",
      protocol: "",
      start_at: "",
      end_at: "",
    });
    setPage(1);
  };

  const totalUpload = stats ? formatBytes(stats.total_upload) : "-";
  const totalDownload = stats ? formatBytes(stats.total_download) : "-";
  const totalCount = stats ? stats.total_count : 0;

  const actions = (
    <>
      <Button
        variant="outline"
        onClick={() => logsQuery.refetch()}
        disabled={logsQuery.isFetching}
      >
        <RefreshCw className="mr-2 h-4 w-4" />
        {logsQuery.isFetching ? t("common.loading") : t("common.refresh")}
      </Button>
      <Button
        variant="destructive"
        onClick={() => cleanupMutation.mutate()}
        disabled={cleanupMutation.isPending || total === 0}
      >
        <Trash2 className="mr-2 h-4 w-4" />
        {cleanupMutation.isPending ? t("common.loading") : t("admin.accessLogs.cleanup")}
      </Button>
    </>
  );

  const statsCards = (
    <div className="grid gap-3 sm:grid-cols-3">
      <div className="rounded-lg border border-border bg-card p-4">
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-full bg-primary/10 text-primary">
            <ClipboardList className="h-4 w-4" />
          </div>
          <div>
            <p className="text-xs text-muted-foreground">{t("admin.accessLogs.stats.total")}</p>
            <p className="text-lg font-semibold text-foreground">{totalCount}</p>
          </div>
        </div>
      </div>
      <div className="rounded-lg border border-border bg-card p-4">
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-full bg-success/15 text-success">
            <Filter className="h-4 w-4" />
          </div>
          <div>
            <p className="text-xs text-muted-foreground">{t("admin.accessLogs.stats.upload")}</p>
            <p className="text-lg font-semibold text-foreground">{totalUpload}</p>
          </div>
        </div>
      </div>
      <div className="rounded-lg border border-border bg-card p-4">
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-full bg-primary/10 text-primary">
            <Filter className="h-4 w-4" />
          </div>
          <div>
            <p className="text-xs text-muted-foreground">{t("admin.accessLogs.stats.download")}</p>
            <p className="text-lg font-semibold text-foreground">{totalDownload}</p>
          </div>
        </div>
      </div>
    </div>
  );

  const toolbar = (
    <div className="rounded-lg border border-border bg-card p-4">
      <div className="flex flex-col gap-4">
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-6">
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.accessLogs.filters.agent")}</label>
            <Select
              value={filters.agent_host_id || "all"}
              onValueChange={(value) =>
                handleFilterChange("agent_host_id", value === "all" ? "" : value)
              }
            >
              <SelectTrigger>
                <SelectValue placeholder={t("admin.accessLogs.filters.agentPlaceholder")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("common.all")}</SelectItem>
                {agentHosts.map((agent) => (
                  <SelectItem key={String(agent.id)} value={String(agent.id)}>
                    {agent.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.accessLogs.filters.user")}</label>
            <Input
              value={filters.user_id}
              placeholder={t("admin.accessLogs.filters.userPlaceholder")}
              onChange={(event) => handleFilterChange("user_id", event.target.value)}
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.accessLogs.filters.domain")}</label>
            <div className="relative">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                className="pl-9"
                value={filters.target_domain}
                placeholder={t("admin.accessLogs.filters.domainPlaceholder")}
                onChange={(event) => handleFilterChange("target_domain", event.target.value)}
              />
            </div>
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.accessLogs.filters.source")}</label>
            <Input
              value={filters.source_ip}
              placeholder={t("admin.accessLogs.filters.sourcePlaceholder")}
              onChange={(event) => handleFilterChange("source_ip", event.target.value)}
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.accessLogs.filters.protocol")}</label>
            <Select
              value={filters.protocol || "all"}
              onValueChange={(value) =>
                handleFilterChange("protocol", value === "all" ? "" : value)
              }
            >
              <SelectTrigger>
                <SelectValue placeholder={t("admin.accessLogs.filters.protocolPlaceholder")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("common.all")}</SelectItem>
                {protocolOptions.map((protocol) => (
                  <SelectItem key={protocol} value={protocol}>
                    {protocol.toUpperCase()}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.accessLogs.filters.date")}</label>
            <div className="grid grid-cols-1 gap-2">
              <div className="relative">
                <Calendar className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  type="date"
                  className="pl-9"
                  value={filters.start_at}
                  onChange={(event) => handleFilterChange("start_at", event.target.value)}
                />
              </div>
              <div className="relative">
                <Calendar className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  type="date"
                  className="pl-9"
                  value={filters.end_at}
                  onChange={(event) => handleFilterChange("end_at", event.target.value)}
                />
              </div>
            </div>
          </div>
        </div>

        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap items-center gap-3">
            <div className="text-xs text-muted-foreground">
              {t("admin.accessLogs.filters.active", { count: total })}
            </div>
            <div className="text-xs text-muted-foreground">{t("common.autoApplied")}</div>
            {statsQuery.isFetching && (
              <Badge variant="secondary">{t("common.loading")}</Badge>
            )}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="outline" onClick={handleClearFilters}>
              <Filter className="mr-2 h-4 w-4" />
              {t("admin.accessLogs.filters.clear")}
            </Button>
            <Button
              variant="outline"
              onClick={() => logsQuery.refetch()}
              disabled={logsQuery.isFetching}
            >
              <RefreshCw className="mr-2 h-4 w-4" />
              {logsQuery.isFetching ? t("common.loading") : t("common.refresh")}
            </Button>
          </div>
        </div>
      </div>
    </div>
  );

  let content = <Loading />;

  if (agentHostsQuery.error) {
    content = (
      <EmptyState
        title={t("admin.accessLogs.agentLoadError")}
        action={
          <Button variant="outline" onClick={() => agentHostsQuery.refetch()}>
            {t("common.retry")}
          </Button>
        }
      />
    );
  } else if (!agentHostsQuery.isLoading && logsQuery.error) {
    content = (
      <EmptyState
        title={t("admin.accessLogs.loadError")}
        action={
          <Button variant="outline" onClick={() => logsQuery.refetch()}>
            {t("common.retry")}
          </Button>
        }
      />
    );
  } else if (!agentHostsQuery.isLoading && !logsQuery.isLoading && logs.length === 0) {
    content = (
      <EmptyState
        icon={<ClipboardList className="h-full w-full" />}
        title={t("admin.accessLogs.empty")}
        description={t("admin.accessLogs.emptyDescription")}
        size="md"
      />
    );
  } else if (!agentHostsQuery.isLoading && !logsQuery.isLoading) {
    content = (
      <div className="space-y-4">
        <div className="overflow-x-auto">
          <Table aria-label={t("admin.accessLogs.title")}>
            <TableHeader>
              <TableRow>
                <TableHead>{t("admin.accessLogs.table.time")}</TableHead>
                <TableHead>{t("admin.accessLogs.table.user")}</TableHead>
                <TableHead>{t("admin.accessLogs.table.agent")}</TableHead>
                <TableHead>{t("admin.accessLogs.table.source")}</TableHead>
                <TableHead>{t("admin.accessLogs.table.target")}</TableHead>
                <TableHead>{t("admin.accessLogs.table.protocol")}</TableHead>
                <TableHead>{t("admin.accessLogs.table.traffic")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {logs.map((log) => (
                <TableRow key={log.id}>
                  <TableCell className="whitespace-nowrap">
                    {formatDateTime(log.created_at)}
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-col gap-1">
                      <span className="font-medium text-foreground">
                        {log.user_email || log.user_id || "-"}
                      </span>
                      <span className="text-xs text-muted-foreground">#{log.user_id ?? "-"}</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="text-sm text-foreground">
                      {resolveAgentName(agentHosts, log.agent_host_id)}
                    </div>
                  </TableCell>
                  <TableCell>{log.source_ip || "-"}</TableCell>
                  <TableCell>
                    <div className="flex flex-col">
                      <span className="font-medium text-foreground">
                        {log.target_domain || log.target_ip || "-"}
                      </span>
                      <span className="text-xs text-muted-foreground">
                        {log.target_ip ? `${log.target_ip}:${log.target_port}` : log.target_port}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="secondary">{log.protocol || "-"}</Badge>
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-col text-xs">
                      <span>{t("admin.accessLogs.table.upload", { value: formatBytes(log.upload) })}</span>
                      <span>{t("admin.accessLogs.table.download", { value: formatBytes(log.download) })}</span>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
        <Pagination page={page} totalPages={totalPages} onPageChange={setPage} />
      </div>
    );
  }

  return (
    <AdminPageShell
      title={t("admin.accessLogs.title")}
      description={t("admin.accessLogs.subtitle")}
      actions={actions}
      stats={statsCards}
      toolbar={toolbar}
    >
      {content}
    </AdminPageShell>
  );
}
