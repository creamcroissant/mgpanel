import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { Diff, GitCompare } from "lucide-react";
import { QUERY_KEYS } from "@/lib/constants";
import { listConfigCenterAppliedSnapshot } from "@/api/admin";
import { formatDateTime } from "@/lib/format";
import type { ConfigCenterAppliedSnapshot, ConfigCenterInboundIndex, ConfigCenterInventory } from "@/types";
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
  TabsContent,
} from "@/components/ui";
import type { CoreTypeOption } from "./configCenterPageTypes";
import { useIsMobileViewport } from "@/hooks";

interface SnapshotTabProps {
  selectedHostId: number | null;
  selectedCoreType: CoreTypeOption;
}

function renderInventoryMobileList(items: ConfigCenterInventory[], t: (key: string) => string) {
  return (
    <ResponsiveList label={t("admin.configCenter.snapshot.inventoryTitle")}>
      {items.map((item) => (
        <ResponsiveListItem key={item.id}>
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0 break-all font-medium text-foreground">{item.filename}</div>
            <Badge variant={item.parse_status === "ok" ? "success" : "warning"}>{item.parse_status}</Badge>
          </div>
          <dl className="mt-4 grid grid-cols-2 gap-3">
            <ResponsiveListField label={t("admin.configCenter.fields.source")}>
              <Badge variant="secondary">{t(`admin.configCenter.source.${item.source}`)}</Badge>
            </ResponsiveListField>
            <ResponsiveListField label={t("admin.configCenter.fields.filename")}>
              <span className="break-all">{item.filename}</span>
            </ResponsiveListField>
            <ResponsiveListField label={t("admin.configCenter.fields.parseStatus")}>
              <Badge variant={item.parse_status === "ok" ? "success" : "warning"}>{item.parse_status}</Badge>
            </ResponsiveListField>
            <ResponsiveListField label={t("admin.configCenter.fields.lastSeenAt")}>
              {formatDateTime(item.last_seen_at)}
            </ResponsiveListField>
          </dl>
        </ResponsiveListItem>
      ))}
    </ResponsiveList>
  );
}

function renderInboundMobileList(items: ConfigCenterInboundIndex[], t: (key: string) => string) {
  return (
    <ResponsiveList label={t("admin.configCenter.snapshot.inboundTitle")}>
      {items.map((item) => (
        <ResponsiveListItem key={item.id}>
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0 break-all font-medium text-foreground">{item.tag}</div>
            <Badge variant="secondary">{item.protocol || "-"}</Badge>
          </div>
          <dl className="mt-4 grid grid-cols-2 gap-3">
            <ResponsiveListField label={t("admin.configCenter.fields.source")}>
              <Badge variant="secondary">{t(`admin.configCenter.source.${item.source}`)}</Badge>
            </ResponsiveListField>
            <ResponsiveListField label={t("admin.configCenter.fields.tag")}>
              <span className="break-all">{item.tag}</span>
            </ResponsiveListField>
            <ResponsiveListField label={t("admin.configCenter.fields.protocol")}>
              {item.protocol || "-"}
            </ResponsiveListField>
            <ResponsiveListField label={t("admin.configCenter.fields.listen")}>
              {item.listen || "-"}
            </ResponsiveListField>
            <ResponsiveListField label={t("admin.configCenter.fields.port")}>{item.port ?? "-"}</ResponsiveListField>
          </dl>
        </ResponsiveListItem>
      ))}
    </ResponsiveList>
  );
}

