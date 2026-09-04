import { adminApi } from "./client";
import type { AdminServerGroup } from "@/types/admin";

/**
 * Fetch all server groups for the admin user form (group dropdown).
 * Backend endpoint: GET /{securePath}/server/group/fetch -> { data: AdminServerGroup[], count }
 */
export async function getServerGroups(): Promise<AdminServerGroup[]> {
  const response = await adminApi.get<{ data: AdminServerGroup[] }>("/server/group/fetch");
  return response.data.data;
}
