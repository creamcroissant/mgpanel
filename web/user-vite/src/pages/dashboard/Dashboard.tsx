import { type ReactNode } from "react";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import {
  BarChart3,
  BookOpen,
  CreditCard,
  Database,
  Link as LinkIcon,
  MonitorDot,
  RefreshCw,
  Server,
  Settings,
  Shield,
  Wifi,
  Users,
} from "lucide-react";
import { fetchUserInfo } from "@/api/user";
import { getQueueStats, getSystemStatus } from "@/api/admin";
import { useAuth } from "@/providers/AuthProvider";
import {
  ADMIN_HOME_ROUTE,
  QUERY_KEYS,
  ROUTES,
} from "@/lib/constants";
import { formatBytes, formatDate, daysUntil, isExpired } from "@/lib/format";
import {
  Badge,
  Button,
  CopyField,
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

function MetricTile({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="rounded-md border bg-muted/25 p-3">
      <p className="text-xs font-medium uppercase tracking-[0.08em] text-muted-foreground">{label}</p>
      <div className="mt-1 break-words text-lg font-semibold leading-tight text-foreground">{value}</div>
    </div>
  );
}

export default function Dashboard() {
  const { t } = useTranslation();
  const { isAdmin } = useAuth();
  const {
    data: user,
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: QUERY_KEYS.USER_INFO,
    queryFn: fetchUserInfo,
  });
  const { data: systemStatus } = useQuery({
    queryKey: QUERY_KEYS.ADMIN_SYSTEM,
    queryFn: getSystemStatus,
    enabled: isAdmin,
    refetchInterval: 60000,
  });
  const { data: queueStats } = useQuery({
    queryKey: QUERY_KEYS.ADMIN_SYSTEM_QUEUE,
    queryFn: getQueueStats,
    enabled: isAdmin,
    refetchInterval: 60000,
  });

  if (isLoading) return <Loading />;
  if (error) return <ErrorBanner message={t("error.loadProfile")} onRetry={refetch} />;
  if (!user) return <ErrorBanner message={t("error.loadProfile")} onRetry={refetch} />;

  const transferUsed = (user.u || 0) + (user.d || 0);
  const transferEnable = user.transfer_enable || 0;
  const remainingTraffic = Math.max(transferEnable - transferUsed, 0);
  const usagePercent = transferEnable > 0 ? (transferUsed / transferEnable) * 100 : 0;
  const expired = user.expired_at ? isExpired(user.expired_at) : false;
  const days = user.expired_at ? daysUntil(user.expired_at) : Infinity;
  const usageTone = usagePercent >= 90 ? "danger" : usagePercent >= 70 ? "warning" : "success";
  const planStatus = !user.plan_id ? t("dashboard.noPlan") : expired ? t("dashboard.expired") : t("dashboard.active");
  const planBadgeVariant = expired ? "danger" : !user.plan_id ? "secondary" : days <= 7 ? "warning" : "success";
  const expiryText = !user.expired_at
    ? t("dashboard.never")
    : expired
      ? t("dashboard.expired")
      : days <= 7
        ? `${days} ${t("dashboard.daysLeft")}`
        : formatDate(user.expired_at);

  const quickActions = [
    {
      href: ROUTES.SERVERS,
      icon: <Server className="h-5 w-5" />,
      title: t("nav.servers"),
      description: t("dashboard.openServersHint"),
      action: t("dashboard.openServers"),
    },
    {
      href: ROUTES.SUBSCRIPTION,
      icon: <LinkIcon className="h-5 w-5" />,
      title: t("dashboard.subscriptionWorkspace"),
      description: t("dashboard.openSubscriptionHint"),
      action: t("dashboard.openSubscription"),
    },
    {
      href: ROUTES.TRAFFIC,
      icon: <BarChart3 className="h-5 w-5" />,
      title: t("nav.traffic"),
      description: t("dashboard.openTrafficHint"),
      action: t("dashboard.openTraffic"),
    },
    {
      href: ROUTES.PLANS,
      icon: <CreditCard className="h-5 w-5" />,
      title: t("nav.plans"),
      description: t("dashboard.openPlansHint"),
      action: t("dashboard.openPlans"),
    },
    {
      href: ROUTES.KNOWLEDGE,
      icon: <BookOpen className="h-5 w-5" />,
      title: t("nav.knowledge"),
      description: t("dashboard.openKnowledgeHint"),
      action: t("dashboard.openKnowledge"),
    },
    {
      href: ROUTES.SETTINGS,
      icon: <Settings className="h-5 w-5" />,
      title: t("nav.settings"),
      description: t("dashboard.openSettingsHint"),
      action: t("dashboard.openSettings"),
    },
  ];

  const usageItems: KeyValueItem[] = [
    { label: t("dashboard.usedTraffic"), value: formatBytes(transferUsed) },
    { label: t("dashboard.totalTraffic"), value: formatBytes(transferEnable) },
    { label: t("dashboard.remainingTraffic"), value: formatBytes(remainingTraffic) },
    { label: t("dashboard.currentPlan"), value: user.plan?.name || planStatus },
    { label: t("dashboard.expiredAt"), value: expiryText },
    { label: t("dashboard.planStatus"), value: <Badge variant={planBadgeVariant}>{planStatus}</Badge> },
  ];

  return (
    <PageShell
      data-testid="portal-hero"
      title={t("dashboard.portalTitle")}
      description={
        <>
          {t("dashboard.portalSubtitle")} <span className="font-medium text-foreground">{user.email}</span>
        </>
      }
      actions={
        <>
          {isAdmin && (
            <Button asChild variant="outline" className="gap-2">
              <Link to={ADMIN_HOME_ROUTE}>
                <Shield className="h-4 w-4" />
                {t("dashboard.manageAdmin")}
              </Link>
            </Button>
          )}
          <Button variant="outline" className="gap-2" onClick={() => refetch()}>
            <RefreshCw className="h-4 w-4" />
            {t("common.refresh")}
          </Button>
        </>
      }
    >
      <div className="grid min-w-0 gap-5 xl:grid-cols-[minmax(0,1.15fr)_minmax(320px,0.85fr)]">
        <SectionCard
          data-testid="subscription-quick-card"
          title={t("dashboard.subscriptionWorkspace")}
          description={t("dashboard.subscriptionWorkspaceHint")}
        >
          <CopyField
            label={t("dashboard.subscribeUrl")}
            value={user.subscribe_url}
            emptyLabel={t("dashboard.noSubscription")}
            copyLabel={t("dashboard.copySubscription")}
            copiedLabel={t("common.copied")}
            helperText={t("dashboard.copyHint")}
            buttonAriaLabel={t("dashboard.copySubscription")}
          />
        </SectionCard>

        <SectionCard
          title={t("dashboard.usageSummary")}
          description={t("dashboard.usageSummaryHint")}
          className="min-w-0"
        >
          <div className="flex min-w-0 flex-col gap-5 lg:flex-row lg:items-center">
            <UsageRing
              value={usagePercent}
              label={t("common.used")}
              detail={`${formatBytes(transferUsed)} / ${formatBytes(transferEnable)}`}
              tone={usageTone}
            />
            <KeyValueGrid items={usageItems} className="flex-1 xl:grid-cols-2" />
          </div>
        </SectionCard>
      </div>

      <section data-testid="quick-action-grid" className="space-y-3">
        <div className="flex min-w-0 flex-col gap-1">
          <h2 className="text-lg font-semibold tracking-tight text-foreground">{t("dashboard.quickActions")}</h2>
          <p className="text-sm text-muted-foreground">{t("dashboard.quickActionsHint")}</p>
        </div>
        <ResponsiveGrid minColWidth={260} gap={16}>
          {quickActions.map((action) => (
            <ResourceCard
              key={action.href}
              icon={action.icon}
              title={action.title}
              description={action.description}
              actions={
                <Button asChild variant="outline" size="sm">
                  <Link to={action.href}>{action.action}</Link>
                </Button>
              }
            />
          ))}
        </ResponsiveGrid>
      </section>

      {isAdmin && systemStatus && (
        <SectionCard
          data-testid="admin-overview"
          title={t("dashboard.adminOverview")}
          description={t("dashboard.adminOverviewHint")}
        >
          <ResponsiveGrid minColWidth={180} gap={12}>
            <MetricTile label={t("admin.system.totalUsers")} value={systemStatus.user_count} />
            <MetricTile label={t("admin.system.totalServers")} value={systemStatus.server_count} />
            <MetricTile label={t("admin.system.totalAgents")} value={systemStatus.agent_count} />
            <MetricTile label={t("admin.system.onlineAgents")} value={systemStatus.online_agent_count} />
            <MetricTile label={t("dashboard.recentJobs")} value={queueStats?.recentJobs ?? 0} />
            <MetricTile label={t("dashboard.failedJobs")} value={queueStats?.failedJobs ?? 0} />
          </ResponsiveGrid>
          <div className="mt-4 flex flex-wrap gap-2 text-sm text-muted-foreground">
            <span className="inline-flex items-center gap-2 rounded-md border bg-muted/25 px-3 py-2">
              <Database className="h-4 w-4" />
              {t("dashboard.environment")}: {systemStatus.environment || "-"}
            </span>
            <span className="inline-flex items-center gap-2 rounded-md border bg-muted/25 px-3 py-2">
              <MonitorDot className="h-4 w-4" />
              {t("admin.system.version")}: {systemStatus.version || "go-dev"}
            </span>
            <span className="inline-flex items-center gap-2 rounded-md border bg-muted/25 px-3 py-2">
              <Users className="h-4 w-4" />
              {t("dashboard.jobsPerMinute")}: {(queueStats?.jobsPerMinute ?? 0).toFixed(1)}
            </span>
            <span className="inline-flex items-center gap-2 rounded-md border bg-muted/25 px-3 py-2">
              <Wifi className="h-4 w-4" />
              {t("dashboard.maxThroughputQueue")}: {queueStats?.queueWithMaxThroughput?.name || "-"}
            </span>
          </div>
        </SectionCard>
      )}
    </PageShell>
  );
}
