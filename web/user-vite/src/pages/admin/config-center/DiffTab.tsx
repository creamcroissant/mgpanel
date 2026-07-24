import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { QUERY_KEYS } from "@/lib/constants";
import { getConfigCenterTextDiff, getConfigCenterSemanticDiff } from "@/api/admin";
import type { ConfigCenterSemanticDiffItem } from "@/types";
import {
  Badge,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
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
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui";
import {
  formatDriftVariant,
  formatQueryErrorMessage,
  parseOptionalPositiveRevision,
} from "./configCenterPageUtils";
import type { CoreTypeOption } from "./configCenterPageTypes";
import { useIsMobileViewport } from "@/hooks";

interface DiffTabProps {
  selectedHostId: number | null;
  selectedCoreType: CoreTypeOption;
}

function renderFieldDiffs(item: ConfigCenterSemanticDiffItem) {
  return item.field_diffs && item.field_diffs.length > 0 ? (
    <div className="space-y-1 text-xs text-muted-foreground">
      {item.field_diffs.map((fieldDiff, fdIndex) => (
        <div key={`${fieldDiff.field}-${fdIndex}`} className="break-all">
          <span className="font-medium text-foreground">{fieldDiff.field}</span>
          <span className="mx-1">:</span>
          <span>{fieldDiff.desired}</span>
          <span className="mx-1">&rarr;</span>
          <span>{fieldDiff.applied}</span>
        </div>
      ))}
    </div>
  ) : (
    <span className="text-xs text-muted-foreground">-</span>
  );
}

function renderSemanticDiffMobileList(
  items: ConfigCenterSemanticDiffItem[],
  label: string,
  showFieldDiffs: boolean,
  t: (key: string) => string,
) {
  return (
    <ResponsiveList label={label}>
      {items.map((item, index) => (
        <ResponsiveListItem key={`${item.tag}-${index}`}>
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0 font-medium text-foreground break-all">{item.tag}</div>
            <Badge variant={formatDriftVariant(item.drift_type)}>{item.drift_type}</Badge>
          </div>
          <dl className="mt-4 grid grid-cols-2 gap-3">
            <ResponsiveListField label={t("admin.configCenter.fields.tag")} className="col-span-2">
              <span className="break-all">{item.tag}</span>
            </ResponsiveListField>
            <ResponsiveListField label={t("admin.configCenter.fields.driftType")}>
              <Badge variant={formatDriftVariant(item.drift_type)}>{item.drift_type}</Badge>
            </ResponsiveListField>
            {showFieldDiffs ? (
              <ResponsiveListField label={t("admin.configCenter.fields.fieldDiffs")} className="col-span-2">
                {renderFieldDiffs(item)}
              </ResponsiveListField>
            ) : null}
          </dl>
        </ResponsiveListItem>
      ))}
    </ResponsiveList>
  );
}

export default function DiffTab({ selectedHostId, selectedCoreType }: DiffTabProps) {
  const { t } = useTranslation();
  const isMobileViewport = useIsMobileViewport();
  const [activeDiffTab, setActiveDiffTab] = useState("text");
  const [diffFilename, setDiffFilename] = useState("");
  const [diffTag, setDiffTag] = useState("");
  const [diffRevision, setDiffRevision] = useState("");

  const parsedDiffRevision = parseOptionalPositiveRevision(diffRevision);

  const textDiffQuery = useQuery({
    queryKey: [
      ...QUERY_KEYS.ADMIN_CONFIG_CENTER_DIFF_TEXT,
      selectedHostId,
      selectedCoreType,
      diffRevision,
      diffFilename,
      diffTag,
    ],
    queryFn: () =>
      getConfigCenterTextDiff({
        agent_host_id: selectedHostId ?? 0,
        core_type: selectedCoreType,
        desired_revision: parsedDiffRevision,
        filename: (diffFilename || "").trim() || undefined,
        tag: (diffTag || "").trim() || undefined,
      }),
    enabled: selectedHostId !== null && Boolean((diffFilename || "").trim() || (diffTag || "").trim()),
  });

  const semanticDiffQuery = useQuery({
    queryKey: [
      ...QUERY_KEYS.ADMIN_CONFIG_CENTER_DIFF_SEMANTIC,
      selectedHostId,
      selectedCoreType,
      diffRevision,
      diffTag,
    ],
    queryFn: () =>
      getConfigCenterSemanticDiff({
        agent_host_id: selectedHostId ?? 0,
        core_type: selectedCoreType,
        desired_revision: parsedDiffRevision,
        tag: (diffTag || "").trim() || undefined,
      }),
    enabled: selectedHostId !== null,
  });

  return (
    <TabsContent value="diff" className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>{t("admin.configCenter.diff.title")}</CardTitle>
          <CardDescription>{t("admin.configCenter.diff.description")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-3 md:grid-cols-3">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.fields.revision")}</label>
              <Input
                value={diffRevision}
                onChange={(event) => setDiffRevision(event.target.value)}
                placeholder={t("admin.configCenter.placeholders.optional")}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.fields.filename")}</label>
              <Input
                value={diffFilename}
                onChange={(event) => setDiffFilename(event.target.value)}
                placeholder={t("admin.configCenter.placeholders.optional")}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.fields.tag")}</label>
              <Input
                value={diffTag}
                onChange={(event) => setDiffTag(event.target.value)}
                placeholder={t("admin.configCenter.placeholders.optional")}
              />
            </div>
          </div>

          <Tabs value={activeDiffTab} onValueChange={setActiveDiffTab}>
            <TabsList>
              <TabsTrigger value="text">{t("admin.configCenter.diff.text")}</TabsTrigger>
              <TabsTrigger value="semantic">{t("admin.configCenter.diff.semantic")}</TabsTrigger>
            </TabsList>

            <TabsContent value="text" className="space-y-3">
              {!((diffFilename || "").trim() || (diffTag || "").trim()) ? (
                <div className="rounded-md border border-border bg-muted/30 p-3 text-sm text-muted-foreground">
                  {t("admin.configCenter.empty.selectTextDiffSelector")}
                </div>
              ) : textDiffQuery.isLoading ? (
                <Loading />
              ) : textDiffQuery.error ? (
                <div className="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
                  {t("admin.configCenter.messages.textDiffFailed")}
                  <div className="mt-1 text-xs opacity-80">{formatQueryErrorMessage(textDiffQuery.error)}</div>
                </div>
              ) : textDiffQuery.data ? (
                <>
                  <div className="grid gap-3 md:grid-cols-2">
                    <div className="rounded-md border border-border p-3">
                      <p className="mb-2 text-xs text-muted-foreground">{t("admin.configCenter.diff.desired")}</p>
                      <pre className="max-h-60 overflow-auto whitespace-pre-wrap text-xs">
                        {textDiffQuery.data.desired_text}
                      </pre>
                    </div>
                    <div className="rounded-md border border-border p-3">
                      <p className="mb-2 text-xs text-muted-foreground">{t("admin.configCenter.diff.applied")}</p>
                      <pre className="max-h-60 overflow-auto whitespace-pre-wrap text-xs">
                        {textDiffQuery.data.applied_text}
                      </pre>
                    </div>
                  </div>
                  <div className="rounded-md border border-border p-3">
                    <p className="mb-2 text-xs text-muted-foreground">{t("admin.configCenter.diff.unified")}</p>
                    <pre className="max-h-80 overflow-auto whitespace-pre-wrap text-xs">
                      {textDiffQuery.data.unified_diff || "-"}
                    </pre>
                  </div>
                </>
              ) : null}
            </TabsContent>

            <TabsContent value="semantic">
              {semanticDiffQuery.isLoading ? (
                <Loading />
              ) : semanticDiffQuery.error ? (
                <div className="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
                  {t("admin.configCenter.messages.semanticDiffFailed")}
                </div>
              ) : semanticDiffQuery.data ? (
                isMobileViewport ? (
                  semanticDiffQuery.data.items && semanticDiffQuery.data.items.length > 0 ? (
                    renderSemanticDiffMobileList(
                      semanticDiffQuery.data.items,
                      t("admin.configCenter.diff.semantic"),
                      true,
                      t,
                    )
                  ) : (
                    <p className="text-sm text-muted-foreground">
                      {t("admin.configCenter.empty.noSemanticDiff")}
                    </p>
                  )
                ) : (
                  <Table aria-label={t("admin.configCenter.diff.semantic") as string}>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t("admin.configCenter.fields.tag")}</TableHead>
                        <TableHead>{t("admin.configCenter.fields.driftType")}</TableHead>
                        <TableHead>{t("admin.configCenter.fields.fieldDiffs")}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {!semanticDiffQuery.data.items || semanticDiffQuery.data.items.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={3} className="text-center text-muted-foreground">
                            {t("admin.configCenter.empty.noSemanticDiff")}
                          </TableCell>
                        </TableRow>
                      ) : (
                        semanticDiffQuery.data.items.map((item, index) => (
                          <TableRow key={`${item.tag}-${index}`}>
                            <TableCell>{item.tag}</TableCell>
                            <TableCell>
                              <Badge variant={formatDriftVariant(item.drift_type)}>{item.drift_type}</Badge>
                            </TableCell>
                            <TableCell>{renderFieldDiffs(item)}</TableCell>
                          </TableRow>
                        ))
                      )}
                    </TableBody>
                  </Table>
                )
              ) : null}
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>
    </TabsContent>
  );
}
