import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { RefreshCw, Workflow } from "lucide-react";
import { QUERY_KEYS } from "@/lib/constants";
import {
  createConfigCenterApplyRun,
  createConfigCenterSpec,
  deleteConfigCenterSpec,
  getConfigCenterSpecHistory,
  listConfigCenterSpecs,
  updateConfigCenterSpec,
} from "@/api/admin";
import { getAgentHosts } from "@/api/admin/agentHost";
import type {
  ConfigCenterSpec,
  ConfigCenterSpecRevision,
} from "@/types/configCenter";
import {
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  Loading,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui";
import { formatDateTime } from "@/lib/format";

import {
  type ConfigCenterSearchState,
  type ConfigCenterTabValue,
  type ApplyFormState,
  type SpecFormState,
  type CoreTypeOption,
  CORE_OPTIONS,
  CONFIG_CENTER_TAB_VALUES,
  defaultSpecFormState,
  defaultApplyFormState,
} from "./configCenterPageTypes";
import {
  parseConfigCenterSearchParams,
  writeConfigCenterSearchParams,
  formatCoreType,
} from "./configCenterPageUtils";
import SpecsTab from "./SpecsTab";
import { ApplyRunTab } from "./ApplyRunTab";
import DiffTab from "./DiffTab";
import DriftTab from "./DriftTab";
import SnapshotTab from "./SnapshotTab";
import { SpecEditorDialog } from "./SpecEditorDialog";

export default function ConfigCenterPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const [configSearchState, setConfigSearchState] = useState(() => parseConfigCenterSearchParams(searchParams));
  const configSearchStateRef = useRef(configSearchState);

  useEffect(() => {
    configSearchStateRef.current = configSearchState;
  }, [configSearchState]);

  useEffect(() => {
    const handlePopState = () => {
      const nextSearchState = parseConfigCenterSearchParams(new URLSearchParams(window.location.search));
      configSearchStateRef.current = nextSearchState;
      setConfigSearchState(nextSearchState);
    };
    window.addEventListener("popstate", handlePopState);
    return () => window.removeEventListener("popstate", handlePopState);
  }, []);

  const selectedHostId = configSearchState.hostId;
  const selectedCoreType = configSearchState.coreType;
  const activeTab = configSearchState.tab;

  // ---- Spec editor state ----
  const [selectedSpec, setSelectedSpec] = useState<ConfigCenterSpec | null>(null);
  const [isSpecDialogOpen, setIsSpecDialogOpen] = useState(false);
  const [isHistoryDialogOpen, setIsHistoryDialogOpen] = useState(false);
  const [isDeleteConfirmOpen, setIsDeleteConfirmOpen] = useState(false);
  const [deleteSpecId, setDeleteSpecId] = useState<number | null>(null);
  const [deleteSpecTag, setDeleteSpecTag] = useState("");
  const [specForm, setSpecForm] = useState<SpecFormState>(defaultSpecFormState);
  const [applyForm, setApplyForm] = useState<ApplyFormState>(defaultApplyFormState);
  const [historyTarget, setHistoryTarget] = useState<ConfigCenterSpec | null>(null);

  // ---- Diff / Apply shared state ----
  const [diffFilename] = useState("");
  const [diffTag] = useState("");

  // ---- URL sync ----
  const updateConfigCenterSearch = (updates: {
    hostId?: number | null;
    coreType?: CoreTypeOption;
    tab?: ConfigCenterTabValue;
  }) => {
    const current = configSearchStateRef.current;
    const nextSearchState: ConfigCenterSearchState = {
      hostId: updates.hostId !== undefined ? updates.hostId : current.hostId,
      coreType: updates.coreType ?? current.coreType,
      tab: updates.tab ?? current.tab,
    };
    configSearchStateRef.current = nextSearchState;
    setConfigSearchState(nextSearchState);
    setSearchParams((currentSearchParams) => {
      const nextSearchParams = writeConfigCenterSearchParams(currentSearchParams, nextSearchState);
      if (nextSearchParams.toString() === currentSearchParams.toString()) {
        return currentSearchParams;
      }
      return nextSearchParams;
    });
  };

  // ---- Queries ----
  const hostQuery = useQuery({
    queryKey: QUERY_KEYS.ADMIN_AGENTS,
    queryFn: () => getAgentHosts({ page: 1, page_size: 100 }),
  });

  const specListQuery = useQuery({
    queryKey: [
      ...QUERY_KEYS.ADMIN_CONFIG_CENTER_SPECS,
      selectedHostId,
      selectedCoreType,
    ],
    queryFn: () =>
      listConfigCenterSpecs({
        agent_host_id: selectedHostId ?? undefined,
        core_type: selectedCoreType,
        limit: 100,
        offset: 0,
      }),
    enabled: true,
  });

  const historyQuery = useQuery({
    queryKey: [
      ...QUERY_KEYS.ADMIN_CONFIG_CENTER_SPEC_HISTORY,
      historyTarget?.id,
    ],
    queryFn: () => getConfigCenterSpecHistory(historyTarget?.id ?? 0, { limit: 50, offset: 0 }),
    enabled: Boolean(historyTarget?.id) && isHistoryDialogOpen,
  });

  // ---- Mutations ----
  const invalidateSpecs = () =>
    queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_CONFIG_CENTER_SPECS });

  const createSpecMutation = useMutation({
    mutationFn: (payload: Parameters<typeof createConfigCenterSpec>[0]) => createConfigCenterSpec(payload),
    onSuccess: () => { invalidateSpecs(); setIsSpecDialogOpen(false); toast.success(t("admin.configCenter.messages.saveSuccess")); },
    onError: (err: Error) => toast.error(t("admin.configCenter.messages.saveFailed"), { description: err.message }),
  });

  const updateSpecMutation = useMutation({
    mutationFn: ({ specId, payload }: { specId: number; payload: Parameters<typeof updateConfigCenterSpec>[1] }) =>
      updateConfigCenterSpec(specId, payload),
    onSuccess: () => { invalidateSpecs(); setIsSpecDialogOpen(false); toast.success(t("admin.configCenter.messages.saveSuccess")); },
    onError: (err: Error) => toast.error(t("admin.configCenter.messages.saveFailed"), { description: err.message }),
  });

  const deleteSpecMutation = useMutation({
    mutationFn: deleteConfigCenterSpec,
    onSuccess: () => { invalidateSpecs(); setIsDeleteConfirmOpen(false); toast.success(t("admin.configCenter.messages.deleteSuccess")); },
    onError: (err: Error) => toast.error(t("admin.configCenter.messages.deleteFailed"), { description: err.message }),
  });


  const applyMutation = useMutation({
    mutationFn: (payload: Parameters<typeof createConfigCenterApplyRun>[0]) => createConfigCenterApplyRun(payload),
    onSuccess: () => { invalidateSpecs(); toast.success(t("admin.configCenter.messages.applySuccess")); },
    onError: (err: Error) => toast.error(t("admin.configCenter.messages.applyFailed"), { description: err.message }),
  });

  // ---- Spec handlers ----
  const openCreateDialog = (asTemplate = false) => {
    setSelectedSpec(null);
    setSpecForm({
      ...defaultSpecFormState,
      agent_host_id: asTemplate ? null : selectedHostId,
      core_type: selectedCoreType,
      is_template: asTemplate,
    });
    setIsSpecDialogOpen(true);
  };

  const openEditDialog = (spec: ConfigCenterSpec) => {
    setSelectedSpec(spec);
    setSpecForm({
      agent_host_id: spec.agent_host_id,
      core_type: formatCoreType(spec.core_type),
      tag: spec.tag,
      enabled: spec.enabled,
      semantic_spec: JSON.stringify(spec.semantic_spec, null, 2),
      core_specific: JSON.stringify(spec.core_specific, null, 2),
      change_note: "",
      is_template: spec.agent_host_id == null,
    });
    // SpecJSONErrors, AdvancedJson, XHTTPDraft moved to SpecEditorDialog
    setIsSpecDialogOpen(true);
  };

  const openHistoryDialog = (spec: ConfigCenterSpec) => {
    setHistoryTarget(spec);
    setIsHistoryDialogOpen(true);
  };

  const handleSpecDialogOpenChange = (open: boolean) => {
    setIsSpecDialogOpen(open);
    if (!open) setSelectedSpec(null);
  };

  const handleSaveSpec = (payload?: { core_specific?: string }) => {
    const effectiveForm = payload?.core_specific
      ? { ...specForm, core_specific: payload.core_specific }
      : specForm;
    if (!effectiveForm.tag.trim()) {
      toast.warning(t("admin.configCenter.messages.requiredFields"));
      return;
    }
    if (!effectiveForm.is_template && !effectiveForm.agent_host_id) {
      toast.warning(t("admin.configCenter.messages.selectHostFirst"));
      return;
    }
    const semanticSpec = (() => {
      try { return JSON.parse(effectiveForm.semantic_spec); }
      catch { toast.error(t("admin.configCenter.messages.invalidJson")); return undefined; }
    })();
    if (!semanticSpec) return;
    const coreSpecific = (() => {
      try { return JSON.parse(effectiveForm.core_specific); }
      catch { toast.error(t("admin.configCenter.messages.invalidJson")); return undefined; }
    })();
    if (!coreSpecific) return;

    if (typeof semanticSpec === "object" && semanticSpec !== null) {
      (semanticSpec as Record<string, unknown>).tag = effectiveForm.tag.trim();
    }

    const mutationPayload = {
      core_type: effectiveForm.core_type,
      tag: effectiveForm.tag.trim(),
      enabled: effectiveForm.enabled,
      semantic_spec: semanticSpec,
      core_specific: coreSpecific,
      change_note: effectiveForm.change_note.trim() || undefined,
      agent_host_id: effectiveForm.is_template ? 0 : (effectiveForm.agent_host_id || 0),
    };

    if (selectedSpec) {
      updateSpecMutation.mutate({ specId: selectedSpec.id, payload: mutationPayload });
    } else {
      createSpecMutation.mutate(mutationPayload);
    }
  };

  const handleDeleteSpec = (specId: number) => {
    setDeleteSpecId(specId);
    setIsDeleteConfirmOpen(true);
  };


  const confirmDeleteSpec = () => {
    if (!deleteSpecId) return;
    setDeleteSpecId(null);
    setDeleteSpecTag("");
    deleteSpecMutation.mutate(deleteSpecId);
  };

  // ---- Derived data ----
  const hosts = useMemo(() => {
    return hostQuery.data && 'data' in hostQuery.data
      ? (hostQuery.data as { data: Array<{ id: number; name?: string; host?: string }> }).data
      : Array.isArray(hostQuery.data) ? hostQuery.data : [];
  }, [hostQuery.data]);
  const hostOptions = useMemo(() => {
    return hosts.map((h) => ({ id: h.id, name: h.name || h.host || `Host #${h.id}` }));
  }, [hosts]);

  const specs = useMemo(() => specListQuery.data?.data ?? [], [specListQuery.data]);
  const latestDesiredRevision = useMemo(
    () => Math.max(0, ...specs.map((s: ConfigCenterSpec) => s.desired_revision ?? 0)),
    [specs],
  );

  // ---- Render ----
  const renderFilterBar = () => (
    <div className="flex flex-wrap items-center gap-3">
      {/* Host selector */}
      <div className="w-48">
        <Select
          value={selectedHostId ? String(selectedHostId) : "all"}
          onValueChange={(value) =>
            updateConfigCenterSearch({ hostId: value !== "all" ? Number(value) : null })
          }
        >
          <SelectTrigger>
            <SelectValue placeholder={t("admin.configCenter.placeholders.selectHost")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t("admin.configCenter.allHosts")}</SelectItem>
            {hostOptions.map((host) => (
              <SelectItem key={host.id} value={String(host.id)}>
                {host.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      {/* Core selector */}
      <div className="w-36">
        <Select
          value={selectedCoreType}
          onValueChange={(value) =>
            updateConfigCenterSearch({ coreType: value as CoreTypeOption })
          }
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {CORE_OPTIONS.map((opt) => (
              <SelectItem key={opt} value={opt}>
                {opt}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <Button
        variant="outline"
        size="icon"
        onClick={() => specListQuery.refetch()}
      >
        <RefreshCw className="h-4 w-4" />
      </Button>
    </div>
  );

  return (
    <div className="space-y-4 p-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">{t("admin.configCenter.title")}</h1>
      </div>

      {renderFilterBar()}

      <Tabs
        value={activeTab}
        onValueChange={(value) => {
          if (CONFIG_CENTER_TAB_VALUES.has(value as ConfigCenterTabValue)) {
            updateConfigCenterSearch({ tab: value as ConfigCenterTabValue });
          }
        }}
      >
        <TabsList className="flex-wrap">
          <TabsTrigger value="specs">{t("admin.configCenter.tabs.specs")}</TabsTrigger>
          <TabsTrigger value="apply">{t("admin.configCenter.tabs.apply")}</TabsTrigger>
          <TabsTrigger value="diff">{t("admin.configCenter.tabs.diff")}</TabsTrigger>
          <TabsTrigger value="drift">{t("admin.configCenter.tabs.drift")}</TabsTrigger>
          <TabsTrigger value="snapshot">{t("admin.configCenter.tabs.snapshot")}</TabsTrigger>
        </TabsList>

        {/* Specs Tab */}
        <TabsContent value="specs" className="space-y-4">
          <ConfigCenterTopologyHint />
          <SpecsTab
            specs={specs}
            isLoading={specListQuery.isLoading}
            error={specListQuery.error ?? null}
            latestDesiredRevision={latestDesiredRevision}
            onRefresh={() => specListQuery.refetch()}
            onCreateSpec={openCreateDialog}
            onEditSpec={openEditDialog}
            onDeleteSpec={handleDeleteSpec}
            onHistorySpec={openHistoryDialog}
            applyForm={applyForm}
            onApplyFormChange={setApplyForm}
            applyPending={applyMutation.isPending}
            selectedCoreType={selectedCoreType}
          />
        </TabsContent>

        {/* Apply Tab */}
        <TabsContent value="apply">
          <ApplyRunTab
            selectedHostId={selectedHostId}
            selectedCoreType={selectedCoreType}
            diffFilename={diffFilename}
            diffTag={diffTag}
          />
        </TabsContent>

        {/* Diff Tab */}
        <TabsContent value="diff">
          <DiffTab
            selectedHostId={selectedHostId}
            selectedCoreType={selectedCoreType}
          />
        </TabsContent>

        {/* Drift Tab */}
        <TabsContent value="drift">
          <DriftTab
            selectedHostId={selectedHostId}
            selectedCoreType={selectedCoreType}
          />
        </TabsContent>

        {/* Snapshot Tab */}
        <TabsContent value="snapshot">
          <SnapshotTab
            selectedHostId={selectedHostId}
            selectedCoreType={selectedCoreType}
          />
        </TabsContent>
      </Tabs>

      {/* Spec Editor Dialog */}
      <SpecEditorDialog
        open={isSpecDialogOpen}
        onOpenChange={handleSpecDialogOpenChange}
        selectedSpec={selectedSpec}
        specForm={specForm}
        onSpecFormChange={setSpecForm}
        hostOptions={hostOptions}
        onSave={handleSaveSpec}
        isSaving={createSpecMutation.isPending || updateSpecMutation.isPending}
        selectedHostId={selectedHostId}
        selectedCoreType={selectedCoreType}
      />

      {/* History Dialog */}
      <Dialog open={isHistoryDialogOpen} onOpenChange={setIsHistoryDialogOpen}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>{t("admin.configCenter.history.title")}</DialogTitle>
          </DialogHeader>
          {historyQuery.isLoading ? (
            <Loading />
          ) : historyQuery.data?.data?.length ? (
            <div className="space-y-2">
              {historyQuery.data.data.map((rev: ConfigCenterSpecRevision) => (
                <div key={rev.revision} className="rounded-md border border-border p-3 text-sm">
                  <div className="flex items-center justify-between">
                    <span className="font-medium">r{rev.revision}</span>
                    <span className="text-muted-foreground">{formatDateTime(rev.created_at)}</span>
                  </div>
                  {rev.change_note && <p className="mt-1 text-muted-foreground">{rev.change_note}</p>}
                </div>
              ))}
            </div>
          ) : (
            <EmptyState title={t("admin.configCenter.empty.noHistoryTitle")} size="sm" />
          )}
        </DialogContent>
      </Dialog>

      {/* Delete Confirm Dialog */}
      <Dialog open={isDeleteConfirmOpen} onOpenChange={setIsDeleteConfirmOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("admin.configCenter.specs.deleteTitle")}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            {t("admin.configCenter.specs.deleteConfirmation", {
              tag: deleteSpecTag,
            })}
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsDeleteConfirmOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button variant="destructive" onClick={confirmDeleteSpec}>
              {t("common.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

/** specs 区块顶部：引流到拓扑画布的共存提示 */
function ConfigCenterTopologyHint() {
  const { t } = useTranslation();
  return (
    <a
      href="/routing?tab=topology"
      className="flex items-center gap-1.5 rounded-md border border-dashed px-3 py-2 text-xs text-muted-foreground transition-colors hover:border-primary/50 hover:text-foreground"
    >
      <Workflow className="h-3.5 w-3.5" aria-hidden />
      {t("admin.topology.config_center_hint")}
    </a>
  );
}
