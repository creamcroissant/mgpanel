import { adminApi } from "./client";
import type { QueueStats, SystemStatus } from "@/types/admin";

/**
 * Get system status
 */
export async function getSystemStatus(): Promise<SystemStatus> {
  const response = await adminApi.get<SystemStatus>("/system/status");
  return response.data;
}

/**
 * Get queue stats
 */
export async function getQueueStats(): Promise<QueueStats> {
  const response = await adminApi.get<QueueStats>("/system/getQueueStats");
  return response.data;
}

/**
 * Get system configuration (partial, safe to expose)
 */
export async function getSystemConfig(): Promise<Record<string, unknown>> {
  const response = await adminApi.get<{ data: Record<string, unknown> }>("/config");
  return response.data.data;
}

/**
 * Update system configuration
 */
export async function updateSystemConfig(
  config: Record<string, unknown>
): Promise<void> {
  await adminApi.post("/config/save", config);
}
