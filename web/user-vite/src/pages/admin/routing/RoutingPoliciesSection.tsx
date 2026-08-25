import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Plus, Pencil, Trash2, RouteOff } from "lucide-react";
import { QUERY_KEYS } from "@/lib/constants";
import {
  listExitNodeSets,
  listRoutingPolicies,
  createRoutingPolicy,
  updateRoutingPolicy,
  deleteRoutingPolicy,
} from "@/api/admin";
import { listConfigCenterSpecs } from "@/api/admin/configCenter";
import {
  Badge,
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  Input,
  Loading,
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
  RoutingPolicy,
  CreateRoutingPolicyRequest,
} from "@/types/admin";

/**
 * 流媒体/域名分流策略管理（嵌入路由规则页面）。
 * 定义 geosite / domain / ip_cidr 匹配 → 出口集合的路由策略。
 */
export function RoutingPoliciesSection() {
  const { t } = useTranslation();
  // 枚举展示标签（值本身是 API 协议常量，不翻译；仅展示层映射）
  const matchTypeLabels: Record<string, string> = {
    geosite: t("admin.routingPolicies.matchTypes.geosite"),
    domain: t("admin.routingPolicies.matchTypes.domain"),
    ip_cidr: t("admin.routingPolicies.matchTypes.ip_cidr"),
  };
  const queryClient = useQueryClient();
  const [isOpen, setIsOpen] = useState(false);
  const [editing, setEditing] = useState<RoutingPolicy | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<RoutingPolicy | null>(null);
  const [form, setForm] = useState<CreateRoutingPolicyRequest>({
    name: "",
    core_type: "sing-box",
    priority: 0,
    match_type: "geosite",
    match_value: "",
    action: "route_to_set",
    target_set_id: 0,
    spec_id: null,
    enabled: true,
  });

  const { data: policies, isLoading } = useQuery({
    queryKey: [QUERY_KEYS.ADMIN_ROUTING_POLICIES],
    queryFn: () => listRoutingPolicies(),
  });

  const { data: sets } = useQuery({
    queryKey: [QUERY_KEYS.ADMIN_EXIT_NODE_SETS],
    queryFn: listExitNodeSets,
  });

  // 入站 spec 列表（作用域下拉用；仅启用项）
  const { data: specsData } = useQuery({
    queryKey: [QUERY_KEYS.ADMIN_CONFIG_CENTER_SPECS],
    queryFn: () => listConfigCenterSpecs({ enabled: true }),
  });
  const specs = specsData?.data ?? [];
  const specTagById = new Map(specs.map((s) => [s.id, s.tag] as const));

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: [QUERY_KEYS.ADMIN_ROUTING_POLICIES] });
  };

  const createMut = useMutation({
    mutationFn: createRoutingPolicy,
    onSuccess: () => {
      toast.success(t("admin.routingPolicies.created"));
      invalidate();
      setIsOpen(false);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const updateMut = useMutation({
    mutationFn: ({ id, req }: { id: number; req: CreateRoutingPolicyRequest }) => updateRoutingPolicy(id, req),
    onSuccess: () => {
      toast.success(t("admin.routingPolicies.updated"));
      invalidate();
      setIsOpen(false);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const deleteMut = useMutation({
    mutationFn: deleteRoutingPolicy,
    onSuccess: () => {
      toast.success(t("admin.routingPolicies.deleted"));
      invalidate();
      setDeleteTarget(null);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const handleSubmit = () => {
    const payload = {
      ...form,
      target_set_id: form.target_set_id || undefined,
      spec_id: form.spec_id ?? null,
    };
    if (editing) {
      updateMut.mutate({ id: editing.id, req: payload });
    } else {
      createMut.mutate(payload);
    }
  };

  const openCreate = () => {
    setEditing(null);
    setForm({
      name: "", core_type: "sing-box", priority: 0, match_type: "geosite",
      match_value: "", action: "route_to_set", target_set_id: sets?.[0]?.set.id ?? 0,
      spec_id: null, enabled: true,
    });
    setIsOpen(true);
  };

  const openEdit = (p: RoutingPolicy) => {
    setEditing(p);
    setForm({
      name: p.name, core_type: p.core_type, priority: p.priority, match_type: p.match_type,
      match_value: p.match_value, action: p.action, target_set_id: p.target_set_id ?? 0,
      spec_id: p.spec_id ?? null, enabled: p.enabled,
    });
    setIsOpen(true);
  };

  if (isLoading) return <Loading />;

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <Button onClick={openCreate}>
          <Plus className="mr-2 h-4 w-4" />
          {t("admin.routingPolicies.create")}
        </Button>
      </div>

      {!policies || policies.length === 0 ? (
        <EmptyState
          icon={<RouteOff className="h-6 w-6" />}
          title={t("admin.routingPolicies.empty")}
          description={t("admin.routingPolicies.emptyDescription")}
          action={
            <Button onClick={openCreate}>
              <Plus className="mr-2 h-4 w-4" />
              {t("admin.routingPolicies.create")}
            </Button>
          }
        />
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("admin.routingPolicies.name")}</TableHead>
              <TableHead>{t("admin.routingPolicies.match")}</TableHead>
              <TableHead>{t("admin.routing.scope.label")}</TableHead>
              <TableHead>{t("admin.routingPolicies.target")}</TableHead>
              <TableHead className="text-right">{t("admin.routingPolicies.priority")}</TableHead>
              <TableHead>{t("admin.routingPolicies.status")}</TableHead>
              <TableHead className="text-right">{t("common.actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {policies.map((p) => {
              const target = sets?.find((s) => s.set.id === p.target_set_id);
              return (
                <TableRow key={p.id}>
                  <TableCell className="font-medium">{p.name}</TableCell>
                  <TableCell>
                    <Badge variant="secondary">{matchTypeLabels[p.match_type] ?? p.match_type}</Badge>
                    <span className="ml-2">{p.match_value}</span>
                  </TableCell>
                  <TableCell>
                    {p.spec_id ? (
                      <Badge
                        variant="outline"
                        className="border-warning/60 text-warning-foreground dark:text-warning"
                        title={t("admin.routing.scope.specTooltip", { tag: specTagById.get(p.spec_id) ?? `#${p.spec_id}` })}
                      >
                        🔒 {specTagById.get(p.spec_id) ?? `#${p.spec_id}`}
                      </Badge>
                    ) : (
                      <Badge variant="outline">{t("admin.routing.scope.global")}</Badge>
                    )}
                  </TableCell>
                  <TableCell>{target?.set.name || `#${p.target_set_id ?? "-"}`}</TableCell>
                  <TableCell className="text-right tabular-nums">{p.priority}</TableCell>
                  <TableCell>
                    <Badge variant={p.enabled ? "success" : "secondary"}>
                      {p.enabled ? t("common.enabled") : t("common.disabled")}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-2">
                      <Button variant="ghost" size="sm" onClick={() => openEdit(p)}>
                        <Pencil className="h-4 w-4" />
                      </Button>
                      <Button variant="ghost" size="sm" onClick={() => setDeleteTarget(p)}>
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      )}

      <Dialog open={isOpen} onOpenChange={setIsOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editing ? t("admin.routingPolicies.edit") : t("admin.routingPolicies.create")}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm">{t("admin.routingPolicies.name")}</label>
              <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
            </div>
            <div className="space-y-2">
              <label className="text-sm">{t("admin.routingPolicies.matchType")}</label>
              <Select value={form.match_type} onValueChange={(v) => setForm({ ...form, match_type: v })}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {["geosite", "domain", "ip_cidr"].map((s) => (
                    <SelectItem key={s} value={s}>{matchTypeLabels[s] ?? s}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <label className="text-sm">{t("admin.routingPolicies.matchValue")}</label>
              <Input
                value={form.match_value}
                onChange={(e) => setForm({ ...form, match_value: e.target.value })}
                placeholder="netflix"
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm">{t("admin.routingPolicies.targetSet")}</label>
              <Select
                value={String(form.target_set_id)}
                onValueChange={(v) => setForm({ ...form, target_set_id: Number(v) })}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {sets?.map((s) => (
                    <SelectItem key={s.set.id} value={String(s.set.id)}>{s.set.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <label className="text-sm">{t("admin.routing.scope.label")}</label>
              <Select
                value={form.spec_id ? String(form.spec_id) : "global"}
                onValueChange={(v) =>
                  setForm({ ...form, spec_id: v === "global" ? null : Number(v) })
                }
              >
                <SelectTrigger>
                  <SelectValue placeholder={t("admin.routing.scope.label")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="global">{t("admin.routing.scope.global")}</SelectItem>
                  {specs.map((s) => (
                    <SelectItem key={s.id} value={String(s.id)}>
                      🔒 {s.tag}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">
                {t("admin.routing.scope.hint")}
              </p>
            </div>
            <div className="space-y-2">
              <label className="text-sm">{t("admin.routingPolicies.priority")}</label>
              <Input
                type="number"
                value={form.priority}
                onChange={(e) => setForm({ ...form, priority: Number(e.target.value) })}
              />
            </div>
            <div className="flex items-center gap-2">
              <Switch
                checked={!!form.enabled}
                onCheckedChange={(v) => setForm({ ...form, enabled: v })}
              />
              <span className="text-sm">{t("common.enabled")}</span>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={handleSubmit}>{t("common.save")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("admin.routingPolicies.confirmDelete")}</DialogTitle>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(null)}>
              {t("common.cancel")}
            </Button>
            <Button variant="destructive" onClick={() => deleteTarget && deleteMut.mutate(deleteTarget.id)}>
              {t("common.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}