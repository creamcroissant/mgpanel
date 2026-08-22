import { api } from "./client";
import type { UserProfile, ApiResponse } from "@/types";

export async function fetchUserInfo(): Promise<UserProfile> {
  const response = await api.get<ApiResponse<UserProfile>>("/user/info");
  return response.data.data;
}

export async function fetchUserProfile(): Promise<UserProfile> {
  const response = await api.get<ApiResponse<UserProfile>>("/user/profile");
  return response.data.data;
}

export async function getSubscribeUrl(): Promise<string> {
  const response = await api.get<ApiResponse<{ subscribe_url: string }>>("/user/getSubscribe");
  return response.data.data.subscribe_url;
}

export async function changePassword(oldPassword: string, newPassword: string): Promise<void> {
  await api.post("/user/changePassword", {
    old_password: oldPassword,
    new_password: newPassword,
  });
}

export async function resetSubscribeToken(): Promise<string> {
  const response = await api.get<ApiResponse<{ token: string }>>("/user/resetSecurity");
  return response.data.data.token;
}

export async function updateProfile(data: Partial<UserProfile>): Promise<void> {
  await api.post("/user/profile", data);
}
