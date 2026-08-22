import { adminApi } from "./client";
import type {
  ConfigCenterAppliedSnapshot,
  ConfigCenterApplyRun,
  ConfigCenterApplyRunDetail,
  ConfigCenterApplyRunListResponse,
  ConfigCenterArtifactListResponse,
  ConfigCenterDriftStateListResponse,
  ConfigCenterSemanticDiff,
  ConfigCenterSpec,
  ConfigCenterSpecHistoryResponse,
  ConfigCenterSpecListResponse,
  ConfigCenterTextDiff,
  CreateConfigCenterApplyRunRequest,
  GetConfigCenterApplyRunDetailParams,
  GetConfigCenterSemanticDiffParams,
  GetConfigCenterTextDiffParams,
  ImportConfigCenterSpecRequest,
  ImportConfigCenterSpecResult,
  ListConfigCenterAppliedSnapshotParams,
  ListConfigCenterApplyRunsParams,
  ListConfigCenterArtifactsParams,
  ListConfigCenterDriftStatesParams,
  ListConfigCenterRecoveryStatesParams,
  ListConfigCenterSpecsParams,
  UpsertConfigCenterSpecRequest,
  UpsertConfigCenterSpecResult,
  BindSpecRequest,
  CoreConfigItem,
  ListCoreConfigItemsParams,
  UpsertCoreConfigItemRequest,
  UpsertCoreConfigItemResult,
} from "@/types/configCenter";

type DataEnvelope<T> = {
  data: T | null;
};

type DataWithTotalEnvelope<T> = {
  data: T | null;
  total: number;
};

const emptySemanticDiff = (): ConfigCenterSemanticDiff => ({
  desired_revision: 0,
  items: [],
});

function normalizeSemanticDiff(data: ConfigCenterSemanticDiff | null | undefined): ConfigCenterSemanticDiff {
  if (!data) {
    return emptySemanticDiff();
  }
  return {
    ...data,
    desired_revision: data.desired_revision ?? 0,
    items: Array.isArray(data.items) ? data.items : [],
  };
}

function normalizeAppliedSnapshot(data: ConfigCenterAppliedSnapshot | null | undefined): ConfigCenterAppliedSnapshot {
  return {
    ...(data ?? {}),
    inventories: Array.isArray(data?.inventories) ? data.inventories : [],
    inbound_indexes: Array.isArray(data?.inbound_indexes) ? data.inbound_indexes : [],
  };
}

function normalizeApplyRunDetail(data: ConfigCenterApplyRunDetail | null | undefined): ConfigCenterApplyRunDetail {
  if (!data) {
    return {} as ConfigCenterApplyRunDetail;
  }
  return {
    ...data,
    semantic_diff: normalizeSemanticDiff(data.semantic_diff),
    issues: Array.isArray(data.issues) ? data.issues : [],
  };
}

function normalizeUpsertSpecResult(data: UpsertConfigCenterSpecResult | null | undefined): UpsertConfigCenterSpecResult {
  return {
    spec_id: data?.spec_id ?? 0,
    desired_revision: data?.desired_revision ?? 0,
  };
}

function normalizeImportSpecResult(data: ImportConfigCenterSpecResult | null | undefined): ImportConfigCenterSpecResult {
  return {
    created_count: data?.created_count ?? 0,
  };
}

function normalizeTextDiff(data: ConfigCenterTextDiff | null | undefined): ConfigCenterTextDiff {
  return {
    desired_revision: data?.desired_revision ?? 0,
    filename: data?.filename ?? "",
    tag: data?.tag ?? "",
    desired_text: data?.desired_text ?? "",
    applied_text: data?.applied_text ?? "",
    unified_diff: data?.unified_diff ?? "",
    different: data?.different ?? false,
  };
}

