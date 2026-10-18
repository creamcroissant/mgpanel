import { isAdminApiError } from "@/api/admin/client";
import type { AdminApiErrorDetails, AgentCoreOperation, AgentOperationBlocker } from "@/types";

export const FILTER_ALL = "__all__";

const ACTIVE_OPERATION_STATUSES = new Set(["pending", "claimed", "in_progress"]);

export function normalizeTemplateId(value: string): number | undefined {
  if (!value) return undefined;
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined;
}

export function formatPorts(ports: number[]): string {
  if (!ports || ports.length === 0) return "-";
  return ports.join(", ");
}

export function isOperationActive(status: string): boolean {
  return ACTIVE_OPERATION_STATUSES.has(status);
}

export function buildOperationTarget(operation: AgentCoreOperation): string {
  const payload = operation.request_payload ?? {};
  if (operation.operation_type === "create") {
    return String(payload.instance_id || operation.core_type || "-");
  }
  if (operation.operation_type === "switch") {
    const from = String(payload.from_instance_id || "-");
    return `${from} → ${operation.core_type}`;
  }
  return operation.core_type || "-";
}

export function extractBlocker(error: unknown): AgentOperationBlocker | null {
  if (!isAdminApiError(error) || error.status !== 409) {
    return null;
  }
  const details = error.details as AdminApiErrorDetails | undefined;
  return details?.blocker ?? null;
}

export function describeOperationPayload(operation: AgentCoreOperation): string {
  if (operation.error_message) {
    return operation.error_message;
  }
  if (!operation.result_payload || Object.keys(operation.result_payload).length === 0) {
    return "-";
  }
  return JSON.stringify(operation.result_payload);
}
