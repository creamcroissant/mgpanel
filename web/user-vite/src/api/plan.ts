import { api } from "./client";
import type { Plan, ApiResponse } from "@/types";

export async function fetchUserPlans(): Promise<Plan[]> {
  const response = await api.get<ApiResponse<Plan[]>>("/user/plan/fetch");
  return response.data.data;
}
