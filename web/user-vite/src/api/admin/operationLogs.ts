import { adminApi, getAdminApiURL } from "./client";
import type {
  BinaryVersionComponent,
  BinaryVersionState,
  ListOperationLogsParams,
  OperationLogEntry,
  OperationLogsResponse,
} from "@/types/admin";

export async function listOperationLogs(
  params: ListOperationLogsParams
): Promise<OperationLogsResponse> {
  const response = await adminApi.get<{ data: OperationLogEntry[]; total: number }>(
    "/operation-logs",
    {
      params: {
        scope: params.scope,
        target_id: params.target_id,
        level: params.level,
        agent_host_id: params.agent_host_id,
        after_id: params.after_id,
        limit: params.limit,
        offset: params.offset,
      },
    }
  );

  return {
    logs: response.data.data,
    total: response.data.total,
  };
}

export function getOperationLogStreamURL(params: ListOperationLogsParams): string {
  const url = new URL(getAdminApiURL("/operation-logs/stream"), window.location.origin);
  url.searchParams.set("scope", params.scope);
  url.searchParams.set("target_id", params.target_id);
  if (params.level) {
    url.searchParams.set("level", params.level);
  }
  if (params.agent_host_id !== undefined) {
    url.searchParams.set("agent_host_id", String(params.agent_host_id));
  }
  if (params.after_id !== undefined) {
    url.searchParams.set("after_id", String(params.after_id));
  }
  if (params.limit !== undefined) {
    url.searchParams.set("limit", String(params.limit));
  }
  return url.toString();
}

export async function listAgentBinaryVersions(
  agentHostId: number
): Promise<BinaryVersionState[]> {
  const response = await adminApi.get<{ data: BinaryVersionState[] }>(
    `/agent-hosts/${agentHostId}/versions`
  );
  return response.data.data;
}

export async function refreshAgentBinaryVersion(
  agentHostId: number,
  component: BinaryVersionComponent
): Promise<BinaryVersionState> {
  const response = await adminApi.post<{ data: BinaryVersionState }>(
    `/agent-hosts/${agentHostId}/versions/${component}/refresh`
  );
  return response.data.data;
}
