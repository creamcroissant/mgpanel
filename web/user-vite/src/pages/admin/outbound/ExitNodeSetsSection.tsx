import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Plus, Pencil, Trash2 } from "lucide-react";
import { QUERY_KEYS } from "@/lib/constants";
import {
  listExitNodeSets,
  createExitNodeSet,
  updateExitNodeSet,
  deleteExitNodeSet,
  removeExitNodeSetMember,
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
  EmptyState,
  Input,
  Loading,
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
  AgentHost,
  ExitNodeSetDetail,
  CreateExitNodeSetRequest,
} from "@/types/admin";

/**
 * 出口节点集合管理（嵌入出站配置页面）。
 * 定义一组可作为出口的 agent 节点，支持权重与负载均衡策略。
 */
export function ExitNodeSetsSection() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [isOpen, setIsOpen] = useState(false);
  const [editing, setEditing] = useState<ExitNodeSetDetail | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<ExitNodeSetDetail | null>(null);
  const [form, setForm] = useState({
    name: "",
    description: "",
    tags: "",
    strategy: "round_robin",
    agent_host_ids: [] as number[],
  });

  const { data: sets, isLoading } = useQuery({
    queryKey: [QUERY_KEYS.ADMIN_EXIT_NODE_SETS],
    queryFn: listExitNodeSets,
  });

  const { data: agents } = useQuery({
    queryKey: [QUERY_KEYS.ADMIN_AGENTS],
    queryFn: () => getAgentHosts({ page: 1, page_size: 100 }),
  });

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: [QUERY_KEYS.ADMIN_EXIT_NODE_SETS] });
  };

  const createMut = useMutation({
    mutationFn: (req: CreateExitNodeSetRequest) => createExitNodeSet(req),
    onSuccess: () => {
      toast.success(t("admin.exitNodeSets.created"));
      invalidate();
      setIsOpen(false);
      setForm({ name: "", description: "", tags: "", strategy: "round_robin", agent_host_ids: [] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const updateMut = useMutation({
    mutationFn: ({ id, req }: { id: number; req: CreateExitNodeSetRequest }) => updateExitNodeSet(id, req),
    onSuccess: () => {
      toast.success(t("admin.exitNodeSets.updated"));
      invalidate();
      setIsOpen(false);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const deleteMut = useMutation({
    mutationFn: deleteExitNodeSet,
    onSuccess: () => {
      toast.success(t("admin.exitNodeSets.deleted"));
      invalidate();
      setDeleteTarget(null);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const removeMemberMut = useMutation({
    mutationFn: ({ id, agentHostId }: { id: number; agentHostId: number }) =>
      removeExitNodeSetMember(id, agentHostId),
    onSuccess: () => {
      toast.success(t("admin.exitNodeSets.memberRemoved"));
      invalidate();
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const handleSubmit = () => {
    const memberIds = form.agent_host_ids;
    const payload = {
      name: form.name,
      description: form.description,
      tags: form.tags.split(",").map((s) => s.trim()).filter(Boolean),
      strategy: form.strategy,
      members: memberIds.map((id) => ({ agent_host_id: id, weight: 1 })),
    };
    if (editing) {
      updateMut.mutate({ id: editing.set.id, req: payload });
    } else {
      createMut.mutate(payload);
    }
  };

  const openCreate = () => {
    setEditing(null);
    setForm({ name: "", description: "", tags: "", strategy: "round_robin", agent_host_ids: [] });
    setIsOpen(true);
  };

  const openEdit = (d: ExitNodeSetDetail) => {
    setEditing(d);
    setForm({
      name: d.set.name,
      description: d.set.description,
      tags: d.set.tags,
      strategy: d.set.strategy,
      agent_host_ids: d.members.map((m) => m.agent_host_id),
    });
    setIsOpen(true);
  };

  if (isLoading) return <Loading />;

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <Button onClick={openCreate}>
          <Plus className="mr-2 h-4 w-4" />
          {t("admin.exitNodeSets.create")}
        </Button>
      </div>

      {!sets || sets.length === 0 ? (
        <EmptyState title={t("admin.exitNodeSets.empty")} />
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("admin.exitNodeSets.name")}</TableHead>
              <TableHead>{t("admin.exitNodeSets.tags")}</TableHead>
              <TableHead>{t("admin.exitNodeSets.strategy")}</TableHead>
              <TableHead>{t("admin.exitNodeSets.members")}</TableHead>
              <TableHead>{t("admin.exitNodeSets.status")}</TableHead>
              <TableHead className="text-right">{t("common.actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sets.map((d) => (
              <TableRow key={d.set.id}>
                <TableCell className="font-medium">{d.set.name}</TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-1">
                    {d.set.tags.split(",").filter(Boolean).map((tag, i) => (
                      <Badge key={i} variant="secondary">{tag}</Badge>
                    ))}
                  </div>
                </TableCell>
                <TableCell>{d.set.strategy}</TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-1">
                    {d.members.map((m) => {
                      const host = d.host_name?.[m.agent_host_id] || `#${m.agent_host_id}`;
                      return (
                        <Badge key={m.id} variant="outline">
                          {host}
                          {m.weight > 1 ? ` x${m.weight}` : ""}
                          <button
                            className="ml-1 text-muted-foreground hover:text-destructive"
                            onClick={() => removeMemberMut.mutate({ id: d.set.id, agentHostId: m.agent_host_id })}
                            title={t("admin.exitNodeSets.removeMember")}
                          >
                            ×
                          </button>
                        </Badge>
                      );
                    })}
                  </div>
                </TableCell>
                <TableCell>
                  <Badge variant={d.set.enabled ? "success" : "secondary"}>
                    {d.set.enabled ? t("common.enabled") : t("common.disabled")}
                  </Badge>
                </TableCell>
                <TableCell className="text-right">
                  <div className="flex justify-end gap-2">
                    <Button variant="ghost" size="sm" onClick={() => openEdit(d)}>
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="sm" onClick={() => setDeleteTarget(d)}>
                      <Trash2 className="h-4 w-4 text-destructive" />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      <Dialog open={isOpen} onOpenChange={setIsOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editing ? t("admin.exitNodeSets.edit") : t("admin.exitNodeSets.create")}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm">{t("admin.exitNodeSets.name")}</label>
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="jp-unlock"
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm">{t("admin.exitNodeSets.description")}</label>
              <Input
                value={form.description}
                onChange={(e) => setForm({ ...form, description: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm">{t("admin.exitNodeSets.tags")}</label>
              <Input
                value={form.tags}
                onChange={(e) => setForm({ ...form, tags: e.target.value })}
                placeholder="region:jp,unlock:netflix"
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm">{t("admin.exitNodeSets.strategy")}</label>
              <Select
                value={form.strategy}
                onValueChange={(v) => setForm({ ...form, strategy: v })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {["round_robin", "least_ping", "random", "weighted_random"].map((s) => (
                    <SelectItem key={s} value={s}>{s}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <label className="text-sm">{t("admin.exitNodeSets.selectMembers")}</label>
              <div className="max-h-48 overflow-y-auto rounded border p-2">
                {agents?.data?.map((a: AgentHost) => (
                  <label key={a.id} className="flex items-center gap-2 py-1 text-sm">
                    <input
                      type="checkbox"
                      checked={form.agent_host_ids.includes(a.id)}
                      onChange={(e) => {
                        const ids = e.target.checked
                          ? [...form.agent_host_ids, a.id]
                          : form.agent_host_ids.filter((x) => x !== a.id);
                        setForm({ ...form, agent_host_ids: ids });
                      }}
                    />
                    {a.name || a.host}
                  </label>
                ))}
              </div>
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
            <DialogTitle>{t("admin.exitNodeSets.confirmDelete")}</DialogTitle>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(null)}>
              {t("common.cancel")}
            </Button>
            <Button variant="destructive" onClick={() => deleteTarget && deleteMut.mutate(deleteTarget.set.id)}>
              {t("common.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
