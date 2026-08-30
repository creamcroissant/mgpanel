import { adminApi } from "./client";
import type {
  AgentHost,
  CreateAgentHostRequest,
  CreateAgentHostResponse,
  UpdateAgentHostRequest,
  PaginatedResponse,
  PaginationParams,
} from "@/types/admin";

/**
 * Get all agent hosts with pagination
 */
export async function getAgentHosts(
  params?: PaginationParams
): Promise<PaginatedResponse<AgentHost>> {
  const response = await adminApi.get<PaginatedResponse<AgentHost>>("/agent-hosts", {
    params,
  });
  return response.data;
}

/**
 * Get agent host by ID
 */
export async function getAgentHost(id: number): Promise<AgentHost> {
  const response = await adminApi.get<{ data: AgentHost }>(`/agent-hosts/${id}`);
  return response.data.data;
}

/**
 * Create a new agent host
 */
export async function createAgentHost(data: CreateAgentHostRequest): Promise<CreateAgentHostResponse> {
  const response = await adminApi.post<{ data: CreateAgentHostResponse }>("/agent-hosts", data);
  return response.data.data;
}

/**
 * Update an agent host
 */
export async function updateAgentHost(data: UpdateAgentHostRequest): Promise<AgentHost> {
  const { id, ...body } = data;
  const response = await adminApi.put<{ data: AgentHost }>(`/agent-hosts/${id}`, body);
  return response.data.data;
}

/**
 * Delete an agent host
 */
export async function deleteAgentHost(id: number): Promise<void> {
  await adminApi.delete(`/agent-hosts/${id}`);
}

/**
 * Refresh agent host status (trigger heartbeat check)
 */
export async function refreshAgentHosts(): Promise<void> {
  await adminApi.post("/agent-hosts/refresh");
}

/**
 * Refresh geo attribution (country/region) for all agent hosts via GeoIP.
 * Returns the number of hosts whose country was filled.
 */
/**
 * Trigger immediate GeoIP probe command for all online agent hosts.
 * Panel pushes a "geo_refresh" lifecycle operation to each online agent;
 * the agent then runs Reporter.RunOnce and reports (host, country, region) back.
 */
export async function triggerAgentGeoRefreshAll(): Promise<void> {
  await adminApi.post("/agent-hosts/geo-refresh-all");
}


/**
 * Fetch all agent hosts without pagination (single large page).
 */
export async function fetchAgentHostsAll(): Promise<AgentHost[]> {
  const response = await getAgentHosts({ page: 1, page_size: 1000 });
  return response.data;
}
