import { api } from "./client";
import type { ServerNode, ApiResponse } from "@/types";

export async function fetchUserServers(): Promise<ServerNode[]> {
  const response = await api.get<ApiResponse<ServerNode[]>>("/user/server/fetch");
  return response.data.data;
}

export async function fetchServerSelection(): Promise<number[]> {
  const response = await api.get<ApiResponse<number[]>>("/user/server/selection");
  return response.data.data;
}

export async function saveServerSelection(serverIds: number[]): Promise<void> {
  await api.post("/user/server/save", { server_ids: serverIds });
}
