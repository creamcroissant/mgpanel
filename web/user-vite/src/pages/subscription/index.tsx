import { useState, type ReactNode } from "react";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { KeyRound, Link as LinkIcon, MonitorSmartphone, RefreshCw, ShieldCheck, Smartphone, Wifi } from "lucide-react";
import { fetchUserInfo, resetSubscribeToken } from "@/api/user";
import { QUERY_KEYS, ROUTES } from "@/lib/constants";
import { formatBytes, formatDate } from "@/lib/format";
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
  type KeyValueItem,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui";

interface ClientGuide {
  key: string;
  icon: ReactNode;
  name: string;
  description: string;
}

function maskToken(value?: string | null): string {
  const token = value?.trim();
  if (!token) return "-";
  if (token.length <= 8) return "••••";
  return `${token.slice(0, 4)}••••${token.slice(-4)}`;
}

export default function SubscriptionWorkspace() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [confirmOpen, setConfirmOpen] = useState(false);

  const {
    data: user,
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: QUERY_KEYS.USER_INFO,
    queryFn: fetchUserInfo,
  });

  const resetTokenMutation = useMutation({
    mutationFn: resetSubscribeToken,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: QUERY_KEYS.USER_INFO });
      setConfirmOpen(false);
      toast.success(t("common.success"), {
        description: t("settings.tokenReset"),
      });
    },
    onError: (mutationError) => {
      toast.error(t("common.error"), {
        description: mutationError.message,
      });
    },
  });

  if (isLoading) return <Loading />;
  if (error) return <ErrorBanner message={t("error.loadProfile")} onRetry={refetch} />;
  if (!user) return <ErrorBanner message={t("error.loadProfile")} onRetry={refetch} />;

  const transferUsed = Math.max(user.transfer_used ?? (user.u || 0) + (user.d || 0), 0);
  const transferEnable = user.transfer_enable ?? 0;
  const hasTrafficQuota = transferEnable > 0;
  const remainingTraffic = hasTrafficQuota ? Math.max(transferEnable - transferUsed, 0) : 0;
  const expiresAt = user.expired_at ? formatDate(user.expired_at) : t("subscription.neverExpires");

  const planItems: KeyValueItem[] = [
    { label: t("plans.currentPlan"), value: user.plan?.name || t("plans.noPlan") },
    { label: t("subscription.trafficQuota"), value: hasTrafficQuota ? formatBytes(transferEnable) : t("plans.unlimited") },
    { label: t("subscription.usedTraffic"), value: formatBytes(transferUsed) },
    { label: t("subscription.remainingTraffic"), value: hasTrafficQuota ? formatBytes(remainingTraffic) : t("plans.unlimited") },
    { label: t("subscription.expiresAt"), value: expiresAt },
    { label: t("subscription.accountId"), value: user.uuid || String(user.id) },
  ];

  const clients: ClientGuide[] = [
    {
      key: "clash",
      icon: <Wifi className="h-5 w-5" />,
      name: t("subscription.clients.clash"),
      description: `${t("subscription.clientStepImport")} · ${t("subscription.clientStepRefresh")}`,
    },
    {
      key: "v2rayn",
      icon: <MonitorSmartphone className="h-5 w-5" />,
      name: t("subscription.clients.v2rayn"),
      description: `${t("subscription.clientStepImport")} · ${t("subscription.clientStepConnect")}`,
    },
    {
      key: "singbox",
      icon: <ShieldCheck className="h-5 w-5" />,
      name: t("subscription.clients.singbox"),
      description: `${t("subscription.clientStepImport")} · ${t("subscription.clientStepRefresh")}`,
    },
    {
      key: "mobile",
      icon: <Smartphone className="h-5 w-5" />,
      name: t("subscription.clients.mobile"),
      description: `${t("subscription.clientStepImport")} · ${t("subscription.clientStepConnect")}`,
    },
  ];

  return (
    <PageShell
      data-testid="subscription-workspace"
      title={t("subscription.title")}
      description={t("subscription.subtitle")}
      actions={
        <Button variant="outline" className="gap-2" onClick={() => refetch()}>
          <RefreshCw className="h-4 w-4" />
          {t("common.refresh")}
        </Button>
      }
    >
      <div className="grid min-w-0 gap-5 xl:grid-cols-[minmax(0,1.1fr)_minmax(320px,0.9fr)]">
        <SectionCard
          title={t("subscription.copyTitle")}
          description={t("subscription.copyHint")}
          actions={<Badge variant={user.subscribe_url ? "success" : "secondary"}>{t("nav.subscription")}</Badge>}
        >
          <CopyField
            label={t("dashboard.subscribeUrl")}
            value={user.subscribe_url}
            emptyLabel={t("subscription.noSubscription")}
            copyLabel={t("subscription.copySubscription")}
            copiedLabel={t("common.copied")}
            buttonAriaLabel={t("subscription.copySubscription")}
          />
        </SectionCard>

        <SectionCard title={t("subscription.security")} description={t("subscription.securityHint")}>
          <div className="space-y-4">
            <div className="grid min-w-0 gap-3 sm:grid-cols-2">
              <div className="min-w-0 rounded-md border bg-muted/25 p-3">
                <div className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground">
                  {t("subscription.tokenStatus")}
                </div>
                <div className="mt-1 flex min-w-0 items-center gap-2 text-sm font-semibold text-foreground">
                  <KeyRound className="h-4 w-4 text-muted-foreground" />
                  <span className="min-w-0 break-all">{maskToken(user.token)}</span>
                </div>
                <p className="mt-1 text-xs text-muted-foreground">{t("subscription.tokenProtected")}</p>
              </div>
              <div className="min-w-0 rounded-md border bg-muted/25 p-3">
                <div className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground">
                  {t("subscription.accountId")}
                </div>
                <div className="mt-1 break-all text-sm font-semibold text-foreground">{user.uuid || user.id}</div>
                <p className="mt-1 text-xs text-muted-foreground">{user.email}</p>
              </div>
            </div>
            <Button variant="outline" className="gap-2" onClick={() => setConfirmOpen(true)}>
              <RefreshCw className="h-4 w-4" />
              {t("subscription.resetToken")}
            </Button>
          </div>
        </SectionCard>
      </div>

      <SectionCard title={t("subscription.planSnapshot")} description={t("subscription.planSnapshotHint")}>
        <KeyValueGrid items={planItems} />
      </SectionCard>

      <SectionCard title={t("subscription.clientSetup")} description={t("subscription.clientSetupHint")}>
        <ResponsiveGrid minColWidth={240} gap={16}>
          {clients.map((client) => (
            <ResourceCard
              key={client.key}
              icon={client.icon}
              title={client.name}
              description={client.description}
              actions={
                <Button asChild variant="outline" size="sm">
                  <Link to={ROUTES.KNOWLEDGE}>
                    <LinkIcon className="mr-2 h-4 w-4" />
                    {t("dashboard.openKnowledge")}
                  </Link>
                </Button>
              }
            />
          ))}
        </ResponsiveGrid>
      </SectionCard>

      <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("subscription.resetConfirmTitle")}</DialogTitle>
            <DialogDescription>{t("subscription.resetConfirmDescription")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setConfirmOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button
              type="button"
              variant="destructive"
              disabled={resetTokenMutation.isPending}
              onClick={() => resetTokenMutation.mutate()}
            >
              {resetTokenMutation.isPending ? t("common.loading") : t("common.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </PageShell>
  );
}
