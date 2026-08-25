import { useTranslation } from "react-i18next";
import { CheckCircle2, GitCommitHorizontal, History } from "lucide-react";
import { formatDateTime } from "@/lib/format";
import type { ConfigCenterSpec } from "@/types";
import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  EmptyState,
  Input,
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
import { formatCoreType } from "./configCenterPageUtils";
import type { ApplyFormState } from "./configCenterPageTypes";
import { useIsMobileViewport } from "@/hooks";

interface SpecsTabProps {
  specs: ConfigCenterSpec[];
  isLoading: boolean;
  error: Error | null;
  latestDesiredRevision: number;
  onRefresh: () => void;
  onCreateSpec: () => void;
  onEditSpec: (spec: ConfigCenterSpec) => void;
  onDeleteSpec: (id: number) => void;
  onHistorySpec: (spec: ConfigCenterSpec) => void;
  applyForm: ApplyFormState;
  onApplyFormChange: React.Dispatch<React.SetStateAction<ApplyFormState>>;
  onApply: () => void;
  applyPending: boolean;
}

function parseSpecProtocolAndPort(spec: ConfigCenterSpec): { protocol: string; port: number | string; transport: string; security: string } {
  try {
    const semantic = typeof spec.semantic_spec === "string" ? JSON.parse(spec.semantic_spec) : spec.semantic_spec;
    if (semantic && typeof semantic === "object") {
      const rec = semantic as Record<string, unknown>;
      let security = "-";
      if (typeof rec.tls === "object" && rec.tls !== null) {
        const tlsConfig = rec.tls as Record<string, unknown>;
        const reality = tlsConfig.reality as { enabled?: boolean } | undefined;
        security = reality?.enabled ? "Reality" : "TLS";
      }
      return {
        protocol: typeof rec.protocol === "string" ? rec.protocol : "-",
        port: typeof rec.port === "number" ? rec.port : "-",
        transport: typeof rec.transport === "object" && rec.transport ? String((rec.transport as Record<string, unknown>).type ?? "-") : "-",
        security,
      };
    }
  } catch { /* ignore parse errors */ }
  return { protocol: "-", port: "-", transport: "-", security: "-" };
}