export default function SnapshotTab({ selectedHostId, selectedCoreType }: SnapshotTabProps) {
  const { t } = useTranslation();
  const isMobileViewport = useIsMobileViewport();

  const snapshotQuery = useQuery({
    queryKey: [
      ...QUERY_KEYS.ADMIN_CONFIG_CENTER_SNAPSHOT,
      selectedHostId,
      selectedCoreType,
    ],
    queryFn: () =>
      listConfigCenterAppliedSnapshot({
        agent_host_id: selectedHostId ?? 0,
        core_type: selectedCoreType,
        limit: 200,
        offset: 0,
      }),
    enabled: selectedHostId !== null,
  });

  const snapshot = snapshotQuery.data as ConfigCenterAppliedSnapshot | undefined;
  const snapshotInventories = useMemo(
    () => (Array.isArray(snapshot?.inventories) ? snapshot.inventories : []),
    [snapshot],
  );
  const snapshotInboundIndexes = useMemo(
    () => (Array.isArray(snapshot?.inbound_indexes) ? snapshot.inbound_indexes : []),
    [snapshot],
  );

  return (
    <TabsContent value="snapshot" className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>{t("admin.configCenter.snapshot.title")}</CardTitle>
          <CardDescription>{t("admin.configCenter.snapshot.description")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {snapshotQuery.isLoading ? (
            <Loading />
          ) : snapshotQuery.error ? (
            <p className="text-sm text-destructive">{t("admin.configCenter.messages.snapshotLoadFailed")}</p>
          ) : (
            <>
              <div>
                <h3 className="mb-3 text-sm font-semibold text-foreground">
                  {t("admin.configCenter.snapshot.inventoryTitle")}
                </h3>
                {snapshotInventories.length === 0 ? (
                  <EmptyState
                    icon={<Diff className="h-full w-full" />}
                    title={t("admin.configCenter.empty.noInventoryTitle")}
                    description={t("admin.configCenter.empty.noInventoryDescription")}
                    size="sm"
                  />
                ) : (
                  isMobileViewport ? (
                    renderInventoryMobileList(snapshotInventories, t)
                  ) : (
                    <Table aria-label={t("admin.configCenter.snapshot.inventoryTitle") as string}>
                      <TableHeader>
                        <TableRow>
                          <TableHead>{t("admin.configCenter.fields.source")}</TableHead>
                          <TableHead>{t("admin.configCenter.fields.filename")}</TableHead>
                          <TableHead>{t("admin.configCenter.fields.parseStatus")}</TableHead>
                          <TableHead>{t("admin.configCenter.fields.lastSeenAt")}</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {snapshotInventories.map((item) => (
                          <TableRow key={item.id}>
                            <TableCell>
                              <Badge variant="secondary">{t(`admin.configCenter.source.${item.source}`)}</Badge>
                            </TableCell>
                            <TableCell>{item.filename}</TableCell>
                            <TableCell>
                              <Badge variant={item.parse_status === "ok" ? "success" : "warning"}>
                                {item.parse_status}
                              </Badge>
                            </TableCell>
                            <TableCell>{formatDateTime(item.last_seen_at)}</TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  )
                )}
              </div>

              <div>
                <h3 className="mb-3 text-sm font-semibold text-foreground">
                  {t("admin.configCenter.snapshot.inboundTitle")}
                </h3>
                {snapshotInboundIndexes.length === 0 ? (
                  <EmptyState
                    icon={<GitCompare className="h-full w-full" />}
                    title={t("admin.configCenter.empty.noInboundIndexTitle")}
                    description={t("admin.configCenter.empty.noInboundIndexDescription")}
                    size="sm"
                  />
                ) : (
                  isMobileViewport ? (
                    renderInboundMobileList(snapshotInboundIndexes, t)
                  ) : (
                    <Table aria-label={t("admin.configCenter.snapshot.inboundTitle") as string}>
                      <TableHeader>
                        <TableRow>
                          <TableHead>{t("admin.configCenter.fields.source")}</TableHead>
                          <TableHead>{t("admin.configCenter.fields.tag")}</TableHead>
                          <TableHead>{t("admin.configCenter.fields.protocol")}</TableHead>
                          <TableHead>{t("admin.configCenter.fields.listen")}</TableHead>
                          <TableHead>{t("admin.configCenter.fields.port")}</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {snapshotInboundIndexes.map((item) => (
                          <TableRow key={item.id}>
                            <TableCell>
                              <Badge variant="secondary">{t(`admin.configCenter.source.${item.source}`)}</Badge>
                            </TableCell>
                            <TableCell>{item.tag}</TableCell>
                            <TableCell>{item.protocol || "-"}</TableCell>
                            <TableCell>{item.listen || "-"}</TableCell>
                            <TableCell>{item.port ?? "-"}</TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  )
                )}
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </TabsContent>
  );
}
