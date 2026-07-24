import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  createCoreConfigItem,
  deleteCoreConfigItem,
  listCoreConfigItems,
  updateCoreConfigItem,
} from "@/api/admin";
import { Badge, Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, EmptyState, Input, Loading, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, Switch, Table, TableBody, TableCell, TableHead, TableHeader, TableRow, Textarea } from "@/components/ui";
import { QUERY_KEYS } from "@/lib/constants";
import type { ConfigCenterCoreType, CoreConfigItem, CoreConfigType, UpsertCoreConfigItemRequest } from "@/types";
import { OutboundEditor } from "./OutboundEditor";
import { CoreSettingsEditor } from "./CoreSettingsEditor";
import { RoutingRuleEditor } from "./RoutingRuleEditor";
import { DnsConfigEditor } from "./DnsConfigEditor";

interface CoreConfigTabProps {
  configType: CoreConfigType;
  selectedHostId: number | null;

  selectedCoreType: ConfigCenterCoreType;
}

type FormState = {
  agent_host_id: number | null;
  core_type: ConfigCenterCoreType;
  config_type: CoreConfigType;
  tag: string;
  enabled: boolean;
  config_data: string;
  change_note: string;
  is_template: boolean;
};

function defaultFormState(configType: CoreConfigType): FormState {
  return {
    agent_host_id: 0,
    core_type: "sing-box",
    config_type: configType,
    tag: "",
    enabled: true,
    config_data: "{}",
    change_note: "",
    is_template: false,
  };
}

