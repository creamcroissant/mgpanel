import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useMemo } from "react";
import { toast } from "sonner";
import type { TopologySnapshot } from "@/lib/topology/types";
import {
  addExitNodeSetMember,
  createExitNodeSet,
  createRoutingPolicy,
  deleteExitNodeSet,
  deleteRoutingPolicy,
  removeExitNodeSetMember,
  updateExitNodeSet,
  updateRoutingPolicy,
} from "@/api/admin/exitNodeSet";
import {
  listConfigCenterSpecs,
  updateConfigCenterSpec,
} from "@/api/admin/configCenter";
import type { TopologyPolicy, TopologySnapshot } from "@/lib/topology/types";
import {
  createRelayPath,
  updateRelayPath,
  deleteRelayPath,
} from "@/lib/topology/api";

/**
 * 拓扑画布实体变更 mutations（f4 波次经 DrawerPanel/连线接入）。
 * 全部走既有 REST 端点；乐观更新拓扑快照缓存，失败回滚并 toast。
 * 快照 queryKey 与 TopologyTab 保持一致：["admin","topology",coreType]。
 */

export function topologyQueryKey(coreType: string) {
  return ["admin", "topology", coreType] as const;
}

interface SnapshotCtx {
  prev?: TopologySnapshot;
}

function useSnapshotCache(coreType: string) {
  const queryClient = useQueryClient();
  return {
    get: () =>
      queryClient.getQueryData<TopologySnapshot>(topologyQueryKey(coreType)),
    set: (next: TopologySnapshot) =>
      queryClient.setQueryData<TopologySnapshot>(topologyQueryKey(coreType), next),
    rollback: (ctx: SnapshotCtx | undefined) => {
      if (ctx?.prev) queryClient.setQueryData(topologyQueryKey(coreType), ctx.prev);
    },
    invalidate: () => queryClient.invalidateQueries({ queryKey: topologyQueryKey(coreType) }),
    cancel: () => queryClient.cancelQueries({ queryKey: topologyQueryKey(coreType) }),
  };
}

function errMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

// ===== Policy（规则节点）=====

export interface PolicyPayload {
  name: string;
  match_type: string;
  match_value: string;
  priority: number;
  enabled: boolean;
  /** 更新时可选改绑目标集（连线交互）；后端 Update 为 Partial 契约 */
  target_set_id?: number;
}

export function useCreatePolicy(coreType: string) {
  const cache = useSnapshotCache(coreType);
  return useMutation({
    mutationFn: async (p: PolicyPayload & { target_set_id: number }) => {
      // 创建策略必须带 target_set_id（后端契约必填）；画布连线改绑由 f4 走 update。
      const created = await createRoutingPolicy({
        name: p.name,
        match_type: p.match_type,
        match_value: p.match_value,
        priority: p.priority,
        enabled: p.enabled,
        action: "route_to_set",
        target_set_id: p.target_set_id,
        core_type: coreType,
      });
      return created;
    },
    onMutate: async (p): Promise<SnapshotCtx> => {
      await cache.cancel();
      const prev = cache.get();
      if (prev) {
        cache.set({
          ...prev,
          policies: [
            ...prev.policies,
            {
              id: -Date.now(), // 临时负数 id，invalidate 后被服务端真值覆盖
              name: p.name,
              priority: p.priority,
              match_type: p.match_type,
              match_value: p.match_value,
              action: "route_to_set",
              target_set_id: p.target_set_id,
              enabled: p.enabled,
            },
          ],
        });
      }
      return { prev };
    },
    onError: (e, _p, ctx) => {
      cache.rollback(ctx);
      toast.error(`创建规则失败：${errMessage(e)}`);
    },
    onSuccess: () => toast.success("规则已创建"),
    onSettled: () => cache.invalidate(),
  });
}

export function useUpdatePolicy(coreType: string) {
  const cache = useSnapshotCache(coreType);
  return useMutation({
    mutationFn: async (v: { id: number } & PolicyPayload) => {
      const updated = await updateRoutingPolicy(v.id, {
        name: v.name,
        match_type: v.match_type,
        match_value: v.match_value,
        priority: v.priority,
        enabled: v.enabled,
        ...(v.target_set_id != null ? { target_set_id: v.target_set_id } : {}),
      });
      return updated;
    },
    onMutate: async (v): Promise<SnapshotCtx> => {
      await cache.cancel();
      const prev = cache.get();
      if (prev) {
        cache.set({
          ...prev,
          policies: prev.policies.map((pol) =>
            pol.id === v.id
              ? {
                  ...pol,
                  name: v.name,
                  match_type: v.match_type,
                  match_value: v.match_value,
                  priority: v.priority,
                  enabled: v.enabled,
                  ...(v.target_set_id != null ? { target_set_id: v.target_set_id } : {}),
                }
              : pol
          ),
        });
      }
      return { prev };
    },
    onError: (e, _v, ctx) => {
      cache.rollback(ctx);
      toast.error(`保存规则失败：${errMessage(e)}`);
    },
    onSuccess: () => toast.success("规则已保存"),
    onSettled: () => cache.invalidate(),
  });
}

