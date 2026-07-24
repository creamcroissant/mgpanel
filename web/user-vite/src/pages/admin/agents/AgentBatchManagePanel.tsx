import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  Cpu,
  Download,
  RefreshCw,
  Trash2,
  Upload,
  X,
} from "lucide-react";
import {
  createAgentUpdateOperation,
  deleteAgentHost,
  installAgentCore,
  refreshAgentHosts,
} from "@/api/admin";
import { QUERY_KEYS } from "@/lib/constants";
import {
  Badge,
  Button,
  Checkbox,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  EmptyState,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui";
import { AgentStatus, type AgentHost } from "@/types";

interface AgentBatchManagePanelProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  agents: AgentHost[];
  onRefetch: () => void;
}

type CoreAction = "install" | "upgrade" | "uninstall";

const CORE_TYPES = ["sing-box", "xray"] as const;

/** After any batch operation, invalidate per-agent query keys so
 *  AgentCorePanel etc. see the new operations on next open. */
function invalidateAgentQueries(
  queryClient: ReturnType<typeof useQueryClient>,
  agentIds: number[],
) {
  // Global (list-style) keys
  queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_AGENTS });
  queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_AGENT_CORE_OPERATIONS });
  queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_AGENT_CORE_INSTANCES });
  queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_AGENT_CORES });
  queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_AGENT_BINARY_VERSIONS });
  queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_AGENT_LIFECYCLE_OPERATIONS });
  // Per-agent composite keys (used by AgentCorePanel & AgentUpdatePanel)
  for (const id of agentIds) {
    queryClient.invalidateQueries({ queryKey: [...QUERY_KEYS.ADMIN_AGENT_CORE_OPERATIONS, id] });
    queryClient.invalidateQueries({ queryKey: [...QUERY_KEYS.ADMIN_AGENT_CORE_INSTANCES, id] });
    queryClient.invalidateQueries({ queryKey: [...QUERY_KEYS.ADMIN_AGENT_CORES, id] });
    queryClient.invalidateQueries({ queryKey: [...QUERY_KEYS.ADMIN_AGENT_BINARY_VERSIONS, id] });
    queryClient.invalidateQueries({ queryKey: [...QUERY_KEYS.ADMIN_AGENT_LIFECYCLE_OPERATIONS, id] });
  }
}

