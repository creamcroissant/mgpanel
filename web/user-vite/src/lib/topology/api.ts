import { adminApi } from "@/api/admin/client";
import type {
  RelayPathInfo,
  TopologySnapshot,
  TopologyValidation,
} from "./types";

/** 后端统一响应信封 */
type DataEnvelope<T> = { data: T };

/** 拉取拓扑原子快照（服务端一次性聚合） */
export async function fetchTopology(coreType: string): Promise<TopologySnapshot> {
  const resp = await adminApi.get<DataEnvelope<TopologySnapshot>>("/topology", {
    params: { core_type: coreType },
  });
  return resp.data.data;
}

/** 服务端校验当前持久化拓扑（悬空引用/端口冲突等） */
export async function validateTopology(): Promise<TopologyValidation> {
  const resp = await adminApi.post<DataEnvelope<TopologyValidation>>(
    "/topology/validate",
    {}
  );
  return resp.data.data;
}

/** 批量重排路由策略优先级：按数组顺序重写 priority */
export async function reorderPolicies(orderedIds: number[]): Promise<number> {
  const resp = await adminApi.put<DataEnvelope<{ updated: number }>>(
    "/routing-policies/reorder",
    { ordered_ids: orderedIds }
  );
  return resp.data.data.updated;
}

// ===== 中继链路（服务器拓扑，冻结契约） =====

export interface RelayPathPayload {
  name: string;
  description?: string;
  core_type?: string;
  enabled?: boolean;
  nodes: { sequence: number; agent_host_id: number }[];
}

/** 创建中继链路（含有序节点序列） */
export async function createRelayPath(p: RelayPathPayload): Promise<RelayPathInfo> {
  const resp = await adminApi.post<DataEnvelope<RelayPathInfo>>("/relay-paths", p);
  return resp.data.data;
}

/** 更新中继链路（全量字段） */
export async function updateRelayPath(id: number, p: RelayPathPayload): Promise<void> {
  await adminApi.put(`/relay-paths/${id}`, p);
}

/** 删除中继链路（级联节点） */
export async function deleteRelayPath(id: number): Promise<void> {
  await adminApi.delete(`/relay-paths/${id}`);
}

/** 中继链路校验：循环检测/重复节点/agent 可达性；issue.entity_type=agent 指向具体节点 */
export async function validateRelayPaths(): Promise<TopologyValidation> {
  const resp = await adminApi.post<DataEnvelope<TopologyValidation>>(
    "/relay-paths/validate",
    {}
  );
  return resp.data.data;
}
