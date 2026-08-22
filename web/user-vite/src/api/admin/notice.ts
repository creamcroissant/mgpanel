import { adminApi } from "./client";
import type {
  AdminNotice,
  CreateNoticeRequest,
  UpdateNoticeRequest,
  PaginatedResponse,
  PaginationParams,
} from "@/types/admin";

/**
 * Get all notices with pagination
 */
export async function getNotices(
  params?: PaginationParams
): Promise<PaginatedResponse<AdminNotice>> {
  const response = await adminApi.get<PaginatedResponse<AdminNotice>>("/notice", {
    params,
  });
  return response.data;
}

/**
 * Get notice by ID
 */
export async function getNotice(id: number): Promise<AdminNotice> {
  const response = await adminApi.get<{ data: AdminNotice }>(`/notice/${id}`);
  return response.data.data;
}

/**
 * Create a new notice
 */
export async function createNotice(data: CreateNoticeRequest): Promise<AdminNotice> {
  const response = await adminApi.post<{ data: AdminNotice }>("/notice", data);
  return response.data.data;
}

/**
 * Update a notice
 */
export async function updateNotice(data: UpdateNoticeRequest): Promise<AdminNotice> {
  const { id, ...body } = data;
  const response = await adminApi.put<{ data: AdminNotice }>(`/notice/${id}`, body);
  return response.data.data;
}

/**
 * Delete a notice
 */
export async function deleteNotice(id: number): Promise<void> {
  await adminApi.delete(`/notice/${id}`);
}

/**
 * Toggle notice visibility
 */
export async function toggleNoticeVisibility(id: number, show: boolean): Promise<void> {
  await adminApi.put(`/notice/${id}`, { show });
}
