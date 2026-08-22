import { adminApi } from "./client";
import type {
  ForwardingRulesResponse,
  ForwardingRule,
  CreateForwardingRuleRequest,
  UpdateForwardingRuleRequest,
  ForwardingRuleLogsParams,
  ForwardingRuleLogsResponse,
} from "@/types/admin";

type ForwardingRuleLogPayload = {
  ID: number;
  RuleID: number | null;
  AgentHostID: number;
  Action: string;
  OperatorID: number | null;
  Detail: string;
  CreatedAt: number;
};

const mapForwardingRuleLog = (log: ForwardingRuleLogPayload) => ({
  id: log.ID,
  rule_id: log.RuleID ?? undefined,
  agent_host_id: log.AgentHostID,
  action: log.Action as "create" | "update" | "delete" | "apply" | "fail",
  operator_id: log.OperatorID ?? undefined,
  detail: log.Detail,
  created_at: log.CreatedAt,
});

type ForwardingRulePayload = {
  ID: number;
  AgentHostID: number;
  Name: string;
  Protocol: string;
  ListenPort: number;
  TargetAddress: string;
  TargetPort: number;
  Enabled: boolean;
  Priority: number;
  Remark: string;
  Version: number;
  CreatedAt: number;
  UpdatedAt: number;
};

const mapForwardingRule = (rule: ForwardingRulePayload): ForwardingRule => ({
  id: rule.ID,
  agent_host_id: rule.AgentHostID,
  name: rule.Name,
  protocol: rule.Protocol as ForwardingRule["protocol"],
  listen_port: rule.ListenPort,
  target_address: rule.TargetAddress,
  target_port: rule.TargetPort,
  enabled: rule.Enabled,
  priority: rule.Priority,
  remark: rule.Remark,
  version: rule.Version,
  created_at: rule.CreatedAt,
  updated_at: rule.UpdatedAt,
});

/**
 * List forwarding rules by agent host.
 */
export async function listForwardingRules(agentHostId: number): Promise<ForwardingRulesResponse> {
  const response = await adminApi.get<{
    data: { rules: ForwardingRulePayload[]; version: number };
  }>("/forwarding/rules", {
    params: { agent_host_id: agentHostId },
  });
  return {
    rules: response.data.data.rules.map(mapForwardingRule),
    version: response.data.data.version,
  };
}

/**
 * Create a forwarding rule.
 */
export async function createForwardingRule(
  data: CreateForwardingRuleRequest
): Promise<ForwardingRule> {
  const response = await adminApi.post<{ data: ForwardingRulePayload }>("/forwarding/rules", data);
  return mapForwardingRule(response.data.data);
}

/**
 * Update a forwarding rule.
 */
export async function updateForwardingRule(
  id: number,
  data: UpdateForwardingRuleRequest
): Promise<ForwardingRule> {
  const response = await adminApi.put<{ data: ForwardingRulePayload }>(`/forwarding/rules/${id}`, data);
  return mapForwardingRule(response.data.data);
}

/**
 * Delete a forwarding rule.
 */
export async function deleteForwardingRule(id: number): Promise<void> {
  await adminApi.delete(`/forwarding/rules/${id}`);
}

export async function listForwardingLogs(
  params: ForwardingRuleLogsParams
): Promise<ForwardingRuleLogsResponse> {
  const response = await adminApi.get<{
    data: { logs: ForwardingRuleLogPayload[]; total: number };
  }>("/forwarding/logs", {
    params: {
      agent_host_id: params.agent_host_id,
      rule_id: params.rule_id,
      start_at: params.start_at,
      end_at: params.end_at,
      limit: params.limit,
      offset: params.offset,
    },
  });
  return {
    logs: response.data.data.logs.map(mapForwardingRuleLog),
    total: response.data.data.total,
  };
}

