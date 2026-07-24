import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import {
  getSubscriptionFilterSummary,
  listSubscriptionFilterReasons,
} from "@/api/admin/subscription";
import { QUERY_KEYS } from "@/lib/constants";
import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  EmptyState,
  Table,
  TableBody,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui";
import ErrorBanner from "@/components/ui/ErrorBanner";
import FilterReasonRow from "./SubscriptionDiagnostics";

const FILTER_REASON_LIMIT = 8;

function MetricTile({
  label,
  value,
}: { label: string; value: number | string }) {
  return (
    <div className="rounded-md border border-border bg-muted/20 p-3">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="mt-1 text-xl font-semibold tabular-nums text-foreground">
        {value}
      </p>
    </div>
  );
}

function getFilterReasonLabel(
  reason: string,
  t: (key: string) => string,
): string {
  return t(`admin.system.subscription.filterReasonOptions.${reason}`);
}

export default function SubscriptionSummaryPanel() {
  const { t } = useTranslation();

  const summaryQuery = useQuery({
    queryKey: QUERY_KEYS.ADMIN_SUBSCRIPTION_FILTER_SUMMARY,
    queryFn: () => getSubscriptionFilterSummary(),
  });
  const reasonsQuery = useQuery({
    queryKey: [
      ...QUERY_KEYS.ADMIN_SUBSCRIPTION_FILTER_REASONS,
      FILTER_REASON_LIMIT,
    ],
    queryFn: () => listSubscriptionFilterReasons({ limit: FILTER_REASON_LIMIT }),
  });

  const summary = summaryQuery.data;
  const reasons = reasonsQuery.data?.reasons ?? [];
  const reasonCounts = Object.entries(summary?.reason_counts ?? {})
    .filter(([, count]) => count > 0)
    .sort((a, b) => b[1] - a[1]);

  return (
    <Card className="border border-border shadow-none">
      <CardHeader>
        <CardTitle>
          {t("admin.system.subscription.filterDiagnosticsTitle")}
        </CardTitle>
        <CardDescription>
          {t("admin.system.subscription.filterDiagnosticsDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">
        {summaryQuery.error ? (
          <ErrorBanner
            message={t("admin.system.subscription.filterSummaryLoadError")}
            onRetry={() => summaryQuery.refetch()}
          />
        ) : (
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
            <MetricTile
              label={t("admin.system.subscription.availableNodeCount")}
              value={summary?.available_node_count ?? "-"}
            />
            <MetricTile
              label={t("admin.system.subscription.filteredNodeCount")}
              value={summary?.filtered_node_count ?? "-"}
            />
            <MetricTile
              label={t("admin.system.subscription.totalNodeCount")}
              value={summary?.total_node_count ?? "-"}
            />
            <MetricTile
              label={t("admin.system.subscription.sourceNodeCount")}
              value={summary?.source_node_count ?? "-"}
            />
            <MetricTile
              label={t("admin.system.subscription.enabledSourceCount")}
              value={summary?.enabled_source_count ?? "-"}
            />
            <MetricTile
              label={t("admin.system.subscription.selfHostedCount")}
              value={summary?.self_hosted_count ?? "-"}
            />
          </div>
        )}

        <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
          <div className="rounded-md border border-border p-4">
            <h4 className="text-sm font-medium text-foreground">
              {t("admin.system.subscription.reasonDistribution")}
            </h4>
            {summaryQuery.isLoading ? (
              <p className="mt-3 text-sm text-muted-foreground">
                {t("common.loading")}
              </p>
            ) : reasonCounts.length === 0 ? (
              <p className="mt-3 text-sm text-muted-foreground">
                {t("admin.system.subscription.noReasonDistribution")}
              </p>
            ) : (
              <div className="mt-3 space-y-2">
                {reasonCounts.map(([reason, count]) => (
                  <div
                    key={reason}
                    className="flex items-center justify-between gap-3 text-sm"
                  >
                    <span className="text-muted-foreground">
                      {getFilterReasonLabel(reason, t)}
                    </span>
                    <Badge variant="outline">{count}</Badge>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="rounded-md border border-border p-4">
            <div className="flex items-start justify-between gap-3">
              <div>
                <h4 className="text-sm font-medium text-foreground">
                  {t("admin.system.subscription.recentFilteredNodes")}
                </h4>
                <p className="mt-1 text-xs text-muted-foreground">
                  {t("admin.system.subscription.recentFilteredNodesHint")}
                </p>
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={() => reasonsQuery.refetch()}
              >
                {t("common.refresh")}
              </Button>
            </div>

            {reasonsQuery.isLoading ? (
              <p className="mt-4 text-sm text-muted-foreground">
                {t("common.loading")}
              </p>
            ) : reasonsQuery.error ? (
              <div className="mt-4">
                <ErrorBanner
                  message={t(
                    "admin.system.subscription.filterReasonsLoadError",
                  )}
                  onRetry={() => reasonsQuery.refetch()}
                />
              </div>
            ) : reasons.length === 0 ? (
              <EmptyState
                size="sm"
                title={t("admin.system.subscription.filtersEmpty")}
                description={t(
                  "admin.system.subscription.filtersEmptyDescription",
                )}
              />
            ) : (
              <div className="mt-4 overflow-x-auto rounded-md border border-border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>
                        {t("admin.system.subscription.nodeName")}
                      </TableHead>
                      <TableHead>
                        {t("admin.system.subscription.reason")}
                      </TableHead>
                      <TableHead>
                        {t("admin.system.subscription.source")}
                      </TableHead>
                      <TableHead>
                        {t("admin.system.subscription.detail")}
                      </TableHead>
                      <TableHead>
                        {t("admin.system.subscription.createdAt")}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {reasons.map((reason) => (
                      <FilterReasonRow
                        key={reason.id}
                        reason={reason}
                      />
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