export function useDeletePolicy(coreType: string) {
  const cache = useSnapshotCache(coreType);
  return useMutation({
    mutationFn: (id: number) => deleteRoutingPolicy(id),
    onMutate: async (id): Promise<SnapshotCtx> => {
      await cache.cancel();
      const prev = cache.get();
      if (prev) cache.set({ ...prev, policies: prev.policies.filter((p) => p.id !== id) });
      return { prev };
    },
    onError: (e, _id, ctx) => {
      cache.rollback(ctx);
      toast.error(`删除规则失败：${errMessage(e)}`);
    },
    onSuccess: () => toast.success("规则已删除"),
    onSettled: () => cache.invalidate(),
  });
}

// ===== Exit Set（出口集节点，members 随集合整体保存）=====

export interface MemberPayload {
  agent_host_id: number;
  weight: number;
}

export interface SetPayload {
  id?: number; // 有 id = 更新，无 = 创建
  name: string;
  description: string;
  strategy: string;
  enabled: boolean;
  members: MemberPayload[];
}

/** 成员同步：按 agent_host_id 差异化 增/删/换权重(删+加) */
async function syncMembers(setId: number, current: { agent_host_id: number; weight: number }[], desired: MemberPayload[]) {
  const curMap = new Map(current.map((m) => [m.agent_host_id, m.weight]));
  const desMap = new Map(desired.map((m) => [m.agent_host_id, m.weight]));
  for (const [aid, w] of desMap) {
    const cw = curMap.get(aid);
    if (cw === undefined || cw !== w) {
      if (cw !== undefined) await removeExitNodeSetMember(setId, aid);
      await addExitNodeSetMember(setId, { agent_host_id: aid, weight: w });
    }
  }
  for (const aid of curMap.keys()) {
    if (!desMap.has(aid)) await removeExitNodeSetMember(setId, aid);
  }
}

export function useSaveSet(coreType: string) {
  const cache = useSnapshotCache(coreType);
  return useMutation({
    mutationFn: async (p: SetPayload) => {
      if (p.id == null) {
        const created = await createExitNodeSet({
          name: p.name,
          description: p.description || undefined,
          strategy: p.strategy,
          members: p.members.map((m) => ({ agent_host_id: m.agent_host_id, weight: m.weight })),
        });
        return created.id as number;
      }
      await updateExitNodeSet(p.id, {
        name: p.name,
        description: p.description || undefined,
        strategy: p.strategy,
        enabled: p.enabled,
      });
      // UpdateExitNodeSetRequest 不含 members：按快照里的当前成员做差量同步
      const snap = cache.get();
      const cur = snap?.exit_sets.find((s) => s.id === p.id)?.members ?? [];
      await syncMembers(p.id, cur, p.members);
      return p.id;
    },
    onMutate: async (p): Promise<SnapshotCtx> => {
      await cache.cancel();
      const prev = cache.get();
      if (prev && p.id != null) {
        cache.set({
          ...prev,
          exit_sets: prev.exit_sets.map((s) =>
            s.id === p.id ? { ...s, name: p.name, strategy: p.strategy, enabled: p.enabled, members: p.members.map((m) => ({ ...m, name: "", host: "" })) } : s
          ),
        });
      }
      return { prev };
    },
    onError: (e, _p, ctx) => {
      cache.rollback(ctx);
      toast.error(`保存出口集失败：${errMessage(e)}`);
    },
    onSuccess: (_id, p) => toast.success(p.id == null ? "出口集已创建" : "出口集已保存"),
    onSettled: () => cache.invalidate(), // 成员真值以服务端为准，最终统一重拉
  });
}

export function useDeleteSet(coreType: string) {
  const cache = useSnapshotCache(coreType);
  return useMutation({
    mutationFn: (id: number) => deleteExitNodeSet(id),
    onMutate: async (id): Promise<SnapshotCtx> => {
      await cache.cancel();
      const prev = cache.get();
      if (prev) {
        cache.set({
          ...prev,
          exit_sets: prev.exit_sets.filter((s) => s.id !== id),
          // 集合删掉后，指向它的策略成为悬空引用——服务端 ON DELETE SET NULL，
          // 本地同步置空保持图一致性，validate 会再兜底提示
          policies: prev.policies.map((pol) =>
            pol.target_set_id === id ? { ...pol, target_set_id: null } : pol
          ),
        });
      }
      return { prev };
    },
    onError: (e, _id, ctx) => {
      cache.rollback(ctx);
      toast.error(`删除出口集失败：${errMessage(e)}`);
    },
    onSuccess: () => toast.success("出口集已删除"),
    onSettled: () => cache.invalidate(),
  });
}

/** f4 接线用的聚合入口：一次拿到全部 mutation */
export function useTopologyMutations(coreType: string) {
  return {
    createPolicy: useCreatePolicy(coreType),
    updatePolicy: useUpdatePolicy(coreType),
    deletePolicy: useDeletePolicy(coreType),
    saveSet: useSaveSet(coreType),
    deleteSet: useDeleteSet(coreType),
    saveSpecBinding: useSaveSpecBinding(coreType),
  };
}

