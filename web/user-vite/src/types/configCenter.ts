export type ConfigCenterCoreType = "sing-box" | "xray" | "singbox";
export type ConfigCenterSource = "legacy" | "managed" | "merged";
export type ConfigCenterXrayTransport = "tcp" | "ws" | "grpc" | "http" | "xhttp";
export type ConfigCenterXHTTPMode = "auto" | "packet-up" | "stream-up" | "stream-one";

export interface ConfigCenterXHTTPSettings {
  host?: string;
  path?: string;
  mode?: ConfigCenterXHTTPMode | string;
  headers?: Record<string, string>;
  extra?: Record<string, unknown>;
  xmux?: Record<string, unknown>;
  downloadSettings?: Record<string, unknown>;
}
export type ConfigCenterParseStatus = "ok" | "parse_error";
export type ConfigCenterApplyRunStatus =
  | "pending"
  | "applying"
  | "success"
  | "failed"
  | "rolled_back";
export type ConfigCenterDriftType =
  | "hash_mismatch"
  | "missing_tag"
  | "tag_conflict"
  | "parse_error";
export type ConfigCenterDriftStatus = "drift" | "recovered";

export interface ConfigCenterSpec {
  id: number;
  agent_host_id: number | null;
  core_type: ConfigCenterCoreType;
  tag: string;
  enabled: boolean;
  semantic_spec: unknown;
  core_specific: unknown;
  desired_revision: number;
  created_by: number;
  updated_by: number;
  created_at: number;
  updated_at: number;
}

export interface ConfigCenterSpecRevision {
  id: number;
  spec_id: number;
  revision: number;
  snapshot: unknown;
  change_note: string;
  operator_id: number;
  created_at: number;
}

export interface ListConfigCenterSpecsParams {
  agent_host_id?: number;
  core_type?: ConfigCenterCoreType;
  tag?: string;
  enabled?: boolean;
  is_template?: boolean;
  limit?: number;
  offset?: number;
}

export interface ConfigCenterSpecListResponse {
  data: ConfigCenterSpec[];
  total: number;
}

export interface ConfigCenterSpecHistoryParams {
  limit?: number;
  offset?: number;
}

export interface ConfigCenterSpecHistoryResponse {
  data: ConfigCenterSpecRevision[];
  total: number;
}

export interface UpsertConfigCenterSpecRequest {
  agent_host_id?: number;
  core_type: ConfigCenterCoreType;
  tag: string;
  enabled?: boolean;
  semantic_spec: unknown;
  core_specific: unknown;
  change_note?: string;
  /** 出口绑定三选一（后端优先级：relay_path_id > exit_node_set_id > exit_agent_host_id）；未选项显式 null 以清除 */
  exit_agent_host_id?: number | null;
  exit_node_set_id?: number | null;
  relay_path_id?: number | null;
}

export interface UpsertConfigCenterSpecResult {
  spec_id: number;
  desired_revision: number;
}

export interface ImportConfigCenterSpecRequest {
  agent_host_id: number;
  core_type: ConfigCenterCoreType;
  source?: ConfigCenterSource;
  filename?: string;
  tag?: string;
  enabled?: boolean;
  change_note?: string;
  overwrite_existing: boolean;
}

export interface ImportConfigCenterSpecResult {
  created_count: number;
}

export interface ListConfigCenterArtifactsParams {
  agent_host_id: number;
  core_type: ConfigCenterCoreType;
  desired_revision?: number;
  tag?: string;
  filename?: string;
  include_content?: boolean;
  limit?: number;
  offset?: number;
}

export interface ConfigCenterArtifact {
  id: number;
  agent_host_id: number;
  core_type: ConfigCenterCoreType;
  desired_revision: number;
  filename: string;
  source_tag: string;
  content_hash: string;
  generated_at: number;
  content?: string;
}

export interface ConfigCenterArtifactListResponse {
  desired_revision: number;
  total: number;
  data: ConfigCenterArtifact[];
}

export interface GetConfigCenterTextDiffParams {
  agent_host_id: number;
  core_type: ConfigCenterCoreType;
  desired_revision?: number;
  filename?: string;
  tag?: string;
}

export interface ConfigCenterTextDiff {
  desired_revision: number;
  filename: string;
  tag: string;
  desired_text: string;
  applied_text: string;
  unified_diff: string;
  different: boolean;
}

export interface GetConfigCenterSemanticDiffParams {
  agent_host_id: number;
  core_type: ConfigCenterCoreType;
  desired_revision?: number;
  tag?: string;
}

export interface ConfigCenterSemanticFieldDiff {
  field: string;
  desired: string;
  applied: string;
}

export interface ConfigCenterSemanticDiffItem {
  tag: string;
  desired_filename?: string;
  applied_filename?: string;
  drift_type: ConfigCenterDriftType;
  field_diffs?: ConfigCenterSemanticFieldDiff[];
}

