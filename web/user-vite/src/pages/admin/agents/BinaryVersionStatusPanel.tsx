import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Download, MoreHorizontal, RefreshCw, Trash2, Upload, ArrowUp } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { listAgentBinaryVersions, refreshAgentBinaryVersion, installAgentCore, createAgentUpdateOperation } from "@/api/admin";
import { QUERY_KEYS } from "@/lib/constants";
import { formatDateTime } from "@/lib/format";
import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  EmptyState,
  Loading,
} from "@/components/ui";
import { useState } from "react";
import type { AgentCoreOperation, BinaryVersionComponent, BinaryVersionState, BinaryVersionStatus, AgentLifecycleOperation } from "@/types";

interface BinaryVersionStatusPanelProps {
  agentHostId: number;
  onCoreOperationSubmitted?: (operation: AgentCoreOperation | AgentLifecycleOperation) => void;
}

const COMPONENTS: BinaryVersionComponent[] = ["agent", "sing-box", "xray"];

function getVersionStatusVariant(status: BinaryVersionStatus): "success" | "warning" | "danger" | "secondary" {
  switch (status) {
    case "up_to_date":
    case "installed":
      return "success";
    case "outdated":
      return "warning";
    case "missing":
      return "danger";
    default:
      return "secondary";
  }
}

function normalizeVersion(value?: string): string {
  const trimmed = value?.trim();
  return trimmed || "-";
}

function buildRows(states: BinaryVersionState[]): BinaryVersionState[] {
  const byComponent = new Map(states.map((state) => [state.component, state]));
  return COMPONENTS.map((component) =>
    byComponent.get(component) ?? {
      agent_host_id: 0,
      component,
      local_version: "",
      status: "unknown",
    }
  );
}

