import { adminApi } from "./client";
import type { AdminServerNode } from "@/types/admin";

/**
 * Fetch all visible server nodes for the admin user form (denylist picker).
 * Backend endpoint: GET /{securePath}/server/manage/fetch -> { data: AdminServerNode[], count }
 */
export async function getAdminServerNodes(): Promise<AdminServerNode[]> {
  const response = await adminApi.get<{ data: AdminServerNode[] }>("/server/manage/fetch");
  return response.data.data;
}