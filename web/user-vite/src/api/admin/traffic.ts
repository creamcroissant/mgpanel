import { adminApi } from "./client";
import type {
  AgentTrafficPolicyStatus,
  AgentTrafficResetCycleRequest,
  AgentTrafficResetResult,
  UpdateAgentTrafficPolicyRequest,
} from "@/types/admin";

export async function getAgentTrafficPolicy(agentHostId: number): Promise<AgentTrafficPolicyStatus> {
  const response = await adminApi.get<{ data: AgentTrafficPolicyStatus }>(
    `/agent-hosts/${agentHostId}/traffic-policy`
  );
  return response.data.data;
}

export async function updateAgentTrafficPolicy(
  agentHostId: number,
  data: UpdateAgentTrafficPolicyRequest
): Promise<AgentTrafficPolicyStatus> {
  const response = await adminApi.put<{ data: AgentTrafficPolicyStatus }>(
    `/agent-hosts/${agentHostId}/traffic-policy`,
    data
  );
  return response.data.data;
}

export async function getAgentTrafficStatus(agentHostId: number): Promise<AgentTrafficPolicyStatus> {
  const response = await adminApi.get<{ data: AgentTrafficPolicyStatus }>(
    `/agent-hosts/${agentHostId}/traffic-status`
  );
  return response.data.data;
}

export async function resetAgentTrafficCycle(
  agentHostId: number,
  data: AgentTrafficResetCycleRequest = {}
): Promise<AgentTrafficResetResult> {
  const response = await adminApi.post<{ data: AgentTrafficResetResult }>(
    `/agent-hosts/${agentHostId}/traffic-cycle/reset`,
    data
  );
  return response.data.data;
}