export default function AgentBatchManagePanel({
  open,
  onOpenChange,
  agents,
  onRefetch,
}: AgentBatchManagePanelProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [activeCoreDialog, setActiveCoreDialog] = useState<{
    action: CoreAction;
    coreType: string;
  } | null>(null);
  const [isCoreDialogOpen, setIsCoreDialogOpen] = useState(false);

  const allSelected = agents.length > 0 && selectedIds.size === agents.length;
  const someSelected = selectedIds.size > 0 && !allSelected;

  const handleToggleAll = () => {
    setSelectedIds(allSelected ? new Set() : new Set(agents.map((a) => a.id)));
  };

  const handleToggleOne = (id: number) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const handleClearSelection = () => setSelectedIds(new Set());

  const handleClose = () => {
    onOpenChange(false);
    setSelectedIds(new Set());
  };

  function notifyBatchResults(
    results: { name: string; success: boolean; message: string }[],
    successKey: string,
    partialKey: string,
    failKey: string,
  ) {
    const succeeded = results.filter((r) => r.success).length;
    const failed = results.filter((r) => !r.success).length;
    if (failed === 0) {
      toast.success(t(successKey, { count: succeeded }));
    } else if (succeeded === 0) {
      toast.error(t(failKey, { count: failed }));
    } else {
      toast.warning(t(partialKey, { ok: succeeded, fail: failed }));
    }
    const firstFail = results.find((r) => !r.success);
    if (firstFail) {
      toast.error(t("admin.agents.batch.coreErrorDetail", { name: firstFail.name }), {
        description: firstFail.message,
      });
    }
  }

  // ---- basic ops ----
  const refreshMutation = useMutation({
    mutationFn: () => refreshAgentHosts(),
    onSuccess: () => {
      onRefetch();
      toast.success(t("admin.agents.refreshSuccess"));
    },
    onError: (err: Error) => {
      toast.error(t("admin.agents.refreshError"), { description: err.message });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async (agentIds: number[]) => {
      const selected = agents.filter((a) => agentIds.includes(a.id));
      const results: { name: string; success: boolean; message: string }[] = [];
      for (const agent of selected) {
        try {
          await deleteAgentHost(agent.id);
          results.push({ name: agent.name, success: true, message: "" });
        } catch (err) {
          results.push({
            name: agent.name,
            success: false,
            message: err instanceof Error ? err.message : String(err),
          });
        }
      }
      return { results, agentIds };
    },
    onSuccess: ({ results, agentIds }) => {
      invalidateAgentQueries(queryClient, agentIds);
      setSelectedIds(new Set());
      toast.success(t("admin.agents.batch.deleteSuccess", { count: results.filter((r) => r.success).length }));
    },
    onError: (err: Error) => {
      toast.error(t("admin.agents.batch.deleteError"), { description: err.message });
    },
  });

  // ---- batch core operation ----
  const coreMutation = useMutation({
    mutationFn: async ({
      action,
      coreType,
      agentIds,
    }: {
      action: CoreAction;
      coreType: string;
      agentIds: number[];
    }) => {
      const selected = agents.filter((a) => agentIds.includes(a.id));
      const results: { name: string; success: boolean; message: string }[] = [];
      for (const agent of selected) {
        try {
          const coreAction = action === "uninstall" ? "uninstall" : action === "upgrade" ? "upgrade" : "install";
          await installAgentCore(agent.id, {
            core_type: coreType,
            action: coreAction,
            activate: action !== "uninstall",
          });
          results.push({ name: agent.name, success: true, message: "" });
        } catch (err) {
          results.push({
            name: agent.name,
            success: false,
            message: err instanceof Error ? err.message : String(err),
          });
        }
      }
      return { results, agentIds };
    },
    onSuccess: ({ results, agentIds }) => {
      invalidateAgentQueries(queryClient, agentIds);
      setSelectedIds(new Set());
      notifyBatchResults(results, "admin.agents.batch.coreSuccess", "admin.agents.batch.corePartial", "admin.agents.batch.coreFailed");
    },
    onError: (err: Error) => {
      toast.error(t("admin.agents.batch.coreError"), { description: err.message });
    },
  });

  // ---- batch agent update ----
  const updateMutation = useMutation({
    mutationFn: async (agentIds: number[]) => {
      const selected = agents.filter((a) => agentIds.includes(a.id));
      const results: { name: string; success: boolean; message: string }[] = [];
      for (const agent of selected) {
        try {
          await createAgentUpdateOperation(agent.id, {});
          results.push({ name: agent.name, success: true, message: "" });
        } catch (err) {
          results.push({
            name: agent.name,
            success: false,
            message: err instanceof Error ? err.message : String(err),
          });
        }
      }
      return { results, agentIds };
    },
    onSuccess: ({ results, agentIds }) => {
      invalidateAgentQueries(queryClient, agentIds);
      setSelectedIds(new Set());
      notifyBatchResults(results, "admin.agents.batch.updateSuccess", "admin.agents.batch.updatePartial", "admin.agents.batch.updateFailed");
    },
    onError: (err: Error) => {
      toast.error(t("admin.agents.batch.updateError"), { description: err.message });
    },
  });

  const openCoreDialog = (action: CoreAction, coreType: string) => {
    setActiveCoreDialog({ action, coreType });
    setIsCoreDialogOpen(true);
  };

  const confirmCoreAction = () => {
    if (!activeCoreDialog) return;
    setIsCoreDialogOpen(false);
    coreMutation.mutate({
      action: activeCoreDialog.action,
      coreType: activeCoreDialog.coreType,
      agentIds: [...selectedIds],
    });
  };

  const isDeleting = deleteMutation.isPending;
  const isRefreshing = refreshMutation.isPending;
  const isCoreBusy = coreMutation.isPending;
  const isUpdateBusy = updateMutation.isPending;
  const hasSelection = selectedIds.size > 0;

  const getStatusBadgeVariant = (status: number): "success" | "warning" | "danger" => {
    switch (status) {
      case AgentStatus.Online:
        return "success";
      case AgentStatus.Warning:
        return "warning";
      default:
        return "danger";
    }
  };

  const getStatusLabel = (status: number): string => {
    switch (status) {
      case AgentStatus.Online:
        return t("admin.agents.status.online");
      case AgentStatus.Warning:
        return t("admin.agents.status.warning");
      default:
        return t("admin.agents.status.offline");
    }
  };

  // ---- Empty state ----
  if (agents.length === 0) {
    return (
      <Dialog open={open} onOpenChange={handleClose}>
        <DialogContent className="sm:max-w-4xl">
          <DialogHeader>
            <DialogTitle>{t("admin.agents.batch.title")}</DialogTitle>
          </DialogHeader>
          <EmptyState icon={<Trash2 className="h-full w-full" />} title={t("admin.agents.empty")} size="sm" />
          <DialogFooter>
            <Button variant="outline" onClick={handleClose}>
              {t("common.close")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    );
  }

  return (
    <>
      <Dialog open={open} onOpenChange={handleClose}>
        <DialogContent className="sm:max-w-6xl">
          <DialogHeader>
            <DialogTitle>{t("admin.agents.batch.title")}</DialogTitle>
          </DialogHeader>

          {/* ---- Top toolbar: select + basic ops ---- */}
          <div className="flex flex-wrap items-center gap-2 pb-2">
            <label className="flex items-center gap-2 text-sm font-medium">
              <Checkbox
                checked={allSelected}
                data-state={allSelected ? "checked" : someSelected ? "indeterminate" : "unchecked"}
                onCheckedChange={handleToggleAll}
                aria-label={t("admin.agents.batch.selectAll")}
              />
              {t("admin.agents.batch.selectAll")}
              <span className="text-muted-foreground">
                ({selectedIds.size}/{agents.length})
              </span>
            </label>

            <div className="ml-auto flex flex-wrap items-center gap-2">
              {hasSelection && (
                <>
                  <Button size="sm" variant="outline" onClick={() => refreshMutation.mutate()} disabled={isRefreshing}>
                    <RefreshCw className="mr-1 h-4 w-4" />
                    {isRefreshing ? t("common.loading") : t("admin.agents.batch.refresh")}
                  </Button>
                  <Button size="sm" variant="destructive" onClick={() => deleteMutation.mutate([...selectedIds])} disabled={isDeleting}>
                    <Trash2 className="mr-1 h-4 w-4" />
                    {isDeleting ? t("common.loading") : t("admin.agents.batch.delete", { count: selectedIds.size })}
                  </Button>
                </>
              )}
              <Button size="sm" variant="ghost" onClick={handleClearSelection}>
                <X className="mr-1 h-4 w-4" />
                {t("admin.agents.batch.clearSelection")}
              </Button>
            </div>
          </div>

          {/* ---- Second toolbar: core install/upgrade/uninstall + agent update ---- */}
          {hasSelection && (
            <div className="flex flex-wrap items-center gap-2 border-t pt-3 pb-1">
              <span className="mr-1 text-xs font-medium text-muted-foreground">
                {t("admin.agents.batch.coreAction")}:
              </span>

              {CORE_TYPES.map((coreType) => (
                <div key={coreType} className="flex items-center gap-0.5">
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => openCoreDialog("install", coreType)}
                    disabled={isCoreBusy}
                    className="rounded-r-none"
                  >
                    <Download className="mr-1 h-3.5 w-3.5" />
                    {t("admin.agents.batch.installCore", { core: coreType })}
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => openCoreDialog("upgrade", coreType)}
                    disabled={isCoreBusy}
                    className="rounded-l-none border-l-0 px-2"
                  >
                    <Upload className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => openCoreDialog("uninstall", coreType)}
                    disabled={isCoreBusy}
                    className="rounded-l-none border-l-0 px-2"
                  >
                    <Trash2 className="h-3.5 w-3.5 text-destructive" />
                  </Button>
                </div>
              ))}

              <span className="mx-1 h-5 w-px bg-border" />

              <Button
                size="sm"
                variant="outline"
                onClick={() => updateMutation.mutate([...selectedIds])}
                disabled={isUpdateBusy}
              >
                <Cpu className="mr-1 h-3.5 w-3.5" />
                {isUpdateBusy ? t("common.loading") : t("admin.agents.batch.updateAgent")}
              </Button>
            </div>
          )}

          {/* ---- Agent table ---- */}
          <div className="max-h-[50vh] overflow-auto rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-10">
                    <Checkbox
                      checked={allSelected}
                      data-state={allSelected ? "checked" : someSelected ? "indeterminate" : "unchecked"}
                      onCheckedChange={handleToggleAll}
                      aria-label={t("admin.agents.batch.selectAll")}
                    />
                  </TableHead>
                  <TableHead>{t("admin.agents.name")}</TableHead>
                  <TableHead className="hidden sm:table-cell">{t("admin.agents.host")}</TableHead>
                  <TableHead className="w-20">{t("admin.agents.status.column")}</TableHead>
                  <TableHead className="hidden md:table-cell">{t("admin.agents.agentVersion")}</TableHead>
                  <TableHead className="hidden lg:table-cell">{t("admin.agents.currentCore")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {agents.map((agent) => (
                  <TableRow key={agent.id} className={selectedIds.has(agent.id) ? "bg-primary/5" : undefined}>
                    <TableCell>
                      <Checkbox
                        checked={selectedIds.has(agent.id)}
                        onCheckedChange={() => handleToggleOne(agent.id)}
                        aria-label={t("admin.agents.batch.selectAgent", { name: agent.name })}
                      />
                    </TableCell>
                    <TableCell className="font-medium">{agent.name}</TableCell>
                    <TableCell className="hidden font-mono text-xs text-muted-foreground sm:table-cell">
                      {agent.host}
                      {agent.port ? `:${agent.port}` : ""}
                    </TableCell>
                    <TableCell>
                      <Badge variant={getStatusBadgeVariant(agent.status)}>
                        {getStatusLabel(agent.status)}
                      </Badge>
                    </TableCell>
                    <TableCell className="hidden text-xs text-muted-foreground md:table-cell">
                      {agent.agent_version || "-"}
                    </TableCell>
                    <TableCell className="hidden text-xs text-muted-foreground lg:table-cell">
                      {agent.current_core_type || "-"}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>

          <DialogFooter className="gap-2 sm:gap-0">
            <Button variant="outline" onClick={handleClose}>
              {t("common.close")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ---- Core action confirmation dialog ---- */}
      <Dialog open={isCoreDialogOpen} onOpenChange={setIsCoreDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>
              {activeCoreDialog?.action === "install" && t("admin.agents.batch.confirmInstall")}
              {activeCoreDialog?.action === "upgrade" && t("admin.agents.batch.confirmUpgrade")}
              {activeCoreDialog?.action === "uninstall" && t("admin.agents.batch.confirmUninstall")}
            </DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            {activeCoreDialog &&
              t("admin.agents.batch.confirmCoreMessage", {
                action:
                  activeCoreDialog.action === "install"
                    ? t("admin.agents.batch.actionInstall")
                    : activeCoreDialog.action === "upgrade"
                      ? t("admin.agents.batch.actionUpgrade")
                      : t("admin.agents.batch.actionUninstall"),
                coreType: activeCoreDialog.coreType,
                count: selectedIds.size,
              })}
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsCoreDialogOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={confirmCoreAction} disabled={isCoreBusy}>
              {isCoreBusy ? t("common.loading") : t("common.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
