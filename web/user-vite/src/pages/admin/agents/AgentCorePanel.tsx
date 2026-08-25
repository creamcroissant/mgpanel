import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient, useIsFetching } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import {
  Download,
  History,
  Plus,
  RefreshCw,
  Repeat,
  Trash2,
} from "lucide-react";
import {
  createAgentCoreInstance,
  deleteAgentCoreInstance,
  installAgentCore,
  listAgentCoreInstances,
  listAgentCoreOperations,
  listAgentCoreSwitchLogs,
  listAgentCores,
  switchAgentCore,
} from "@/api/admin";
import { isAdminApiError } from "@/api/admin/client";
import { QUERY_KEYS } from "@/lib/constants";
import { formatDateTime } from "@/lib/format";
import { AgentCommandQueuePanel } from "./AgentCommandQueuePanel";
import { AgentUpdatePanel } from "./AgentUpdatePanel";
import { BinaryVersionStatusPanel } from "./BinaryVersionStatusPanel";
import { OperationLogTimeline } from "./OperationLogTimeline";
import { TrafficCycleStatusCard } from "./TrafficCycleStatusCard";
import { TrafficPolicyForm } from "./TrafficPolicyForm";
import { MeshStatusCard } from "./MeshStatusCard";
import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  Input,
  Loading,
  Pagination,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui";
import type {
  AgentCoreInstance,
  AgentCoreOperation,
  AgentCoreSwitchLog,
  AgentLifecycleOperation,
  AgentOperationBlocker,
  AdminApiErrorDetails,
  CreateAgentCoreInstanceRequest,
  InstallAgentCoreRequest,
  SwitchAgentCoreRequest,
} from "@/types";

interface AgentCorePanelProps {
  agentHostId: number;
  agentName?: string;
}

type CoreInstanceForm = {
  core_type: string;
  instance_id: string;
  config_template_id: string;
};

type CoreSwitchForm = {
  from_instance_id: string;
  to_core_type: string;
  config_template_id: string;
};

const DEFAULT_FORM: CoreInstanceForm = {
  core_type: "",
  instance_id: "",
  config_template_id: "",
};

const DEFAULT_SWITCH_FORM: CoreSwitchForm = {
  from_instance_id: "",
  to_core_type: "",
  config_template_id: "",
};

const LOGS_PAGE_SIZE = 10;
const OPERATIONS_PAGE_SIZE = 10;
const FILTER_ALL = "__all__";
const ACTIVE_OPERATION_STATUSES = new Set(["pending", "claimed", "in_progress"]);

function normalizeTemplateId(value: string): number | undefined {
  if (!value) return undefined;
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined;
}

function formatPorts(ports: number[]): string {
  if (!ports || ports.length === 0) return "-";
  return ports.join(", ");
}

function getStatusVariant(status: string): "success" | "warning" | "danger" | "secondary" {
  switch (status) {
    case "running":
    case "completed":
      return "success";
    case "pending":
    case "claimed":
    case "in_progress":
      return "warning";
    case "failed":
    case "rolled_back":
      return "danger";
    case "stopped":
      return "secondary";
    default:
      return "secondary";
  }
}

function isOperationActive(status: string): boolean {
  return ACTIVE_OPERATION_STATUSES.has(status);
}

function buildOperationTarget(operation: AgentCoreOperation): string {
  const payload = operation.request_payload ?? {};
  if (operation.operation_type === "create") {
    return String(payload.instance_id || operation.core_type || "-");
  }
  if (operation.operation_type === "switch") {
    const from = String(payload.from_instance_id || "-");
    return `${from} → ${operation.core_type}`;
  }
  return operation.core_type || "-";
}

function extractBlocker(error: unknown): AgentOperationBlocker | null {
  if (!isAdminApiError(error) || error.status !== 409) {
    return null;
  }
  const details = error.details as AdminApiErrorDetails | undefined;
  return details?.blocker ?? null;
}

function describeOperationPayload(operation: AgentCoreOperation): string {
  if (operation.error_message) {
    return operation.error_message;
  }
  if (!operation.result_payload || Object.keys(operation.result_payload).length === 0) {
    return "-";
  }
  return JSON.stringify(operation.result_payload);
}

function BusyBlockerAlert({
  blocker,
  onDismiss,
}: {
  blocker: AgentOperationBlocker;
  onDismiss: () => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="rounded-md border border-warning/30 bg-warning/10 p-4 text-sm">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="space-y-2">
          <div className="font-medium text-warning-foreground dark:text-warning">{t("admin.cores.busyTitle")}</div>
          <div className="text-muted-foreground">{t("admin.cores.busyDescription")}</div>
          <div className="grid gap-2 text-xs text-muted-foreground sm:grid-cols-2 lg:grid-cols-4">
            <div>
              <span className="font-medium text-foreground">{t("admin.cores.busyScope")}: </span>
              {blocker.scope}
            </div>
            <div>
              <span className="font-medium text-foreground">{t("admin.cores.busyOperation")}: </span>
              {blocker.operation_type}
            </div>
            <div>
              <span className="font-medium text-foreground">{t("admin.cores.busyStatus")}: </span>
              {blocker.status}
            </div>
            <div>
              <span className="font-medium text-foreground">{t("admin.cores.busyStartedAt")}: </span>
              {formatDateTime(blocker.created_at)}
            </div>
          </div>
          <div className="break-all font-mono text-xs text-muted-foreground">{blocker.id}</div>
        </div>
        <Button size="sm" variant="outline" onClick={onDismiss}>
          {t("common.close")}
        </Button>
      </div>
    </div>
  );
}

