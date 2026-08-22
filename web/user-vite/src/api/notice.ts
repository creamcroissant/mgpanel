import { api } from "./client";
import type { ApiResponse } from "@/types";

export interface UserNotice {
  id: number;
  title: string;
  content: string;
  img_url?: string;
  tags?: string[];
  created_at: number;
  updated_at: number;
}

export async function fetchUnreadNotice(): Promise<UserNotice | null> {
  try {
    const response = await api.get<ApiResponse<UserNotice>>("/user/notice/unread");
    return response.data.data;
  } catch (error) {
    const err = error as Error & { response?: { status?: number } };
    if (err?.response?.status === 404) {
      return null;
    }
    throw error;
  }
}

export async function markNoticeRead(id: number): Promise<void> {
  await api.post("/user/notice/read", { id });
}
