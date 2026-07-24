/** ConfigCenterPage 共享类型、表单状态、常量 */

import type { ConfigCenterCoreType, ConfigCenterApplyRunStatus, ConfigCenterXrayTransport } from "@/types/configCenter";

export type CoreTypeOption = ConfigCenterCoreType;
export type ConfigCenterTabValue = "specs" | "apply" | "diff" | "drift" | "snapshot" | "outbound" | "routing" | "dns" | "coreSettings";

export type ConfigCenterSearchState = {
  hostId: number | null;
  coreType: CoreTypeOption;
  tab: ConfigCenterTabValue;
};

export type SpecFormState = {
  agent_host_id: number | null;
  core_type: CoreTypeOption;
  tag: string;
  enabled: boolean;
  semantic_spec: string;
  core_specific: string;
  change_note: string;
  is_template: boolean;
};

export type SpecJSONField = "semantic_spec" | "core_specific";
export type SpecJSONErrors = Partial<Record<SpecJSONField, string>>;
export type GeneratorOverwriteAction = "uuid" | "realityKey" | "shortId";

export type ImportFormState = {
  source: "legacy" | "managed" | "merged";
  filename: string;
  tag: string;
  enabled: boolean;
  overwrite_existing: boolean;
  change_note: string;
};

export type ApplyFormState = {
  target_revision: string;
  previous_revision: string;
};

export const CORE_OPTIONS: CoreTypeOption[] = ["sing-box", "xray"];
export const CONFIG_CENTER_TAB_VALUES = new Set<ConfigCenterTabValue>([
  "specs", "apply", "diff", "drift", "snapshot",
  "outbound", "routing", "dns", "coreSettings",
]);
export const XRAY_TRANSPORT_OPTIONS: ConfigCenterXrayTransport[] = ["tcp", "ws", "grpc", "http", "xhttp"];
export const ACTIVE_APPLY_STATUSES = new Set<ConfigCenterApplyRunStatus>(["pending", "applying"]);
export const GENERATOR_OVERWRITE_MESSAGE_KEYS: Record<GeneratorOverwriteAction, string> = {
  uuid: "admin.configCenter.generator.confirm.uuid",
  realityKey: "admin.configCenter.generator.confirm.realityKey",
  shortId: "admin.configCenter.generator.confirm.shortId",
};

export const defaultSpecFormState: SpecFormState = {
  agent_host_id: 0,
  core_type: "sing-box",
  tag: "",
  enabled: true,
  semantic_spec: "{}",
  core_specific: "{}",
  change_note: "",
  is_template: false,
};

export const defaultImportFormState: ImportFormState = {
  source: "merged",
  filename: "",
  tag: "",
  enabled: true,
  overwrite_existing: true,
  change_note: "",
};

export const defaultApplyFormState: ApplyFormState = {
  target_revision: "",
  previous_revision: "",
};