/** 类型辅助：供 DrawerPanel 消费，避免直接耦合内部实现 */
export type { TopologyPolicy };

// ===== RelayPath（服务器中继链路）=====

export interface RelayPathFormValues {
  id: number;
  name: string;
  description: string;
  enabled: boolean;
  nodes: { sequence: number; agent_host_id: number }[];
}

/** 拓扑快照缓存访问器：乐观更新/回滚/失效的统一封装（queryKey 与 TopologyTab 保持一致） */
export function useTopologyCache(coreType: string) {
  const qc = useQueryClient();
  return useMemo(() => {
    const key = ["admin", "topology", coreType] as const;
    return {
      cancel: () => qc.cancelQueries({ queryKey: key }),
      get: () => qc.getQueryData<TopologySnapshot>(key),
      set: (next: TopologySnapshot) => qc.setQueryData(key, next),
      rollback: (ctx?: { prev?: TopologySnapshot }) => {
        if (ctx?.prev) qc.setQueryData(key, ctx.prev);
      },
      invalidate: () => qc.invalidateQueries({ queryKey: key }),
    };
  }, [qc, coreType]);
}

/** 创建链路：乐观插入空快照 relay_paths，invalidate 后被服务端真值覆盖 */
export function useCreateRelayPath(coreType: string) {
  const cache = useTopologyCache(coreType);
  return useMutation({
    mutationFn: async (p: {
      name: string;
      description?: string;
      enabled?: boolean;
      nodes: { sequence: number; agent_host_id: number }[];
    }) => {
      const created = await createRelayPath(p);
      toast.success(`链路「${p.name}」已创建`);
      return created;
    },
    onSettled: () => cache.invalidate(),
    onError: (e) => toast.error(errMessage(e)),
  });
}

export function useUpdateRelayPath(coreType: string) {
  const cache = useTopologyCache(coreType);
  return useMutation({
    mutationFn: async (v: RelayPathFormValues) => {
      await updateRelayPath(v.id, {
        name: v.name,
        description: v.description,
        enabled: v.enabled,
        nodes: v.nodes,
      });
      return v.id;
    },
    onMutate: async (v) => {
      await cache.cancel();
      const prev = cache.get();
      if (prev) {
        cache.set({
          ...prev,
          relay_paths: (prev.relay_paths ?? []).map((rp) =>
            rp.id === v.id
              ? { ...rp, name: v.name, description: v.description, enabled: v.enabled, nodes: [...v.nodes].sort((a, b) => a.sequence - b.sequence) }
              : rp
          ),
        });
      }
      return { prev };
    },
    onError: (e, _v, ctx) => {
      cache.rollback(ctx);
      toast.error(errMessage(e));
    },
    onSettled: () => cache.invalidate(),
  });
}

export function useDeleteRelayPath(coreType: string) {
  const cache = useTopologyCache(coreType);
  return useMutation({
    mutationFn: async (id: number) => {
      await deleteRelayPath(id);
      return id;
    },
    onMutate: async (id) => {
      await cache.cancel();
      const prev = cache.get();
      if (prev) {
        cache.set({ ...prev, relay_paths: (prev.relay_paths ?? []).filter((rp) => rp.id !== id) });
      }
      return { prev };
    },
    onError: (e, _id, ctx) => {
      cache.rollback(ctx);
      toast.error(errMessage(e));
    },
    onSettled: () => cache.invalidate(),
  });
}

// ===== 入站 spec 出口绑定（三选一：固定 agent / 出口集 / 中继链路）=====

export interface SpecExitBindingPayload {
  id: number;
  mode: "agent" | "set" | "relay";
  agentId: number | null;
  setId: number | null;
  pathId: number | null;
}

/**
 * 保存入站出口绑定：拉全量 spec 保留 semantic/core_specific，
 * 三选一互斥——未选项显式 null 让后端清除旧绑定。
 */
export function useSaveSpecBinding(coreType: string) {
  const cache = useSnapshotCache(coreType);
  return useMutation({
    mutationFn: async (v: SpecExitBindingPayload) => {
      const list = await listConfigCenterSpecs({ core_type: coreType as never, limit: 500 });
      const full = list.data.find((s) => s.id === v.id);
      if (!full) throw new Error(`spec #${v.id} not found`);
      await updateConfigCenterSpec(v.id, {
        agent_host_id: full.agent_host_id ?? undefined,
        core_type: full.core_type,
        tag: full.tag,
        enabled: full.enabled,
        semantic_spec: full.semantic_spec,
        core_specific: full.core_specific,
        change_note: "拓扑画布出口绑定更新",
        exit_agent_host_id: v.mode === "agent" ? v.agentId : null,
        exit_node_set_id: v.mode === "set" ? v.setId : null,
        relay_path_id: v.mode === "relay" ? v.pathId : null,
      });
    },
    onSuccess: () => {
      toast.success("出口绑定已保存");
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : String(err));
    },
    onSettled: () => {
      // 快照重取由 TopologyTab 层 refetch 处理；此处仅保证缓存一致性
    },
  });
}
