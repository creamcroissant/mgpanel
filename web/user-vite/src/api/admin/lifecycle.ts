import { adminApi } from "./client";
import type {
  AgentLifecycleOperation,
  AgentLifecycleOperationsResponse,
  AgentLifecycleUpdateRequest,
  AgentTrafficResetOperationRequest,
  ListAgentLifecycleOperationsParams,
} from "@/types/admin";

const toLifecycleOperationParams = (params: ListAgentLifecycleOperationsParams = {}) => ({
  operation_type: params.operation_type,
  status: params.status,
  statuses: params.statuses?.join(","),
  source: params.source,
  start_at: params.start_at,
  end_at: params.end_at,
  limit: params.limit,
  offset: params.offset,
});

export async function listAgentLifecycleOperations(
  agentHostId: number,
  params?: ListAgentLifecycleOperationsParams
): Promise<AgentLifecycleOperationsResponse> {
  const response = await adminApi.get<{ data: AgentLifecycleOperation[]; total: number }>(
    `/agent-hosts/${agentHostId}/lifecycle-operations`,
    { params: toLifecycleOperationParams(params) }
  );

  return {
    operations: response.data.data,
    total: response.data.total,
  };
}

export async function createAgentUpdateCheckOperation(
  agentHostId: number,
  data: AgentLifecycleUpdateRequest = {}
): Promise<AgentLifecycleOperation> {
  const response = await adminApi.post<{ data: AgentLifecycleOperation }>(
    `/agent-hosts/${agentHostId}/update-check`,
    data
  );
  return response.data.data;
}

export async function createAgentUpdateOperation(
  agentHostId: number,
  data: AgentLifecycleUpdateRequest = {}
): Promise<AgentLifecycleOperation> {
  const response = await adminApi.post<{ data: AgentLifecycleOperation }>(
    `/agent-hosts/${agentHostId}/update`,
    data
  );
  return response.data.data;
}

export async function createAgentTrafficResetOperation(
  agentHostId: number,
  data: AgentTrafficResetOperationRequest = {}
): Promise<AgentLifecycleOperation> {
  const response = await adminApi.post<{ data: AgentLifecycleOperation }>(
    `/agent-hosts/${agentHostId}/traffic-reset`,
    data
  );
  return response.data.data;
}
