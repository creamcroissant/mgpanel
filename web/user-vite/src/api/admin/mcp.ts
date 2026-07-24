/**
 * MCP API key management API
 */
import { adminApi } from "./client";
import { ADMIN_API_VERSION } from "@/lib/constants";

export interface MCPApiKey {
  id: number;
  name: string;
  prefix: string;
  key?: string;
  enabled: boolean;
  last_used_at?: number;
  created_by: number;
  created_at: number;
  updated_at: number;
}

export interface CreateKeyRequest {
  name: string;
}

/** Fetch all MCP API keys */
export async function fetchMCPKeys(): Promise<MCPApiKey[]> {
  const res = await adminApi.get(`${ADMIN_API_VERSION}/mcp/keys`);
  return res.data;
}

/** Create a new MCP API key */
export async function createMCPKey(req: CreateKeyRequest): Promise<MCPApiKey> {
  const res = await adminApi.post(`${ADMIN_API_VERSION}/mcp/keys`, req);
  return res.data;
}

/** Revoke an MCP API key */
export async function revokeMCPKey(id: number): Promise<void> {
  await adminApi.post(`${ADMIN_API_VERSION}/mcp/keys/${id}/revoke`);
}

/** Delete an MCP API key */
export async function deleteMCPKey(id: number): Promise<void> {
  await adminApi.delete(`${ADMIN_API_VERSION}/mcp/keys/${id}`);
}
