import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { AlertTriangle, ArrowDown, ArrowUp, X } from "lucide-react";
import { toast } from "sonner";
import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui";
import { reorderPolicies } from "@/lib/topology/api";
import type {
  TopologyExitSet,
  TopologyIssue,
  TopologyPolicy,
  TopologySnapshot,
} from "@/lib/topology/types";

/**
 * 拓扑画布交互装配层（f4-wire 独占）：
 * - ConfirmDialog：连线改绑/创建策略前的摘要确认
 * - CreateRuleDialog：新规则保存时补选目标出口集（后端契约必填 target_set_id）
 * - ReorderLane：规则链优先级重排（HTML5 拖拽 + 上移/下移双支持，防抖提交）
 * - IssuesStrip：validate 结果列表，点击定位节点
 * 全部为纯展示/逻辑组件，API 调用经父层传入的回调完成，保持可单测。
 */

const tf = (t: (k: string, d: string) => string, k: string, d: string) =>
  t(k, d) as string;

// ===== 通用确认弹窗 =====

export interface ConfirmDialogProps {
  open: boolean;
  title: string;
  description?: string;
  summary?: string;
  confirmText?: string;
  destructive?: boolean;
  busy?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

export function ConfirmDialog({
  open,
  title,
  description,
  summary,
  confirmText,
  destructive = false,
  busy = false,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const t = useTf();
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onCancel()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {description && (
            <DialogDescription>{description}</DialogDescription>
          )}
        </DialogHeader>
        {summary && (
          <p className="rounded-md border bg-muted/40 px-3 py-2 text-sm text-muted-foreground">
            {summary}
          </p>
        )}
        <DialogFooter>
          <Button variant="outline" size="sm" onClick={onCancel} disabled={busy}>
            {tf(t, "admin.topology.confirm.cancel", "取消")}
          </Button>
          <Button
            variant={destructive ? "destructive" : "default"}
            size="sm"
            onClick={onConfirm}
            disabled={busy}
          >
            {confirmText ?? tf(t, "admin.topology.confirm.ok", "确认")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ===== 新建规则：补选目标出口集 =====

export interface CreateRuleDialogProps {
  open: boolean;
  sets: TopologyExitSet[];
  pendingValues: { name: string; match_type: string; match_value: string } | null;
  busy?: boolean;
  onConfirm: (targetSetId: number) => void;
  onCancel: () => void;
}

export function CreateRuleDialog({
  open,
  sets,
  pendingValues,
  busy = false,
  onConfirm,
  onCancel,
}: CreateRuleDialogProps) {
  const t = useTf();
  const [setId, setSetId] = useState<string>("");
  useEffect(() => {
    if (open) setSetId(sets[0] != null ? String(sets[0].id) : "");
  }, [open, sets]);

  const summary = pendingValues
    ? `${pendingValues.match_type}: ${pendingValues.match_value}（${pendingValues.name}）`
    : "";

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onCancel()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{tf(t, "admin.topology.createRule.title", "新建分流规则")}</DialogTitle>
          <DialogDescription>
            {tf(t, "admin.topology.createRule.desc", "选择该规则命中的流量要转发到的出口集")}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <p className="rounded-md border bg-muted/40 px-3 py-2 text-sm text-muted-foreground">
            {summary}
          </p>
          <Select value={setId} onValueChange={setSetId}>
            <SelectTrigger aria-label="目标出口集">
              <SelectValue placeholder={tf(t, "admin.topology.createRule.pickSet", "选择出口集")} />
            </SelectTrigger>
            <SelectContent>
              {sets.map((s) => (
                <SelectItem key={s.id} value={String(s.id)}>
                  {s.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <DialogFooter>
          <Button variant="outline" size="sm" onClick={onCancel} disabled={busy}>
            {tf(t, "admin.topology.confirm.cancel", "取消")}
          </Button>
          <Button
            size="sm"
            disabled={busy || setId === ""}
            onClick={() => onConfirm(Number(setId))}
          >
            {tf(t, "admin.topology.createRule.ok", "创建")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ===== 优先级重排 =====

/** 按 priority 升序的求值顺序（画布 eval-order 链同源） */
export function sortedPolicies(policies: TopologyPolicy[]): TopologyPolicy[] {
  return [...policies].sort((a, b) => a.priority - b.priority || a.id - b.id);
}

function reordered(list: TopologyPolicy[], from: number, to: number): TopologyPolicy[] {
  const next = [...list];
  const [moved] = next.splice(from, 1);
  next.splice(to, 0, moved);
  return next;
}

interface ReorderState {
  order: TopologyPolicy[];
  move: (from: number, to: number) => void;
  draggingId: number | null;
  setDraggingId: (id: number | null) => void;
}

function useReorder(
  policies: TopologyPolicy[],
  commitOrder: (orderedIds: number[]) => void
): ReorderState {
  const base = useMemo(() => sortedPolicies(policies), [policies]);
  // 本地乐观顺序：拖拽期间立即生效；快照刷新后以服务端为准
  const [local, setLocal] = useState<TopologyPolicy[] | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [draggingId, setDraggingId] = useState<number | null>(null);

  useEffect(() => {
    setLocal(null); // 服务端数据到达后丢弃本地暂态
  }, [base]);

  const order = local ?? base;

  const apply = useCallback(
    (next: TopologyPolicy[]) => {
      setLocal(next);
      if (timer.current) clearTimeout(timer.current);
      timer.current = setTimeout(() => {
        commitOrder(next.map((p) => p.id));
      }, 600);
    },
    [commitOrder]
  );

  const move = useCallback(
    (from: number, to: number) => {
      if (to < 0 || to >= order.length || from === to) return;
      apply(reordered(order, from, to));
    },
    [order, apply]
  );

  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    []
  );

  return { order, move, draggingId, setDraggingId };
}

export interface ReorderLaneProps {
  policies: TopologyPolicy[];
  selectedPolicyId: number | null;
  onCommitOrder: (orderedIds: number[]) => void;
  onSelect: (policyId: number) => void;
  disabled?: boolean;
}

/**
 * 左侧求值顺序轨道：自上而下即匹配顺序。
 * HTML5 拖拽与 ↑↓ 按钮双支持；变更后防抖提交 reorder API。
 */
export function ReorderLane({
  policies,
  selectedPolicyId,
  onCommitOrder,
  onSelect,
  disabled = false,
}: ReorderLaneProps) {
  const t = useTf();
  const { order, move, draggingId, setDraggingId } = useReorder(policies, onCommitOrder);
  const [overIndex, setOverIndex] = useState<number | null>(null);

  return (
    <aside
      aria-label={tf(t, "admin.topology.lane.title", "求值顺序")}
      className="flex w-52 shrink-0 flex-col overflow-hidden rounded-md border bg-card"
    >
      <p className="border-b border-border px-3 py-1.5 text-xs font-medium text-muted-foreground">
        {tf(t, "admin.topology.lane.title", "求值顺序")}
        <span className="ml-1 text-[10px] opacity-70">自上而下</span>
      </p>
      <ol className="flex-1 space-y-1 overflow-y-auto p-1.5">
        {order.map((p, i) => (
          <li
            key={p.id}
            draggable={!disabled}
            onDragStart={() => setDraggingId(p.id)}
            onDragEnd={() => {
              setDraggingId(null);
              setOverIndex(null);
            }}
            onDragOver={(e) => {
              e.preventDefault();
              setOverIndex(i);
            }}
            onDrop={(e) => {
              e.preventDefault();
              const from = order.findIndex((x) => x.id === draggingId);
              if (from >= 0) move(from, i);
              setDraggingId(null);
              setOverIndex(null);
            }}
            className={`group flex cursor-grab items-center gap-1 rounded-sm border px-1.5 py-1 text-xs transition-colors ${
              overIndex === i && draggingId !== null && draggingId !== p.id
                ? "border-primary bg-primary/10"
                : "bg-background"
            } ${selectedPolicyId === p.id ? "border-primary" : "border-transparent"} ${
              draggingId === p.id ? "opacity-50" : ""
            }`}
            onClick={() => onSelect(p.id)}
          >
            <span className="shrink-0 rounded-sm bg-primary/10 px-1 font-semibold tabular-nums text-primary">
              #{i + 1}
            </span>
            <span className="truncate" title={`${p.name} · ${p.match_type}:${p.match_value}`}>
              {p.name}
            </span>
            {!disabled && (
              <span className="ml-auto hidden shrink-0 flex-col group-hover:flex">
                <button
                  type="button"
                  aria-label="上移"
                  className="rounded-sm px-0.5 hover:bg-muted disabled:opacity-30"
                  disabled={i === 0}
                  onClick={(e) => {
                    e.stopPropagation();
                    move(i, i - 1);
                  }}
                >
                  <ArrowUp className="h-3 w-3" />
                </button>
                <button
                  type="button"
                  aria-label="下移"
                  className="rounded-sm px-0.5 hover:bg-muted disabled:opacity-30"
                  disabled={i === order.length - 1}
                  onClick={(e) => {
                    e.stopPropagation();
                    move(i, i + 1);
                  }}
                >
                  <ArrowDown className="h-3 w-3" />
                </button>
              </span>
            )}
          </li>
        ))}
        {order.length === 0 && (
          <li className="px-2 py-4 text-center text-xs text-muted-foreground">
            {tf(t, "admin.topology.lane.empty", "暂无分流规则")}
          </li>
        )}
      </ol>
    </aside>
  );
}

// ===== 校验结果 =====

/** 后端实体 → 画布节点 id 前缀映射 */
export function issueNodeId(issue: TopologyIssue): string | null {
  switch (issue.entity_type) {
    case "policy":
      return `rule-${issue.entity_id}`;
    case "exit_set":
    case "set":
      return `set-${issue.entity_id}`;
    case "spec":
    case "inbound":
      return `inbound-${issue.entity_id}`;
    default:
      return null;
  }
}

export function groupIssuesByNode(issues: TopologyIssue[]): Record<string, TopologyIssue[]> {
  const map: Record<string, TopologyIssue[]> = {};
  for (const iss of issues) {
    const nid = issueNodeId(iss);
    if (!nid) continue;
    (map[nid] ??= []).push(iss);
  }
  return map;
}

export interface IssuesStripProps {
  issues: TopologyIssue[];
  onClose: () => void;
  onSelectEntity: (entityType: string, entityId: number) => void;
}

export function IssuesStrip({ issues, onClose, onSelectEntity }: IssuesStripProps) {
  const t = useTf();
  const errors = issues.filter((i) => i.severity === "error");
  const warnings = issues.filter((i) => i.severity === "warning");
  return (
    <div
      role="alert"
      className="flex items-start gap-2 rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm"
    >
      <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-destructive" aria-hidden />
      <div className="min-w-0 flex-1 space-y-0.5">
        <p className="font-medium text-destructive">
          {tf(t, "admin.topology.validate.failed", "校验未通过")}
          <span className="ml-2 text-xs font-normal text-muted-foreground">
            {errors.length} 错误 / {warnings.length} 警告
          </span>
        </p>
        <ul className="space-y-0.5">
          {issues.map((iss, idx) => (
            <li key={`${iss.entity_type}-${iss.entity_id}-${idx}`}>
              <button
                type="button"
                className="text-left underline-offset-2 hover:underline"
                onClick={() => onSelectEntity(iss.entity_type, iss.entity_id)}
              >
                [{iss.code}] {iss.message}
              </button>
            </li>
          ))}
        </ul>
      </div>
      <button
        type="button"
        aria-label={tf(t, "admin.topology.validate.dismiss", "关闭")}
        onClick={onClose}
        className="shrink-0 rounded-sm p-0.5 hover:bg-muted"
      >
        <X className="h-3.5 w-3.5" />
      </button>
    </div>
  );
}

// ===== hooks =====

function useTf() {
  const { t } = useTranslation();
  return (key: string, fallback: string) => t(key, fallback) as string;
}

/** 防抖提交 reorder 的 mutation 包装（乐观更新由 ReorderLane 本地态承担） */
export function useReorderCommit(coreType: string, onDone: () => void) {
  return useMutation({
    mutationFn: (orderedIds: number[]) => reorderPolicies(orderedIds),
    onError: (e) => {
      toast.error(`优先级保存失败：${e instanceof Error ? e.message : String(e)}`);
    },
    onSuccess: (n) => {
      toast.success(`优先级已更新（${n} 条）`);
      onDone();
    },
    // mutation key 绑定 core 便于调试观察
    mutationKey: ["topology-reorder", coreType],
  });
}

/** 快照中按 id 找策略名（确认弹窗摘要用） */
export function policyName(snapshot: TopologySnapshot | undefined, id: number): string {
  return snapshot?.policies.find((p) => p.id === id)?.name ?? `#${id}`;
}