export default function SpecsTab({
  specs,
  isLoading,
  error,
  latestDesiredRevision,
  onRefresh,
  onCreateSpec,
  onEditSpec,
  onDeleteSpec,
  onHistorySpec,
  applyForm,
  onApplyFormChange,
  onApply,
  applyPending,
}: SpecsTabProps) {
  const { t } = useTranslation();
  const isMobileViewport = useIsMobileViewport();

  const renderSpecEnabledBadge = (enabled: boolean) => (
    <Badge variant={enabled ? "success" : "secondary"}>
      {enabled
        ? t("admin.configCenter.status.enabled")
        : t("admin.configCenter.status.disabled")}
    </Badge>
  );

  const renderSpecActions = (spec: ConfigCenterSpec, layout: "desktop" | "mobile") => (
    <div className={layout === "mobile" ? "mt-4 grid grid-cols-2 gap-2" : "flex flex-wrap gap-2"}>
      <Button size={layout === "mobile" ? "default" : "sm"} variant="outline" onClick={() => onEditSpec(spec)}>
        {t("common.edit")}
      </Button>
      <Button size={layout === "mobile" ? "default" : "sm"} variant="ghost" onClick={() => onHistorySpec(spec)}>
        <History className="mr-1 h-3 w-3" />
        {t("admin.configCenter.actions.history")}
      </Button>
      <Button size={layout === "mobile" ? "default" : "sm"} variant="destructive" onClick={() => onDeleteSpec(spec.id)}>
        {t("common.delete")}
      </Button>
    </div>
  );

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>{t("admin.configCenter.specs.title")}</CardTitle>
          <CardDescription>
            {t("admin.configCenter.specs.description", { count: specs.length, revision: latestDesiredRevision })}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-3 md:grid-cols-[1fr_1fr_auto] md:items-end">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.apply.targetRevision")}</label>
              <Input
                value={applyForm.target_revision}
                onChange={(event) => onApplyFormChange((prev) => ({ ...prev, target_revision: event.target.value }))}
                placeholder={latestDesiredRevision > 0 ? String(latestDesiredRevision) : "1"}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.apply.previousRevision")}</label>
              <Input
                value={applyForm.previous_revision}
                onChange={(event) =>
                  onApplyFormChange((prev) => ({ ...prev, previous_revision: event.target.value }))
                }
                placeholder={t("admin.configCenter.placeholders.optional")}
              />
            </div>
            <Button onClick={onApply} disabled={applyPending || specs.length === 0}>
              <CheckCircle2 className="mr-2 h-4 w-4" />
              {applyPending
                ? t("common.loading")
                : t("admin.configCenter.actions.apply")}
            </Button>
          </div>

          {isLoading ? (
            <Loading />
          ) : error ? (
            <div className="flex flex-col items-center justify-center gap-3 py-10">
              <p className="text-sm text-destructive">{t("admin.configCenter.messages.specLoadFailed")}</p>
              <Button variant="outline" onClick={onRefresh}>
                {t("common.retry")}
              </Button>
            </div>
          ) : specs.length === 0 ? (
            <EmptyState
              icon={<GitCommitHorizontal className="h-full w-full" />}
              title={t("admin.configCenter.empty.noSpecTitle")}
              description={t("admin.configCenter.empty.noSpecDescription")}
              action={<Button onClick={onCreateSpec}>{t("admin.configCenter.actions.createSpec")}</Button>}
              size="sm"
            />
          ) : (
            isMobileViewport ? (
              <ResponsiveList label={t("admin.configCenter.specs.mobileListLabel")}>
                {specs.map((spec) => (
                  <ResponsiveListItem key={spec.id}>
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0 space-y-1">
                        <div className="truncate font-medium text-foreground">{spec.tag}</div>
                        <div className="text-sm text-muted-foreground">
                          {spec.agent_host_id == null ? (
                            <Badge variant="outline">{t("admin.configCenter.template.badge")}</Badge>
                          ) : null}
                          <span className="ml-1">r{spec.desired_revision}</span>
                        </div>
                      </div>
                      {renderSpecEnabledBadge(spec.enabled)}
                    </div>

                    <dl className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
                      <ResponsiveListField label={t("admin.configCenter.fields.tag")} className="col-span-2">
                        <span className="break-all">{spec.tag}</span>
                      </ResponsiveListField>
                      <ResponsiveListField label={t("admin.configCenter.fields.coreType")}>
                        <Badge variant="secondary">{formatCoreType(spec.core_type)}</Badge>
                      </ResponsiveListField>
                      <ResponsiveListField label={t("admin.configCenter.inbound.protocol")}>
                        <span className="font-mono text-xs">{parseSpecProtocolAndPort(spec).protocol}</span>
                      </ResponsiveListField>
                      <ResponsiveListField label={t("admin.configCenter.inbound.port")}>
                        <span className="font-mono text-xs">{String(parseSpecProtocolAndPort(spec).port)}</span>
                      </ResponsiveListField>
                      <ResponsiveListField label={t("admin.configCenter.inbound.transportType")}>
                        <span className="text-xs text-muted-foreground">{parseSpecProtocolAndPort(spec).transport}</span>
                      </ResponsiveListField>
                      <ResponsiveListField label={t("admin.configCenter.inbound.security")}>
                        {parseSpecProtocolAndPort(spec).security !== "-" ? <Badge variant="outline">{parseSpecProtocolAndPort(spec).security}</Badge> : <span className="text-xs text-muted-foreground">-</span>}
                      </ResponsiveListField>
                      <ResponsiveListField label={t("admin.configCenter.fields.revision")}>
                        {spec.desired_revision}
                      </ResponsiveListField>
                      <ResponsiveListField label={t("admin.configCenter.fields.enabled")}>
                        {renderSpecEnabledBadge(spec.enabled)}
                      </ResponsiveListField>
                      <ResponsiveListField label={t("admin.configCenter.fields.updatedAt")} className="col-span-2">
                        {formatDateTime(spec.updated_at)}
                      </ResponsiveListField>
                    </dl>

                    {renderSpecActions(spec, "mobile")}
                  </ResponsiveListItem>
                ))}
              </ResponsiveList>
            ) : (
              <Table aria-label={t("admin.configCenter.specs.title") as string}>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("admin.configCenter.fields.tag")}</TableHead>
                    <TableHead>{t("admin.configCenter.fields.coreType")}</TableHead>
                    <TableHead className="hidden sm:table-cell">{t("admin.configCenter.fields.type")}</TableHead>
                    <TableHead>{t("admin.configCenter.inbound.protocol")}</TableHead>
                    <TableHead className="hidden lg:table-cell">{t("admin.configCenter.inbound.port")}</TableHead>
                    <TableHead className="hidden xl:table-cell">{t("admin.configCenter.inbound.transportType")}</TableHead>
                    <TableHead className="hidden xl:table-cell">{t("admin.configCenter.inbound.security")}</TableHead>
                    <TableHead className="text-right">{t("admin.configCenter.fields.revision")}</TableHead>
                    <TableHead>{t("admin.configCenter.fields.enabled")}</TableHead>
                    <TableHead>{t("admin.configCenter.fields.updatedAt")}</TableHead>
                    <TableHead>{t("common.actions")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {specs.map((spec) => {
                    const info = parseSpecProtocolAndPort(spec);
                    return (
                      <TableRow key={spec.id}>
                        <TableCell className="font-medium">{spec.tag}</TableCell>
                        <TableCell>
                          <Badge variant="secondary">{formatCoreType(spec.core_type)}</Badge>
                        </TableCell>
                        <TableCell className="hidden sm:table-cell">
                          {spec.agent_host_id == null ? (
                            <Badge variant="outline">{t("admin.configCenter.template.badge")}</Badge>
                          ) : (
                            <span className="text-xs text-muted-foreground">-</span>
                          )}
                        </TableCell>
                        <TableCell className="font-mono text-xs">{info.protocol}</TableCell>
                        <TableCell className="hidden lg:table-cell font-mono text-xs">{info.port === "-" ? "-" : String(info.port)}</TableCell>
                        <TableCell className="hidden xl:table-cell text-xs text-muted-foreground">{info.transport}</TableCell>
                        <TableCell className="hidden xl:table-cell">{info.security !== "-" ? <Badge variant="outline">{info.security}</Badge> : <span className="text-xs text-muted-foreground">-</span>}</TableCell>
                        <TableCell className="text-right tabular-nums">{spec.desired_revision}</TableCell>
                        <TableCell>{renderSpecEnabledBadge(spec.enabled)}</TableCell>
                        <TableCell>{formatDateTime(spec.updated_at)}</TableCell>
                        <TableCell>{renderSpecActions(spec, "desktop")}</TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            )
          )}
        </CardContent>
      </Card>
    </div>
  );
}
