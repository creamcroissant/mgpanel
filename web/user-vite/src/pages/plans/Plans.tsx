import { type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import DOMPurify from "dompurify";
import { useQuery } from "@tanstack/react-query";
import { Calendar, CreditCard, Database, Gauge, ListChecks, RefreshCw, Smartphone } from "lucide-react";
import { fetchUserInfo } from "@/api/user";
import { formatBytes, formatDate } from "@/lib/format";
import { QUERY_KEYS } from "@/lib/constants";
import type { Plan, PlanPrice } from "@/types";
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

const PERIOD_LABEL_KEYS: Record<string, string> = {
  monthly: "plans.periods.monthly",
  quarterly: "plans.periods.quarterly",
  half_yearly: "plans.periods.halfYearly",
  yearly: "plans.periods.yearly",
  two_yearly: "plans.periods.twoYearly",
  three_yearly: "plans.periods.threeYearly",
  onetime: "plans.periods.onetime",
  reset_traffic: "plans.periods.resetTraffic",
};

type PlanPrices = Plan["prices"] | Record<string, number> | null | undefined;

function normalizePrices(prices: PlanPrices): PlanPrice[] {
  if (!prices) return [];
  if (Array.isArray(prices)) return prices.filter((item) => item.period && Number.isFinite(item.price));
  return Object.entries(prices)
    .filter(([, price]) => Number.isFinite(price))
    .map(([period, price]) => ({ period, price }));
}

function formatPrice(value: number): string {
  return new Intl.NumberFormat(undefined, {
    minimumFractionDigits: value % 1 === 0 ? 0 : 2,
    maximumFractionDigits: 2,
  }).format(value);
}

function limitValue(value: number | undefined, unit: string, unlimited: string): ReactNode {
  if (!value) return unlimited;
  return `${value} ${unit}`;
}

export default function Plans() {
  const { t } = useTranslation();
  const {
    data: user,
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: QUERY_KEYS.USER_INFO,
    queryFn: fetchUserInfo,
  });

  if (isLoading) return <Loading />;
  if (error) return <ErrorBanner message={t("error.loadPlans")} onRetry={refetch} />;

  const plan = user?.plan;
  const transferEnable = plan?.transfer_enable ?? user?.transfer_enable ?? 0;
  const transferUsed = Math.max(user?.transfer_used ?? (user ? user.u + user.d : 0), 0);
  const usagePercent = transferEnable > 0 ? Math.min(100, (transferUsed / transferEnable) * 100) : 0;
  const prices = normalizePrices(plan?.prices);

  const usageItems: KeyValueItem[] = [
    {
      label: t("plans.traffic"),
      value: formatBytes(transferEnable),
      hint: t("plans.trafficHint"),
    },
    {
      label: t("dashboard.usedTraffic"),
      value: formatBytes(transferUsed),
    },
    {
      label: t("dashboard.remainingTraffic"),
      value: formatBytes(Math.max(transferEnable - transferUsed, 0)),
    },
    {
      label: t("plans.expiresAt"),
      value: user?.expired_at ? formatDate(user.expired_at) : t("dashboard.never"),
    },
  ];

  const limitItems: KeyValueItem[] = [
    {
      label: t("plans.speedLimit"),
      value: limitValue(plan?.speed_limit, "Mbps", t("plans.unlimited")),
      hint: t("plans.speedLimitHint"),
    },
    {
      label: t("plans.deviceLimit"),
      value: plan?.device_limit ? plan.device_limit : t("plans.unlimited"),
      hint: t("plans.deviceLimitHint"),
    },
    {
      label: t("plans.planId"),
      value: plan?.id ?? user?.plan_id ?? "-",
    },
    {
      label: t("plans.renewal"),
      value: plan?.renew ? t("plans.renewable") : t("plans.notRenewable"),
      hint: t("plans.readonlyHint"),
    },
  ];

  return (
    <PageShell
      data-testid="plan-resource-detail"
      title={t("plans.title")}
      description={t("plans.subtitle")}
      actions={
        <Button variant="outline" className="gap-2" onClick={() => refetch()}>
          <RefreshCw className="h-4 w-4" />
          {t("common.refresh")}
        </Button>
      }
    >
      {!plan ? (
        <SectionCard data-testid="current-plan-card" title={t("plans.noPlan")} description={t("plans.noPlanHint")}>
          <EmptyState icon={<CreditCard className="h-full w-full" />} title={t("plans.noPlan")} description={t("plans.noPlanHint")} />
        </SectionCard>
      ) : (
        <>
          <SectionCard
            data-testid="current-plan-card"
            title={plan.name}
            description={t("plans.currentPlanHint")}
            actions={<Badge variant="success">{t("plans.currentPlan")}</Badge>}
          >
            <div className="flex min-w-0 flex-col gap-5 xl:flex-row xl:items-center">
              <div className="flex justify-center xl:w-52 xl:justify-start">
                <UsageRing
                  value={usagePercent}
                  label={t("common.used")}
                  detail={`${formatBytes(transferUsed)} / ${formatBytes(transferEnable)}`}
                  tone={usagePercent >= 90 ? "danger" : usagePercent >= 70 ? "warning" : "success"}
                />
              </div>
              <KeyValueGrid items={usageItems} className="flex-1 xl:grid-cols-2" />
            </div>
          </SectionCard>

          <SectionCard
            data-testid="plan-limit-section"
            title={t("plans.resourceLimits")}
            description={t("plans.resourceLimitsHint")}
          >
            <KeyValueGrid items={limitItems} className="lg:grid-cols-4" />
          </SectionCard>

          <SectionCard
            data-testid="plan-price-section"
            title={t("plans.readonlyPrices")}
            description={t("plans.readonlyPricesHint")}
          >
            {prices.length > 0 ? (
              <ResponsiveGrid minColWidth={190} gap={12}>
                {prices.map((price) => {
                  const labelKey = PERIOD_LABEL_KEYS[price.period];
                  const periodLabel = labelKey ? t(labelKey) : t("plans.periods.custom", { period: price.period });
                  return (
                    <ResourceCard
                      key={price.period}
                      data-testid="plan-price-card"
                      icon={<Calendar className="h-5 w-5" />}
                      title={periodLabel}
                      description={t("plans.priceReadonlyDescription")}
                      meta={<Badge variant="outline">{t("plans.readonly")}</Badge>}
                      footer={<span className="text-base font-semibold">{t("plans.priceValue", { price: formatPrice(price.price) })}</span>}
                    />
                  );
                })}
              </ResponsiveGrid>
            ) : (
              <EmptyState title={t("plans.noPrices")} description={t("plans.noPricesHint")} />
            )}
          </SectionCard>

          {plan.content && (
            <SectionCard
              data-testid="plan-content-section"
              title={t("plans.planDetails")}
              description={t("plans.planDetailsHint")}
            >
              <div className="prose prose-sm max-w-none break-words text-foreground dark:prose-invert" dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(plan.content) }} />
            </SectionCard>
          )}
        </>
      )}

      <ResponsiveGrid minColWidth={220} gap={12}>
        <ResourceCard
          icon={<Database className="h-5 w-5" />}
          title={t("plans.traffic")}
          description={t("plans.trafficHint")}
        />
        <ResourceCard
          icon={<Gauge className="h-5 w-5" />}
          title={t("plans.speedLimit")}
          description={t("plans.speedLimitHint")}
        />
        <ResourceCard
          icon={<Smartphone className="h-5 w-5" />}
          title={t("plans.deviceLimit")}
          description={t("plans.deviceLimitHint")}
        />
        <ResourceCard
          icon={<ListChecks className="h-5 w-5" />}
          title={t("plans.readonly")}
          description={t("plans.readonlyHint")}
        />
      </ResponsiveGrid>
    </PageShell>
  );
}
