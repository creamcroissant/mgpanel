import { useCallback, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toPng } from "html-to-image";
import { Plus, ShieldCheck, ImageDown } from "lucide-react";
import { Button, EmptyState, ErrorBanner, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui";
import { fetchTopology, validateTopology, validateRelayPaths } from "@/lib/topology/api";
import { useReorderCommit } from "./interactions";
import type { TopologyIssue, TopologyPolicy } from "@/lib/topology/types";
import { assembleCanvasA, assembleCanvasB, assembleCanvasC } from "@/lib/topology/assemble";
import {
  CreateRuleDialog,
  ConfirmDialog,
  IssuesStrip,
  ReorderLane,
  groupIssuesByNode,
} from "./interactions";
import { Palette } from "./palette/Palette";
import { CreateRelayPathDialog } from "./tools/CreateRelayPathDialog";
import { CreateExitSetDialog, CreatePolicyDialog } from "./tools/CreateEntityDialogs";
import { SideToolbar, type ToolMode } from "./tools/SideToolbar";
import { DrawerPanel, type DrawerTarget } from "./drawer/DrawerPanel";
import { RelayPathDrawer } from "./drawer/RelayPathDrawer";
import type { RelayPathFormValues } from "./mutations";
import { useCreateRelayPath, useUpdateRelayPath, useDeleteRelayPath } from "./mutations";
import type { RuleFormValues, SetFormValues } from "./drawer/schema";
import { useTopologyMutations } from "./mutations";
import { TopologyCanvas } from "./canvas";

const CORE_TYPES = ["sing-box", "xray"] as const;
type CoreType = (typeof CORE_TYPES)[number];

/**
 * 拓扑画布 Tab（交互装配波次最终形态）：
 * 三区布局——左：求值顺序轨道（拖拽/按钮重排）；中：画布（点选开 Drawer、拖线改绑）；
 * 右：schema Drawer。顶部工具条：Palette 新建 + core_type 切换 + 校验。
 * 全部实体变更走 mutations.ts 的乐观更新 REST 端点。
 */
export function TopologyTab() {
  const { t } = useTranslation();
  const [coreType, setCoreType] = useState<CoreType>("sing-box");
  const [canvasMode, setCanvasMode] = useState<CanvasMode>("rules");
  const [drawerTarget, setDrawerTarget] = useState<DrawerTarget | null>(null);
  const [exporting, setExporting] = useState(false);
  const paletteRef = useRef<HTMLDivElement>(null);
  // 解锁筛选器："all" 或平台名；激活时命中节点描边、未命中降透明
  const [unlockFilter, setUnlockFilter] = useState<string>("all");
  // 服务器链路工具模式（选择/连线/删除）与点击式连线状态机
  const [toolMode, setToolMode] = useState<ToolMode>("select");
  const [pendingSource, setPendingSource] = useState<string | null>(null);
  const [pendingDeletePathId, setPendingDeletePathId] = useState<number | null>(null);
  // 新建规则两段式：Drawer 收字段 → 本弹窗补选目标出口集
  const [pendingRule, setPendingRule] = useState<RuleFormValues | null>(null);
  // 连线改绑确认：{policyId, setName}
  const [rebind, setRebind] = useState<{ policyId: number; setId: number; summary: string } | null>(null);
  const [issues, setIssues] = useState<TopologyIssue[] | null>(null);

  const { data, isLoading, isError, error, refetch, isFetching } = useQuery({
    queryKey: ["admin", "topology", coreType],
    queryFn: () => fetchTopology(coreType),
    staleTime: 30_000,
  });

  const muts = useTopologyMutations(coreType);
  const relayMuts = {
    create: useCreateRelayPath(coreType),
    update: useUpdateRelayPath(coreType),
    del: useDeleteRelayPath(coreType),
  };
  // 中继链路抽屉草稿（id=-1 表示新建）
  const [relayDraft, setRelayDraft] = useState<RelayPathFormValues | null>(null);
  // 连线确认后的初始化弹窗：{srcId,dstId}，保存时才真正 create
  const [pendingConnect, setPendingConnect] = useState<{ srcId: number; dstId: number } | null>(null);
  const [createPolicyOpen, setCreatePolicyOpen] = useState(false);
  const [createSetOpen, setCreateSetOpen] = useState(false);

  const graph = useMemo(() => {
    if (!data) return null;
    return canvasMode === "inbounds" ? assembleCanvasB(data) : canvasMode === "agents" ? assembleCanvasC(data) : assembleCanvasA(data);
  }, [data, canvasMode]);

  // 筛选器数据：各平台可解锁 agent 数 + 命中节点 id 集合
  const PLATFORM_LABELS: Record<string, string> = {
    netflix: "NF",
    disney: "D+",
    youtube: "YT",
    openai: "AI",
    tiktok: "TT",
    reddit: "RD",
  };
  const platforms = useMemo(() => {
    if (!data) return [] as { platform: string; label: string; count: number }[];
    const counts = new Map<string, number>();
    for (const u of data.unlock) {
      if (!u.unlocked) continue;
      counts.set(u.platform, (counts.get(u.platform) ?? 0) + 1);
    }
    return [...counts.entries()]
      .map(([platform, count]) => ({ platform, label: PLATFORM_LABELS[platform] ?? platform, count }))
      .sort((a, b) => b.count - a.count);
  }, [data]);

  const { matchedIds, matchedCount } = useMemo(() => {
    if (!data || unlockFilter === "all") return { matchedIds: null as Set<string> | null, matchedCount: 0 };
    const okAgents = new Set(
      data.unlock.filter((u) => u.platform === unlockFilter && u.unlocked).map((u) => u.agent_host_id)
    );
    const ids = new Set<string>();
    for (const a of okAgents) ids.add(`agent-${a}`);
    for (const s of data.exit_sets) {
      if (s.members.some((m) => okAgents.has(m.agent_host_id))) ids.add(`set-${s.id}`);
    }
    return { matchedIds: ids, matchedCount: ids.size };
  }, [data, unlockFilter]);

  const issuesByNode = useMemo(() => groupIssuesByNode(issues ?? []), [issues]);

  // ===== 校验 =====
  const validateMut = useMutation({
    mutationFn: canvasMode === "agents" ? validateRelayPaths : validateTopology,
    onSuccess: (v) => {
      setIssues(v.issues);
      if (v.valid) toastOnceOk();
    },
  });
  function toastOnceOk() {
    // 轻提示由工具条按钮态呈现，这里不再额外 toast，避免打断
  }

  // ===== 连线：rule→set 改绑 target_set_id =====
  const handleConnectRuleToSet = useCallback(
    (sourceNodeId: string, targetNodeId: string) => {
      if (!data) return;
      const policyId = Number(sourceNodeId.replace("rule-", ""));
      const setId = Number(targetNodeId.replace("set-", ""));
      const policy = data.policies.find((p) => p.id === policyId);
      const set_ = data.exit_sets.find((s) => s.id === setId);
      if (!policy || !set_) return;
      if (policy.target_set_id === setId) return; // 无变化
      setRebind({
        policyId,
        setId,
        summary: `「${policy.name}」(${policy.match_type}: ${policy.match_value}) → 「${set_.name}」`,
      });
    },
    [data]
  );

  // ===== 连线：agent→agent 创建两节点中继链路（打开抽屉预填，保存即建）=====
  const handleConnectAgents = useCallback(
    (sourceNodeId: string, targetNodeId: string) => {
      if (!data) return;
      const srcId = Number(sourceNodeId.replace("agent-", ""));
      const dstId = Number(targetNodeId.replace("agent-", ""));
      const a = data.agents.find((x) => x.id === srcId);
      const b = data.agents.find((x) => x.id === dstId);
      if (!a || !b) return;
      // 弹窗收集初始化信息，用户确认后才创建（不静默建稿）
      setPendingConnect({ srcId, dstId });
    },
    [data]
  );

  // ===== 中继链路：保存 / 删除 / 跳转编辑 =====
  const handleSaveRelay = useCallback(
    (v: RelayPathFormValues) => {
      if (v.id < 0) {
        relayMuts.create.mutate(
          { name: v.name, description: v.description, enabled: v.enabled, nodes: v.nodes },
          { onSettled: () => setRelayDraft(null) }
        );
      } else {
        relayMuts.update.mutate(v, { onSettled: () => setRelayDraft(null) });
      }
    },
    [relayMuts]
  );

  const handleDeleteRelay = useCallback(
    (id: number) => {
      relayMuts.del.mutate(id, { onSettled: () => setRelayDraft(null) });
    },
    [relayMuts]
  );

  const openRelayEditor = useCallback((pathId: number) => {
    const rp = data?.relay_paths?.find((p) => p.id === pathId);
    if (!rp) return;
    setRelayDraft({ id: rp.id, name: rp.name, description: rp.description, enabled: rp.enabled, nodes: [...rp.nodes].sort((a, b) => a.sequence - b.sequence).map((n) => ({ ...n })) });
  }, [data]);

  /** 服务器拓扑模式下点击 agent 节点：打开其所属链路（多条取首条），否则提示 */
  const openRelayEditorFromNode = useCallback(
    (nodeId: string) => {
      const aid = Number(nodeId.replace("agent-", ""));
      const rp = data?.relay_paths?.find((p) => p.nodes.some((n) => n.agent_host_id === aid));
      if (rp) openRelayEditor(rp.id);
      else toast.info(t("admin.topology.relay.noPath"));
    },
    [data, openRelayEditor, t]
  );

  const confirmRebind = useCallback(() => {
    if (!rebind) return;
    const policy = data?.policies.find((p) => p.id === rebind.policyId);
    if (!policy) return;
    muts.updatePolicy.mutate(
      {
        id: policy.id,
        name: policy.name,
        match_type: policy.match_type,
        match_value: policy.match_value,
        priority: policy.priority,
        enabled: policy.enabled,
        target_set_id: rebind.setId,
      },
      { onSettled: () => setRebind(null) }
    );
  }, [rebind, data, muts]);

  // ===== Drawer 回调 =====
  const handleSaveRule = useCallback(
    (v: RuleFormValues & { id: number | null }) => {
      if (v.id == null) {
        // 新规则：契约必填 target_set_id，进入补选弹窗
        setPendingRule(v);
        setDrawerTarget(null);
        return;
      }
      muts.updatePolicy.mutate({ ...v, id: v.id }, { onSettled: () => setDrawerTarget(null) });
    },
    [muts]
  );

  const handleConfirmCreateRule = useCallback(
    (targetSetId: number) => {
      if (!pendingRule) return;
      muts.createPolicy.mutate(
        {
          name: pendingRule.name,
          match_type: pendingRule.match_type,
          match_value: pendingRule.match_value,
          priority: pendingRule.priority,
          enabled: pendingRule.enabled,
          target_set_id: targetSetId,
        },
        { onSettled: () => setPendingRule(null) }
      );
    },
    [pendingRule, muts]
  );

  const handleSaveSet = useCallback(
    (v: SetFormValues & { id: number | null }) => {
      muts.saveSet.mutate({ ...v, id: v.id === null ? undefined : v.id }, { onSettled: () => setDrawerTarget(null) });
    },
    [muts]
  );

  const handleDelete = useCallback(
    (kind: "rule" | "set", id: number) => {
      if (kind === "rule") muts.deletePolicy.mutate(id, { onSettled: () => setDrawerTarget(null) });
      else muts.deleteSet.mutate(id, { onSettled: () => setDrawerTarget(null) });
    },
    [muts]
  );

  // ===== 节点点击 → 工具模式优先，否则 Drawer / issue 定位 =====
  const handleNodeClick = useCallback(
    (nodeId: string, kind: string) => {
      if (!data) return;
      // 连线模式：两次点击建链路（入口 → 出口）
      if (canvasMode === "agents" && toolMode === "connect" && kind === "agent") {
        if (!pendingSource) {
          // 侧栏 hint 已提示“点击出口服务器”，不再叠加 toast（避免遮挡画布点击）
          setPendingSource(nodeId);
          return;
        }
        if (pendingSource === nodeId) {
          setPendingSource(null);
          return;
        }
        handleConnectAgents(pendingSource, nodeId);
        setPendingSource(null);
        return;
      }
      if (kind === "rule") {
        const id = Number(nodeId.replace("rule-", ""));
        const policy = data.policies.find((p) => p.id === id);
        if (policy) setDrawerTarget({ kind: "rule", policy });
      } else if (kind === "set") {
        const id = Number(nodeId.replace("set-", ""));
        const set_ = data.exit_sets.find((s) => s.id === id);
        if (set_) setDrawerTarget({ kind: "set", set: set_ });
      } else if (kind === "inbound") {
        const id = Number(nodeId.replace("inbound-", ""));
        const spec = data.specs.find((s) => s.id === id);
        if (spec) setDrawerTarget({ kind: "spec", spec });
      } else if (canvasMode === "agents" && kind === "agent" && toolMode === "select") {
        // 选择模式：点击 agent 打开其所属链路（多条取首条），否则提示
        openRelayEditorFromNode(nodeId);
      }
    },
    [data, canvasMode, toolMode, pendingSource, handleConnectAgents, t, openRelayEditorFromNode]
  );

  // ===== 重排：ReorderLane 防抖后回调 → 调 reorder API → 刷新快照 =====
  const reorderMut = useReorderCommit(coreType, () => refetch());
  const handleCommitOrder = useCallback(
    (orderedIds: number[]) => {
      reorderMut.mutate(orderedIds);
    },
    [reorderMut]
  );

  // ===== 导出 PNG：截取画布容器（含重排轨道）为 topology-{core}.png =====
  const handleExportPng = useCallback(async () => {
    const el = document.getElementById("topology-canvas");
    if (!el || exporting) return;
    setExporting(true);
    try {
      const dataUrl = await toPng(el, { backgroundColor: "hsl(var(--background))", pixelRatio: 2 });
      const a = document.createElement("a");
      a.download = `topology-${coreType}.png`;
      a.href = dataUrl;
      a.click();
      toast.success(t("admin.topology.toolbar.export_done", { file: `topology-${coreType}.png` }));
    } catch (e) {
      toast.error(t("admin.topology.toolbar.export_failed", { err: e instanceof Error ? e.message : String(e) }));
    } finally {
      setExporting(false);
    }
  }, [coreType, exporting, t]);

  const policies: TopologyPolicy[] = data?.policies ?? [];
  const saving =
    muts.createPolicy.isPending ||
    muts.updatePolicy.isPending ||
    muts.saveSet.isPending ||
    muts.deletePolicy.isPending ||
    muts.deleteSet.isPending ||
    muts.saveSpecBinding.isPending ||
    relayMuts.create.isPending ||
    relayMuts.update.isPending ||
    relayMuts.del.isPending;

  const handleSaveSpecBinding = useCallback(
    (v: { id: number; mode: "agent" | "set" | "relay"; agentId: number | null; setId: number | null; pathId: number | null }) => {
      muts.saveSpecBinding.mutate(v, { onSettled: () => refetch() });
      setDrawerTarget(null);
    },
    [muts.saveSpecBinding, refetch]
  );

  return (
    <div className="space-y-3">
      {/* 工具条：画布切换 + Palette + 筛选 + 导出/校验 + core_type */}
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          {/* 画布切换：规则分流 / 入站路由 */}
          <div className="flex items-center gap-1 rounded-md border bg-muted/40 p-0.5">
            <button
              type="button"
              onClick={() => setCanvasMode("rules")}
              className={`rounded px-2 py-1 text-xs font-medium transition-colors ${
                canvasMode === "rules"
                  ? "bg-card text-foreground shadow-sm"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              {t("admin.topology.canvas.rules")}
            </button>
            <button
              type="button"
              onClick={() => setCanvasMode("inbounds")}
              className={`rounded px-2 py-1 text-xs font-medium transition-colors ${
                canvasMode === "inbounds"
                  ? "bg-card text-foreground shadow-sm"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              {t("admin.topology.canvas.inbounds")}
            </button>
            <button
              type="button"
              onClick={() => setCanvasMode("agents")}
              className={`rounded px-2 py-1 text-xs font-medium transition-colors ${
                canvasMode === "agents"
                  ? "bg-card text-foreground shadow-sm"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              {t("admin.topology.canvas.servers")}
            </button>
          </div>
          <div ref={paletteRef} className="flex items-center gap-2">
            {canvasMode === "agents" ? (
              <Button
                variant="outline"
                size="sm"
                disabled={saving || (data?.agents.length ?? 0) < 2}
                onClick={() =>
                  setRelayDraft({ id: -1, name: "", description: "", enabled: true, nodes: [] })
                }
              >
                + {t("admin.topology.relay.create")}
              </Button>
            ) : (
              <Palette
                onCreateRule={() => setCreatePolicyOpen(true)}
                onCreateSet={() => setCreateSetOpen(true)}
                canCreateRule={canvasMode === "rules" && (data?.exit_sets.length ?? 0) > 0}
                disabled={saving}
              />
            )}
          </div>
          <Select
            value={unlockFilter}
            onValueChange={(v) => setUnlockFilter(v)}
          >
            <SelectTrigger className="h-8 w-44 text-xs" aria-label={t("admin.topology.filter.platform")}>
              <SelectValue placeholder={t("admin.topology.filter.platform")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("admin.topology.filter.all")}</SelectItem>
              {platforms.map((p) => (
                <SelectItem key={p.platform} value={p.platform} disabled={p.count === 0}>
                  {p.label} · {p.count}{p.count > 0 ? ` ✓` : ""}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {unlockFilter !== "all" && matchedCount > 0 && (
            <span className="text-xs text-success">
              {t("admin.topology.filter.matched", { count: matchedCount })}
            </span>
          )}
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={handleExportPng}
            disabled={exporting || !data || (canvasMode !== "rules" ? (data.specs?.length ?? 0) === 0 : data.policies.length + data.exit_sets.length === 0)}
            title={t("admin.topology.toolbar.export_png")}
          >
            <ImageDown className="mr-1 h-3.5 w-3.5" aria-hidden />
            {exporting ? t("admin.topology.toolbar.exporting") : t("admin.topology.toolbar.export_png")}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => validateMut.mutate()}
            disabled={validateMut.isPending || isFetching}
            title={t("admin.topology.toolbar.validate_title")}
          >
            <ShieldCheck className="mr-1 h-3.5 w-3.5" aria-hidden />
            {validateMut.isPending ? t("admin.topology.toolbar.validating") : t("admin.topology.toolbar.validate")}
          </Button>
        </div>
        <Select value={coreType} onValueChange={(v) => setCoreType(v as CoreType)} aria-label={t("admin.topology.toolbar.core_type")}>
          <SelectTrigger className="w-[160px]" aria-label="核心类型">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {CORE_TYPES.map((c) => (
              <SelectItem key={c} value={c}>
                {c}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {isLoading && (
        <div className="h-[560px] w-full animate-pulse rounded-md border bg-muted/40" aria-busy />
      )}
      {isError && (
        <ErrorBanner
          message={error instanceof Error ? error.message : "拓扑加载失败"}
          onRetry={() => refetch()}
        />
      )}

      {!isLoading && !isError && data && (
        <div className="space-y-2">
          {issues && issues.length > 0 && (
            <IssuesStrip
              issues={issues}
              onClose={() => setIssues(null)}
              onSelectEntity={(t, id) =>
                handleNodeClick(
                  `${t === "policy" ? "rule" : t === "exit_set" || t === "set" ? "set" : "inbound"}-${id}`,
                  t === "policy" ? "rule" : t === "exit_set" || t === "set" ? "set" : "inbound"
                )
              }
            />
          )}

          {(canvasMode !== "rules"
            ? (data.specs?.length ?? 0) > 0
            : (data.exit_sets.length > 0 || data.policies.length > 0)) && graph && graph.nodes.length > 0 ? (
            <div className="flex h-[560px] gap-2">
              {canvasMode === "rules" && (
                <ReorderLane
                  policies={policies.filter((p) => p.target_set_id != null)}
                  selectedPolicyId={
                    drawerTarget?.kind === "rule" ? drawerTarget.policy?.id ?? null : null
                  }
                  onCommitOrder={handleCommitOrder}
                  onSelect={(id) => {
                    const policy = policies.find((p) => p.id === id);
                    if (policy) setDrawerTarget({ kind: "rule", policy });
                  }}
                />
              )}
              <div className="flex min-w-0 flex-1 gap-2">
                {canvasMode === "agents" && (
                  <SideToolbar
                    mode={toolMode}
                    onChange={(m) => {
                      setToolMode(m);
                      setPendingSource(null);
                      setPendingDeletePathId(null);
                    }}
                    hint={pendingSource ? t("admin.topology.tools.click_target") : undefined}
                  />
                )}
                  <TopologyCanvas
                    nodes={graph.nodes}
                    edges={graph.edges}
                    selectedId={relayDraft ? null : nodeIdOfDrawer(drawerTarget)}
                    issuesByNode={issuesByNode}
                    onConnectRuleToSet={canvasMode === "agents" ? undefined : handleConnectRuleToSet}
                    onConnectAgents={canvasMode === "agents" ? handleConnectAgents : undefined}
                    onRelayEdgeClick={(pathId) => {
                      const p = (data?.relay_paths ?? []).find((x) => x.id === pathId);
                      if (!p) return;
                      // 删除模式：两次点击同一链路确认删除（防误触）
                      if (canvasMode === "agents" && toolMode === "delete") {
                        if (pendingDeletePathId === pathId) {
                          relayMuts.del.mutate(pathId, {
                            onSettled: () => setPendingDeletePathId(null),
                          });
                        } else {
                          setPendingDeletePathId(pathId);
                          toast.info(t("admin.topology.tools.click_again_delete", { name: p.name }));
                        }
                        return;
                      }
                      setRelayDraft({
                        id: p.id,
                        name: p.name,
                        description: p.description ?? "",
                        enabled: p.enabled,
                        nodes: [...p.nodes].sort((a, b) => a.sequence - b.sequence),
                      });
                    }}
                    onNodeClick={handleNodeClick}
                    highlightIds={matchedIds ?? undefined}
                    filterActive={unlockFilter !== "all"}
                    resetKey={coreType}
                  />
                </div>
            </div>
          ) : (
            <EmptyGuide
              canvasMode={canvasMode}
              hasSpecs={(data.specs?.length ?? 0) > 0}
              onCreateFirst={() => {
                setDrawerTarget({ kind: "rule", policy: null });
                paletteRef.current?.scrollIntoView({ behavior: "smooth", block: "center" });
              }}
            />
          )}
        </div>
      )}

      {/* 右侧属性抽屉 */}
      {relayDraft ? (
        <RelayPathDrawer
          target={relayDraft}
          agents={data?.agents ?? []}
          saving={saving}
          onClose={() => setRelayDraft(null)}
          onDraftChange={setRelayDraft}
          onSave={handleSaveRelay}
          onDelete={handleDeleteRelay}
        />
      ) : (
      <DrawerPanel
        target={drawerTarget}
        agents={data?.agents ?? []}
        exitSets={data?.exit_sets ?? []}
        relayPaths={data?.relay_paths ?? []}
        saving={saving}
        onClose={() => setDrawerTarget(null)}
        onSaveRule={handleSaveRule}
        onSaveSet={handleSaveSet}
        onSaveSpecBinding={handleSaveSpecBinding}
        onDelete={handleDelete}
      />
      )}

      {/* 新建规则补选目标集 */}
      <CreateRuleDialog
        open={pendingRule != null}
        sets={data?.exit_sets ?? []}
        pendingValues={pendingRule}
        busy={muts.createPolicy.isPending}
        onConfirm={handleConfirmCreateRule}
        onCancel={() => setPendingRule(null)}
      />

      {/* 连线改绑确认 */}
      <CreateRelayPathDialog
        open={pendingConnect != null}
        sourceName={
          pendingConnect ? (data?.agents.find((a) => a.id === pendingConnect.srcId)?.name ?? "") : ""
        }
        targetName={
          pendingConnect ? (data?.agents.find((a) => a.id === pendingConnect.dstId)?.name ?? "") : ""
        }
        busy={relayMuts.create.isPending}
        onCancel={() => setPendingConnect(null)}
        onConfirm={(init) => {
          if (!pendingConnect) return;
          relayMuts.create.mutate(
            {
              name: init.name,
              description: init.description,
              enabled: init.enabled,
              nodes: [
                { sequence: 0, agent_host_id: pendingConnect.srcId },
                { sequence: 1, agent_host_id: pendingConnect.dstId },
              ],
            },
            { onSettled: () => {
                setPendingConnect(null);
                setToolMode("select");
              } }
          );
        }}
      />
      <CreatePolicyDialog
        open={createPolicyOpen}
        sets={(data?.exit_sets ?? []).map((x) => ({ id: x.id, name: x.name }))}
        busy={muts.createPolicy.isPending}
        onCancel={() => setCreatePolicyOpen(false)}
        onConfirm={(v) => {
          muts.createPolicy.mutate(v, {
            onSettled: () => setCreatePolicyOpen(false),
          });
        }}
      />
      <CreateExitSetDialog
        open={createSetOpen}
        busy={muts.saveSet.isPending}
        onCancel={() => setCreateSetOpen(false)}
        onConfirm={({ name, strategy, enabled }) => {
          // saveSet 无 id 即创建（SetPayload 契约）；description/members 在编辑抽屉补填
          muts.saveSet.mutate(
            { name, description: "", strategy, enabled, members: [] },
            { onSettled: () => setCreateSetOpen(false) }
          );
        }}
      />
      <ConfirmDialog
        open={rebind != null}
        title="调整分流目标"
        description="将把该规则的命中流量转发到新的出口集"
        summary={rebind?.summary}
        confirmText="改绑"
        busy={muts.updatePolicy.isPending}
        onConfirm={confirmRebind}
        onCancel={() => setRebind(null)}
      />
    </div>
  );
}

function nodeIdOfDrawer(t: DrawerTarget | null): string | null {
  if (!t) return null;
  if (t.kind === "rule" && t.policy && t.policy.id > 0) return `rule-${t.policy.id}`;
  if (t.kind === "set" && t.set && t.set.id > 0) return `set-${t.set.id}`;
  return null;
}

type CanvasMode = "rules" | "inbounds" | "agents";

/** 空态三件套：图标 + 引导文案 + 滚动到 Palette 的行动按钮 */
function EmptyGuide({
  canvasMode,
  hasSpecs,
  onCreateFirst,
}: {
  canvasMode: CanvasMode;
  hasSpecs: boolean;
  onCreateFirst: () => void;
}) {
  const { t } = useTranslation();
  const isRules = canvasMode === "rules";
  return (
    <div className="flex h-[560px] items-center justify-center rounded-md border border-dashed">
      <EmptyState
        icon={<Plus className="h-10 w-10 text-muted-foreground" />}
        title={
          isRules
            ? hasSpecs
              ? t("admin.topology.empty.title_no_rules")
              : t("admin.topology.empty.title_no_specs")
            : t("admin.topology.empty.title_no_specs_inbounds")
        }
        description={
          isRules
            ? hasSpecs
              ? t("admin.topology.empty.desc_create")
              : t("admin.topology.empty.desc_specs_first")
            : t("admin.topology.empty.desc_no_specs_inbounds")
        }
        action={
          isRules && hasSpecs ? (
            <Button type="button" size="sm" onClick={onCreateFirst}>
              <Plus className="mr-1 h-3.5 w-3.5" aria-hidden />
              {t("admin.topology.empty.action_to_palette")}
            </Button>
          ) : undefined
        }
      />
    </div>
  );
}
