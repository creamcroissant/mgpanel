import { adminApi } from "./client";
import type {
  AdminUser,
  CreateUserRequest,
  UpdateUserRequest,
  PaginatedResponse,
  PaginationParams,
} from "@/types/admin";

interface UserSearchParams extends PaginationParams {
  search?: string;
  plan_id?: number;
  status?: number;
  is_admin?: boolean;
}

/**
 * Get all users with pagination and filters
 */
export async function getUsers(
  params?: UserSearchParams
): Promise<PaginatedResponse<AdminUser>> {
  const response = await adminApi.get<PaginatedResponse<AdminUser>>("/user", {
    params,
  });
  return response.data;
}

/**
 * Get user by ID
 */
export async function getUser(id: number): Promise<AdminUser> {
  const response = await adminApi.get<{ data: AdminUser }>(`/user/${id}`);
  return response.data.data;
}

/**
 * Create a new user
 */
export async function createUser(data: CreateUserRequest): Promise<AdminUser> {
  const response = await adminApi.post<{ data: AdminUser }>("/user", data);
  return response.data.data;
}

/**
 * Update a user
 */
export async function updateUser(data: UpdateUserRequest): Promise<AdminUser> {
  const { id, ...body } = data;
  const response = await adminApi.put<{ data: AdminUser }>(`/user/${id}`, body);
  return response.data.data;
}

/**
 * Delete a user
 */
export async function deleteUser(id: number): Promise<void> {
  await adminApi.delete(`/user/${id}`);
}

/**
 * Ban/Unban a user
 */
export async function toggleUserBan(id: number, banned: boolean): Promise<void> {
  await adminApi.put(`/user/${id}`, { banned });
}

/**
 * Reset user password
 */
export async function resetUserPassword(id: number, password: string): Promise<void> {
  await adminApi.put(`/user/${id}`, { password });
}
