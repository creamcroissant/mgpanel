import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { CheckCircle2, ShieldAlert } from "lucide-react";
import { QUERY_KEYS } from "@/lib/constants";
import { listConfigCenterDriftStates, listConfigCenterRecoveryStates } from "@/api/admin";
import { formatDateTime } from "@/lib/format";
import type { ConfigCenterDriftState } from "@/types";
import {
  Badge,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  EmptyState,
  Loading,
  ResponsiveList,
  ResponsiveListField,
  ResponsiveListItem,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui";
import { formatDriftVariant } from "./configCenterPageUtils";
import type { CoreTypeOption } from "./configCenterPageTypes";
import { useIsMobileViewport } from "@/hooks";

interface DriftTabProps {
  selectedHostId: number | null;
  selectedCoreType: CoreTypeOption;
}

function renderDriftStateMobileList(
  items: ConfigCenterDriftState[],
  label: string,
  recovered: boolean,
  t: (key: string) => string,
) {
  return (
    <ResponsiveList label={label}>
      {items.map((item) => (
        <ResponsiveListItem key={item.id}>
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0 space-y-1">
              <div className="break-all font-medium text-foreground">{item.tag}</div>
              <div className="break-all text-sm text-muted-foreground">{item.filename}</div>
            </div>
            <Badge variant={recovered ? "secondary" : formatDriftVariant(item.drift_type)}>{item.drift_type}</Badge>
          </div>
          <dl className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
            <ResponsiveListField label={t("admin.configCenter.fields.tag")}>
              <span className="break-all">{item.tag}</span>
            </ResponsiveListField>
            <ResponsiveListField label={t("admin.configCenter.fields.filename")}>
              <span className="break-all">{item.filename}</span>
            </ResponsiveListField>
            <ResponsiveListField label={t("admin.configCenter.fields.driftType")}>
              <Badge variant={recovered ? "secondary" : formatDriftVariant(item.drift_type)}>{item.drift_type}</Badge>
            </ResponsiveListField>
            <ResponsiveListField label={t("admin.configCenter.fields.updatedAt")}>
              {formatDateTime(item.last_changed_at)}
            </ResponsiveListField>
          </dl>
        </ResponsiveListItem>
      ))}
    </ResponsiveList>
  );
}

export default function DriftTab({ selectedHostId, selectedCoreType }: DriftTabProps) {
  const { t } = useTranslation();
  const isMobileViewport = useIsMobileViewport();

  const driftQuery = useQuery({
    queryKey: [
      ...QUERY_KEYS.ADMIN_CONFIG_CENTER_DRIFT,
      selectedHostId,
      selectedCoreType,
    ],
    queryFn: () =>
      listConfigCenterDriftStates({
        agent_host_id: selectedHostId ?? 0,
        core_type: selectedCoreType,
        limit: 200,
        offset: 0,
      }),
    enabled: selectedHostId !== null,
  });

  const recoveryQuery = useQuery({
    queryKey: [
      ...QUERY_KEYS.ADMIN_CONFIG_CENTER_RECOVER,
      selectedHostId,
      selectedCoreType,
    ],
    queryFn: () =>
      listConfigCenterRecoveryStates({
        agent_host_id: selectedHostId ?? 0,
        core_type: selectedCoreType,
        limit: 200,
        offset: 0,
      }),
    enabled: selectedHostId !== null,
  });

  const driftStates = useMemo(() => driftQuery.data?.data ?? [], [driftQuery.data?.data]);
  const recoveryStates = useMemo(() => recoveryQuery.data?.data ?? [], [recoveryQuery.data?.data]);

  return (
    <div className="space-y-4">
      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>{t("admin.configCenter.drift.activeTitle")}</CardTitle>
            <CardDescription>{t("admin.configCenter.drift.activeDescription")}</CardDescription>
          </CardHeader>
          <CardContent>
            {driftQuery.isLoading ? (
              <Loading />
            ) : driftQuery.error ? (
              <p className="text-sm text-destructive">{t("admin.configCenter.messages.driftLoadFailed")}</p>
            ) : driftStates.length === 0 ? (
              <EmptyState
                icon={<ShieldAlert className="h-full w-full" />}
                title={t("admin.configCenter.empty.noDriftTitle")}
                description={t("admin.configCenter.empty.noDriftDescription")}
                size="sm"
              />
            ) : (
              isMobileViewport ? (
                renderDriftStateMobileList(
                  driftStates,
                  t("admin.configCenter.drift.activeTitle"),
                  false,
                  t,
                )
              ) : (
                <Table aria-label={t("admin.configCenter.drift.activeTitle") as string}>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("admin.configCenter.fields.tag")}</TableHead>
                      <TableHead>{t("admin.configCenter.fields.filename")}</TableHead>
                      <TableHead>{t("admin.configCenter.fields.driftType")}</TableHead>
                      <TableHead>{t("admin.configCenter.fields.updatedAt")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {driftStates.map((item) => (
                      <TableRow key={item.id}>
                        <TableCell>{item.tag}</TableCell>
                        <TableCell>{item.filename}</TableCell>
                        <TableCell>
                          <Badge variant={formatDriftVariant(item.drift_type)}>{item.drift_type}</Badge>
                        </TableCell>
                        <TableCell>{formatDateTime(item.last_changed_at)}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t("admin.configCenter.drift.recoveredTitle")}</CardTitle>
            <CardDescription>{t("admin.configCenter.drift.recoveredDescription")}</CardDescription>
          </CardHeader>
          <CardContent>
            {recoveryQuery.isLoading ? (
              <Loading />
            ) : recoveryQuery.error ? (
              <p className="text-sm text-destructive">{t("admin.configCenter.messages.recoveryLoadFailed")}</p>
            ) : recoveryStates.length === 0 ? (
              <EmptyState
                icon={<CheckCircle2 className="h-full w-full" />}
                title={t("admin.configCenter.empty.noRecoveryTitle")}
                description={t("admin.configCenter.empty.noRecoveryDescription")}
                size="sm"
              />
            ) : (
              isMobileViewport ? (
                renderDriftStateMobileList(
                  recoveryStates,
                  t("admin.configCenter.drift.recoveredTitle"),
                  true,
                  t,
                )
              ) : (
                <Table aria-label={t("admin.configCenter.drift.recoveredTitle") as string}>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("admin.configCenter.fields.tag")}</TableHead>
                      <TableHead>{t("admin.configCenter.fields.filename")}</TableHead>
                      <TableHead>{t("admin.configCenter.fields.driftType")}</TableHead>
                      <TableHead>{t("admin.configCenter.fields.updatedAt")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {recoveryStates.map((item) => (
                      <TableRow key={item.id}>
                        <TableCell>{item.tag}</TableCell>
                        <TableCell>{item.filename}</TableCell>
                        <TableCell>
                          <Badge variant="secondary">{item.drift_type}</Badge>
                        </TableCell>
                        <TableCell>{formatDateTime(item.last_changed_at)}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
