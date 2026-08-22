import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Plus, MoreVertical, Pencil, Trash2, Shuffle, History, Calendar } from "lucide-react";
import { QUERY_KEYS } from "@/lib/constants";
import {
  listForwardingRules,
  createForwardingRule,
  updateForwardingRule,
  deleteForwardingRule,
  listForwardingLogs,
} from "@/api/admin";
import { getAgentHosts } from "@/api/admin/agentHost";
import {
  Badge,
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  EmptyState,
  Input,
  Loading,
  Pagination,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Switch,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui";
import type {
  AgentHost,
  ForwardingRule,
  CreateForwardingRuleRequest,
  ForwardingRuleLog,
} from "@/types";

const defaultFormData: CreateForwardingRuleRequest = {
  agent_host_id: 0,
  name: "",
  protocol: "tcp",
  listen_port: 0,
  target_address: "",
  target_port: 0,
  enabled: true,
  priority: 100,
  remark: "",
};

const protocolOptions = ["tcp", "udp", "both"] as const;

export default function ForwardingList() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [isOpen, setIsOpen] = useState(false);
  const [isDeleteOpen, setIsDeleteOpen] = useState(false);
  const [isLogsOpen, setIsLogsOpen] = useState(false);
  const [agentHostId, setAgentHostId] = useState<number | null>(null);
  const [editingRule, setEditingRule] = useState<ForwardingRule | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<ForwardingRule | null>(null);
  const [logsTarget, setLogsTarget] = useState<ForwardingRule | null>(null);
  const [logsPage, setLogsPage] = useState(1);
  const [logsDateRange, setLogsDateRange] = useState<{ start: string; end: string }>({
    start: "",
    end: "",
  });
  const [formData, setFormData] = useState<CreateForwardingRuleRequest>(defaultFormData);

  const agentHostsQuery = useQuery({
    queryKey: QUERY_KEYS.ADMIN_AGENTS,
    queryFn: () => getAgentHosts({ page: 1, page_size: 100 }),
  });

  const rulesQuery = useQuery({
    queryKey: [...QUERY_KEYS.ADMIN_FORWARDING, agentHostId],
    queryFn: () => listForwardingRules(agentHostId as number),
    enabled: agentHostId !== null,
  });

  const logsQuery = useQuery({
    queryKey: [
      ...QUERY_KEYS.ADMIN_FORWARDING_LOGS,
      agentHostId,
      logsTarget?.id,
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
      return listForwardingLogs({
        agent_host_id: agentHostId as number,
        rule_id: logsTarget?.id,
        start_at: startAt,
        end_at: endAt,
        limit: 10,
        offset: (logsPage - 1) * 10,
      });
    },
    enabled: agentHostId !== null && isLogsOpen,
  });

  const createMutation = useMutation({
    mutationFn: createForwardingRule,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_FORWARDING });
      handleDialogChange(false);
      toast.success(t("admin.forwarding.createSuccess"));
    },
    onError: (err: Error) => {
      toast.error(t("admin.forwarding.createError"), { description: err.message });
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: Partial<CreateForwardingRuleRequest> }) =>
      updateForwardingRule(id, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_FORWARDING });
      handleDialogChange(false);
      toast.success(t("admin.forwarding.updateSuccess"));
    },
    onError: (err: Error) => {
      toast.error(t("admin.forwarding.updateError"), { description: err.message });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteForwardingRule,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_FORWARDING });
      handleDeleteDialogChange(false);
      toast.success(t("admin.forwarding.deleteSuccess"));
    },
    onError: (err: Error) => {
      toast.error(t("admin.forwarding.deleteError"), { description: err.message });
    },
  });

  const handleDialogChange = (open: boolean) => {
    setIsOpen(open);
    if (!open) {
      setEditingRule(null);
      setFormData({ ...defaultFormData, agent_host_id: agentHostId ?? 0 });
    }
  };

  const handleDeleteDialogChange = (open: boolean) => {
    setIsDeleteOpen(open);
    if (!open) {
      setDeleteTarget(null);
    }
  };

  const handleLogsDialogChange = (open: boolean) => {
    setIsLogsOpen(open);
    if (!open) {
      setLogsTarget(null);
      setLogsPage(1);
      setLogsDateRange({ start: "", end: "" });
    }
  };

  const handleDeleteConfirm = () => {
    if (!deleteTarget) {
      return;
    }
    deleteMutation.mutate(deleteTarget.id);
  };

  const handleDeleteRequest = (rule: ForwardingRule) => {
    setDeleteTarget(rule);
    setIsDeleteOpen(true);
  };

  const handleEdit = (rule: ForwardingRule) => {
    setEditingRule(rule);
    setFormData({
      agent_host_id: rule.agent_host_id,
      name: rule.name,
      protocol: rule.protocol,
      listen_port: rule.listen_port,
      target_address: rule.target_address,
      target_port: rule.target_port,
      enabled: rule.enabled,
      priority: rule.priority,
      remark: rule.remark,
    });
    setIsOpen(true);
  };

  const handleSubmit = () => {
    if (!agentHostId) {
      toast.warning(t("admin.forwarding.agentRequired"));
      return;
    }
    if (!formData.name || !formData.protocol || !formData.listen_port || !formData.target_address || !formData.target_port) {
      toast.warning(t("admin.forwarding.fieldsRequired"));
      return;
    }
    if (editingRule) {
      const payload: Partial<CreateForwardingRuleRequest> = {
        name: formData.name,
        protocol: formData.protocol,
        listen_port: formData.listen_port,
        target_address: formData.target_address,
        target_port: formData.target_port,
        enabled: formData.enabled,
        priority: formData.priority,
        remark: formData.remark,
      };
      updateMutation.mutate({ id: editingRule.id, payload });
    } else {
      createMutation.mutate({ ...formData, agent_host_id: agentHostId });
    }
  };

  const handleToggleEnabled = (rule: ForwardingRule) => {
    updateMutation.mutate({ id: rule.id, payload: { enabled: !rule.enabled } });
  };

  const handleAgentChange = (value: string) => {
    const id = Number(value);
    if (!Number.isNaN(id)) {
      setAgentHostId(id);
      setFormData((prev) => ({ ...prev, agent_host_id: id }));
    }
  };

  const handleLogsClose = () => {
    setIsLogsOpen(false);
    setLogsTarget(null);
    setLogsPage(1);
    setLogsDateRange({ start: "", end: "" });
  };

  const handleLogsDateChange = (next: { start?: string; end?: string }) => {
    setLogsDateRange((prev) => ({ ...prev, ...next }));
    setLogsPage(1);
  };

  const handleLogsOpen = (rule: ForwardingRule) => {
    setLogsTarget(rule);
    setLogsPage(1);
    setIsLogsOpen(true);
  };

  const formatLogTime = (timestamp: number) => {
    return new Date(timestamp * 1000).toLocaleString();
  };

  const formatLogAction = (action: ForwardingRuleLog["action"]) => {
    switch (action) {
      case "create":
        return t("admin.forwarding.logsActionCreate");
      case "update":
        return t("admin.forwarding.logsActionUpdate");
      case "delete":
        return t("admin.forwarding.logsActionDelete");
      case "apply":
        return t("admin.forwarding.logsActionApply");
      case "fail":
        return t("admin.forwarding.logsActionFail");
      default:
        return action;
    }
  };

  const agentHosts = useMemo<AgentHost[]>(() => agentHostsQuery.data?.data ?? [], [agentHostsQuery.data?.data]);
  const rules = useMemo(() => rulesQuery.data?.rules ?? [], [rulesQuery.data?.rules]);
  const rulesVersion = rulesQuery.data?.version ?? 0;
  const logs = useMemo(() => logsQuery.data?.logs ?? [], [logsQuery.data?.logs]);
  const logsTotal = logsQuery.data?.total || 0;
  const logsTotalPages = Math.ceil(logsTotal / 10);

  const sortedRules = useMemo(() => {
    return [...rules].sort((a, b) => a.priority - b.priority || a.id - b.id);
  }, [rules]);

  const selectedAgent = useMemo(
    () => agentHosts.find((host) => host.id === agentHostId) || null,
    [agentHosts, agentHostId]
  );

  if (agentHostsQuery.isLoading) {
    return <Loading />;
  }

  if (agentHostsQuery.error) {
    return (
      <div className="flex flex-col items-center justify-center gap-3 py-20">
        <p className="text-sm text-destructive">{t("admin.forwarding.agentLoadError")}</p>
        <Button variant="outline" onClick={() => agentHostsQuery.refetch()}>
          {t("common.retry")}
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold">{t("admin.forwarding.title")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t("admin.forwarding.subtitle")}</p>
        </div>
        <Button onClick={() => setIsOpen(true)} disabled={!agentHostId}>
          <Plus className="mr-2 h-4 w-4" />
          {t("admin.forwarding.add")}
        </Button>
      </div>

      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:gap-4">
        <div className="w-full max-w-xs space-y-2">
          <label className="text-sm font-medium">{t("admin.forwarding.agent")}</label>
          <Select value={agentHostId ? String(agentHostId) : undefined} onValueChange={handleAgentChange}>
            <SelectTrigger>
              <SelectValue placeholder={t("admin.forwarding.agentPlaceholder")} />
            </SelectTrigger>
            <SelectContent>
              {agentHosts.map((host) => (
                <SelectItem key={String(host.id)} value={String(host.id)}>
                  {host.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        {selectedAgent && (
          <p className="text-sm text-muted-foreground">
            {t("admin.forwarding.selectedAgent", { name: selectedAgent.name, host: selectedAgent.host })}
          </p>
        )}
        {agentHostId !== null && !rulesQuery.isLoading && !rulesQuery.error && (
          <Badge variant="secondary">{t("admin.forwarding.version", { version: rulesVersion })}</Badge>
        )}
      </div>

      {!agentHostId ? (
        <EmptyState
          icon={<Shuffle className="h-full w-full" />}
          title={t("admin.forwarding.emptyAgent")}
          description={t("admin.forwarding.emptyAgentDescription")}
          size="md"
        />
      ) : rulesQuery.isLoading ? (
        <Loading />
      ) : rulesQuery.error ? (
        <div className="flex flex-col items-center justify-center gap-3 py-20">
          <p className="text-sm text-destructive">{t("admin.forwarding.loadError")}</p>
          <Button variant="outline" onClick={() => rulesQuery.refetch()}>
            {t("common.retry")}
          </Button>
        </div>
      ) : rules.length === 0 ? (
        <EmptyState
          icon={<Shuffle className="h-full w-full" />}
          title={t("admin.forwarding.empty")}
          description={t("admin.forwarding.emptyDescription")}
          action={
            <Button onClick={() => setIsOpen(true)}>
              <Plus className="mr-2 h-4 w-4" />
              {t("admin.forwarding.add")}
            </Button>
          }
          size="md"
        />
      ) : (
        <Table aria-label={t("admin.forwarding.title")}>
          <TableHeader>
            <TableRow>
              <TableHead>{t("admin.forwarding.name")}</TableHead>
              <TableHead>{t("admin.forwarding.protocol")}</TableHead>
              <TableHead>{t("admin.forwarding.listen")}</TableHead>
              <TableHead>{t("admin.forwarding.target")}</TableHead>
              <TableHead>{t("admin.forwarding.priority")}</TableHead>
              <TableHead>{t("admin.forwarding.status")}</TableHead>
              <TableHead>{t("common.actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sortedRules.map((rule) => (
              <TableRow key={rule.id}>
                <TableCell className="font-medium">
                  <div className="flex flex-col">
                    <span>{rule.name}</span>
                    {rule.remark && (
                      <span className="text-xs text-muted-foreground truncate">{rule.remark}</span>
                    )}
                  </div>
                </TableCell>
                <TableCell>
                  <Badge variant="secondary">{rule.protocol.toUpperCase()}</Badge>
                </TableCell>
                <TableCell>
                  <span className="font-mono text-sm">{rule.listen_port}</span>
                </TableCell>
                <TableCell>
                  <div className="flex flex-col">
                    <span className="font-mono text-sm">{rule.target_address}</span>
                    <span className="text-xs text-muted-foreground">:{rule.target_port}</span>
                  </div>
                </TableCell>
                <TableCell>{rule.priority}</TableCell>
                <TableCell>
                  <div className="flex items-center gap-2">
                    <Switch
                      checked={rule.enabled}
                      onCheckedChange={() => handleToggleEnabled(rule)}
                      disabled={updateMutation.isPending}
                    />
                    <span className="text-xs text-muted-foreground">
                      {rule.enabled ? t("admin.forwarding.enabled") : t("admin.forwarding.disabled")}
                    </span>
                  </div>
                </TableCell>
                <TableCell>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" size="icon" aria-label={t("common.actions")}>
                        <MoreVertical className="h-4 w-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem className="gap-2" onSelect={() => handleEdit(rule)}>
                        <Pencil className="h-4 w-4" />
                        {t("common.edit")}
                      </DropdownMenuItem>
                      <DropdownMenuItem className="gap-2" onSelect={() => handleLogsOpen(rule)}>
                        <History className="h-4 w-4" />
                        {t("admin.forwarding.logsTitle")}
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        className="gap-2 text-red-600 focus:text-red-600"
                        onSelect={() => handleDeleteRequest(rule)}
                      >
                        <Trash2 className="h-4 w-4" />
                        {t("common.delete")}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      <Dialog open={isOpen} onOpenChange={handleDialogChange}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {editingRule ? t("admin.forwarding.editTitle") : t("admin.forwarding.addTitle")}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.forwarding.name")}</label>
              <Input
                placeholder={t("admin.forwarding.namePlaceholder")}
                value={formData.name}
                onChange={(event) => setFormData({ ...formData, name: event.target.value })}
                required
              />
            </div>
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.forwarding.protocol")}</label>
                <Select
                  value={formData.protocol}
                  onValueChange={(value) =>
                    setFormData({
                      ...formData,
                      protocol: value as CreateForwardingRuleRequest["protocol"],
                    })
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {protocolOptions.map((option) => (
                      <SelectItem key={option} value={option}>
                        {option.toUpperCase()}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.forwarding.listen")}</label>
                <Input
                  type="number"
                  placeholder="8080"
                  value={formData.listen_port ? String(formData.listen_port) : ""}
                  onChange={(event) =>
                    setFormData({
                      ...formData,
                      listen_port: parseInt(event.target.value, 10) || 0,
                    })
                  }
                  required
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.forwarding.targetAddress")}</label>
                <Input
                  placeholder="127.0.0.1"
                  value={formData.target_address}
                  onChange={(event) => setFormData({ ...formData, target_address: event.target.value })}
                  required
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.forwarding.targetPort")}</label>
                <Input
                  type="number"
                  placeholder="80"
                  value={formData.target_port ? String(formData.target_port) : ""}
                  onChange={(event) =>
                    setFormData({
                      ...formData,
                      target_port: parseInt(event.target.value, 10) || 0,
                    })
                  }
                  required
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.forwarding.priority")}</label>
                <Input
                  type="number"
                  placeholder="100"
                  value={formData.priority ? String(formData.priority) : ""}
                  onChange={(event) =>
                    setFormData({
                      ...formData,
                      priority: parseInt(event.target.value, 10) || 0,
                    })
                  }
                />
              </div>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.forwarding.remark")}</label>
              <Input
                placeholder={t("admin.forwarding.remarkPlaceholder")}
                value={formData.remark || ""}
                onChange={(event) => setFormData({ ...formData, remark: event.target.value })}
              />
            </div>
            <label className="flex items-center gap-2 text-sm">
              <Switch
                checked={formData.enabled}
                onCheckedChange={(value) => setFormData({ ...formData, enabled: value })}
              />
              {t("admin.forwarding.enabled")}
            </label>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => handleDialogChange(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={handleSubmit} disabled={createMutation.isPending || updateMutation.isPending}>
              {createMutation.isPending || updateMutation.isPending
                ? t("common.loading")
                : editingRule
                ? t("common.save")
                : t("common.create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={isLogsOpen} onOpenChange={handleLogsDialogChange}>
        <DialogContent className="sm:max-w-5xl">
          <DialogHeader>
            <DialogTitle>
              {t("admin.forwarding.logsTitle")} {logsTarget ? `- ${logsTarget.name}` : ""}
            </DialogTitle>
          </DialogHeader>
          <div className="flex flex-col gap-4 py-2">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.forwarding.logsStart")}</label>
                <div className="relative">
                  <Calendar className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    type="date"
                    className="pl-9"
                    value={logsDateRange.start}
                    onChange={(event) => handleLogsDateChange({ start: event.target.value })}
                  />
                </div>
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.forwarding.logsEnd")}</label>
                <div className="relative">
                  <Calendar className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    type="date"
                    className="pl-9"
                    value={logsDateRange.end}
                    onChange={(event) => handleLogsDateChange({ end: event.target.value })}
                  />
                </div>
              </div>
              <Button variant="outline" onClick={() => handleLogsDateChange({ start: "", end: "" })}>
                {t("admin.forwarding.logsClear")}
              </Button>
            </div>

            {logsQuery.isLoading ? (
              <Loading />
            ) : logsQuery.error ? (
              <div className="flex flex-col items-center justify-center gap-3 py-6">
                <p className="text-sm text-destructive">{t("admin.forwarding.logsLoadError")}</p>
                <Button variant="outline" onClick={() => logsQuery.refetch()}>
                  {t("common.retry")}
                </Button>
              </div>
            ) : logs.length === 0 ? (
              <EmptyState
                icon={<History className="h-full w-full" />}
                title={t("admin.forwarding.logsEmpty")}
                description={t("admin.forwarding.logsEmptyDescription")}
                size="sm"
              />
            ) : (
              <Table aria-label={t("admin.forwarding.logsTitle")}>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("admin.forwarding.logsTime")}</TableHead>
                    <TableHead>{t("admin.forwarding.logsAction")}</TableHead>
                    <TableHead>{t("admin.forwarding.logsOperator")}</TableHead>
                    <TableHead>{t("admin.forwarding.logsRule")}</TableHead>
                    <TableHead>{t("admin.forwarding.logsAgent")}</TableHead>
                    <TableHead>{t("admin.forwarding.logsDetail")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {logs.map((log) => (
                    <TableRow key={log.id}>
                      <TableCell className="whitespace-nowrap">
                        {formatLogTime(log.created_at)}
                      </TableCell>
                      <TableCell>
                        <Badge variant="secondary">{formatLogAction(log.action)}</Badge>
                      </TableCell>
                      <TableCell>{log.operator_id ?? "-"}</TableCell>
                      <TableCell>{log.rule_id ?? "-"}</TableCell>
                      <TableCell>{log.agent_host_id ?? "-"}</TableCell>
                      <TableCell>
                        <div className="max-w-xl truncate text-xs text-muted-foreground">
                          {log.detail || "-"}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}

            <Pagination page={logsPage} totalPages={logsTotalPages} onPageChange={setLogsPage} />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={handleLogsClose}>
              {t("common.close")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={isDeleteOpen} onOpenChange={handleDeleteDialogChange}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{t("admin.forwarding.deleteTitle")}</DialogTitle>
          </DialogHeader>
          <div className="py-2 text-sm text-muted-foreground">
            {t("admin.forwarding.deleteConfirm", { name: deleteTarget?.name })}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => handleDeleteDialogChange(false)}>
              {t("common.cancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={handleDeleteConfirm}
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending ? t("common.loading") : t("common.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
