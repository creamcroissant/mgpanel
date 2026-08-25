import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import {
  Button,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Switch,
  Textarea,
} from "@/components/ui";
import type { TopologyAgent, TopologyExitSet, TopologyPolicy, TopologySpec, RelayPathInfo } from "@/lib/topology/types";
import {
  RULE_FIELDS,
  SET_FIELDS,
  flattenZodErrors,
  ruleFormSchema,
  setFormSchema,
  type FieldSpec,
  type MemberRow,
  type RuleFormValues,
  type SetFormValues,
} from "./schema";

/**
 * 拓扑画布右侧滑出编辑面板。
 * 按 kind(rule|set) 渲染 schema 驱动的表单；保存回调由父层接 mutations（f4 接线）。
 * 本组件不直接调用 API，保持与 mutations 解耦便于单测与复用。
 */

export interface DrawerTarget {
  kind: "rule" | "set" | "spec";
  /** null = 新建 */
  policy?: TopologyPolicy | null;
  set?: TopologyExitSet | null;
  spec?: TopologySpec | null;
}

interface DrawerPanelProps {
  target: DrawerTarget | null;
  agents: TopologyAgent[];
  exitSets?: TopologyExitSet[];
  relayPaths?: RelayPathInfo[];
  saving?: boolean;
  onClose: () => void;
  onSaveRule: (v: RuleFormValues & { id: number | null }) => void;
  onSaveSet: (v: SetFormValues & { id: number | null }) => void;
  onSaveSpecBinding?: (v: SpecExitBindingValues & { id: number }) => void;
  onDelete?: (kind: "rule" | "set", id: number) => void;
}

function useT() {
  const { t } = useTranslation();
  return (key: string, fallback: string) => t(key, fallback) as string;
}

/** 脏检查：序列化对比初始值与当前值 */
function useDirty<T>(initial: string, current: T) {
  return useMemo(() => initial !== JSON.stringify(current), [initial, current]);
}

export function DrawerPanel({ target, agents, exitSets = [], relayPaths = [], saving = false, onClose, onSaveRule, onSaveSet, onSaveSpecBinding, onDelete }: DrawerPanelProps) {
  const tf = useT();
  const open = target != null;
  // 挂载延迟以触发 CSS 过渡：先渲染容器再位移
  const [shown, setShown] = useState(false);
  useEffect(() => {
    if (open) requestAnimationFrame(() => setShown(true));
    else setShown(false);
  }, [open]);

  if (!target) return null;

  return (
    <aside
      aria-label={tf("admin.topology.drawer.title", "属性编辑")}
      className={`fixed right-0 top-0 z-40 flex h-full w-[400px] max-w-full flex-col border-l border-border bg-card shadow-lg transition-transform duration-200 ${shown ? "translate-x-0" : "translate-x-full"}`}
    >
      <DrawerBody key={`${target.kind}-${target.policy?.id ?? target.set?.id ?? target.spec?.id ?? "new"}`} target={target} agents={agents} exitSets={exitSets} relayPaths={relayPaths} saving={saving} onClose={onClose} onSaveRule={onSaveRule} onSaveSet={onSaveSet} onSaveSpecBinding={onSaveSpecBinding} onDelete={onDelete} />
    </aside>
  );
}

