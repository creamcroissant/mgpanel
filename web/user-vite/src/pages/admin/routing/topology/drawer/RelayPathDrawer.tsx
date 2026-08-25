import { useTranslation } from "react-i18next";
import { ArrowUp, ArrowDown, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { TopologyAgent } from "@/lib/topology/types";
import type { RelayPathFormValues } from "../mutations";

export interface RelayDrawerTarget {
  /** null = 新建（新建走连线或 Palette，均预填两节点后进入本抽屉） */
  path: RelayPathFormValues | null;
}

interface RelayPathDrawerProps {
  target: RelayPathFormValues | null;
  agents: TopologyAgent[];
  saving: boolean;
  onClose: () => void;
  /** 草稿受控回调：编辑过程实时回传父层 */
  onDraftChange: (v: RelayPathFormValues) => void;
  onSave: (v: RelayPathFormValues) => void;
  onDelete?: (id: number) => void;
}

/**
 * 中继链路编辑抽屉：名称/描述/启停 + 有序节点序列编辑
 * （上移/下移改 sequence、删行、下拉加行）。sequence 由数组下标推导。
 */
export function RelayPathDrawer({ target, agents, saving, onClose, onDraftChange, onSave, onDelete }: RelayPathDrawerProps) {
  const { t } = useTranslation();
  const agentById = new Map(agents.map((a) => [a.id, a]));
  if (!target) return null;

  const usedIds = new Set(target.nodes.map((n) => n.agent_host_id));
  const available = agents.filter((a) => !usedIds.has(a.id));

  const setNodes = (nodes: RelayPathFormValues["nodes"]) =>
    onDraftChange({ ...target, nodes: nodes.map((n, i) => ({ ...n, sequence: i })) });

  const move = (idx: number, dir: -1 | 1) => {
    const nodes = [...target.nodes];
    const j = idx + dir;
    if (j < 0 || j >= nodes.length) return;
    [nodes[idx], nodes[j]] = [nodes[j], nodes[idx]];
    setNodes(nodes);
  };

  return (
    <div className="flex h-full flex-col">
      <div className="border-b px-4 py-3">
        <h3 className="text-sm font-semibold">{target.id < 0 ? t("admin.topology.relay.create") : t("admin.topology.relay.edit")}</h3>
      </div>
      <div className="flex-1 space-y-4 overflow-y-auto px-4 py-3">
        <div className="space-y-1.5">
          <label className="text-xs text-muted-foreground">{t("admin.topology.relay.name")}</label>
          <Input
            value={target.name}
            onChange={(e) => onChange({ ...target, name: e.target.value })}
            placeholder={t("admin.topology.relay.namePlaceholder")}
          />
        </div>
        <div className="space-y-1.5">
          <label className="text-xs text-muted-foreground">{t("admin.topology.relay.description")}</label>
          <Input
            value={target.description}
            onChange={(e) => onChange({ ...target, description: e.target.value })}
          />
        </div>
        <div className="flex items-center justify-between">
          <label className="text-xs text-muted-foreground">{t("admin.topology.relay.enabled")}</label>
          <Switch checked={target.enabled} onCheckedChange={(v) => onChange({ ...target, enabled: v })} />
        </div>

        <div className="space-y-1.5">
          <label className="text-xs text-muted-foreground">{t("admin.topology.relay.hops")}</label>
          <ol className="space-y-1.5">
            {target.nodes.map((n, i) => {
              const a = agentById.get(n.agent_host_id);
              return (
                <li key={`${n.agent_host_id}-${i}`} className="flex items-center gap-1.5 rounded-md border bg-card px-2 py-1.5 text-xs">
                  <span className="w-10 shrink-0 font-medium tabular-nums text-muted-foreground">
                    {i === 0 ? t("admin.topology.relay.entry") : i === target.nodes.length - 1 ? t("admin.topology.relay.exitLabel") : `#${i}`}
                  </span>
                  <span className="min-w-0 flex-1 truncate">
                    {a ? `${a.name} (${a.host})` : `agent#${n.agent_host_id}`}
                    {a && !a.online && <span className="ml-1 text-destructive">●</span>}
                  </span>
                  <Button variant="ghost" size="icon" className="h-6 w-6" disabled={i === 0} onClick={() => move(i, -1)} aria-label="up">
                    <ArrowUp className="h-3.5 w-3.5" />
                  </Button>
                  <Button variant="ghost" size="icon" className="h-6 w-6" disabled={i === target.nodes.length - 1} onClick={() => move(i, 1)} aria-label="down">
                    <ArrowDown className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-6 w-6 text-destructive"
                    disabled={target.nodes.length <= 2}
                    onClick={() => setNodes(target.nodes.filter((_, j) => j !== i))}
                    aria-label="remove"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </li>
              );
            })}
          </ol>
          {available.length > 0 && (
            <Select
              onValueChange={(v) => {
                const id = Number(v);
                if (!usedIds.has(id)) setNodes([...target.nodes, { sequence: target.nodes.length, agent_host_id: id }]);
              }}
            >
              <SelectTrigger className="h-8 text-xs">
                <SelectValue placeholder={t("admin.topology.relay.addHop")} />
              </SelectTrigger>
              <SelectContent>
                {available.map((a) => (
                  <SelectItem key={a.id} value={String(a.id)}>
                    {a.name} ({a.host}){a.online ? "" : " · offline"}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>
      </div>
      <div className="flex items-center justify-between gap-2 border-t px-4 py-3">
        {target.id > 0 && onDelete ? (
          <Button variant="ghost" size="sm" className="text-destructive" disabled={saving} onClick={() => onDelete(target.id)}>
            <Trash2 className="mr-1 h-3.5 w-3.5" />{t("common.delete")}
          </Button>
        ) : (
          <span />
        )}
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={onClose}>{t("common.cancel")}</Button>
          <Button size="sm" disabled={saving || !target.name.trim() || target.nodes.length < 2} onClick={() => onSave(target)}>
            {t("common.save")}
          </Button>
        </div>
      </div>
    </div>
  );
}

