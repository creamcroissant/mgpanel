/** ConfigCenterPage 通用工具函数 */

import { isAdminApiError } from "@/api/admin/client";
import type { ConfigCenterApplyRunStatus, ConfigCenterXrayTransport } from "@/types/configCenter";
import {
  type ConfigCenterSearchState,
  type ConfigCenterTabValue,
  type CoreTypeOption,
  CONFIG_CENTER_TAB_VALUES,
  XRAY_TRANSPORT_OPTIONS,
  ACTIVE_APPLY_STATUSES,
} from "./configCenterPageTypes";

export function parseConfigCenterHostParam(value: string | null): number | null {
  if (!value) return null;
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : null;
}

export function parseConfigCenterCoreParam(value: string | null): CoreTypeOption {
  return value === "xray" ? "xray" : "sing-box";
}

export function parseConfigCenterTabParam(value: string | null): ConfigCenterTabValue {
  if (CONFIG_CENTER_TAB_VALUES.has(value as ConfigCenterTabValue)) {
    return value as ConfigCenterTabValue;
  }
  return "specs";
}

export function parseConfigCenterSearchParams(params: URLSearchParams): ConfigCenterSearchState {
  return {
    hostId: parseConfigCenterHostParam(params.get("host")),
    coreType: parseConfigCenterCoreParam(params.get("core")),
    tab: parseConfigCenterTabParam(params.get("tab")),
  };
}

export function writeConfigCenterSearchParams(
  baseParams: URLSearchParams,
  state: ConfigCenterSearchState,
): URLSearchParams {
  const next = new URLSearchParams(baseParams);
  if (state.hostId && state.hostId > 0) {
    next.set("host", String(state.hostId));
  } else {
    next.delete("host");
  }
  next.set("core", state.coreType);
  next.set("tab", state.tab);
  return next;
}

export function formatCoreType(coreType: string): CoreTypeOption {
  return coreType === "xray" ? "xray" : "sing-box";
}

export function generateCompactUUID(): string {
  if (typeof globalThis.crypto?.randomUUID === "function") {
    return globalThis.crypto.randomUUID().replaceAll("-", "").toLowerCase();
  }
  if (typeof globalThis.crypto?.getRandomValues === "function") {
    const bytes = new Uint8Array(16);
    globalThis.crypto.getRandomValues(bytes);
    bytes[6] = (bytes[6] & 0x0f) | 0x40;
    bytes[8] = (bytes[8] & 0x3f) | 0x80;
    return Array.from(bytes, (item) => item.toString(16).padStart(2, "0")).join("");
  }
  return `${Date.now().toString(16)}${Math.random().toString(16).slice(2)}`.slice(0, 32);
}

export function generateHexString(length: number): string {
  const size = Math.max(1, Math.ceil(length / 2));
  if (typeof globalThis.crypto?.getRandomValues === "function") {
    const bytes = new Uint8Array(size);
    globalThis.crypto.getRandomValues(bytes);
    return Array.from(bytes, (item) => item.toString(16).padStart(2, "0"))
      .join("")
      .slice(0, length);
  }
  return Array.from({ length }, () => Math.floor(Math.random() * 16).toString(16)).join("");
}

export async function generateRealityKeyPair(): Promise<{ privateKey: string; publicKey: string }> {
  if (!globalThis.crypto?.subtle) throw new Error("WebCrypto unavailable");
  const keyPair = (await globalThis.crypto.subtle.generateKey(
    { name: "X25519" } as unknown as AlgorithmIdentifier,
    true,
    ["deriveBits"],
  )) as CryptoKeyPair;
  const [privateJwk, publicJwk] = await Promise.all([
    globalThis.crypto.subtle.exportKey("jwk", keyPair.privateKey),
    globalThis.crypto.subtle.exportKey("jwk", keyPair.publicKey),
  ]);
  if (typeof privateJwk.d !== "string" || typeof publicJwk.x !== "string") {
    throw new Error("Failed to export X25519 jwk");
  }
  return { privateKey: privateJwk.d, publicKey: publicJwk.x };
}

export function normalizeXrayTransport(value: unknown): ConfigCenterXrayTransport {
  const normalized = typeof value === "string" ? value.trim().toLowerCase() : "";
  if (normalized === "splithttp") return "xhttp";
  if (XRAY_TRANSPORT_OPTIONS.includes(normalized as ConfigCenterXrayTransport)) {
    return normalized as ConfigCenterXrayTransport;
  }
  return "tcp";
}

export function hasMeaningfulValue(value: unknown): boolean {
  if (typeof value === "string") return value.trim().length > 0;
  if (Array.isArray(value)) return value.length > 0;
  return value !== null && value !== undefined;
}

export function formatDriftVariant(driftType: string): "danger" | "warning" | "secondary" {
  switch (driftType) {
    case "hash_mismatch":
    case "tag_conflict": return "danger";
    case "missing_tag":
    case "parse_error": return "warning";
    default: return "secondary";
  }
}

export function formatApplyStatusVariant(
  status: string,
): "success" | "warning" | "danger" | "secondary" {
  switch (status) {
    case "success": return "success";
    case "failed":
    case "rolled_back": return "danger";
    case "pending":
    case "applying": return "warning";
    default: return "secondary";
  }
}

export function isApplyRunActive(status: ConfigCenterApplyRunStatus): boolean {
  return ACTIVE_APPLY_STATUSES.has(status);
}

export function isApplyRunTerminal(status: ConfigCenterApplyRunStatus): boolean {
  return status === "success" || status === "failed" || status === "rolled_back";
}

export function formatApplyRunTerminalMessageKey(status: ConfigCenterApplyRunStatus): string {
  if (status === "success") return "admin.configCenter.messages.applyCompleted";
  if (status === "rolled_back") return "admin.configCenter.messages.applyRolledBack";
  return "admin.configCenter.messages.applyRunFailed";
}

export function formatAdminErrorDetails(error: unknown): string | undefined {
  if (!isAdminApiError(error)) return undefined;
  if (error.error_str) return error.error_str;
  if (!error.details || typeof error.details !== "object") return undefined;

  const details = error.details as {
    violations?: Array<{ field?: string; message?: string }>;
    conflict?: {
      kind?: string;
      field?: string;
      value?: string;
      existing_spec_id?: number;
      existing_tag?: string;
    };
  };

  if (Array.isArray(details.violations) && details.violations.length > 0) {
    return details.violations
      .map((item) => {
        const field = item.field?.trim();
        const message = item.message?.trim();
        if (field && message) return `${field}: ${message}`;
        return field || message || "";
      })
      .filter(Boolean)
      .join("\n");
  }

  if (details.conflict) {
    const parts = [
      details.conflict.kind,
      details.conflict.field,
      details.conflict.value,
      details.conflict.existing_tag,
      details.conflict.existing_spec_id ? `spec_id=${details.conflict.existing_spec_id}` : undefined,
    ].filter(Boolean);
    if (parts.length > 0) return parts.join(" | ");
  }

  return undefined;
}

export function formatQueryErrorMessage(error: unknown): string {
  if (error instanceof Error && error.message.trim()) return error.message;
  return "Request failed";
}

export function parseOptionalPositiveRevision(value: string): number | undefined {
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  const parsed = Number(trimmed);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : undefined;
}
