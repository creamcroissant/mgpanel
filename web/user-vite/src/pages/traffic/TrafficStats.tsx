import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { ArrowDown, ArrowUp, BarChart3, Clock, Database, RefreshCw } from "lucide-react";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from "recharts";
import { fetchTrafficLogs } from "@/api/traffic";
import { fetchUserInfo } from "@/api/user";
import { daysUntil, formatBytes, formatDate, formatDateTime } from "@/lib/format";
import { QUERY_KEYS } from "@/lib/constants";
import type { TrafficLog } from "@/types";
import {
  Badge,
  Button,
  EmptyState,
  ErrorBanner,
  KeyValueGrid,
  Loading,
  PageShell,
  ResourceCard,
  ResponsiveGrid,
  SectionCard,
  UsageRing,
  type KeyValueItem,
} from "@/components/ui";

function trafficTotal(log: TrafficLog): number {
  return Math.max(log.u, 0) + Math.max(log.d, 0);
}

function formatRemainingDays(timestamp?: number): string {
  if (!timestamp) return "-";
  const days = daysUntil(timestamp);
  if (!Number.isFinite(days)) return "-";
  return String(Math.max(days, 0));
}

export default function TrafficStats() {
  const { t } = useTranslation();
  const {
    data: logs = [],
    isLoading: isTrafficLoading,
    error: trafficError,
    refetch: refetchTraffic,
  } = useQuery({
    queryKey: QUERY_KEYS.TRAFFIC,
    queryFn: fetchTrafficLogs,
  });
  const {
    data: user,
    isLoading: isUserLoading,
    error: userError,
    refetch: refetchUser,
  } = useQuery({
    queryKey: QUERY_KEYS.USER_INFO,
    queryFn: fetchUserInfo,
  });

  const isLoading = isTrafficLoading || isUserLoading;
  const error = trafficError || userError;

  const chartData = useMemo(() => {
    return logs
      .slice()
      .sort((a, b) => a.record_at - b.record_at)
      .map((log) => ({
        date: formatDate(log.record_at),
        upload: Math.max(log.u, 0),
        download: Math.max(log.d, 0),
        total: trafficTotal(log),
      }));
  }, [logs]);

  if (isLoading) return <Loading />;
  if (error) {
    return (
      <ErrorBanner
        message={t("error.loadTraffic")}
        onRetry={() => {
          refetchTraffic();
          refetchUser();
        }}
      />
    );
  }

  const totalUpload = logs.reduce((sum, log) => sum + Math.max(log.u, 0), 0);
  const totalDownload = logs.reduce((sum, log) => sum + Math.max(log.d, 0), 0);
  const recentTotal = totalUpload + totalDownload;
  const transferEnable = user?.transfer_enable ?? 0;
  const transferUsed = Math.max(user?.transfer_used ?? (user ? user.u + user.d : 0), 0);
  const remainingTraffic = transferEnable > 0 ? Math.max(transferEnable - transferUsed, 0) : 0;
  const usagePercent = transferEnable > 0 ? Math.min(100, (transferUsed / transferEnable) * 100) : 0;

  const summaryItems: KeyValueItem[] = [
    {
      label: t("traffic.upload"),
      value: formatBytes(totalUpload),
      hint: t("traffic.uploadHint"),
    },
    {
      label: t("traffic.download"),
      value: formatBytes(totalDownload),
      hint: t("traffic.downloadHint"),
    },
    {
      label: t("traffic.total"),
      value: formatBytes(recentTotal),
      hint: t("traffic.totalHint"),
    },
    {
      label: t("traffic.remaining"),
      value: transferEnable > 0 ? formatBytes(remainingTraffic) : t("plans.unlimited"),
      hint: t("traffic.remainingHint"),
    },
  ];

  const cycleItems: KeyValueItem[] = [
    {
      label: t("dashboard.totalTraffic"),
      value: transferEnable > 0 ? formatBytes(transferEnable) : t("plans.unlimited"),
    },
    {
      label: t("dashboard.usedTraffic"),
      value: formatBytes(transferUsed),
    },
    {
      label: t("traffic.expiresAt"),
      value: user?.expired_at ? formatDate(user.expired_at) : t("dashboard.never"),
    },
    {
      label: t("traffic.daysRemaining"),
      value: user?.expired_at ? t("traffic.daysValue", { days: formatRemainingDays(user.expired_at) }) : t("dashboard.never"),
    },
  ];

  return (
    <PageShell
      data-testid="traffic-usage-center"
      title={t("traffic.title")}
      description={t("traffic.subtitle")}
      actions={
        <Button
          variant="outline"
          className="gap-2"
          onClick={() => {
            refetchTraffic();
            refetchUser();
          }}
        >
          <RefreshCw className="h-4 w-4" />
          {t("common.refresh")}
        </Button>
      }
    >
      <SectionCard title={t("traffic.usageSummary")} description={t("traffic.usageSummaryHint")}>
        <div className="flex min-w-0 flex-col gap-5 xl:flex-row xl:items-center">
          <div className="flex justify-center xl:w-52 xl:justify-start">
            <UsageRing
              value={usagePercent}
              label={t("common.used")}
              detail={transferEnable > 0 ? `${formatBytes(transferUsed)} / ${formatBytes(transferEnable)}` : formatBytes(transferUsed)}
              tone={usagePercent >= 90 ? "danger" : usagePercent >= 70 ? "warning" : "success"}
            />
          </div>
          <KeyValueGrid items={summaryItems} className="flex-1 xl:grid-cols-2" />
        </div>
      </SectionCard>

      <SectionCard title={t("traffic.cycleProgress")} description={t("traffic.cycleProgressHint")}>
        <KeyValueGrid items={cycleItems} className="lg:grid-cols-4" />
      </SectionCard>

      <ResponsiveGrid minColWidth={220} gap={12}>
        <ResourceCard
          icon={<ArrowUp className="h-5 w-5" />}
          title={t("traffic.upload")}
          description={formatBytes(totalUpload)}
          status={<Badge variant="outline">{t("traffic.last30Days")}</Badge>}
        />
        <ResourceCard
          icon={<ArrowDown className="h-5 w-5" />}
          title={t("traffic.download")}
          description={formatBytes(totalDownload)}
          status={<Badge variant="outline">{t("traffic.last30Days")}</Badge>}
        />
        <ResourceCard
          icon={<Database className="h-5 w-5" />}
          title={t("traffic.total")}
          description={formatBytes(recentTotal)}
          status={<Badge variant="secondary">{logs.length}</Badge>}
        />
      </ResponsiveGrid>

      <SectionCard
        data-testid="traffic-trend-section"
        title={t("traffic.trend")}
        description={t("traffic.trendHint")}
      >
        {chartData.length > 0 ? (
          <div data-testid="traffic-trend-chart" className="h-80 min-w-0">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={chartData} margin={{ top: 12, right: 16, bottom: 0, left: 0 }}>
                <CartesianGrid strokeDasharray="3 3" opacity={0.18} />
                <XAxis dataKey="date" tick={{ fontSize: 12 }} tickLine={false} minTickGap={16} />
                <YAxis
                  width={64}
                  tickFormatter={(value) => formatBytes(value)}
                  tick={{ fontSize: 12 }}
                  tickLine={false}
                  axisLine={false}
                />
                <Tooltip
                  formatter={(value) => formatBytes(value as number)}
                  contentStyle={{
                    backgroundColor: "hsl(var(--card))",
                    border: "1px solid hsl(var(--border))",
                    borderRadius: "6px",
                    boxShadow: "none",
                  }}
                  labelStyle={{ color: "hsl(var(--foreground))" }}
                />
                <Legend />
                <Line type="monotone" dataKey="upload" name={t("traffic.upload")} stroke="hsl(var(--primary))" strokeWidth={2} dot={false} />
                <Line type="monotone" dataKey="download" name={t("traffic.download")} stroke="hsl(var(--success))" strokeWidth={2} dot={false} />
              </LineChart>
            </ResponsiveContainer>
          </div>
        ) : (
          <EmptyState icon={<BarChart3 className="h-full w-full" />} title={t("traffic.noData")} description={t("traffic.noDataHint")} />
        )}
      </SectionCard>

      <SectionCard
        data-testid="traffic-log-list"
        title={t("traffic.logs")}
        description={t("traffic.logsHint")}
      >
        {logs.length > 0 ? (
          <div className="space-y-2">
            {logs.map((log) => (
              <ResourceCard
                key={log.id}
                data-testid="traffic-log-row"
                icon={<Clock className="h-5 w-5" />}
                title={formatDateTime(log.record_at)}
                description={t("traffic.logTotal", { total: formatBytes(trafficTotal(log)) })}
                meta={
                  <>
                    <Badge variant="outline">{t("traffic.upload")}: {formatBytes(Math.max(log.u, 0))}</Badge>
                    <Badge variant="secondary">{t("traffic.download")}: {formatBytes(Math.max(log.d, 0))}</Badge>
                  </>
                }
              />
            ))}
          </div>
        ) : (
          <EmptyState title={t("traffic.noData")} description={t("traffic.noDataHint")} />
        )}
      </SectionCard>
    </PageShell>
  );
}
