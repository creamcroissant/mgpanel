import { api } from "./client";
import type { KnowledgeArticle, ApiResponse } from "@/types";

// Normalize language code: en-US -> en, zh-CN -> zh-CN
function normalizeLanguage(lang: string): string {
  if (lang.startsWith("en")) return "en";
  return lang;
}

export async function fetchKnowledgeArticles(
  language: string
): Promise<Record<string, KnowledgeArticle[]>> {
  const normalizedLang = normalizeLanguage(language);
  const response = await api.get<ApiResponse<Record<string, KnowledgeArticle[]>>>(
    `/user/knowledge/fetch?language=${encodeURIComponent(normalizedLang)}`
  );
  const payload = response.data?.data ?? response.data;
  return payload ?? {};
}

export async function fetchKnowledgeArticle(id: number): Promise<KnowledgeArticle> {
  const response = await api.get<ApiResponse<KnowledgeArticle>>(`/user/knowledge/fetch?id=${id}`);
  return response.data.data;
}

export async function fetchKnowledgeCategories(): Promise<string[]> {
  const response = await api.get<ApiResponse<string[]>>("/user/knowledge/getCategory");
  return response.data.data;
}
