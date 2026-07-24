import { adminApi } from "./client";

export interface MeshPeer {
  id: number;
  agent_host_id: number;
  wg_public_key: string;
  wg_ip: string;
  wg_listen_port: number;
  network_id: string;
  online: boolean;
  created_at: number;
  updated_at: number;
  latency_ms?: number;
  packet_loss?: number;
  total_probes?: number;
}

export interface MeshStatusResponse {
  data: MeshPeer[];
}

/**
 * Fetch mesh network peer status
 */
export async function fetchMeshStatus(
  networkId?: string
): Promise<MeshStatusResponse> {
  const nid = networkId || "default";
  const response = await adminApi.get<MeshStatusResponse>(`/mesh/network/${nid}`);
  return response.data;
}

/**
 * Fetch mesh status for a specific agent host.
 */
export async function fetchAgentMeshStatus(
  agentHostId: number
): Promise<MeshPeer | null> {
  const response = await adminApi.get<{ data: MeshPeer }>(
    `/agent-hosts/${agentHostId}/mesh/status`
  );
  return response.data.data ?? null;
}
