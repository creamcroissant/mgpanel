import { adminApi } from "./client";
import type {
  ExitNodeSet,
  ExitNodeSetDetail,
  CreateExitNodeSetRequest,
  UpdateExitNodeSetRequest,
  AddExitNodeSetMemberRequest,
  RoutingPolicy,
  CreateRoutingPolicyRequest,
  UpdateRoutingPolicyRequest,
} from "@/types/admin";

type DataEnvelope<T> = { data: T };

// ===== Exit Node Sets =====

export async function listExitNodeSets(): Promise<ExitNodeSetDetail[]> {
  const resp = await adminApi.get<DataEnvelope<ExitNodeSetDetail[]>>("/exit-node-sets/");
  return resp.data.data ?? [];
}

export async function getExitNodeSet(id: number): Promise<ExitNodeSetDetail> {
  const resp = await adminApi.get<DataEnvelope<ExitNodeSetDetail>>(`/exit-node-sets/${id}`);
  return resp.data.data;
}

export async function createExitNodeSet(
  req: CreateExitNodeSetRequest
): Promise<ExitNodeSet> {
  const resp = await adminApi.post<DataEnvelope<ExitNodeSet>>("/exit-node-sets/", req);
  return resp.data.data;
}

export async function updateExitNodeSet(
  id: number,
  req: UpdateExitNodeSetRequest
): Promise<ExitNodeSet> {
  const resp = await adminApi.put<DataEnvelope<ExitNodeSet>>(`/exit-node-sets/${id}`, req);
  return resp.data.data;
}

export async function deleteExitNodeSet(id: number): Promise<void> {
  await adminApi.delete(`/exit-node-sets/${id}`);
}

export async function addExitNodeSetMember(
  id: number,
  req: AddExitNodeSetMemberRequest
): Promise<void> {
  await adminApi.post(`/exit-node-sets/${id}/members`, req);
}

export async function removeExitNodeSetMember(
  id: number,
  agentHostId: number
): Promise<void> {
  await adminApi.delete(`/exit-node-sets/${id}/members/${agentHostId}`);
}

// ===== Routing Policies =====

export async function listRoutingPolicies(
  coreType?: string
): Promise<RoutingPolicy[]> {
  const params = coreType ? { core_type: coreType } : undefined;
  const resp = await adminApi.get<DataEnvelope<RoutingPolicy[]>>("/routing-policies/", { params });
  return resp.data.data ?? [];
}

export async function createRoutingPolicy(
  req: CreateRoutingPolicyRequest
): Promise<RoutingPolicy> {
  const resp = await adminApi.post<DataEnvelope<RoutingPolicy>>("/routing-policies/", req);
  return resp.data.data;
}

export async function updateRoutingPolicy(
  id: number,
  req: UpdateRoutingPolicyRequest
): Promise<RoutingPolicy> {
  const resp = await adminApi.put<DataEnvelope<RoutingPolicy>>(`/routing-policies/${id}`, req);
  return resp.data.data;
}

export async function deleteRoutingPolicy(id: number): Promise<void> {
  await adminApi.delete(`/routing-policies/${id}`);
}
