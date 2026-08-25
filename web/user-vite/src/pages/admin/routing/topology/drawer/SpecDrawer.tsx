import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link2, Server, Layers } from "lucide-react";
import type { TopologyAgent, TopologyExitSet, TopologySpec, RelayPathInfo } from "@/lib/topology/types";

/**
 * 入站 spec 出口绑定 Drawer：固定 Agent / 出口集 / 中继链路 三选一。
 * 切换模式即互斥清空另两项（保存时未选项显式 null，后端清除旧绑定）。
 */

export type SpecExitMode = "agent" | "set" | "relay";

export interface SpecExitBindingValues {
  mode: SpecExitMode;
  agentId: number | null;
  setId: number | null;
  pathId: number | null;
}

interface SpecDrawerProps {
  spec: TopologySpec;
  agents: TopologyAgent[];
  exitSets: TopologyExitSet[];
  relayPaths: RelayPathInfo[];
  saving?: boolean;
  onClose: () => void;
  onSave: (v: SpecExitBindingValues & { id: number }) => void;
}

function useT() {
  const { t } = useTranslation();
  return (key: string, fallback: string) => t(key, fallback) as string;
}

const MODES: { key: SpecExitMode; icon: typeof Server; labelKey: string; fallback: string }[] = [
  { key: "agent", icon: Server, labelKey: "admin.topology.relay.bind_fixed_agent", fallback: "固定 Agent" },
  { key: "set", icon: Layers, labelKey: "admin.topology.relay.bind_exit_set", fallback: "出口集" },
  { key: "relay", icon: Link2, labelKey: "admin.topology.relay.bind_relay", fallback: "中继链路" },
];

export function SpecDrawer({ spec, agents, exitSets, relayPaths, saving, onClose, onSave }: SpecDrawerProps) {
  const tf = useT();
  const initialMode: SpecExitMode =
    spec.relay_path_id != null && spec.relay_path_id > 0
      ? "relay"
      : spec.exit_node_set_id != null && spec.exit_node_set_id > 0
        ? "set"
        : "agent";
  const [mode, setMode] = useState<SpecExitMode>(initialMode);
  const [agentId, setAgentId] = useState<number | null>(
    mode === "agent" ? (spec.exit_agent_host_id ?? null) : null
  );
  const [setId, setSetId] = useState<number | null>(
    mode === "set" ? (spec.exit_node_set_id ?? null) : null
  );
  const [pathId, setPathId] = useState<number | null>(
    mode === "relay" ? (spec.relay_path_id ?? null) : null
  );

  const enabledPaths = useMemo(() => relayPaths.filter((p) => p.enabled), [relayPaths]);

  const switchMode = (m: SpecExitMode) => {
    setMode(m);
    // 三选一互斥：切换即清空另两个字段
    setAgentId(m === "agent" ? (spec.exit_agent_host_id ?? null) : null);
    setSetId(m === "set" ? (spec.exit_node_set_id ?? null) : null);
    setPathId(m === "relay" ? (spec.relay_path_id ?? null) : null);
  };

  const canSave =
    (mode === "agent" && agentId != null && agentId > 0) ||
    (mode === "set" && setId != null && setId > 0) ||
    (mode === "relay" && pathId != null && pathId > 0);

  return (
    <div className="space-y-3 p-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold">{tf("admin.topology.relay.bind_title", "出口绑定")}</h3>
        <button type="button" onClick={onClose} className="text-xs text-muted-foreground hover:text-foreground">
          ✕
        </button>
      </div>

      {/* spec 摘要 */}
      <p className="rounded-md bg-muted/50 px-2 py-1.5 font-mono text-xs text-muted-foreground">
        {spec.tag} · {spec.protocol}:{spec.port}
      </p>

      {/* 三选一 */}
      <div className="grid grid-cols-3 gap-1.5">
        {MODES.map(({ key, icon: Icon, labelKey, fallback }) => (
          <button
            key={key}
            type="button"
            onClick={() => switchMode(key)}
            className={`flex flex-col items-center gap-1 rounded-md border px-1 py-2 text-[11px] transition-colors ${
              mode === key
                ? "border-primary bg-primary/10 font-medium text-primary"
                : "border-border text-muted-foreground hover:bg-muted/50"
            }`}
            aria-pressed={mode === key}
          >
            <Icon className="h-4 w-4" aria-hidden />
            {tf(labelKey, fallback)}
          </button>
        ))}
      </div>

      {/* 对应下拉 */}
      {mode === "agent" && (
        <select
          className="w-full rounded-md border bg-background px-2 py-1.5 text-sm"
          value={agentId ?? ""}
          onChange={(e) => setAgentId(e.target.value ? Number(e.target.value) : null)}
          aria-label={tf("admin.topology.relay.bind_fixed_agent", "固定 Agent")}
        >
          <option value="">{tf("admin.topology.relay.bind_placeholder", "选择…")}</option>
          {agents.map((a) => (
            <option key={a.id} value={a.id}>
              {a.name} ({a.host})
            </option>
          ))}
        </select>
      )}
      {mode === "set" && (
        <select
          className="w-full rounded-md border bg-background px-2 py-1.5 text-sm"
          value={setId ?? ""}
          onChange={(e) => setSetId(e.target.value ? Number(e.target.value) : null)}
          aria-label={tf("admin.topology.relay.bind_exit_set", "出口集")}
        >
          <option value="">{tf("admin.topology.relay.bind_placeholder", "选择…")}</option>
          {exitSets.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
              {s.enabled ? "" : "（禁用）"}
            </option>
          ))}
        </select>
      )}
      {mode === "relay" && (
        <select
          className="w-full rounded-md border bg-background px-2 py-1.5 text-sm"
          value={pathId ?? ""}
          onChange={(e) => setPathId(e.target.value ? Number(e.target.value) : null)}
          aria-label={tf("admin.topology.relay.bind_relay", "中继链路")}
        >
          <option value="">{tf("admin.topology.relay.bind_placeholder", "选择…")}</option>
          {enabledPaths.map((p) => (
            <option key={p.id} value={p.id}>
              ⛓ {p.name}（{p.nodes.length} 跳）
            </option>
          ))}
          {enabledPaths.length === 0 && (
            <option value="" disabled>
              {tf("admin.topology.relay.bind_no_paths", "暂无启用的中继链路")}
            </option>
          )}
        </select>
      )}

      <button
        type="button"
        disabled={!canSave || saving}
        onClick={() =>
          onSave({
            id: spec.id,
            mode,
            agentId: mode === "agent" ? agentId : null,
            setId: mode === "set" ? setId : null,
            pathId: mode === "relay" ? pathId : null,
          })
        }
        className="w-full rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground disabled:opacity-50"
      >
        {saving ? tf("admin.topology.toolbar.exporting", "保存中…") : tf("admin.topology.relay.bind_save", "保存绑定")}
      </button>
    </div>
  );
}