function normalizeApplyRun(data: ConfigCenterApplyRun | null | undefined): ConfigCenterApplyRun {
  return {
    run_id: data?.run_id ?? "",
    agent_host_id: data?.agent_host_id ?? 0,
    core_type: data?.core_type ?? "sing-box",
    target_revision: data?.target_revision ?? 0,
    status: data?.status ?? "pending",
    error_message: data?.error_message ?? "",
    previous_revision: data?.previous_revision ?? 0,
    rollback_revision: data?.rollback_revision ?? 0,
    operator_id: data?.operator_id ?? 0,
    started_at: data?.started_at ?? 0,
    finished_at: data?.finished_at ?? 0,
  };
}

export async function listConfigCenterSpecs(
  params?: ListConfigCenterSpecsParams
): Promise<ConfigCenterSpecListResponse> {
  const response = await adminApi.get<DataWithTotalEnvelope<ConfigCenterSpec[]>>("/config-center/specs", {
    params,
  });
  return {
    data: response.data.data ?? [],
    total: response.data.total ?? 0,
  };
}

export async function createConfigCenterSpec(
  payload: UpsertConfigCenterSpecRequest
): Promise<UpsertConfigCenterSpecResult> {
  const response = await adminApi.post<DataEnvelope<UpsertConfigCenterSpecResult>>(
    "/config-center/specs",
    payload
  );
  return normalizeUpsertSpecResult(response.data.data);
}

export async function updateConfigCenterSpec(
  specId: number,
  payload: UpsertConfigCenterSpecRequest
): Promise<UpsertConfigCenterSpecResult> {
  const response = await adminApi.put<DataEnvelope<UpsertConfigCenterSpecResult>>(
    `/config-center/specs/${specId}`,
    payload
  );
  return normalizeUpsertSpecResult(response.data.data);
}

export async function deleteConfigCenterSpec(specId: number): Promise<void> {
  await adminApi.delete(`/config-center/specs/${specId}`);
}

export async function getConfigCenterSpecHistory(
  specId: number,
  params?: { limit?: number; offset?: number }
): Promise<ConfigCenterSpecHistoryResponse> {
  const response = await adminApi.get<DataWithTotalEnvelope<ConfigCenterSpecHistoryResponse["data"]>>(
    `/config-center/specs/${specId}/history`,
    { params }
  );
  return {
    data: response.data.data ?? [],
    total: response.data.total ?? 0,
  };
}

export async function importConfigCenterSpecsFromApplied(
  payload: ImportConfigCenterSpecRequest
): Promise<ImportConfigCenterSpecResult> {
  const response = await adminApi.post<DataEnvelope<ImportConfigCenterSpecResult>>(
    "/config-center/specs/import-from-applied",
    payload
  );
  return normalizeImportSpecResult(response.data.data);
}

export async function listConfigCenterArtifacts(
  params: ListConfigCenterArtifactsParams
): Promise<ConfigCenterArtifactListResponse> {
  const response = await adminApi.get<ConfigCenterArtifactListResponse>("/config-center/artifacts", {
    params,
  });
  return {
    desired_revision: response.data.desired_revision,
    total: response.data.total,
    data: response.data.data ?? [],
  };
}

export async function getConfigCenterTextDiff(
  params: GetConfigCenterTextDiffParams
): Promise<ConfigCenterTextDiff> {
  const response = await adminApi.get<DataEnvelope<ConfigCenterTextDiff>>("/config-center/diff/text", {
    params,
  });
  return normalizeTextDiff(response.data.data);
}

export async function getConfigCenterSemanticDiff(
  params: GetConfigCenterSemanticDiffParams
): Promise<ConfigCenterSemanticDiff> {
  const response = await adminApi.get<DataEnvelope<ConfigCenterSemanticDiff>>(
    "/config-center/diff/semantic",
    {
      params,
    }
  );
  return normalizeSemanticDiff(response.data.data);
}

export async function createConfigCenterApplyRun(
  payload: CreateConfigCenterApplyRunRequest
): Promise<ConfigCenterApplyRun> {
  const response = await adminApi.post<DataEnvelope<ConfigCenterApplyRun>>(
    "/config-center/apply-runs",
    payload
  );
  return normalizeApplyRun(response.data.data);
}