export function BinaryVersionStatusPanel({ agentHostId, onCoreOperationSubmitted }: BinaryVersionStatusPanelProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [openMenu, setOpenMenu] = useState<string | null>(null);

  const versionsQuery = useQuery({
    queryKey: [...QUERY_KEYS.ADMIN_AGENT_BINARY_VERSIONS, agentHostId],
    queryFn: () => listAgentBinaryVersions(agentHostId),
  });

  const refreshMutation = useMutation({
    mutationFn: (component: BinaryVersionComponent) => refreshAgentBinaryVersion(agentHostId, component),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_AGENT_BINARY_VERSIONS });
      toast.success(t("admin.cores.versionRefreshSuccess"));
    },
    onError: (error: Error) => {
      toast.error(t("admin.cores.versionRefreshError"), { description: error.message });
    },
  });

  const installMutation = useMutation({
    mutationFn: (args: { coreType: string; action: string }) =>
      installAgentCore(agentHostId, { core_type: args.coreType, action: args.action, activate: true }),
    onSuccess: (operation) => {
      queryClient.invalidateQueries({ queryKey: ["admin", "agents"] });
      if (onCoreOperationSubmitted) {
        onCoreOperationSubmitted(operation);
        return;
      }
      toast.success(t("admin.cores.operationSubmitted"));
    },
    onError: (error: Error) => {
      toast.error(t("admin.cores.operationError"), { description: error.message });
    },
  });

  const updateMutation = useMutation({
    mutationFn: () => createAgentUpdateOperation(agentHostId, { release_tag: "latest" }),
    onSuccess: (operation) => {
      queryClient.invalidateQueries({ queryKey: ["admin", "agents"] });
      if (onCoreOperationSubmitted) {
        onCoreOperationSubmitted(operation);
        return;
      }
      toast.success(t("admin.cores.updateSubmitted") || "Update submitted");
    },
    onError: (error: Error) => {
      toast.error(t("admin.cores.updateError") || "Update failed", { description: error.message });
    },
  });

  const rows = buildRows(versionsQuery.data ?? []);

  return (
    <Card className="border border-border shadow-none">
      <CardHeader className="pb-3">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <CardTitle className="text-base">{t("admin.cores.versionStatusTitle")}</CardTitle>
            <CardDescription>{t("admin.cores.versionStatusDescription")}</CardDescription>
          </div>
          <Button
            size="sm"
            variant="outline"
            onClick={() => versionsQuery.refetch()}
            disabled={versionsQuery.isFetching}
          >
            <RefreshCw className="mr-2 h-3.5 w-3.5" />
            {t("common.refresh")}
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {versionsQuery.isLoading ? (
          <Loading />
        ) : versionsQuery.error ? (
          <div className="flex flex-col items-center justify-center gap-3 py-6">
            <p className="text-sm text-destructive">{t("admin.cores.versionLoadError")}</p>
            <Button variant="outline" onClick={() => versionsQuery.refetch()}>
              {t("common.retry")}
            </Button>
          </div>
        ) : rows.length === 0 ? (
          <EmptyState
            icon={<RefreshCw className="h-full w-full" />}
            title={t("admin.cores.versionEmpty")}
            description={t("admin.cores.versionEmptyDescription")}
            size="sm"
          />
        ) : (
          <div className="grid gap-3 lg:grid-cols-3">
            {rows.map((state) => {
              const isRefreshing = refreshMutation.isPending && refreshMutation.variables === state.component;
              return (
                <div key={state.component} className="rounded-md border border-border p-3">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <div className="font-semibold">{t(`admin.cores.binaryComponent.${state.component}`)}</div>
                      <div className="mt-1 text-xs text-muted-foreground">
                        {t("admin.cores.versionCheckedAt")}: {formatDateTime(state.last_checked_at ?? 0)}
                      </div>
                    </div>
                    <Badge variant={getVersionStatusVariant(state.status)}>
                      {t(`admin.cores.versionState.${state.status}`)}
                    </Badge>
                  </div>
                  <div className="mt-3 grid gap-2 text-sm">
                    <div className="flex justify-between gap-3">
                      <span className="text-muted-foreground">{t("admin.cores.versionLocal")}</span>
                      <span className="font-mono text-xs">{normalizeVersion(state.local_version)}</span>
                    </div>
                    <div className="flex justify-between gap-3">
                      <span className="text-muted-foreground">{t("admin.cores.versionRemote")}</span>
                      <span className="font-mono text-xs">{normalizeVersion(state.remote_version)}</span>
                    </div>
                  </div>
                  {state.last_check_error && (
                    <div className="mt-3 rounded-md border border-warning/30 bg-warning/10 p-2 text-xs text-warning-foreground dark:text-warning">
                      {state.last_check_error}
                    </div>
                  )}
                  <div className="mt-3 flex items-center gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => refreshMutation.mutate(state.component)}
                      disabled={isRefreshing}
                    >
                      <RefreshCw className="mr-2 h-3.5 w-3.5" />
                      {isRefreshing ? t("common.loading") : t("admin.cores.versionRefresh")}
                    </Button>
                    {state.component === "agent" ? (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => updateMutation.mutate()}
                        disabled={updateMutation.isPending}
                      >
                        <ArrowUp className="mr-2 h-3.5 w-3.5" />
                        {updateMutation.isPending ? t("common.loading") : t("admin.cores.coreUpgrade")}
                      </Button>
                    ) : (
                      <DropdownMenu open={openMenu === state.component} onOpenChange={(open) => setOpenMenu(open ? state.component : null)}>
                        <DropdownMenuTrigger asChild>
                          <Button size="sm" variant="outline" className="px-2">
                            <MoreHorizontal className="h-4 w-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end" className="w-36">
                          <DropdownMenuItem
                            onClick={() => {
                              setOpenMenu(null);
                              installMutation.mutate({ coreType: state.component, action: "install" });
                            }}
                            disabled={installMutation.isPending}
                          >
                            <Download className="mr-2 h-4 w-4" />
                            {t("admin.cores.coreInstall")}
                          </DropdownMenuItem>
                          <DropdownMenuItem
                            onClick={() => {
                              setOpenMenu(null);
                              installMutation.mutate({ coreType: state.component, action: "upgrade" });
                            }}
                            disabled={installMutation.isPending}
                          >
                            <Upload className="mr-2 h-4 w-4" />
                            {t("admin.cores.coreUpgrade")}
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem
                            onClick={() => {
                              setOpenMenu(null);
                              installMutation.mutate({ coreType: state.component, action: "uninstall" });
                            }}
                            disabled={installMutation.isPending}
                            className="text-destructive focus:text-destructive"
                          >
                            <Trash2 className="mr-2 h-4 w-4" />
                            {t("admin.cores.coreUninstall")}
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
