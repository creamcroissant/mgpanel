import { adminApi } from "./client";
import type {
  ListSubscriptionFilterReasonsParams,
  ListSubscriptionSourcesParams,
  SubscriptionFilterReasonListResponse,
  SubscriptionFilterSummary,
  SubscriptionFilterSummaryParams,
  SubscriptionSource,
  SubscriptionSourceListResponse,
  SubscriptionSourceSyncResult,
  UpsertSubscriptionSourceRequest,
} from "@/types/admin";

export async function listSubscriptionSources(
  params: ListSubscriptionSourcesParams = {}
): Promise<SubscriptionSourceListResponse> {
  const response = await adminApi.get<{ data: SubscriptionSourceListResponse }>(
    "/subscription/sources",
    { params }
  );
  return response.data.data;
}

export async function createSubscriptionSource(
  data: UpsertSubscriptionSourceRequest
): Promise<SubscriptionSource> {
  const response = await adminApi.post<{ data: SubscriptionSource }>(
    "/subscription/sources",
    data
  );
  return response.data.data;
}

export async function getSubscriptionSource(id: number): Promise<SubscriptionSource> {
  const response = await adminApi.get<{ data: SubscriptionSource }>(`/subscription/sources/${id}`);
  return response.data.data;
}

export async function updateSubscriptionSource(
  id: number,
  data: UpsertSubscriptionSourceRequest
): Promise<SubscriptionSource> {
  const response = await adminApi.put<{ data: SubscriptionSource }>(
    `/subscription/sources/${id}`,
    data
  );
  return response.data.data;
}

export async function deleteSubscriptionSource(id: number): Promise<void> {
  await adminApi.delete(`/subscription/sources/${id}`);
}

export async function syncSubscriptionSource(id: number): Promise<SubscriptionSourceSyncResult> {
  const response = await adminApi.post<{ data: SubscriptionSourceSyncResult }>(
    `/subscription/sources/${id}/sync`
  );
  return response.data.data;
}

export async function listSubscriptionFilterReasons(
  params: ListSubscriptionFilterReasonsParams = {}
): Promise<SubscriptionFilterReasonListResponse> {
  const response = await adminApi.get<{ data: SubscriptionFilterReasonListResponse }>(
    "/subscription/filter-reasons",
    { params }
  );
  return response.data.data;
}

export async function getSubscriptionFilterSummary(
  params: SubscriptionFilterSummaryParams = {}
): Promise<SubscriptionFilterSummary> {
  const response = await adminApi.get<{ data: SubscriptionFilterSummary }>(
    "/subscription/filter-summary",
    { params }
  );
  return response.data.data;
}