export async function listConfigCenterApplyRuns(
  params?: ListConfigCenterApplyRunsParams
): Promise<ConfigCenterApplyRunListResponse> {
  const response = await adminApi.get<DataWithTotalEnvelope<ConfigCenterApplyRun[]>>(
    "/config-center/apply-runs",
    {
      params,
    }
  );
  return {
    data: response.data.data ?? [],
    total: response.data.total ?? 0,
  };
}

export async function getConfigCenterApplyRunDetail(
  runId: string,
  params?: GetConfigCenterApplyRunDetailParams
): Promise<ConfigCenterApplyRunDetail> {
  const response = await adminApi.get<DataEnvelope<ConfigCenterApplyRunDetail>>(
    `/config-center/apply-runs/${encodeURIComponent(runId)}`,
    {
      params,
    }
  );
  return normalizeApplyRunDetail(response.data.data);
}

export async function listConfigCenterAppliedSnapshot(
  params: ListConfigCenterAppliedSnapshotParams
): Promise<ConfigCenterAppliedSnapshot> {
  const response = await adminApi.get<DataEnvelope<ConfigCenterAppliedSnapshot>>(
    "/config-center/snapshot",
    {
      params,
    }
  );
  return normalizeAppliedSnapshot(response.data.data);
}

export async function listConfigCenterDriftStates(
  params: ListConfigCenterDriftStatesParams
): Promise<ConfigCenterDriftStateListResponse> {
  const response = await adminApi.get<DataWithTotalEnvelope<ConfigCenterDriftStateListResponse["data"]>>(
    "/config-center/drift",
    {
      params,
    }
  );
  return {
    data: response.data.data ?? [],
    total: response.data.total ?? 0,
  };
}

export async function listConfigCenterRecoveryStates(
  params: ListConfigCenterRecoveryStatesParams
): Promise<ConfigCenterDriftStateListResponse> {
  const response = await adminApi.get<DataWithTotalEnvelope<ConfigCenterDriftStateListResponse["data"]>>(
    "/config-center/recover",
    {
      params,
    }
  );
  return {
    data: response.data.data ?? [],
    total: response.data.total ?? 0,
  };
}

export async function bindConfigCenterSpec(specId: number, payload: BindSpecRequest): Promise<void> {
  await adminApi.post(`/config-center/specs/${specId}/bind`, payload);
}

export async function unbindConfigCenterSpec(specId: number, agentHostId: number): Promise<void> {
  await adminApi.delete(`/config-center/specs/${specId}/bind`, {
    params: { agent_host_id: agentHostId },
  });
}

export async function listConfigCenterSpecBoundHosts(specId: number): Promise<number[]> {
  const response = await adminApi.get<{ data: number[] }>(
    `/config-center/specs/${specId}/bind`
  );
  return response.data.data ?? [];
}

export async function listCoreConfigItems(
  params?: ListCoreConfigItemsParams
): Promise<{ data: CoreConfigItem[]; total: number }> {
  const response = await adminApi.get<DataWithTotalEnvelope<CoreConfigItem[]>>("/config-center/core-configs", {
    params,
  });
  return {
    data: response.data.data ?? [],
    total: response.data.total ?? 0,
  };
}

export async function createCoreConfigItem(
  payload: UpsertCoreConfigItemRequest
): Promise<UpsertCoreConfigItemResult> {
  const response = await adminApi.post<DataEnvelope<UpsertCoreConfigItemResult>>(
    "/config-center/core-configs",
    payload
  );
  return {
    id: response.data.data?.id ?? 0,
    revision: response.data.data?.revision ?? 0,
  };
}

export async function updateCoreConfigItem(
  id: number,
  payload: UpsertCoreConfigItemRequest
): Promise<UpsertCoreConfigItemResult> {
  const response = await adminApi.put<DataEnvelope<UpsertCoreConfigItemResult>>(
    `/config-center/core-configs/${id}`,
    payload
  );
  return {
    id: response.data.data?.id ?? 0,
    revision: response.data.data?.revision ?? 0,
  };
}

export async function deleteCoreConfigItem(id: number): Promise<void> {
  await adminApi.delete(`/config-center/core-configs/${id}`);
}