export default function AgentCorePanel({ agentHostId, agentName }: AgentCorePanelProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [isSwitchOpen, setIsSwitchOpen] = useState(false);
  const [isDeleteOpen, setIsDeleteOpen] = useState(false);
  const [isLogsOpen, setIsLogsOpen] = useState(false);
  const [isOperationsOpen, setIsOperationsOpen] = useState(false);
  const [createForm, setCreateForm] = useState<CoreInstanceForm>(DEFAULT_FORM);
  const [switchForm, setSwitchForm] = useState<CoreSwitchForm>(DEFAULT_SWITCH_FORM);
  const [deleteTarget, setDeleteTarget] = useState<AgentCoreInstance | null>(null);
  const [logsStatus, setLogsStatus] = useState<string>(FILTER_ALL);

  const [logsPage, setLogsPage] = useState(1);
  const [logsDateRange, setLogsDateRange] = useState<{ start: string; end: string }>({
    start: "",
    end: "",
  });
  const [operationsStatus, setOperationsStatus] = useState<string>(FILTER_ALL);
  const [operationsType, setOperationsType] = useState<string>(FILTER_ALL);
  const [operationsPage, setOperationsPage] = useState(1);
  const [operationsDateRange, setOperationsDateRange] = useState<{ start: string; end: string }>({
    start: "",
    end: "",
  });
  const [trackedOperationIds, setTrackedOperationIds] = useState<string[]>([]);
  const [selectedOperationId, setSelectedOperationId] = useState<string | null>(null);
  const [busyBlocker, setBusyBlocker] = useState<AgentOperationBlocker | null>(null);
  const [installingCoreType, setInstallingCoreType] = useState<string | null>(null);
  const previousOperationStatusesRef = useRef<Record<string, string>>({});

  const handleCreateDialogChange = (open: boolean) => {
    setIsCreateOpen(open);
    if (!open) {
      setCreateForm(DEFAULT_FORM);
    }
  };

  const handleSwitchDialogChange = (open: boolean) => {
    setIsSwitchOpen(open);
    if (!open) {
      setSwitchForm(DEFAULT_SWITCH_FORM);
    }
  };

  const handleLogsDialogChange = (open: boolean) => {
    setIsLogsOpen(open);
    if (!open) {
      setLogsPage(1);
      setLogsStatus("");
      setLogsDateRange({ start: "", end: "" });
    }
  };


  const handleOperationsDialogChange = (open: boolean) => {
    setIsOperationsOpen(open);
    if (!open) {
      setOperationsPage(1);
      setOperationsStatus(FILTER_ALL);
      setOperationsType(FILTER_ALL);
      setOperationsDateRange({ start: "", end: "" });
    }
  };

  const handleLogsDateChange = (next: { start?: string; end?: string }) => {
    setLogsDateRange((prev) => ({ ...prev, ...next }));
    setLogsPage(1);
  };

  const handleOperationsDateChange = (next: { start?: string; end?: string }) => {
    setOperationsDateRange((prev) => ({ ...prev, ...next }));
    setOperationsPage(1);
  };

  const operationStartAt = operationsDateRange.start
    ? Math.floor(new Date(`${operationsDateRange.start}T00:00:00`).getTime() / 1000)
    : undefined;
  const operationEndAt = operationsDateRange.end
    ? Math.floor(new Date(`${operationsDateRange.end}T23:59:59`).getTime() / 1000)
    : undefined;

  const coresQuery = useQuery({
    queryKey: [...QUERY_KEYS.ADMIN_AGENT_CORES, agentHostId],
    queryFn: () => listAgentCores(agentHostId),
  });

  const instancesQuery = useQuery({
    queryKey: [...QUERY_KEYS.ADMIN_AGENT_CORE_INSTANCES, agentHostId],
    queryFn: () => listAgentCoreInstances(agentHostId),
  });

  const operationsQuery = useQuery({
    queryKey: [
      ...QUERY_KEYS.ADMIN_AGENT_CORE_OPERATIONS,
      agentHostId,
      operationsStatus,
      operationsType,
      operationsPage,
      operationsDateRange.start,
      operationsDateRange.end,
    ],
    queryFn: () =>
      listAgentCoreOperations({
        agent_host_id: agentHostId,
        status: operationsStatus === FILTER_ALL ? undefined : operationsStatus,
        operation_type: operationsType === FILTER_ALL ? undefined : operationsType,
        start_at: operationStartAt,
        end_at: operationEndAt,
        limit: OPERATIONS_PAGE_SIZE,
        offset: (operationsPage - 1) * OPERATIONS_PAGE_SIZE,
      }),
    refetchInterval: (query) => {
      const operations = query.state.data?.operations ?? [];
      const hasActive = operations.some((operation) => isOperationActive(operation.status));
      if (hasActive) return 3000;
      if (isOperationsOpen) return 12000;
      return false;
    },
  });

  const logsQuery = useQuery({
    queryKey: [
      ...QUERY_KEYS.ADMIN_AGENT_CORE_SWITCH_LOGS,
      agentHostId,
      logsStatus,
      logsPage,
      logsDateRange.start,
      logsDateRange.end,
    ],
    queryFn: () => {
      const startAt = logsDateRange.start
        ? Math.floor(new Date(`${logsDateRange.start}T00:00:00`).getTime() / 1000)
        : undefined;
      const endAt = logsDateRange.end
        ? Math.floor(new Date(`${logsDateRange.end}T23:59:59`).getTime() / 1000)
        : undefined;
      return listAgentCoreSwitchLogs({
        agent_host_id: agentHostId,
        status: logsStatus === FILTER_ALL ? undefined : logsStatus,
        start_at: startAt,
        end_at: endAt,
        limit: LOGS_PAGE_SIZE,
        offset: (logsPage - 1) * LOGS_PAGE_SIZE,
      });
    },
    enabled: isLogsOpen,
  });

  useEffect(() => {
    const operations = operationsQuery.data?.operations ?? [];
    const previousStatuses = previousOperationStatusesRef.current;
    const nextStatuses: Record<string, string> = {};

    operations.forEach((operation) => {
      nextStatuses[operation.id] = operation.status;
      const wasTracked = trackedOperationIds.includes(operation.id);
      const previousStatus = previousStatuses[operation.id];
      if (!wasTracked || previousStatus === operation.status) {
        return;
      }
      const enteredTerminal =
        !isOperationActive(operation.status)
        && (!previousStatus || isOperationActive(previousStatus));
      if (!enteredTerminal) {
        return;
      }
      if (operation.status === "completed") {
        toast.success(t("admin.cores.operationCompleted"));
      } else {
        toast.error(t("admin.cores.operationFailed"), {
          description: operation.error_message || operation.status,
        });
      }
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_AGENT_CORES });
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_AGENT_CORE_INSTANCES });
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_AGENT_CORE_SWITCH_LOGS });
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_AGENT_BINARY_VERSIONS });
      setTrackedOperationIds((current) => current.filter((id) => id !== operation.id));
    });

    previousOperationStatusesRef.current = nextStatuses;
  }, [operationsQuery.data, queryClient, t, trackedOperationIds]);


  const onOperationSubmitted = (operation: AgentCoreOperation | AgentLifecycleOperation, messageKey: string) => {
    setBusyBlocker(null);
    setSelectedOperationId(operation.id);
    setTrackedOperationIds((current) => Array.from(new Set([...current, operation.id])));
    queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_AGENT_CORE_OPERATIONS });
    toast.success(t(messageKey));
  };

  const handleOperationError = (error: Error, titleKey: string) => {
    const blocker = extractBlocker(error);
    if (blocker) {
      setBusyBlocker(blocker);
      toast.warning(t("admin.cores.busyTitle"), { description: error.message });
      return;
    }
    toast.error(t(titleKey), { description: error.message });
  };

  const createMutation = useMutation({
    mutationFn: (payload: CreateAgentCoreInstanceRequest) => createAgentCoreInstance(agentHostId, payload),
    onSuccess: (operation) => {
      handleCreateDialogChange(false);
      onOperationSubmitted(operation, "admin.cores.operationSubmitted");
    },
    onError: (err: Error) => {
      handleOperationError(err, "admin.cores.createError");
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (instanceId: string) => deleteAgentCoreInstance(agentHostId, instanceId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_AGENT_CORE_INSTANCES });
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_AGENT_CORE_SWITCH_LOGS });
      setIsDeleteOpen(false);
      setDeleteTarget(null);
      toast.success(t("admin.cores.deleteSuccess"));
    },
    onError: (err: Error) => {
      toast.error(t("admin.cores.deleteError"), { description: err.message });
    },
  });

  const switchMutation = useMutation({
    mutationFn: (payload: SwitchAgentCoreRequest) => switchAgentCore(agentHostId, payload),
    onSuccess: (operation) => {
      handleSwitchDialogChange(false);
      onOperationSubmitted(operation, "admin.cores.operationSubmitted");
    },
    onError: (err: Error) => {
      handleOperationError(err, "admin.cores.switchError");
    },
  });

  const installMutation = useMutation({
    mutationFn: (payload: InstallAgentCoreRequest) => installAgentCore(agentHostId, payload),
    onSuccess: (operation) => {
      setInstallingCoreType(null);
      onOperationSubmitted(operation, "admin.cores.operationSubmitted");
    },
    onError: (err: Error) => {
      setInstallingCoreType(null);
      handleOperationError(err, "admin.cores.installError");
    },
  });

  const handleCreate = () => {
    if (!createForm.core_type || !createForm.instance_id) {
      toast.warning(t("admin.cores.validationError"), {
        description: t("admin.cores.fieldsRequired"),
      });
      return;
    }
    const instanceId = createForm.instance_id.trim();
    if (!instanceId) {
      toast.warning(t("admin.cores.validationError"), {
        description: t("admin.cores.fieldsRequired"),
      });
      return;
    }
    createMutation.mutate({
      core_type: createForm.core_type,
      instance_id: instanceId,
      config_template_id: normalizeTemplateId(createForm.config_template_id),
    });
  };

  const handleSwitch = () => {
    if (!switchForm.from_instance_id || !switchForm.to_core_type) {
      toast.warning(t("admin.cores.validationError"), {
        description: t("admin.cores.switchFieldsRequired"),
      });
      return;
    }
    const fromInstance = switchForm.from_instance_id.trim();
    if (!fromInstance) {
      toast.warning(t("admin.cores.validationError"), {
        description: t("admin.cores.switchFieldsRequired"),
      });
      return;
    }
    switchMutation.mutate({
      from_instance_id: fromInstance,
      to_core_type: switchForm.to_core_type,
      config_template_id: normalizeTemplateId(switchForm.config_template_id),
    });
  };

  const handleInstall = (coreType: string) => {
    setInstallingCoreType(coreType);
    installMutation.mutate({
      core_type: coreType,
      action: "install",
      activate: true,
    });
  };

  const handleDeleteRequest = (instance: AgentCoreInstance) => {
    setDeleteTarget(instance);
    setIsDeleteOpen(true);
  };

  const handleDeleteConfirm = () => {
    if (!deleteTarget) return;
    deleteMutation.mutate(deleteTarget.instance_id);
  };

  const cores = useMemo(() => coresQuery.data ?? [], [coresQuery.data]);
  const instances = useMemo(() => instancesQuery.data ?? [], [instancesQuery.data]);
  const operations = useMemo(() => operationsQuery.data?.operations ?? [], [operationsQuery.data?.operations]);
  const operationsTotal = operationsQuery.data?.total ?? 0;
  const operationsTotalPages = Math.ceil(operationsTotal / OPERATIONS_PAGE_SIZE);
  const logs = useMemo(() => logsQuery.data?.logs ?? [], [logsQuery.data?.logs]);
  const logsTotal = logsQuery.data?.total ?? 0;
  const logsTotalPages = Math.ceil(logsTotal / LOGS_PAGE_SIZE);

  const installedCores = useMemo(() => cores.filter((core) => core.installed), [cores]);
  const coreOptions = useMemo(() => installedCores.map((core) => ({ value: core.type, label: core.type })), [installedCores]);
  const allCoreOptions = useMemo(() => cores.map((core) => ({ value: core.type, label: core.type })), [cores]);
  const instanceOptions = useMemo(
    () =>
      instances.map((instance) => ({
        value: instance.instance_id,
        label: `${instance.core_type} · ${instance.instance_id}`,
      })),
    [instances]
  );
  const recentOperations = useMemo(() => operations.slice(0, 5), [operations]);
  const selectedOperation = useMemo(
    () => operations.find((operation) => operation.id === selectedOperationId) ?? recentOperations[0] ?? null,
    [operations, recentOperations, selectedOperationId]
  );
  const hasActiveOperations = useMemo(
    () => operations.some((operation) => isOperationActive(operation.status)),
    [operations]
  );

  const fallbackCoreOptions = useMemo(() => [{ value: "sing-box", label: "sing-box" }, { value: "xray", label: "xray" }], []);
  const availableCreateOptions = coreOptions.length > 0 ? coreOptions : allCoreOptions.length > 0 ? allCoreOptions : fallbackCoreOptions;
  const isLoading = coresQuery.isLoading || instancesQuery.isLoading;
  const hasError = coresQuery.error || instancesQuery.error;
  const isRefreshing = useIsFetching({ queryKey: ["admin", "agents"] }) > 0;

  if (isLoading) {
    return <Loading />;
  }

  if (hasError) {
    return (
      <div className="flex flex-col items-center justify-center gap-3 py-10">
        <p className="text-sm text-destructive">{t("admin.cores.loadError")}</p>
        <Button
          variant="outline"
          onClick={() => {
            queryClient.invalidateQueries({ queryKey: ["admin", "agents"] });
          }}
        >
          {t("common.retry")}
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-6" data-testid="admin-agent-core-panel">
      <Card>
        <CardHeader>
          <CardTitle>{t("admin.cores.title")}</CardTitle>
          <CardDescription>
            {agentName ? t("admin.cores.subtitleWithAgent", { name: agentName }) : t("admin.cores.subtitle")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-wrap items-center gap-2">
            <Button
              variant="outline"
              disabled={isRefreshing}
              onClick={() => {
                queryClient.invalidateQueries({ queryKey: ["admin", "agents"] });
              }}
            >
              <RefreshCw className={`mr-2 h-4 w-4 ${isRefreshing ? "animate-spin" : ""}`} />
              {isRefreshing ? t("common.loading") : t("common.refresh")}
            </Button>
            <Button onClick={() => setIsCreateOpen(true)}>
              <Plus className="mr-2 h-4 w-4" />
              {t("admin.cores.create")}
            </Button>
            <Button variant="secondary" onClick={() => setIsSwitchOpen(true)}>
              <Repeat className="mr-2 h-4 w-4" />
              {t("admin.cores.switch")}
            </Button>
            <Button variant="outline" onClick={() => setIsOperationsOpen(true)}>
              <History className="mr-2 h-4 w-4" />
              {t("admin.cores.operationsTitle")}
            </Button>
            <Button variant="ghost" onClick={() => setIsLogsOpen(true)}>
              <History className="mr-2 h-4 w-4" />
              {t("admin.cores.logsTitle")}
            </Button>
          </div>

          {busyBlocker && <BusyBlockerAlert blocker={busyBlocker} onDismiss={() => setBusyBlocker(null)} />}

          <BinaryVersionStatusPanel
            agentHostId={agentHostId}
            onCoreOperationSubmitted={(operation) => onOperationSubmitted(operation, "admin.cores.operationSubmitted")}
          />

          <div className="grid gap-4 xl:grid-cols-2">
            <TrafficCycleStatusCard agentHostId={agentHostId} />
            <TrafficPolicyForm agentHostId={agentHostId} />
          </div>

          <MeshStatusCard agentHostId={agentHostId} />

          {cores.length === 0 ? (
            <div className="grid gap-3 sm:grid-cols-2">
              {["sing-box", "xray"].map((coreType) => (
                <Card key={coreType} className="border border-border">
                  <CardContent className="space-y-3 p-4">
                    <div className="flex items-center justify-between">
                      <span className="font-semibold">{coreType}</span>
                      <Badge variant="secondary">{t("admin.cores.uninstalled")}</Badge>
                    </div>
                    <div className="text-xs text-muted-foreground">
                      {t("admin.cores.version")}: -
                    </div>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => handleInstall(coreType)}
                      disabled={installMutation.isPending && installingCoreType === coreType}
                    >
                      <Download className="mr-2 h-4 w-4" />
                      {installMutation.isPending && installingCoreType === coreType
                        ? t("common.loading")
                        : t("admin.cores.install")}
                    </Button>
                  </CardContent>
                </Card>
              ))}
            </div>
          ) : (
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {cores.map((core) => (
                <Card key={core.type} className="border border-border">
                  <CardContent className="space-y-3 p-4">
                    <div className="flex items-center justify-between">
                      <span className="font-semibold">{core.type}</span>
                      <Badge variant={core.installed ? "success" : "secondary"}>
                        {core.installed ? t("admin.cores.installed") : t("admin.cores.uninstalled")}
                      </Badge>
                    </div>
                    <div className="text-xs text-muted-foreground">
                      {t("admin.cores.version")}: {core.version || "-"}
                    </div>
                    {core.capabilities.length > 0 && (
                      <div className="flex flex-wrap gap-2">
                        {core.capabilities.map((capability) => (
                          <Badge key={capability} variant="secondary">
                            {capability}
                          </Badge>
                        ))}
                      </div>
                    )}
                    {!core.installed && (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => handleInstall(core.type)}
                        disabled={installMutation.isPending && installingCoreType === core.type}
                      >
                        <Download className="mr-2 h-4 w-4" />
                        {installMutation.isPending && installingCoreType === core.type
                          ? t("common.loading")
                          : t("admin.cores.install")}
                      </Button>
                    )}
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("admin.cores.instancesTitle")}</CardTitle>
          <CardDescription>{t("admin.cores.instancesSubtitle")}</CardDescription>
        </CardHeader>
        <CardContent>
          {instances.length === 0 ? (
            <EmptyState
              icon={<History className="h-full w-full" />}
              title={t("admin.cores.instancesEmpty")}
              description={t("admin.cores.instancesEmptyDescription")}
              size="sm"
            />
          ) : (
            <Table aria-label={t("admin.cores.instancesTitle")}>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("admin.cores.fields.coreType")}</TableHead>
                  <TableHead>{t("admin.cores.fields.instanceId")}</TableHead>
                  <TableHead>{t("admin.cores.fields.status")}</TableHead>
                  <TableHead>{t("admin.cores.fields.template")}</TableHead>
                  <TableHead>{t("admin.cores.fields.ports")}</TableHead>
                  <TableHead>{t("admin.cores.fields.heartbeat")}</TableHead>
                  <TableHead>{t("admin.cores.fields.error")}</TableHead>
                  <TableHead>{t("common.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {instances.map((instance) => (
                  <TableRow key={instance.instance_id}>
                    <TableCell className="font-medium">{instance.core_type}</TableCell>
                    <TableCell className="font-mono text-xs">{instance.instance_id}</TableCell>
                    <TableCell>
                      <Badge variant={getStatusVariant(instance.status)}>{instance.status}</Badge>
                    </TableCell>
                    <TableCell>{instance.config_template_id ?? "-"}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {formatPorts(instance.listen_ports)}
                    </TableCell>
                    <TableCell>{formatDateTime(instance.last_heartbeat_at ?? 0)}</TableCell>
                    <TableCell>
                      <div className="max-w-[200px] truncate text-xs text-muted-foreground">
                        {instance.error_message || "-"}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-2">
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => {
                            setSwitchForm((prev) => ({
                              ...prev,
                              from_instance_id: instance.instance_id,
                            }));
                            setIsSwitchOpen(true);
                          }}
                        >
                          {t("admin.cores.switch")}
                        </Button>
                        <Button
                          size="sm"
                          variant="destructive"
                          onClick={() => handleDeleteRequest(instance)}
                        >
                          <Trash2 className="mr-1 h-3 w-3" />
                          {t("common.delete")}
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

          <AgentUpdatePanel agentHostId={agentHostId} />

      <Dialog open={isCreateOpen} onOpenChange={handleCreateDialogChange}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>{t("admin.cores.createTitle")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.cores.fields.coreType")}</label>
              <Select
                value={createForm.core_type}
                onValueChange={(value) => setCreateForm((prev) => ({ ...prev, core_type: value }))}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t("admin.cores.placeholders.coreType")} />
                </SelectTrigger>
                <SelectContent>
                  {availableCreateOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.cores.fields.instanceId")}</label>
              <Input
                value={createForm.instance_id}
                onChange={(event) => setCreateForm((prev) => ({ ...prev, instance_id: event.target.value }))}
                placeholder={t("admin.cores.placeholders.instanceId")}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.cores.fields.template")}</label>
              <Input
                value={createForm.config_template_id}
                onChange={(event) => setCreateForm((prev) => ({ ...prev, config_template_id: event.target.value }))}
                placeholder={t("admin.cores.placeholders.templateId")}
              />
              <p className="text-xs text-muted-foreground">{t("admin.cores.templateHint")}</p>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => handleCreateDialogChange(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={handleCreate} disabled={createMutation.isPending}>
              {createMutation.isPending ? t("common.loading") : t("common.create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={isSwitchOpen} onOpenChange={handleSwitchDialogChange}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>{t("admin.cores.switchTitle")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.cores.fields.fromInstance")}</label>
              <Select
                value={switchForm.from_instance_id}
                onValueChange={(value) => setSwitchForm((prev) => ({ ...prev, from_instance_id: value }))}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t("admin.cores.placeholders.fromInstance")} />
                </SelectTrigger>
                <SelectContent>
                  {instanceOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.cores.fields.coreType")}</label>
              <Select
                value={switchForm.to_core_type}
                onValueChange={(value) => setSwitchForm((prev) => ({ ...prev, to_core_type: value }))}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t("admin.cores.placeholders.coreType")} />
                </SelectTrigger>
                <SelectContent>
                  {coreOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.cores.fields.template")}</label>
              <Input
                value={switchForm.config_template_id}
                onChange={(event) => setSwitchForm((prev) => ({ ...prev, config_template_id: event.target.value }))}
                placeholder={t("admin.cores.placeholders.templateId")}
              />
              <p className="text-xs text-muted-foreground">{t("admin.cores.templateHint")}</p>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => handleSwitchDialogChange(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={handleSwitch} disabled={switchMutation.isPending}>
              {switchMutation.isPending ? t("common.loading") : t("admin.cores.switch")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={isOperationsOpen} onOpenChange={handleOperationsDialogChange}>
        <DialogContent className="sm:max-w-5xl">
          <DialogHeader>
            <DialogTitle>{t("admin.cores.operationsTitle")}</DialogTitle>
          </DialogHeader>
          <div className="flex max-h-[calc(100dvh-8rem)] flex-col gap-4 overflow-y-auto py-2 pr-1">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.cores.logsStart")}</label>
                <Input
                  type="date"
                  value={operationsDateRange.start}
                  onChange={(event) => handleOperationsDateChange({ start: event.target.value })}
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.cores.logsEnd")}</label>
                <Input
                  type="date"
                  value={operationsDateRange.end}
                  onChange={(event) => handleOperationsDateChange({ end: event.target.value })}
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.cores.logsStatus")}</label>
                <Select value={operationsStatus} onValueChange={(value) => {
                  setOperationsStatus(value);
                  setOperationsPage(1);
                }}>
                  <SelectTrigger>
                    <SelectValue placeholder={t("admin.cores.logsStatusPlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={FILTER_ALL}>{t("common.all")}</SelectItem>
                    <SelectItem value="pending">{t("admin.cores.status.pending")}</SelectItem>
                    <SelectItem value="claimed">{t("admin.cores.status.claimed")}</SelectItem>
                    <SelectItem value="in_progress">{t("admin.cores.status.in_progress")}</SelectItem>
                    <SelectItem value="completed">{t("admin.cores.status.completed")}</SelectItem>
                    <SelectItem value="failed">{t("admin.cores.status.failed")}</SelectItem>
                    <SelectItem value="rolled_back">{t("admin.cores.status.rolled_back")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.cores.operationsType")}</label>
                <Select value={operationsType} onValueChange={(value) => {
                  setOperationsType(value);
                  setOperationsPage(1);
                }}>
                  <SelectTrigger>
                    <SelectValue placeholder={t("admin.cores.operationsTypePlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={FILTER_ALL}>{t("common.all")}</SelectItem>
                    <SelectItem value="create">{t("admin.cores.operationType.create")}</SelectItem>
                    <SelectItem value="switch">{t("admin.cores.operationType.switch")}</SelectItem>
                    <SelectItem value="install">{t("admin.cores.operationType.install")}</SelectItem>
                    <SelectItem value="ensure">{t("admin.cores.operationType.ensure")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            <AgentCommandQueuePanel agentHostId={agentHostId} />

            <Card className="border border-border">
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">{t("admin.cores.operationsSummaryTitle")}</CardTitle>
                <CardDescription>
                  {hasActiveOperations ? t("admin.cores.operationsPolling") : t("admin.cores.operationsIdle")}
                </CardDescription>
              </CardHeader>
              <CardContent>
                {recentOperations.length === 0 ? (
                  <EmptyState
                    icon={<History className="h-full w-full" />}
                    title={t("admin.cores.operationsEmpty")}
                    description={t("admin.cores.operationsEmptyDescription")}
                    size="sm"
                  />
                ) : (
                  <div className="space-y-2">
                    {recentOperations.map((operation) => {
                      const selected = selectedOperation?.id === operation.id;
                      return (
                        <button
                          type="button"
                          key={operation.id}
                          className={`flex w-full flex-col gap-2 rounded-md border p-3 text-left transition-colors sm:flex-row sm:items-center sm:justify-between ${
                            selected ? "border-primary bg-primary/5" : "border-border hover:bg-muted/50"
                          }`}
                          onClick={() => setSelectedOperationId(operation.id)}
                        >
                          <div className="space-y-1">
                            <div className="flex flex-wrap items-center gap-2">
                              <Badge variant={getStatusVariant(operation.status)}>{operation.status}</Badge>
                              <span className="text-sm font-medium">
                                {t(`admin.cores.operationType.${operation.operation_type}`)}
                              </span>
                            </div>
                            <div className="text-xs text-muted-foreground">{buildOperationTarget(operation)}</div>
                            <div className="text-xs text-muted-foreground">{formatDateTime(operation.created_at)}</div>
                          </div>
                          <div className="max-w-[360px] truncate text-xs text-muted-foreground">
                            {describeOperationPayload(operation)}
                          </div>
                        </button>
                      );
                    })}
                  </div>
                )}
              </CardContent>
            </Card>

            <OperationLogTimeline
              agentHostId={agentHostId}
              targetId={selectedOperation?.id}
              enabled={Boolean(selectedOperation?.id)}
            />

            {operationsQuery.isLoading ? (
              <Loading />
            ) : operations.length === 0 ? (
              <EmptyState
                icon={<History className="h-full w-full" />}
                title={t("admin.cores.operationsEmpty")}
                description={t("admin.cores.operationsEmptyDescription")}
                size="sm"
              />
            ) : (
              <Table aria-label={t("admin.cores.operationsTitle")}>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("admin.cores.logsTime")}</TableHead>
                    <TableHead>{t("admin.cores.operationsType")}</TableHead>
                    <TableHead>{t("admin.cores.operationsTarget")}</TableHead>
                    <TableHead>{t("admin.cores.logsStatus")}</TableHead>
                    <TableHead>{t("admin.cores.logsDetail")}</TableHead>
                    <TableHead>{t("admin.cores.operationsFinishedAt")}</TableHead>
                    <TableHead>{t("common.actions")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {operations.map((operation) => (
                    <TableRow
                      key={operation.id}
                      className={selectedOperation?.id === operation.id ? "bg-primary/5" : undefined}
                    >
                      <TableCell>{formatDateTime(operation.created_at)}</TableCell>
                      <TableCell>{t(`admin.cores.operationType.${operation.operation_type}`)}</TableCell>
                      <TableCell>{buildOperationTarget(operation)}</TableCell>
                      <TableCell>
                        <Badge variant={getStatusVariant(operation.status)}>{operation.status}</Badge>
                      </TableCell>
                      <TableCell>
                        <div className="max-w-[320px] truncate text-xs text-muted-foreground">
                          {describeOperationPayload(operation)}
                        </div>
                      </TableCell>
                      <TableCell>{formatDateTime(operation.finished_at ?? 0)}</TableCell>
                      <TableCell>
                        <Button size="sm" variant="outline" onClick={() => setSelectedOperationId(operation.id)}>
                          {t("admin.cores.viewTimeline")}
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}

            <Pagination page={operationsPage} totalPages={operationsTotalPages} onPageChange={setOperationsPage} />

            <details className="rounded-md border border-border">
              <summary className="cursor-pointer select-none p-3 text-sm font-medium text-muted-foreground hover:text-foreground">
                {t("admin.cores.viewTimeline")}
              </summary>
              <div className="border-t border-border p-3">
                <OperationLogTimeline
                  agentHostId={agentHostId}
                  targetId={selectedOperation?.id}
                  enabled={isOperationsOpen && Boolean(selectedOperation?.id)}
                />
              </div>
            </details>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => handleOperationsDialogChange(false)}>
              {t("common.close")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={isLogsOpen} onOpenChange={handleLogsDialogChange}>
        <DialogContent className="sm:max-w-5xl">
          <DialogHeader>
            <DialogTitle>{t("admin.cores.logsTitle")}</DialogTitle>
          </DialogHeader>
          <div className="flex max-h-[calc(100dvh-8rem)] flex-col gap-4 overflow-y-auto py-2 pr-1">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.cores.logsStart")}</label>
                <Input
                  type="date"
                  value={logsDateRange.start}
                  onChange={(event) => handleLogsDateChange({ start: event.target.value })}
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.cores.logsEnd")}</label>
                <Input
                  type="date"
                  value={logsDateRange.end}
                  onChange={(event) => handleLogsDateChange({ end: event.target.value })}
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.cores.logsStatus")}</label>
                <Select value={logsStatus} onValueChange={(value) => {
                  setLogsStatus(value);
                  setLogsPage(1);
                }}>
                  <SelectTrigger>
                    <SelectValue placeholder={t("admin.cores.logsStatusPlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={FILTER_ALL}>{t("common.all")}</SelectItem>
                    <SelectItem value="pending">{t("admin.cores.status.pending")}</SelectItem>
                    <SelectItem value="claimed">{t("admin.cores.status.claimed")}</SelectItem>
                    <SelectItem value="in_progress">{t("admin.cores.status.in_progress")}</SelectItem>
                    <SelectItem value="completed">{t("admin.cores.status.completed")}</SelectItem>
                    <SelectItem value="failed">{t("admin.cores.status.failed")}</SelectItem>
                    <SelectItem value="rolled_back">{t("admin.cores.status.rolled_back")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <Button variant="outline" onClick={() => handleLogsDateChange({ start: "", end: "" })}>
                {t("admin.cores.logsClear")}
              </Button>
            </div>

            {logsQuery.isLoading ? (
              <Loading />
            ) : logsQuery.error ? (
              <div className="flex flex-col items-center justify-center gap-3 py-6">
                <p className="text-sm text-destructive">{t("admin.cores.logsLoadError")}</p>
                <Button variant="outline" onClick={() => logsQuery.refetch()}>
                  {t("common.retry")}
                </Button>
              </div>
            ) : logs.length === 0 ? (
              <EmptyState
                icon={<History className="h-full w-full" />}
                title={t("admin.cores.logsEmpty")}
                description={t("admin.cores.logsEmptyDescription")}
                size="sm"
              />
            ) : (
              <Table aria-label={t("admin.cores.logsTitle")}>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("admin.cores.logsTime")}</TableHead>
                    <TableHead>{t("admin.cores.logsFrom")}</TableHead>
                    <TableHead>{t("admin.cores.logsTo")}</TableHead>
                    <TableHead>{t("admin.cores.logsStatus")}</TableHead>
                    <TableHead>{t("admin.cores.logsOperator")}</TableHead>
                    <TableHead>{t("admin.cores.logsDetail")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {logs.map((log: AgentCoreSwitchLog) => (
                    <TableRow key={log.id}>
                      <TableCell className="whitespace-nowrap">{formatDateTime(log.created_at)}</TableCell>
                      <TableCell className="text-xs text-muted-foreground">
                        {log.from_core_type || "-"}
                        <div className="font-mono">{log.from_instance_id || "-"}</div>
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground">
                        {log.to_core_type}
                        <div className="font-mono">{log.to_instance_id}</div>
                      </TableCell>
                      <TableCell>
                        <Badge variant={getStatusVariant(log.status)}>{log.status}</Badge>
                      </TableCell>
                      <TableCell>{log.operator_id ?? "-"}</TableCell>
                      <TableCell>
                        <div className="max-w-xl truncate text-xs text-muted-foreground">{log.detail || "-"}</div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}

            <Pagination page={logsPage} totalPages={logsTotalPages} onPageChange={setLogsPage} />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => handleLogsDialogChange(false)}>
              {t("common.close")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={isDeleteOpen} onOpenChange={setIsDeleteOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{t("admin.cores.deleteTitle")}</DialogTitle>
          </DialogHeader>
          <div className="py-2 text-sm text-muted-foreground">
            {t("admin.cores.deleteConfirm", {
              instanceId: deleteTarget?.instance_id,
              coreType: deleteTarget?.core_type,
            })}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsDeleteOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button variant="destructive" onClick={handleDeleteConfirm} disabled={deleteMutation.isPending}>
              {deleteMutation.isPending ? t("common.loading") : t("common.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