export interface ConfigCenterSemanticDiff {
  desired_revision: number;
  items: ConfigCenterSemanticDiffItem[];
}

export interface CreateConfigCenterApplyRunRequest {
  agent_host_id: number;
  core_type: ConfigCenterCoreType;
  target_revision: number;
  previous_revision?: number;
}

export interface ListConfigCenterApplyRunsParams {
  agent_host_id?: number;
  core_type?: ConfigCenterCoreType;
  status?: ConfigCenterApplyRunStatus;
  limit?: number;
  offset?: number;
}

export interface ConfigCenterApplyRun {
  run_id: string;
  agent_host_id: number;
  core_type: ConfigCenterCoreType;
  target_revision: number;
  status: ConfigCenterApplyRunStatus;
  error_message: string;
  previous_revision: number;
  rollback_revision: number;
  operator_id: number;
  started_at: number;
  finished_at: number;
}

export interface ConfigCenterApplyRunListResponse {
  data: ConfigCenterApplyRun[];
  total: number;
}

export interface GetConfigCenterApplyRunDetailParams {
  include_text?: boolean;
  text_tag?: string;
  text_file?: string;
}

export interface ConfigCenterApplyDiagnosticIssue {
  code: string;
  message: string;
}

export interface ConfigCenterApplyRunDetail {
  run: ConfigCenterApplyRun;
  semantic_diff?: ConfigCenterSemanticDiff;
  text_diff?: ConfigCenterTextDiff;
  issues?: ConfigCenterApplyDiagnosticIssue[];
}

export interface ListConfigCenterAppliedSnapshotParams {
  agent_host_id: number;
  core_type: ConfigCenterCoreType;
  source?: ConfigCenterSource;
  filename?: string;
  tag?: string;
  protocol?: string;
  parse_status?: ConfigCenterParseStatus;
  limit?: number;
  offset?: number;
}

export interface ConfigCenterInventory {
  id: number;
  agent_host_id: number;
  core_type: ConfigCenterCoreType;
  source: ConfigCenterSource;
  filename: string;
  hash_applied: string;
  parse_status: ConfigCenterParseStatus;
  parse_error: string;
  last_seen_at: number;
}

export interface ConfigCenterInboundIndex {
  id: number;
  agent_host_id: number;
  core_type: ConfigCenterCoreType;
  source: ConfigCenterSource;
  filename: string;
  tag: string;
  protocol: string;
  listen: string;
  port: number;
  tls: unknown;
  transport: unknown;
  multiplex: unknown;
  last_seen_at: number;
}

export interface ConfigCenterAppliedSnapshot {
  inventories: ConfigCenterInventory[];
  inbound_indexes: ConfigCenterInboundIndex[];
}

export type ConfigCenterSnapshot = ConfigCenterAppliedSnapshot;

export interface ListConfigCenterDriftStatesParams {
  agent_host_id: number;
  core_type: ConfigCenterCoreType;
  status?: ConfigCenterDriftStatus;
  drift_type?: ConfigCenterDriftType;
  tag?: string;
  filename?: string;
  limit?: number;
  offset?: number;
}

export interface ListConfigCenterRecoveryStatesParams {
  agent_host_id: number;
  core_type: ConfigCenterCoreType;
  drift_type?: ConfigCenterDriftType;
  tag?: string;
  filename?: string;
  limit?: number;
  offset?: number;
}

export interface ConfigCenterDriftState {
  id: number;
  agent_host_id: number;
  core_type: ConfigCenterCoreType;
  filename: string;
  tag: string;
  desired_revision: number;
  desired_hash: string;
  applied_hash: string;
  drift_type: ConfigCenterDriftType;
  status: ConfigCenterDriftStatus;
  detail: unknown;
  first_detected_at: number;
  last_changed_at: number;
}

export interface ConfigCenterDriftStateListResponse {
  data: ConfigCenterDriftState[];
  total: number;
}

export interface BindSpecRequest {
  agent_host_id: number;
}

export type CoreConfigType = "outbound" | "routing" | "dns" | "core_settings";

export interface CoreConfigItem {
  id: number;
  agent_host_id: number | null;
  core_type: ConfigCenterCoreType;
  config_type: CoreConfigType;
  tag: string;
  enabled: boolean;
  config_data: unknown;
  desired_revision: number;
  created_by: number;
  updated_by: number;
  created_at: number;
  updated_at: number;
}

export interface ListCoreConfigItemsParams {
  agent_host_id?: number;
  core_type?: ConfigCenterCoreType;
  config_type?: CoreConfigType;
  tag?: string;
  enabled?: boolean;
  is_template?: boolean;
  limit?: number;
  offset?: number;
}

export interface UpsertCoreConfigItemRequest {
  agent_host_id?: number;
  core_type: ConfigCenterCoreType;
  config_type: CoreConfigType;
  tag: string;
  enabled?: boolean;
  config_data: unknown;
  change_note?: string;
}

export interface UpsertCoreConfigItemResult {
  id: number;
  revision: number;
}
