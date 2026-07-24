import { adminApi } from "./client";
import type {
  AgentCoreInfo,
  AgentCoreInstance,
  AgentCoreOperation,
  AgentCoreOperationsResponse,
  AgentCoreSwitchLog,
  AgentCoreSwitchLogsParams,
  AgentCoreSwitchLogsResponse,
  ConvertCoreConfigRequest,
  ConvertCoreConfigResult,
  CreateAgentCoreInstanceRequest,
  InstallAgentCoreRequest,
  ListAgentCoreOperationsParams,
  SwitchAgentCoreRequest,
} from "@/types/admin";

type CoreInfoPayload = {
  type: string;
  version: string;
  installed: boolean;
  capabilities: string[];
};

type CoreInstancePayload = {
  ID: number;
  AgentHostID: number;
  InstanceID: string;
  CoreType: string;
  Status: string;
  ListenPorts: number[];
  ConfigTemplateID: number | null;
  ConfigHash: string;
  StartedAt: number | null;
  LastHeartbeatAt: number | null;
  ErrorMessage: string;
  CreatedAt: number;
  UpdatedAt: number;
};

type CoreOperationPayload = {
  id: string;
  agent_host_id: number;
  operation_type: "create" | "switch" | "install" | "ensure";
  core_type: string;
  status: "pending" | "claimed" | "in_progress" | "completed" | "failed" | "rolled_back";
  request_payload?: Record<string, unknown>;
  result_payload?: Record<string, unknown>;
  error_message?: string;
  operator_id?: number | null;
  claimed_by?: string;
  claimed_at?: number | null;
  started_at?: number | null;
  finished_at?: number | null;
  created_at: number;
  updated_at: number;
};

type SwitchLogPayload = {
  ID: number;
  AgentHostID: number;
  FromInstanceID: string | null;
  FromCoreType: string | null;
  ToInstanceID: string;
  ToCoreType: string;
  OperatorID: number | null;
  Status: string;
  Detail: string;
  CreatedAt: number;
  CompletedAt: number | null;
};

const mapCoreInfo = (core: CoreInfoPayload): AgentCoreInfo => ({
  type: core.type,
  version: core.version,
  installed: core.installed,
  capabilities: core.capabilities ?? [],
});

const mapCoreInstance = (instance: CoreInstancePayload): AgentCoreInstance => ({
  id: instance.ID,
  agent_host_id: instance.AgentHostID,
  instance_id: instance.InstanceID,
  core_type: instance.CoreType,
  status: instance.Status,
  listen_ports: instance.ListenPorts ?? [],
  config_template_id: instance.ConfigTemplateID ?? undefined,
  config_hash: instance.ConfigHash,
  started_at: instance.StartedAt ?? undefined,
  last_heartbeat_at: instance.LastHeartbeatAt ?? undefined,
  error_message: instance.ErrorMessage,
  created_at: instance.CreatedAt,
  updated_at: instance.UpdatedAt,
});

const mapCoreOperation = (operation: CoreOperationPayload): AgentCoreOperation => ({
  id: operation.id,
  agent_host_id: operation.agent_host_id,
  operation_type: operation.operation_type,
  core_type: operation.core_type,
  status: operation.status,
  request_payload: operation.request_payload,
  result_payload: operation.result_payload,
  error_message: operation.error_message,
  operator_id: operation.operator_id ?? undefined,
  claimed_by: operation.claimed_by,
  claimed_at: operation.claimed_at ?? undefined,
  started_at: operation.started_at ?? undefined,
  finished_at: operation.finished_at ?? undefined,
  created_at: operation.created_at,
  updated_at: operation.updated_at,
});

const mapSwitchLog = (log: SwitchLogPayload): AgentCoreSwitchLog => ({
  id: log.ID,
  agent_host_id: log.AgentHostID,
  from_instance_id: log.FromInstanceID ?? undefined,
  from_core_type: log.FromCoreType ?? undefined,
  to_instance_id: log.ToInstanceID,
  to_core_type: log.ToCoreType,
  operator_id: log.OperatorID ?? undefined,
  status: log.Status,
  detail: log.Detail,
  created_at: log.CreatedAt,
  completed_at: log.CompletedAt ?? undefined,
});

export async function listAgentCores(agentHostId: number): Promise<AgentCoreInfo[]> {
  const response = await adminApi.get<{ data: CoreInfoPayload[] }>(`/agent-hosts/${agentHostId}/cores`);
  return (response.data.data ?? []).map(mapCoreInfo);
}

export async function listAgentCoreInstances(agentHostId: number): Promise<AgentCoreInstance[]> {
  const response = await adminApi.get<{ data: CoreInstancePayload[] }>(`/agent-hosts/${agentHostId}/core-instances`);
  return (response.data.data ?? []).map(mapCoreInstance);
}

export async function listAgentCoreOperations(
  params: ListAgentCoreOperationsParams
): Promise<AgentCoreOperationsResponse> {
  const response = await adminApi.get<{ data: CoreOperationPayload[]; total: number }>(
    `/agent-hosts/${params.agent_host_id}/core-operations`,
    {
      params: {
        operation_type: params.operation_type,
        core_type: params.core_type,
        status: params.status,
        start_at: params.start_at,
        end_at: params.end_at,
        limit: params.limit,
        offset: params.offset,
      },
    }
  );
  return {
    operations: response.data.data.map(mapCoreOperation),
    total: response.data.total,
  };
}

export async function createAgentCoreInstance(
  agentHostId: number,
  payload: CreateAgentCoreInstanceRequest
): Promise<AgentCoreOperation> {
  const response = await adminApi.post<{ data: CoreOperationPayload }>(
    `/agent-hosts/${agentHostId}/core-instances`,
    payload
  );
  return mapCoreOperation(response.data.data);
}

export async function deleteAgentCoreInstance(agentHostId: number, instanceId: string): Promise<void> {
  await adminApi.delete(`/agent-hosts/${agentHostId}/core-instances/${instanceId}`);
}

export async function switchAgentCore(
  agentHostId: number,
  payload: SwitchAgentCoreRequest
): Promise<AgentCoreOperation> {
  const response = await adminApi.post<{ data: CoreOperationPayload }>(
    `/agent-hosts/${agentHostId}/core-switch`,
    payload
  );
  return mapCoreOperation(response.data.data);
}

export async function installAgentCore(
  agentHostId: number,
  payload: InstallAgentCoreRequest
): Promise<AgentCoreOperation> {
  const response = await adminApi.post<{ data: CoreOperationPayload }>(
    `/agent-hosts/${agentHostId}/core-install`,
    payload
  );
  return mapCoreOperation(response.data.data);
}

export async function convertAgentCoreConfig(
  agentHostId: number,
  payload: ConvertCoreConfigRequest
): Promise<ConvertCoreConfigResult> {
  const response = await adminApi.post<{ data: ConvertCoreConfigResult }>(
    `/agent-hosts/${agentHostId}/core-convert`,
    payload
  );
  return response.data.data;
}

export async function listAgentCoreSwitchLogs(
  params: AgentCoreSwitchLogsParams
): Promise<AgentCoreSwitchLogsResponse> {
  const response = await adminApi.get<{ data: SwitchLogPayload[]; total: number }>(
    `/agent-hosts/${params.agent_host_id}/core-switch-logs`,
    {
      params: {
        status: params.status,
        start_at: params.start_at,
        end_at: params.end_at,
        limit: params.limit,
        offset: params.offset,
      },
    }
  );
  return {
    logs: response.data.data.map(mapSwitchLog),
    total: response.data.total,
  };
}
