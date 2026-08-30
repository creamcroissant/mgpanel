import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { Loader2, Search } from "lucide-react";
import { toast } from "sonner";
import { fetchAgentHostsAll } from "@/api/admin/agentHost";
import { fetchMeshStatus, type MeshPeer } from "@/api/admin/mesh";
import { createConfigCenterApplyRun } from "@/api/admin/configCenter";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { AgentStatus, type AgentHost } from "@/types";
import type { ConfigCenterCoreType } from "@/types/configCenter";

const UNLABELED = "__unlabeled__";

const LATENCY_CONDS = ["all", "has", "none", "lt50", "lt100", "lt200"] as const;
type LatencyCond = (typeof LATENCY_CONDS)[number];

interface SpecApplyDialogProps {
  coreType: ConfigCenterCoreType;
  specLabel?: string;
  defaultTargetRevision?: number;
  defaultPreviousRevision?: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

interface ApplyRow {
  agent: AgentHost;
  latencyMs?: number;
}

interface ApplyResult {
  success: number[];
  failed: { id: number; error: string }[];
}

function geoLabelOf(agent: AgentHost): string {
  return agent.country || agent.region || UNLABELED;
}

function latencyMatches(latencyMs: number | undefined, cond: LatencyCond): boolean {
  switch (cond) {
    case "has":
      return latencyMs != null && latencyMs > 0;
    case "none":
      return latencyMs == null || latencyMs <= 0;
    case "lt50":
      return latencyMs != null && latencyMs < 50;
    case "lt100":
      return latencyMs != null && latencyMs < 100;
    case "lt200":
      return latencyMs != null && latencyMs < 200;
    default:
      return true;
  }
}

export default function SpecApplyDialog({ coreType, specLabel, defaultTargetRevision, defaultPreviousRevision, open, onOpenChange }: SpecApplyDialogProps) {
  const { t } = useTranslation();
  const [search, setSearch] = useState("");
  const [geoFilter, setGeoFilter] = useState<Set<string>>(new Set());
  const [latencyCond, setLatencyCond] = useState<LatencyCond>("all");
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [targetRevision, setTargetRevision] = useState<number>(0);
  const [previousRevision, setPreviousRevision] = useState<string>("");
  const [advanced, setAdvanced] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [progress, setProgress] = useState<{ current: number; total: number } | null>(null);
  const [results, setResults] = useState<ApplyResult | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["spec-apply-agents"],
    queryFn: async () => {
      const [hosts, mesh] = await Promise.all([fetchAgentHostsAll(), fetchMeshStatus()]);
      const latMap = new Map<number, number>();
      (mesh.data as MeshPeer[]).forEach((peer) => {
        if (typeof peer.latency_ms === "number") latMap.set(peer.agent_host_id, peer.latency_ms);
      });
      return (hosts as AgentHost[]).map((agent) => ({ agent, latencyMs: latMap.get(agent.id) }));
    },
    enabled: open,
  });

  const rows: ApplyRow[] = data ?? [];

  useEffect(() => {
    if (!open) return;
    setSearch("");
    setGeoFilter(new Set());
    setLatencyCond("all");
    setSelected(new Set());
    setResults(null);
    setProgress(null);
    setAdvanced(false);
    setPreviousRevision("");
    setTargetRevision(defaultTargetRevision ?? 0);
    setPreviousRevision(defaultPreviousRevision ?? "");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, coreType]);

  const geoOptions = useMemo(() => {
    const set = new Set<string>();
    rows.forEach((r) => set.add(geoLabelOf(r.agent)));
    return Array.from(set);
  }, [rows]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return rows
      .filter((r) => {
        if (q && !`${r.agent.name} ${r.agent.host}`.toLowerCase().includes(q)) return false;
        if (geoFilter.size > 0 && !geoFilter.has(geoLabelOf(r.agent))) return false;
        return latencyMatches(r.latencyMs, latencyCond);
      })
      .sort((a, b) => {
        const ao = a.agent.status === AgentStatus.Online ? 0 : 1;
        const bo = b.agent.status === AgentStatus.Online ? 0 : 1;
        if (ao !== bo) return ao - bo;
        const al = a.latencyMs ?? Infinity;
        const bl = b.latencyMs ?? Infinity;
        return al - bl;
      });
  }, [rows, search, geoFilter, latencyCond]);

  const toggleGeo = (value: string) => {
    setGeoFilter((prev) => {
      const next = new Set(prev);
      if (next.has(value)) next.delete(value);
      else next.add(value);
      return next;
    });
  };

  const toggleSelect = (id: number) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const allFilteredSelected = filtered.length > 0 && filtered.every((r) => selected.has(r.agent.id));
  const toggleSelectAll = () => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (allFilteredSelected) filtered.forEach((r) => next.delete(r.agent.id));
      else filtered.forEach((r) => next.add(r.agent.id));
      return next;
    });
  };
  const invertSelection = () => {
    setSelected((prev) => {
      const next = new Set(prev);
      filtered.forEach((r) => {
        if (next.has(r.agent.id)) next.delete(r.agent.id);
        else next.add(r.agent.id);
      });
      return next;
    });
  };

  const runApply = async (ids: number[]) => {
    const prevRev = previousRevision.trim() ? Number(previousRevision) : undefined;
    const success: number[] = [];
    const failed: { id: number; error: string }[] = [];
    for (let i = 0; i < ids.length; i++) {
      const id = ids[i];
      try {
        await createConfigCenterApplyRun({
          agent_host_id: id,
          core_type: coreType,
          target_revision: targetRevision,
          previous_revision: prevRev,
        });
        success.push(id);
      } catch (e) {
        failed.push({ id, error: e instanceof Error ? e.message : String(e) });
      }
      setProgress({ current: i + 1, total: ids.length });
    }
    setResults({ success, failed });
  };

  const handleConfirm = async () => {
    const ids = rows.filter((r) => selected.has(r.agent.id)).map((r) => r.agent.id);
    if (ids.length === 0) {
      toast.warning(t("admin.configCenter.applyDialog.selected", { count: 0 }));
      return;
    }
    setSubmitting(true);
    setProgress({ current: 0, total: ids.length });
    setResults(null);
    try {
      await runApply(ids);
    } finally {
      setSubmitting(false);
    }
  };

  const handleRetry = async () => {
    if (!results) return;
    const ids = results.failed.map((f) => f.id);
    if (ids.length === 0) return;
    setSubmitting(true);
    setProgress({ current: 0, total: ids.length });
    try {
      await runApply(ids);
    } finally {
      setSubmitting(false);
    }
  };

  const selectedCount = selected.size;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-3xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t("admin.configCenter.applyDialog.title")}</DialogTitle>
          <DialogDescription>
            {t("admin.configCenter.applyDialog.description")}
            {specLabel ? ` · ${specLabel}` : ""}
          </DialogDescription>
        </DialogHeader>

        {/* Filters */}
        <div className="space-y-3 rounded-md border bg-muted/30 p-3">
          <div className="flex items-center gap-2">
            <div className="relative flex-1">
              <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                className="pl-8"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder={t("admin.configCenter.applyDialog.search")}
              />
            </div>
            <Select value={latencyCond} onValueChange={(v) => setLatencyCond(v as LatencyCond)}>
              <SelectTrigger className="w-40">
                <SelectValue placeholder={t("admin.configCenter.applyDialog.latency")} />
              </SelectTrigger>
              <SelectContent>
                {LATENCY_CONDS.map((c) => (
                  <SelectItem key={c} value={c}>
                    {t(`admin.configCenter.applyDialog.latency${c.charAt(0).toUpperCase() + c.slice(1)}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {geoOptions.length > 0 && (
            <div className="flex flex-wrap items-center gap-1.5">
              <span className="text-xs font-medium text-muted-foreground">
                {t("admin.configCenter.applyDialog.countryRegion")}:
              </span>
              {geoOptions.map((opt) => {
                const active = geoFilter.has(opt);
                const label = opt === UNLABELED ? t("admin.configCenter.applyDialog.unlabeled") : opt;
                return (
                  <button
                    key={opt}
                    type="button"
                    onClick={() => toggleGeo(opt)}
                    className={`rounded-full border px-2.5 py-0.5 text-xs transition-colors ${
                      active
                        ? "border-primary bg-primary/10 text-primary"
                        : "border-border bg-background text-muted-foreground hover:border-primary/50"
                    }`}
                  >
                    {label}
                  </button>
                );
              })}
            </div>
          )}
        </div>

        {/* Agent list */}
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <label className="flex items-center gap-2 text-sm font-medium">
              <Checkbox checked={allFilteredSelected} onCheckedChange={toggleSelectAll} />
              {t("admin.configCenter.applyDialog.selected", { count: selectedCount })}
            </label>
            <div className="flex items-center gap-2">
              <Button variant="ghost" size="sm" onClick={toggleSelectAll}>
                {allFilteredSelected
                  ? t("admin.configCenter.applyDialog.invert")
                  : t("admin.configCenter.applyDialog.selectAll")}
              </Button>
              <Button variant="ghost" size="sm" onClick={invertSelection}>
                {t("admin.configCenter.applyDialog.invert")}
              </Button>
            </div>
          </div>

          <div className="max-h-72 space-y-1.5 overflow-y-auto pr-1">
            {isLoading && (
              <div className="flex items-center justify-center gap-2 py-8 text-sm text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" />
                {t("common.loading")}
              </div>
            )}
            {!isLoading && filtered.length === 0 && (
              <p className="py-8 text-center text-sm text-muted-foreground">
                {t("admin.configCenter.applyDialog.noAgents")}
              </p>
            )}
            {filtered.map((r) => {
              const checked = selected.has(r.agent.id);
              const online = r.agent.status === AgentStatus.Online;
              return (
                <label
                  key={r.agent.id}
                  className={`flex cursor-pointer items-center gap-3 rounded-md border p-2.5 transition-colors ${
                    checked ? "border-primary bg-primary/5" : "border-border bg-background hover:border-primary/40"
                  }`}
                >
                  <Checkbox checked={checked} onCheckedChange={() => toggleSelect(r.agent.id)} />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="truncate text-sm font-medium">{r.agent.name}</span>
                      <Badge variant={online ? "success" : "outline"} className="shrink-0">
                        {online ? t("admin.agents.status.online") : t("admin.agents.status.offline")}
                      </Badge>
                    </div>
                    <div className="truncate text-xs text-muted-foreground">
                      {r.agent.host}
                      {r.agent.country || r.agent.region
                        ? ` · ${r.agent.country || ""}${r.agent.country && r.agent.region ? " / " : ""}${r.agent.region || ""}`
                        : ""}
                      {typeof r.latencyMs === "number" && r.latencyMs > 0 ? ` · ${Math.round(r.latencyMs)}ms` : ""}
                    </div>
                  </div>
                </label>
              );
            })}
          </div>
        </div>

        {/* Advanced: target / previous revision */}
        <div className="space-y-2">
          <Button variant="ghost" size="sm" onClick={() => setAdvanced((v) => !v)} className="px-0">
            {advanced ? t("common.collapse") : t("admin.configCenter.applyDialog.advanced")}
          </Button>
          {advanced && (
            <div className="grid gap-3 rounded-md border bg-muted/30 p-3 sm:grid-cols-2">
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-muted-foreground">
                  {t("admin.configCenter.applyDialog.targetRevision")}
                </label>
                <Input
                  type="number"
                  value={targetRevision}
                  onChange={(e) => setTargetRevision(Number(e.target.value))}
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-muted-foreground">
                  {t("admin.configCenter.applyDialog.previousRevision")}
                </label>
                <Input
                  type="number"
                  value={previousRevision}
                  onChange={(e) => setPreviousRevision(e.target.value)}
                  placeholder="—"
                />
              </div>
            </div>
          )}
        </div>

        {/* Progress + result summary */}
        {progress && (
          <div className="text-sm text-muted-foreground">
            {t("admin.configCenter.applyDialog.progress", { current: progress.current, total: progress.total })}
          </div>
        )}
        {results && (
          <div className="space-y-1.5 rounded-md border bg-muted/30 p-3 text-sm">
            <p className="text-success">{t("admin.configCenter.applyDialog.success", { count: results.success.length })}</p>
            {results.failed.length > 0 && (
              <p className="text-destructive">
                {t("admin.configCenter.applyDialog.failed", { count: results.failed.length })}
              </p>
            )}
          </div>
        )}

        <DialogFooter className="flex items-center justify-between gap-2">
          <div className="text-xs text-muted-foreground">
            {t("admin.configCenter.applyDialog.selected", { count: selectedCount })}
          </div>
          <div className="flex items-center gap-2">
            {results && results.failed.length > 0 && !submitting && (
              <Button variant="outline" onClick={handleRetry}>
                {t("admin.configCenter.applyDialog.retry")}
              </Button>
            )}
            <Button variant="outline" onClick={() => onOpenChange(false)}>
              {t("admin.configCenter.applyDialog.close")}
            </Button>
            <Button onClick={handleConfirm} disabled={submitting || selectedCount === 0}>
              {submitting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t("admin.configCenter.applyDialog.confirm")}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