export function CoreConfigTab({ configType, selectedHostId, selectedCoreType }: CoreConfigTabProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [editingItem, setEditingItem] = useState<CoreConfigItem | null>(null);
  const [form, setForm] = useState<FormState>(defaultFormState(configType));
  const [deleteTarget, setDeleteTarget] = useState<CoreConfigItem | null>(null);
  const [isDeleteOpen, setIsDeleteOpen] = useState(false);

  const listQuery = useQuery({
    queryKey: [
      ...QUERY_KEYS.ADMIN_CONFIG_CENTER_CORE_CONFIGS,
      selectedHostId,
      selectedCoreType,
      configType,
    ],
    queryFn: () =>
      listCoreConfigItems({
        agent_host_id: selectedHostId ?? undefined,
        core_type: selectedCoreType,
        config_type: configType,
        limit: 100,
        offset: 0,
      }),
  });

  const items = listQuery.data?.data ?? [];

  const createMutation = useMutation({
    mutationFn: createCoreConfigItem,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_CONFIG_CENTER_CORE_CONFIGS });
      setIsDialogOpen(false);
      setEditingItem(null);
      toast.success(t("admin.configCenter.coreConfig.messages.saveSuccess"));
    },
    onError: (err: Error) => {
      toast.error(t("admin.configCenter.coreConfig.messages.saveFailed"), {
        description: err.message,
      });
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: UpsertCoreConfigItemRequest }) =>
      updateCoreConfigItem(id, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_CONFIG_CENTER_CORE_CONFIGS });
      setIsDialogOpen(false);
      setEditingItem(null);
      toast.success(t("admin.configCenter.coreConfig.messages.saveSuccess"));
    },
    onError: (err: Error) => {
      toast.error(t("admin.configCenter.coreConfig.messages.saveFailed"), {
        description: err.message,
      });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteCoreConfigItem,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_CONFIG_CENTER_CORE_CONFIGS });
      setIsDeleteOpen(false);
      setDeleteTarget(null);
      toast.success(t("admin.configCenter.coreConfig.messages.deleteSuccess"));
    },
    onError: (err: Error) => {
      toast.error(t("admin.configCenter.coreConfig.messages.deleteFailed"), {
        description: err.message,
      });
    },
  });

  const openCreate = () => {
    setEditingItem(null);
    setForm({
      ...defaultFormState(configType),
      agent_host_id: selectedHostId,
      core_type: selectedCoreType,
    });
    setIsDialogOpen(true);
  };

  const openEdit = (item: CoreConfigItem) => {
    setEditingItem(item);
    setForm({
      agent_host_id: item.agent_host_id,
      core_type: (item.core_type || selectedCoreType) as ConfigCenterCoreType,
      config_type: item.config_type as CoreConfigType,
      tag: item.tag,
      enabled: item.enabled,
      config_data: JSON.stringify(item.config_data, null, 2),
      change_note: "",
      is_template: item.agent_host_id === null,
    });
    setIsDialogOpen(true);
  };

  const handleSave = () => {
    if (!form.tag.trim()) {
      toast.warning(t("admin.configCenter.coreConfig.messages.requiredFields"));
      return;
    }

    let parsedJson: Record<string, unknown>;
    try {
      parsedJson = JSON.parse(form.config_data) as Record<string, unknown>;
    } catch {
      toast.error(t("admin.configCenter.messages.invalidJson"));
      return;
    }

    // Ensure outbound/routing config has tag field synced
    if (configType === "outbound" || configType === "routing") {
      parsedJson.tag = form.tag.trim();
    }

    const payloadBase = {
      core_type: form.core_type,
      config_type: form.config_type,
      tag: form.tag.trim(),
      enabled: form.enabled,
      config_data: parsedJson,
      change_note: form.change_note.trim() || undefined,
    };
    const payload: UpsertCoreConfigItemRequest = form.is_template
      ? payloadBase
      : { ...payloadBase, agent_host_id: form.agent_host_id ?? selectedHostId ?? undefined };

    if (editingItem) {
      updateMutation.mutate({ id: editingItem.id, payload });
    } else {
      createMutation.mutate(payload);
    }
  };

  // TODO: define proper typing for dynamic translation keys
  const configTypeLabel = t(`admin.configCenter.configTypes.${configType}` as `admin.configCenter.configTypes.${CoreConfigType}`);

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle>{configTypeLabel}</CardTitle>
            <CardDescription>
              {t("admin.configCenter.coreConfig.description")}
            </CardDescription>
          </div>
          <Button onClick={openCreate} disabled={!selectedHostId && !form.is_template}>
            {t("admin.configCenter.coreConfig.actions.create")}
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {listQuery.isLoading ? (
          <Loading />
        ) : listQuery.error ? (
          <div className="flex flex-col items-center gap-3 py-10">
            <p className="text-sm text-destructive">{t("admin.configCenter.coreConfig.messages.loadFailed")}</p>
            <Button variant="outline" onClick={() => listQuery.refetch()}>{t("common.retry")}</Button>
          </div>
        ) : items.length === 0 ? (
          <EmptyState
            title={t("admin.configCenter.empty.noSpecTitle")}
            description={`No ${configType} config items yet`}
            action={<Button onClick={openCreate}>{t("admin.configCenter.coreConfig.actions.create")}</Button>}
            size="sm"
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("admin.configCenter.coreConfig.fields.tag")}</TableHead>
                <TableHead className="hidden sm:table-cell">{t("admin.configCenter.fields.coreType")}</TableHead>
                <TableHead>{t("admin.configCenter.fields.enabled")}</TableHead>
                <TableHead className="hidden lg:table-cell">{t("admin.configCenter.fields.revision")}</TableHead>
                <TableHead>{t("common.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-medium">{item.tag}</TableCell>
                  <TableCell className="hidden sm:table-cell">
                    <Badge variant="secondary">{item.core_type}</Badge>
                  </TableCell>
                  <TableCell>
                    <Badge variant={item.enabled ? "success" : "secondary"}>
                      {item.enabled ? t("admin.configCenter.status.enabled") : t("admin.configCenter.status.disabled")}
                    </Badge>
                  </TableCell>
                  <TableCell className="hidden lg:table-cell">{item.desired_revision}</TableCell>
                  <TableCell>
                    <div className="flex gap-2">
                      <Button size="sm" variant="outline" onClick={() => openEdit(item)}>
                        {t("admin.configCenter.coreConfig.actions.edit")}
                      </Button>
                      <Button size="sm" variant="destructive" onClick={() => { setDeleteTarget(item); setIsDeleteOpen(true); }}>
                        {t("admin.configCenter.coreConfig.actions.delete")}
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>

      <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
        <DialogContent className="sm:max-w-3xl">
          <DialogHeader>
            <DialogTitle>
              {editingItem
                ? `${t("admin.configCenter.coreConfig.actions.edit")} - ${editingItem.tag}`
                : t("admin.configCenter.coreConfig.actions.create")}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.configCenter.fields.coreType")}</label>
                <Select
                  value={form.core_type}
                  onValueChange={(v) => setForm((prev) => ({ ...prev, core_type: v as ConfigCenterCoreType }))}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="sing-box">sing-box</SelectItem>
                    <SelectItem value="xray">xray</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.configCenter.coreConfig.fields.tag")}</label>
                <Input
                  value={form.tag}
                  onChange={(e) => setForm((prev) => ({ ...prev, tag: e.target.value }))}
                  placeholder={t("admin.configCenter.coreConfig.placeholders.tag")}
                />
              </div>
            </div>

            <label className="flex items-center gap-2 text-sm">
              <Switch
                checked={form.enabled}
                onCheckedChange={(c) => setForm((prev) => ({ ...prev, enabled: c }))}
              />
              {t("admin.configCenter.fields.enabled")}
            </label>

            {editingItem === null && (
              <label className="flex items-center gap-2 text-sm">
                <Switch
                  checked={form.is_template}
                  onCheckedChange={(c) =>
                    setForm((prev) => ({ ...prev, is_template: c, agent_host_id: c ? null : prev.agent_host_id || 0 }))
                  }
                />
                {t("admin.configCenter.template.isTemplate")}
              </label>
            )}

            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.coreConfig.fields.configData")}</label>
              {configType === "outbound" ? (
                <OutboundEditor
                  value={form.config_data}
                  onChange={(v) => setForm((prev) => ({ ...prev, config_data: v }))}
                  coreType={form.core_type}
                />
              ) : configType === "core_settings" ? (
                <CoreSettingsEditor
                  value={form.config_data}
                  onChange={(v) => setForm((prev) => ({ ...prev, config_data: v }))}
                />
              ) : configType === "routing" ? (
                <RoutingRuleEditor
                  value={form.config_data}
                  onChange={(v) => setForm((prev) => ({ ...prev, config_data: v }))}
                />
              ) : configType === "dns" ? (
                <DnsConfigEditor
                  value={form.config_data}
                  onChange={(v) => setForm((prev) => ({ ...prev, config_data: v }))}
                />
              ) : (
                <Textarea
                  className="min-h-[200px] font-mono text-xs"
                  value={form.config_data}
                  onChange={(e) => setForm((prev) => ({ ...prev, config_data: e.target.value }))}
                  placeholder={t("admin.configCenter.coreConfig.placeholders.json")}
                />
              )}
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.coreConfig.fields.changeNote")}</label>
              <Input
                value={form.change_note}
                onChange={(e) => setForm((prev) => ({ ...prev, change_note: e.target.value }))}
                placeholder={t("admin.configCenter.placeholders.optional")}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsDialogOpen(false)}>{t("common.cancel")}</Button>
            <Button onClick={handleSave} disabled={createMutation.isPending || updateMutation.isPending}>
              {createMutation.isPending || updateMutation.isPending ? t("common.loading") : t("common.save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={isDeleteOpen} onOpenChange={setIsDeleteOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("admin.configCenter.specs.deleteTitle")}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            {t("admin.configCenter.coreConfig.deleteConfirmation", { tag: deleteTarget?.tag ?? "" })}
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsDeleteOpen(false)}>{t("common.cancel")}</Button>
            <Button variant="destructive" onClick={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}>
              {t("common.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
