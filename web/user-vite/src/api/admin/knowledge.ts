import { adminApi } from "./client";
import type { AdminKnowledgeSummary, AdminKnowledgeDetail, AdminKnowledgeSaveRequest } from "@/types/admin";

export async function getKnowledgeList(): Promise<AdminKnowledgeSummary[]> {
  const response = await adminApi.get<AdminKnowledgeSummary[]>("/knowledge/fetch");
  return response.data;
}

export async function getKnowledgeDetail(id: number): Promise<AdminKnowledgeDetail> {
  const response = await adminApi.get<AdminKnowledgeDetail>("/knowledge/fetch", {
    params: { id },
  });
  return response.data;
}

export async function getKnowledgeCategories(): Promise<string[]> {
  const response = await adminApi.get<string[]>("/knowledge/getCategory");
  return response.data;
}

export async function saveKnowledgeArticle(payload: AdminKnowledgeSaveRequest): Promise<void> {
  await adminApi.post("/knowledge/save", payload);
}

export async function toggleKnowledgeVisibility(id: number): Promise<void> {
  await adminApi.post("/knowledge/show", { id });
}

export async function deleteKnowledgeArticle(id: number): Promise<void> {
  await adminApi.post("/knowledge/drop", { id });
}

export async function sortKnowledgeArticles(ids: number[]): Promise<void> {
  await adminApi.post("/knowledge/sort", { ids });
}
