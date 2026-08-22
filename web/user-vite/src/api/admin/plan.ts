import { adminApi } from "./client";
import type {
  AdminPlan,
  CreatePlanRequest,
  UpdatePlanRequest,
  PaginatedResponse,
  PaginationParams,
} from "@/types/admin";

/**
 * Get all plans with pagination
 */
export async function getPlans(
  params?: PaginationParams
): Promise<PaginatedResponse<AdminPlan>> {
  const response = await adminApi.get<PaginatedResponse<AdminPlan>>("/plan", {
    params,
  });
  return response.data;
}

/**
 * Get plan by ID
 */
export async function getPlan(id: number): Promise<AdminPlan> {
  const response = await adminApi.get<{ data: AdminPlan }>(`/plan/${id}`);
  return response.data.data;
}

/**
 * Create a new plan
 */
export async function createPlan(data: CreatePlanRequest): Promise<AdminPlan> {
  const response = await adminApi.post<{ data: AdminPlan }>("/plan", data);
  return response.data.data;
}

/**
 * Update a plan
 */
export async function updatePlan(data: UpdatePlanRequest): Promise<AdminPlan> {
  const { id, ...body } = data;
  const response = await adminApi.put<{ data: AdminPlan }>(`/plan/${id}`, body);
  return response.data.data;
}

/**
 * Delete a plan
 */
export async function deletePlan(id: number): Promise<void> {
  await adminApi.delete(`/plan/${id}`);
}

/**
 * Update plan sort order
 */
export async function updatePlanSort(id: number, sort: number): Promise<void> {
  await adminApi.put(`/plan/${id}`, { sort });
}
