import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { List, Network, Plus, RefreshCw, Server } from "lucide-react";
import { QUERY_KEYS } from "@/lib/constants";
import { getAgentHosts, refreshAgentHosts, updateAgentHost } from "@/api/admin";
import { fetchSettings, revealKey } from "@/api/admin/settings";
import { AdminPageShell, AgentStatusCard } from "@/components/admin";
import { EmptyState, Loading, Card, CardContent, ResponsiveGrid, Input } from "@/components/ui";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { AgentStatus, type AgentHost, type UpdateAgentHostRequest } from "@/types";
import AgentCorePanel from "./AgentCorePanel";
import AgentBatchManagePanel from "./AgentBatchManagePanel";
import { MeshPeerTable } from "./MeshPeerTable";


const DEFAULT_DEPLOY_SCRIPT_URL =
  "https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh";

function sanitizeShellArgument(value: string): string {
  return value.replace(/[\r\n]/g, "").trim();
}

function shellEscapeSingleQuoted(value: string): string {
  const sanitized = sanitizeShellArgument(value);
  return `'${sanitized.replace(/'/g, `"'"'`)}'`;
}

function resolveDeployScriptURL(): string {
  const runtimeURL = (window.settings?.deploy_script_url ?? "").trim();
  return runtimeURL || DEFAULT_DEPLOY_SCRIPT_URL;
}

function buildDeployCommand(communicationKey: string, grpcAddress: string, grpcTlsEnabled: boolean): string {
  const deployScriptURL = resolveDeployScriptURL();
  const tlsArg = grpcTlsEnabled ? " -t true" : "";
  return [
    `curl -fsSL ${shellEscapeSingleQuoted(deployScriptURL)} -o /tmp/agent.sh`,
    `sudo INSTALL_DIR=/opt/xboard/agent sh /tmp/agent.sh -k ${shellEscapeSingleQuoted(communicationKey)} -g ${shellEscapeSingleQuoted(grpcAddress)}${tlsArg}`,
  ].join(" && ");
}

function fallbackCopyText(text: string): boolean {
  if (typeof document === "undefined") {
    return false;
  }
  const textArea = document.createElement("textarea");
  textArea.value = text;
  textArea.setAttribute("readonly", "");
  textArea.style.position = "fixed";
  textArea.style.top = "0";
  textArea.style.left = "0";
  textArea.style.opacity = "0";
  document.body.appendChild(textArea);
  textArea.focus();
  textArea.select();
  let copied = false;
  try {
    copied = document.execCommand("copy");
  } catch {
    copied = false;
  }
  document.body.removeChild(textArea);
  return copied;
}

async function copyText(text: string): Promise<boolean> {
  if (!text) return false;
  if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // Fallback to execCommand for insecure context or permission-denied cases.
    }
  }
  return fallbackCopyText(text);
}

export default function AgentList() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [isCorePanelOpen, setIsCorePanelOpen] = useState(false);
  const [isBatchOpen, setIsBatchOpen] = useState(false);
  const [isDeployDialogOpen, setIsDeployDialogOpen] = useState(false);
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false);
  const [isMeshOpen, setIsMeshOpen] = useState(false);
  const [selectedAgent, setSelectedAgent] = useState<AgentHost | null>(null);
  const [deployCommand, setDeployCommand] = useState("");
  const [deployMissingAddress, setDeployMissingAddress] = useState(false);
  const [editForm, setEditForm] = useState({ name: "", host: "" });

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: QUERY_KEYS.ADMIN_AGENTS,
    queryFn: () => getAgentHosts({ page: 1, page_size: 100 }),
    refetchInterval: 30000,
  });

  const deployMutation = useMutation({
    mutationFn: async () => {
      const [nodeSettings, keyInfo] = await Promise.all([fetchSettings("node"), revealKey()]);
      const grpcAddress = (nodeSettings.agent_grpc_address ?? "").trim();
      const communicationKey = (keyInfo.key || "").trim();
      const grpcTlsEnabled = nodeSettings.grpc_tls_enabled === "true";
      return {
        grpcAddress,
        communicationKey,
        grpcTlsEnabled,
      };
    },
    onSuccess: ({ grpcAddress, communicationKey, grpcTlsEnabled }) => {
      setIsDialogOpen(false);
      const hasGrpcAddress = grpcAddress.length > 0;
      const hasCommunicationKey = communicationKey.length > 0;
      setDeployMissingAddress(!hasGrpcAddress);
      setDeployCommand(hasGrpcAddress && hasCommunicationKey ? buildDeployCommand(communicationKey, grpcAddress, grpcTlsEnabled) : "");
      setIsDeployDialogOpen(true);
    },
    onError: (err: Error) => {
      toast.error(t("admin.agents.deploy.title"), { description: err.message });
    },
  });

  const refreshMutation = useMutation({
    mutationFn: refreshAgentHosts,
    onSuccess: () => {
      refetch();
      toast.success(t("admin.agents.refreshSuccess"));
    },
  });

  const updateMutation = useMutation({
    mutationFn: (payload: UpdateAgentHostRequest) => updateAgentHost(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_AGENTS });
      setIsEditDialogOpen(false);
      setSelectedAgent(null);
      setEditForm({ name: "", host: "" });
      toast.success(t("admin.agents.updateSuccess"));
    },
    onError: (err: Error) => {
      toast.error(t("admin.agents.updateError"), { description: err.message });
    },
  });

  const handleOpenDeployDialog = () => {
    deployMutation.mutate();
  };

  const handleBatchDialogChange = (open: boolean) => {
    setIsBatchOpen(open);
  };

  const handleDialogChange = (open: boolean) => {
    setIsDialogOpen(open);
  };

  const handleCorePanelChange = (open: boolean) => {
    setIsCorePanelOpen(open);
    if (!open) {
      setSelectedAgent(null);
    }
  };

  const handleEditDialogChange = (open: boolean) => {
    setIsEditDialogOpen(open);
    if (!open) {
      setSelectedAgent(null);
      setEditForm({ name: "", host: "" });
    }
  };

  const handleDeployDialogChange = (open: boolean) => {
    setIsDeployDialogOpen(open);
    if (!open) {
      setDeployCommand("");
      setDeployMissingAddress(false);
    }
  };

  const handleOpenCorePanel = (agent: AgentHost) => {
    setSelectedAgent(agent);
    setIsCorePanelOpen(true);
  };

  const handleOpenEditDialog = (agent: AgentHost) => {
    setSelectedAgent(agent);
    setEditForm({
      name: agent.name || "",
      host: agent.host || "",
    });
    setIsEditDialogOpen(true);
  };

  const handleSaveEdit = () => {
    if (!selectedAgent) return;
    const name = editForm.name.trim();
    const host = editForm.host.trim();
    if (!name || !host) {
      toast.warning(t("common.error"), {
        description: t("admin.agents.nameHostRequired"),
      });
      return;
    }
    updateMutation.mutate({
      id: selectedAgent.id,
      name,
      host,
    });
  };

  const handleCopyDeployCommand = async () => {
    if (!deployCommand) return;
    const copied = await copyText(deployCommand);
    if (copied) {
      toast.success(t("common.success"), {
        description: t("admin.agents.deploy.copySuccess"),
      });
      return;
    }
    toast.error(t("common.error"), {
      description: t("admin.agents.deploy.copyError"),
    });
  };

  const agents = useMemo<AgentHost[]>(() => data?.data ?? [], [data?.data]);
  const agentStats = useMemo(() => {
    const total = agents.length;
    const online = agents.filter((agent) => agent.status === AgentStatus.Online).length;
    const warning = agents.filter((agent) => agent.status === AgentStatus.Warning).length;
    const offline = total - online - warning;
    const realtime = agents.filter((agent) => agent.last_realtime_report_at && agent.last_realtime_report_at > 0).length;
    return { total, online, warning, offline, realtime };
  }, [agents]);

  if (isLoading) return <Loading />;

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center gap-3 py-20">
        <p className="text-sm text-destructive">{t("admin.agents.loadError")}</p>
        <Button onClick={() => refetch()}>{t("common.retry")}</Button>
      </div>
    );
  }

  const actions = (
    <>
      <Button variant="outline" onClick={() => setIsMeshOpen(true)}>
        <Network className="mr-2 h-4 w-4" />
        {t("admin.agents.meshTable.title")}
      </Button>
      <Button variant="outline" onClick={() => setIsBatchOpen(true)}>
        <List className="mr-2 h-4 w-4" />
        {t("admin.agents.batch.manage")}
      </Button>
      <Button variant="outline" onClick={() => refreshMutation.mutate()} disabled={refreshMutation.isPending}>
        <RefreshCw className="mr-2 h-4 w-4" />
        {refreshMutation.isPending ? t("common.loading") : t("common.refresh")}
      </Button>
      <Button data-testid="admin-agents-add-button" onClick={() => setIsDialogOpen(true)} disabled={deployMutation.isPending}>
        <Plus className="mr-2 h-4 w-4" />
        {deployMutation.isPending ? t("common.loading") : t("admin.agents.add")}
      </Button>
    </>
  );

  const stats = (
    <ResponsiveGrid minColWidth={200} gap={16}>
      <Card className="shadow-none">
        <CardContent className="p-5">
          <p className="text-sm text-muted-foreground">{t("admin.agents.stats.total")}</p>
          <p className="mt-2 text-3xl font-semibold tracking-tight text-foreground">{agentStats.total}</p>
        </CardContent>
      </Card>
      <Card className="shadow-none">
        <CardContent className="p-5">
          <p className="text-sm text-muted-foreground">{t("admin.agents.status.online")}</p>
          <p className="mt-2 text-3xl font-semibold tracking-tight text-emerald-600">{agentStats.online}</p>
        </CardContent>
      </Card>
      <Card className="shadow-none">
        <CardContent className="p-5">
          <p className="text-sm text-muted-foreground">{t("admin.agents.status.warning")}</p>
          <p className="mt-2 text-3xl font-semibold tracking-tight text-amber-600">{agentStats.warning}</p>
        </CardContent>
      </Card>
      <Card className="shadow-none">
        <CardContent className="p-5">
          <p className="text-sm text-muted-foreground">{t("admin.agents.status.offline")}</p>
          <p className="mt-2 text-3xl font-semibold tracking-tight text-muted-foreground">{agentStats.offline}</p>
        </CardContent>
      </Card>
      <Card className="shadow-none">
        <CardContent className="p-5">
          <p className="text-sm text-muted-foreground">{t("admin.agents.stats.realtime")}</p>
          <p className="mt-2 text-3xl font-semibold tracking-tight text-primary">{agentStats.realtime}</p>
        </CardContent>
      </Card>
    </ResponsiveGrid>
  );

  const deployDiagnosticsItems = [
    t("admin.agents.deploy.diagnosticsPlatform"),
    t("admin.agents.deploy.diagnosticsConnectivity"),
    t("admin.agents.deploy.diagnosticsRelease"),
    t("admin.agents.deploy.diagnosticsFailure"),
    t("admin.agents.deploy.diagnosticsToken"),
  ];

  const renderDeployDiagnosticsPanel = () => (
    <Card className="border-border bg-muted/30 shadow-none">
      <CardContent className="space-y-3 p-4">
        <div>
          <p className="text-sm font-medium text-foreground">{t("admin.agents.deploy.diagnosticsTitle")}</p>
          <p className="mt-1 text-sm leading-6 text-muted-foreground">
            {t("admin.agents.deploy.diagnosticsDescription")}
          </p>
        </div>
        <ul className="grid gap-2 text-sm leading-6 text-muted-foreground">
          {deployDiagnosticsItems.map((item) => (
            <li key={item} className="flex gap-2">
              <span className="mt-2.5 h-1.5 w-1.5 shrink-0 rounded-full bg-primary/70" />
              <span>{item}</span>
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  );

  return (
    <>
      <AdminPageShell title={t("admin.agents.title")} description={t("admin.agents.description", { count: agents.length })} actions={actions} stats={stats}>
        {agents.length === 0 ? (
          <EmptyState
            icon={<Server className="h-full w-full" />}
            title={t("admin.agents.empty")}
            description={t("admin.agents.emptyDescription") || "Add your first agent to start monitoring your nodes"}
            action={
              <Button data-testid="admin-agents-add-button-empty" onClick={() => setIsDialogOpen(true)} disabled={deployMutation.isPending}>
                <Plus className="mr-2 h-4 w-4" />
                {deployMutation.isPending ? t("common.loading") : t("admin.agents.add")}
              </Button>
            }
            size="lg"
          />
        ) : (
          <ResponsiveGrid minColWidth={300} gap={18}>
            {agents.map((agent) => (
              <AgentStatusCard key={agent.id} agent={agent} onClick={() => handleOpenCorePanel(agent)} onEdit={() => handleOpenEditDialog(agent)} />
            ))}
          </ResponsiveGrid>
        )}
      </AdminPageShell>

      <AgentBatchManagePanel
        open={isBatchOpen}
        onOpenChange={handleBatchDialogChange}
        agents={agents}
        onRefetch={refetch}
      />

      <Dialog open={isCorePanelOpen} onOpenChange={handleCorePanelChange}>
        <DialogContent className="sm:max-w-6xl">{selectedAgent && <AgentCorePanel agentHostId={selectedAgent.id} agentName={selectedAgent.name} />}</DialogContent>
      </Dialog>

      <Dialog open={isEditDialogOpen} onOpenChange={handleEditDialogChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("admin.agents.addTitle")}</DialogTitle>
            <DialogDescription>{t("admin.agents.description", { count: agents.length })}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 pt-2">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.agents.name")}</label>
              <Input
                value={editForm.name}
                onChange={(event) =>
                  setEditForm((prev) => ({
                    ...prev,
                    name: event.target.value,
                  }))
                }
                placeholder={t("admin.agents.namePlaceholder")}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.agents.host")}</label>
              <Input
                value={editForm.host}
                onChange={(event) =>
                  setEditForm((prev) => ({
                    ...prev,
                    host: event.target.value,
                  }))
                }
                placeholder={t("admin.agents.hostPlaceholder")}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => handleEditDialogChange(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={handleSaveEdit} disabled={updateMutation.isPending}>
              {updateMutation.isPending ? t("common.loading") : t("common.save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={isDialogOpen} onOpenChange={handleDialogChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("admin.agents.deploy.title")}</DialogTitle>
            <DialogDescription>{t("admin.agents.deploy.description")}</DialogDescription>
          </DialogHeader>
          <Card className="bg-muted/40 shadow-none">
            <CardContent className="space-y-3 p-5">
              <p className="text-sm leading-6 text-muted-foreground">{t("admin.agents.deploy.description")}</p>
              <p className="text-sm leading-6 text-muted-foreground">{t("admin.agents.deploy.tokenModel")}</p>
            </CardContent>
          </Card>
          {renderDeployDiagnosticsPanel()}
          <DialogFooter>
            <Button variant="outline" onClick={() => handleDialogChange(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={handleOpenDeployDialog} disabled={deployMutation.isPending}>
              {deployMutation.isPending ? t("common.loading") : t("common.create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={isDeployDialogOpen} onOpenChange={handleDeployDialogChange}>
        <DialogContent className="sm:max-w-3xl">
          <DialogHeader>
            <DialogTitle>{t("admin.agents.deploy.title")}</DialogTitle>
            <DialogDescription>{t("admin.agents.deploy.description")}</DialogDescription>
          </DialogHeader>
          {deployCommand ? (
            <div className="space-y-3">
              <div className="rounded-md border border-border bg-muted/30 p-3 text-sm leading-6 text-muted-foreground">
                {t("admin.agents.deploy.tokenModel")}
              </div>
              {renderDeployDiagnosticsPanel()}
              <label className="text-sm font-medium">{t("admin.agents.deploy.command")}</label>
              <pre className="max-h-72 overflow-x-auto whitespace-pre-wrap break-all rounded-md border bg-muted/40 p-4 text-xs font-mono leading-6">
                {deployCommand}
              </pre>
            </div>
          ) : (
            <p className="text-sm text-amber-600">
              {deployMissingAddress ? t("admin.agents.deploy.missingAddress") : t("admin.agents.deploy.missingCommunicationKey")}
            </p>
          )}
          <DialogFooter>
            {deployCommand && (
              <Button type="button" variant="outline" onClick={handleCopyDeployCommand}>
                {t("admin.agents.deploy.copy")}
              </Button>
            )}
            <Button type="button" onClick={() => handleDeployDialogChange(false)}>
              {t("common.close")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <MeshPeerTable
        open={isMeshOpen}
        onOpenChange={setIsMeshOpen}
        onRefetch={refetch}
      />
    </>
  );
}