function DrawerBody({ target, agents, exitSets, relayPaths, saving, onClose, onSaveRule, onSaveSet, onSaveSpecBinding, onDelete }: Omit<DrawerPanelProps, "target"> & { target: DrawerTarget }) {
  const tf = useT();
  return (
    <>
      <header className="flex items-center justify-between border-b border-border px-4 py-3">
        <h3 className="text-sm font-medium">
          {target.kind === "rule"
            ? `${tf("admin.topology.drawer.editRule", "编辑规则")}${target.policy ? ` #${target.policy.id}` : ""}`
            : target.kind === "spec"
              ? `${tf("admin.topology.relay.bind_title", "出口绑定")}${target.spec ? ` #${target.spec.id}` : ""}`
              : `${tf("admin.topology.drawer.editSet", "编辑出口集")}${target.set ? ` #${target.set.id}` : ""}`}
        </h3>
        <Button variant="ghost" size="sm" onClick={onClose} aria-label="close">
          ✕
        </Button>
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4">
        {target.kind === "rule" ? (
          <RuleForm policy={target.policy ?? null} saving={saving} onSave={onSaveRule} onClose={onClose} />
        ) : target.kind === "spec" && target.spec && onSaveSpecBinding ? (
          <SpecDrawer
            spec={target.spec}
            agents={agents}
            exitSets={exitSets}
            relayPaths={relayPaths}
            saving={saving}
            onClose={onClose}
            onSave={onSaveSpecBinding}
          />
        ) : (
          <SetForm set={target.set ?? null} agents={agents} saving={saving} onSave={onSaveSet} onClose={onClose} />
        )}
      </div>
      {target.kind === "rule" && target.policy
        ? onDelete && (
            <footer className="border-t border-border px-4 py-3">
              <Button variant="destructive" size="sm" disabled={saving} onClick={() => onDelete("rule", target.policy!.id)}>
                {tf("admin.topology.common.delete", "删除")}
              </Button>
            </footer>
          )
        : target.set && onDelete && (
            <footer className="border-t border-border px-4 py-3">
              <Button variant="destructive" size="sm" disabled={saving} onClick={() => onDelete("set", target.set!.id)}>
                {tf("admin.topology.common.delete", "删除")}
              </Button>
            </footer>
          )}
    </>
  );
}

// ===== 表单实现 =====

interface FormShellProps {
  fields: FieldSpec[];
  errors: Record<string, string>;
  values: Record<string, unknown>;
  onChange: (key: string, value: unknown) => void;
  children?: React.ReactNode; // 追加自定义区块（如成员编辑器）
}

function FieldRenderer({ field, error, value, onChange }: { field: FieldSpec; error?: string; value: unknown; onChange: (v: unknown) => void }) {
  const tf = useT();
  const label = (
    <label className="mb-1 block text-xs text-muted-foreground">
      {tf(field.labelKey, field.labelFallback)}
      {field.required && <span className="ml-0.5 text-destructive">*</span>}
    </label>
  );
  return (
    <div className="mb-3">
      {label}
      {field.readOnly ? (
        <div className="rounded-md border border-dashed border-border bg-muted/40 px-3 py-2 text-sm text-muted-foreground">
          {(value as string) || "—"}
        </div>
      ) : field.type === "textarea" ? (
        <Textarea rows={2} value={(value as string) ?? ""} placeholder={field.placeholder} onChange={(e) => onChange(e.target.value)} />
      ) : field.type === "select" ? (
        <Select value={(value as string) ?? ""} onValueChange={(v) => onChange(v)}>
          <SelectTrigger className="w-full">
            <SelectValue placeholder={tf("admin.topology.common.selectPlaceholder", "请选择")} />
          </SelectTrigger>
          <SelectContent>
            {(field.options ?? []).map((o) => (
              <SelectItem key={o.value} value={o.value}>
                {tf(o.labelKey, o.fallback)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : field.type === "switch" ? (
        <Switch checked={Boolean(value)} onCheckedChange={(c) => onChange(c)} />
      ) : (
        <Input
          type={field.type === "number" ? "number" : "text"}
          value={(value as string | number) ?? ""}
          placeholder={field.placeholder}
          onChange={(e) => onChange(field.type === "number" ? e.target.value : e.target.value)}
        />
      )}
      {field.help && !error && <p className="mt-1 text-xs text-muted-foreground">{field.help}</p>}
      {error && <p className="mt-1 text-xs text-destructive">{error}</p>}
    </div>
  );
}

function RuleForm({ policy, saving, onSave, onClose }: { policy: TopologyPolicy | null; saving: boolean; onSave: (v: RuleFormValues & { id: number | null }) => void; onClose: () => void }) {
  const tf = useT();
  const [values, setValues] = useState<Record<string, unknown>>({
    name: policy?.name ?? "",
    match_type: policy?.match_type ?? "geosite",
    match_value: policy?.match_value ?? "",
    priority: policy?.priority ?? 100,
    enabled: policy?.enabled ?? true,
    target_set_name: policy?.target_set_id != null ? `#${policy.target_set_id}` : tf("admin.topology.rule.noTarget", "未绑定（拖线到出口集）"),
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const dirty = useDirty(JSON.stringify(values), values);

  const submit = () => {
    const parsed = ruleFormSchema.safeParse({
      name: values.name,
      match_type: values.match_type,
      match_value: values.match_value,
      priority: values.priority,
      enabled: values.enabled,
    });
    if (!parsed.success) {
      setErrors(flattenZodErrors(parsed.error as z.ZodError));
      return;
    }
    setErrors({});
    onSave({ ...parsed.data, id: policy?.id ?? null });
  };

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        submit();
      }}
    >
      {RULE_FIELDS.map((f) => (
        <FieldRenderer key={f.key} field={f} error={errors[f.key]} value={values[f.key]} onChange={(v) => setValues((s) => ({ ...s, [f.key]: v }))} />
      ))}
      <DirtyBar dirty={dirty && !!policy} saving={saving} onSubmit={submit} onClose={onClose} submitLabel={policy ? tf("admin.topology.common.save", "保存") : tf("admin.topology.common.create", "创建")} />
    </form>
  );
}

function SetForm({ set, agents, saving, onSave, onClose }: { set: TopologyExitSet | null; agents: TopologyAgent[]; saving: boolean; onSave: (v: SetFormValues & { id: number | null }) => void; onClose: () => void }) {
  const tf = useT();
  const [values, setValues] = useState(() => ({
    name: set?.name ?? "",
    description: "",
    strategy: set?.strategy ?? "round_robin",
    enabled: set?.enabled ?? true,
    members: (set?.members ?? []).map<MemberRow>((m) => ({ agent_host_id: m.agent_host_id, weight: m.weight })),
  }));
  const [errors, setErrors] = useState<Record<string, string>>({});
  const dirty = useDirty(JSON.stringify(values), values);

  const updateMember = (idx: number, patch: Partial<MemberRow>) =>
    setValues((s) => ({ ...s, members: s.members.map((m, i) => (i === idx ? { ...m, ...patch } : m)) }));

  const addMember = () => {
    const used = new Set(values.members.map((m) => m.agent_host_id));
    const free = agents.find((a) => !used.has(a.id));
    if (!free) {
      toast.info(tf("admin.topology.set.allAgentsAdded", "所有 Agent 都已加入"));
      return;
    }
    setValues((s) => ({ ...s, members: [...s.members, { agent_host_id: free.id, weight: 1 }] }));
  };

  const submit = () => {
    const parsed = setFormSchema.safeParse(values);
    if (!parsed.success) {
      setErrors(flattenZodErrors(parsed.error as z.ZodError));
      return;
    }
    setErrors({});
    onSave({ ...parsed.data, id: set?.id ?? null });
  };

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        submit();
      }}
    >
      {SET_FIELDS.map((f) => (
        <FieldRenderer key={f.key} field={f} error={f.key === "members" ? undefined : errors[f.key]} value={values[f.key as keyof typeof values]} onChange={(v) => setValues((s) => ({ ...s, [f.key]: v }))} />
      ))}

      {/* 成员编辑器 */}
      <div className="mb-3 rounded-md border border-border p-3">
        <div className="mb-2 flex items-center justify-between">
          <span className="text-xs text-muted-foreground">{tf("admin.topology.set.members", "成员（Agent 与权重）")}</span>
          <Button type="button" variant="outline" size="sm" onClick={addMember}>
            + {tf("admin.topology.set.addMember", "添加成员")}
          </Button>
        </div>
        {values.members.length === 0 && <p className="py-2 text-center text-xs text-muted-foreground">{tf("admin.topology.set.emptyMembers", "暂无成员，至少添加一个")}</p>}
        {values.members.map((m, idx) => (
          <div key={idx} className="mb-2 flex items-center gap-2">
            <Select value={String(m.agent_host_id)} onValueChange={(v) => updateMember(idx, { agent_host_id: Number(v) })}>
              <SelectTrigger className="min-w-0 flex-1">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {agents.map((a) => (
                  <SelectItem key={a.id} value={String(a.id)}>
                    {a.name} ({a.host}){!a.online ? " · offline" : ""}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Input
              type="number"
              min={1}
              max={100}
              className="w-20 shrink-0"
              value={m.weight}
              onChange={(e) => updateMember(idx, { weight: Number(e.target.value) })}
              aria-label="weight"
            />
            <Button type="button" variant="ghost" size="sm" className="shrink-0 text-destructive" onClick={() => setValues((s) => ({ ...s, members: s.members.filter((_, i) => i !== idx) }))} aria-label="remove member">
              ✕
            </Button>
          </div>
        ))}
        {errors.members && <p className="text-xs text-destructive">{errors.members}</p>}
        {errors["members.0.weight"] && <p className="text-xs text-destructive">{errors["members.0.weight"]}</p>}
      </div>

      <DirtyBar dirty={dirty && !!set} saving={saving} onSubmit={submit} onClose={onClose} submitLabel={set ? tf("admin.topology.common.save", "保存") : tf("admin.topology.common.create", "创建")} />
    </form>
  );
}

function DirtyBar({ dirty, saving, onSubmit, submitLabel, onClose }: { dirty: boolean; saving: boolean; onSubmit: () => void; submitLabel: string; onClose: () => void }) {
  const tf = useT();
  const [confirmLeave, setConfirmLeave] = useState(false);
  if (dirty || confirmLeave) {
    return (
      <div className="sticky bottom-0 -mx-4 flex items-center justify-between gap-2 border-t border-border bg-card px-4 py-3">
        {dirty ? (
          <>
            <span className="text-xs text-warning">{tf("admin.topology.common.unsaved", "有未保存的修改")}</span>
            <div className="flex gap-2">
              <Button type="button" variant="outline" size="sm" onClick={onSubmit} disabled={saving}>
                {submitLabel}
              </Button>
            </div>
          </>
        ) : (
          <>
            <span className="text-xs text-muted-foreground">{tf("admin.topology.common.confirmClose", "放弃未保存的修改并关闭？")}</span>
            <div className="flex gap-2">
              <Button type="button" variant="outline" size="sm" onClick={() => setConfirmLeave(false)}>
                {tf("admin.topology.common.cancel", "取消")}
              </Button>
              <Button type="button" variant="destructive" size="sm" onClick={onClose}>
                {tf("admin.topology.common.confirm", "确认关闭")}
              </Button>
            </div>
          </>
        )}
      </div>
    );
  }
  return null;
}
