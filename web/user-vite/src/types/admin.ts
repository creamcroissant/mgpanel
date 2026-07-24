/**
 * Admin module type definitions
 */

// Agent status enum
export const AgentStatus = {
  Offline: 0,
  Online: 1,
  Warning: 2,
} as const;

export type AgentStatus = (typeof AgentStatus)[keyof typeof AgentStatus];

// Agent host (server probe) interface
export interface AgentHost {
  id: number;
  name: string;
  host: string;
  port?: number;
  token?: string;
  host_token?: string;
  template_id?: number;
  status: AgentStatus;
  provision_status?: number;
  cpu_total?: number;
  cpu_used: number;
  mem_total: number;
  mem_used: number;
  disk_total: number;
  disk_used: number;
  upload_total: number;
  download_total: number;
  upload_rate_bps?: number;
  download_rate_bps?: number;
  raw_upload_total_bytes?: number;
  raw_download_total_bytes?: number;
  boot_id?: string;
  last_realtime_report_at?: number;
  last_restart_at?: number;
  agent_version?: string;
  current_core_type?: string;
  core_version?: string;
  last_heartbeat_at: number;
  created_at: number;
  updated_at: number;
}

// Admin user interface (extended from UserProfile)
export interface AdminUser {
  id: number;
  email?: string;
  username?: string;
  uuid: string;
  token: string;
  plan_id?: number;
  plan_name?: string;
  group_id?: number;
  transfer_enable: number;
  transfer_used: number;
  u: number;
  d: number;
  expired_at?: number;
  is_admin: boolean;
  is_staff: boolean;
  status: number;
  banned: boolean;
  telegram_id?: number;
  created_at: number;
  updated_at: number;
}

// Admin plan interface
export interface AdminPlan {
  id: number;
  name: string;
  content?: string;
  transfer_enable: number;
  speed_limit?: number;
  device_limit?: number;
  group_id?: number;
  show: boolean;
  sell: boolean;
  renew: boolean;
  sort: number;
  reset_traffic_method?: number;
  reset_traffic_value?: number;
  created_at: number;
  updated_at: number;
}

// Admin notice interface
export interface AdminNotice {
  id: number;
  title: string;
  content: string;
  img_url?: string;
  show: boolean;
  popup: boolean;
  sort: number;
  created_at: number;
  updated_at: number;
}

// Admin knowledge interface (list)
export interface AdminKnowledgeSummary {
  id: number;
  language: string;
  category: string;
  title: string;
  show: boolean;
  updated_at: number;
}

// Admin knowledge interface (detail)
export interface AdminKnowledgeDetail {
  id: number;
  language: string;
  category: string;
  title: string;
  body: string;
  sort: number;
  show: boolean;
  created_at: number;
  updated_at: number;
}

// Agent core info
export interface AgentCoreInfo {
  type: string;
  version: string;
  installed: boolean;
  capabilities: string[];
}

export interface AgentCoreOperation {
  id: string;
  agent_host_id: number;
  operation_type: "create" | "switch" | "install" | "ensure";
  core_type: string;
  status: "pending" | "claimed" | "in_progress" | "completed" | "failed" | "rolled_back";
  request_payload?: Record<string, unknown>;
  result_payload?: Record<string, unknown>;
  error_message?: string;
  operator_id?: number;
  claimed_by?: string;
  claimed_at?: number;
  started_at?: number;
  finished_at?: number;
  created_at: number;
  updated_at: number;
}

export interface ListAgentCoreOperationsParams {
  agent_host_id: number;
  operation_type?: string;
  core_type?: string;
  status?: string;
  start_at?: number;
  end_at?: number;
  limit?: number;
  offset?: number;
}

export interface AgentCoreOperationsResponse {
  operations: AgentCoreOperation[];
  total: number;
}

export type OperationLogScope =
  | "core_operation"
  | "apply_run"
  | "agent_operation"
  | "agent_traffic"
  | "traffic_reset"
  | "threshold_action";

export type OperationLogLevel = "debug" | "info" | "warn" | "error";

export interface OperationLogEntry {
  id: number;
  scope: OperationLogScope;
  target_id: string;
  agent_host_id: number;
  sequence: number;
  phase: string;
  level: OperationLogLevel;
  message: string;
  payload?: unknown;
  source_event_id?: string;
  reported_at: number;
  created_at: number;
}

export interface ListOperationLogsParams {
  scope: OperationLogScope;
  target_id: string;
  level?: OperationLogLevel;
  agent_host_id?: number;
  after_id?: number;
  limit?: number;
  offset?: number;
}

export interface OperationLogsResponse {
  logs: OperationLogEntry[];
  total: number;
}

export type AgentLifecycleOperationType =
  | "agent_update"
  | "agent_update_check"
  | "traffic_reset"
  | "threshold_action"
  | "reset_links";

export type AgentLifecycleOperationStatus =
  | "pending"
  | "claimed"
  | "in_progress"
  | "success"
  | "failed"
  | "timeout"
  | "cancelled"
  | "unsupported_action"
  | "queue_full";

export type AgentLifecycleOperationSource = "admin" | "system" | (string & {});

export interface AgentLifecycleOperation {
  id: string;
  agent_host_id: number;
  operation_type: AgentLifecycleOperationType | (string & {});
  status: AgentLifecycleOperationStatus | (string & {});
  request_payload?: unknown;
  result_payload?: unknown;
  error_message?: string;
  claimed_by?: string;
  claimed_at?: number;
  started_at?: number;
  finished_at?: number;
  operator_id?: number;
  source: AgentLifecycleOperationSource;
  created_at: number;
  updated_at: number;
}

export interface ListAgentLifecycleOperationsParams {
  operation_type?: AgentLifecycleOperationType | (string & {});
  status?: AgentLifecycleOperationStatus | (string & {});
  statuses?: Array<AgentLifecycleOperationStatus | (string & {})>;
  source?: AgentLifecycleOperationSource;
  start_at?: number;
  end_at?: number;
  limit?: number;
  offset?: number;
}

export interface AgentLifecycleOperationsResponse {
  operations: AgentLifecycleOperation[];
  total: number;
}

export interface AgentLifecycleUpdateRequest {
  target_version?: string;
  release_tag?: string;
  release_repo?: string;
  release_base_url?: string;
  asset_name?: string;
  asset_url?: string;
  checksum_url?: string;
  sha256?: string;
  jitter_min_seconds?: number;
  jitter_max_seconds?: number;
}

export interface AgentTrafficResetOperationRequest {
  reason?: string;
  source?: string;
}

export interface AgentCommandQueueStats {
  capacity: number;
  queued: number;
  inflight: number;
  workers: number;
  available: number;
  active_command_ids?: string[];
  updated_at: number;
}

export type BinaryVersionComponent = "agent" | "sing-box" | "xray";

export type BinaryVersionStatus =
  | "installed"
  | "missing"
  | "outdated"
  | "up_to_date"
  | "unknown";

export interface BinaryVersionState {
  id?: number;
  agent_host_id: number;
  component: BinaryVersionComponent;
  local_version: string;
  remote_version?: string;
  status: BinaryVersionStatus;
  capabilities?: string[];
  build_tags?: string[];
  last_checked_at?: number;
  last_check_error?: string;
  updated_at?: number;
}

export interface AgentOperationBlocker {
  scope: "core_operation" | "apply_run" | string;
  id: string;
  agent_host_id: number;
  operation_type: string;
  status: string;
  created_at: number;
}

export interface AdminApiErrorDetails {
  blocker?: AgentOperationBlocker;
  fields?: Record<string, string>;
  violations?: Array<{
    field?: string;
    message?: string;
  }>;
  [key: string]: unknown;
}

// Agent core instance interface
export interface AgentCoreInstance {
  id: number;
  agent_host_id: number;
  instance_id: string;
  core_type: string;
  status: string;
  listen_ports: number[];
  config_template_id?: number;
  config_hash: string;
  started_at?: number;
  last_heartbeat_at?: number;
  error_message: string;
  created_at: number;
  updated_at: number;
}

export interface CreateAgentCoreInstanceRequest {
  core_type: string;
  instance_id: string;
  config_template_id?: number;
  config_json?: Record<string, unknown>;
}

export interface SwitchAgentCoreRequest {
  from_instance_id?: string;
  to_core_type: string;
  config_template_id?: number;
  config_json?: Record<string, unknown>;
  switch_id?: string;
  listen_ports?: number[];
  zero_downtime?: boolean;
}

export interface InstallAgentCoreRequest {
  core_type: string;
  action: string;
  version?: string;
  channel?: string;
  flavor?: string;
  activate?: boolean;
  request_id?: string;
}

export interface ConvertCoreConfigRequest {
  source_core: string;
  target_core: string;
  config_json: Record<string, unknown>;
}

export interface ConvertCoreConfigResult {
  config_json: unknown;
  warnings?: string[];
}

export interface AgentCoreSwitchLog {
  id: number;
  agent_host_id: number;
  from_instance_id?: string;
  from_core_type?: string;
  to_instance_id: string;
  to_core_type: string;
  operator_id?: number;
  status: string;
  detail: string;
  created_at: number;
  completed_at?: number;
}

export interface AgentCoreSwitchLogsParams {
  agent_host_id: number;
  status?: string;
  start_at?: number;
  end_at?: number;
  limit?: number;
  offset?: number;
}

export interface AgentCoreSwitchLogsResponse {
  logs: AgentCoreSwitchLog[];
  total: number;
}

// Admin knowledge save request
export interface AdminKnowledgeSaveRequest {
  id?: number;
  language: string;
  category: string;
  title: string;
  body: string;
  sort?: number;
  show?: boolean;
}

// System status interface
export interface SystemLogSummary {
  info: number;
  warning: number;
  error: number;
  total: number;
}

export interface SystemStatus {
  version: string;
  go_version: string;
  environment?: string;
  hostname?: string;
  started_at?: string;
  uptime: number;
  user_count: number;
  server_count: number;
  agent_count: number;
  online_agent_count: number;
  logs?: SystemLogSummary;
}

export interface QueueWait {
  default: number;
}

export interface QueueMetric {
  name: string;
  throughput: number;
}

export interface QueueRuntime {
  name: string;
  runtime: number;
}

export interface QueuePeriods {
  recentJobs: number;
  failedJobs: number;
}

export interface QueueStats {
  status: boolean;
  wait?: QueueWait;
  recentJobs: number;
  jobsPerMinute: number;
  queueWithMaxThroughput?: QueueMetric;
  queueWithMaxRuntime?: QueueRuntime;
  failedJobs: number;
  periods?: QueuePeriods;
  processes?: number;
  pausedMasters?: number;
}

export type AgentTrafficLimitType = "upload" | "download" | "sum";

export type AgentTrafficThresholdAction =
  | "notify_only"
  | "subscription_exclude"
  | "disable_servers"
  | "reset_links";

export type AgentTrafficResetMode = "off" | "fixed_day" | "calendar_month" | "interval_days";

export interface AgentTrafficPolicy {
  agent_host_id: number;
  enabled: boolean;
  limit_bytes: number;
  limit_type: AgentTrafficLimitType | (string & {});
  threshold_percent: number;
  threshold_action: AgentTrafficThresholdAction | (string & {});
  threshold_reached: boolean;
  reset_mode: AgentTrafficResetMode | (string & {});
  reset_day: number;
  interval_days: number;
  anchor_at: number;
  last_reset_at: number;
  last_reset_cycle_key: string;
  updated_at: number;
}

export interface AgentTrafficState {
  agent_host_id: number;
  boot_id: string;
  last_raw_upload_bytes: number;
  last_raw_download_bytes: number;
  counter_seen: boolean;
  cycle_upload_bytes: number;
  cycle_download_bytes: number;
  updated_at: number;
}

export interface AgentTrafficPolicyStatus {
  agent_host_id: number;
  policy: AgentTrafficPolicy;
  state?: AgentTrafficState;
  usage_bytes: number;
  threshold_bytes: number;
  threshold_reached: boolean;
  next_reset_at?: number;
  next_reset_cycle_key?: string;
  cycle_upload_bytes: number;
  cycle_download_bytes: number;
  cycle_total_bytes: number;
  last_raw_upload_bytes?: number;
  last_raw_download_bytes?: number;
}

export interface UpdateAgentTrafficPolicyRequest {
  enabled: boolean;
  limit_bytes: number;
  limit_type: AgentTrafficLimitType;
  threshold_percent: number;
  threshold_action: AgentTrafficThresholdAction;
  reset_mode: AgentTrafficResetMode;
  reset_day: number;
  interval_days: number;
  anchor_at: number;
}

export interface AgentTrafficResetCycleRequest {
  source?: string;
}

export interface AgentTrafficResetResult {
  agent_host_id: number;
  source: string;
  reset_at: number;
  cycle_key: string;
  state_reset: boolean;
  threshold_cleared: boolean;
  restored_servers: number;
  cleared_filter_reasons: boolean;
}

export type SubscriptionSourceType = "self_hosted" | "imported_subscription" | "custom_node";

export interface SubscriptionSource {
  id: number;
  type: SubscriptionSourceType | (string & {});
  name: string;
  url?: string;
  content?: string;
  enabled: boolean;
  last_sync_at?: number;
  last_sync_err?: string;
  created_at: number;
  updated_at: number;
}

export interface UpsertSubscriptionSourceRequest {
  type: SubscriptionSourceType;
  name: string;
  url?: string;
  content?: string;
  enabled: boolean;
}

export interface ListSubscriptionSourcesParams {
  type?: SubscriptionSourceType;
  enabled?: boolean;
  keyword?: string;
  limit?: number;
  offset?: number;
}

export interface SubscriptionSourceListResponse {
  sources: SubscriptionSource[];
  total: number;
}

export interface SubscriptionSourceSyncResult {
  source: SubscriptionSource;
  success: boolean;
  node_count: number;
  error?: string;
  synced_at: number;
}

export type SubscriptionFilterReason =
  | "hidden"
  | "offline"
  | "blocked"
  | "threshold_reached"
  | "protocol_disabled"
  | "group_denied"
  | "tag_mismatch"
  | "type_mismatch";

export interface SubscriptionFilterReasonEntry {
  id: number;
  source_type: SubscriptionSourceType | (string & {});
  source_id: number;
  server_id: number;
  node_name: string;
  reason: SubscriptionFilterReason | (string & {});
  detail?: string;
  created_at: number;
}

export interface ListSubscriptionFilterReasonsParams {
  source_type?: SubscriptionSourceType | (string & {});
  source_id?: number;
  server_id?: number;
  reason?: SubscriptionFilterReason | (string & {});
  created_after?: number;
  created_before?: number;
  limit?: number;
  offset?: number;
  types?: string;
  filter?: string;
  tags?: string;
}

export interface SubscriptionFilterReasonListResponse {
  reasons: SubscriptionFilterReasonEntry[];
  total: number;
  available_node_count: number;
  filtered_node_count: number;
  total_node_count: number;
  reason_counts: Record<string, number>;
}

export interface SubscriptionFilterSummaryParams {
  types?: string;
  filter?: string;
  tags?: string;
}

export interface SubscriptionFilterSummary {
  available_node_count: number;
  filtered_node_count: number;
  total_node_count: number;
  self_hosted_count: number;
  source_node_count: number;
  enabled_source_count: number;
  reason_counts: Record<string, number>;
}

// Pagination params
export interface PaginationParams {
  page?: number;
  page_size?: number;
}

// Paginated response
export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  page_size: number;
}

// Create/Update agent host request
export interface CreateAgentHostRequest {
  name: string;
  host: string;
  port: number;
  token?: string;
  template_id?: number;
}

export interface CreateAgentHostResponse {
  id: number;
  name: string;
  host: string;
  token?: string;
  host_token?: string;
  provision_status?: number;
}

export interface RegisterAgentHostRequest {
  communication_key: string;
  hostname: string;
  advertise_host?: string;
}

export interface RegisterAgentHostResponse {
  id: number;
  name: string;
  host: string;
  host_token: string;
  provision_status: number;
}

export interface UpdateAgentHostRequest extends Partial<CreateAgentHostRequest> {
  id: number;
}

// Create/Update user request
export interface CreateUserRequest {
  email?: string;
  username?: string;
  password: string;
  plan_id?: number;
  group_id?: number;
  is_admin?: boolean;
  is_staff?: boolean;
  expired_at?: number;
  transfer_enable?: number;
}

export interface UpdateUserRequest {
  id: number;
  email?: string;
  username?: string;
  password?: string;
  plan_id?: number;
  group_id?: number;
  is_admin?: boolean;
  is_staff?: boolean;
  banned?: boolean;
  expired_at?: number;
  transfer_enable?: number;
}

// Create/Update plan request
export interface CreatePlanRequest {
  name: string;
  content?: string;
  transfer_enable: number;
  speed_limit?: number;
  device_limit?: number;
  group_id?: number;
  show?: boolean;
  sell?: boolean;
  renew?: boolean;
  sort?: number;
}

export interface UpdatePlanRequest extends Partial<CreatePlanRequest> {
  id: number;
}

// Create/Update notice request
export interface CreateNoticeRequest {
  title: string;
  content: string;
  img_url?: string;
  show?: boolean;
  popup?: boolean;
  sort?: number;
}

export interface UpdateNoticeRequest extends Partial<CreateNoticeRequest> {
  id: number;
}

// Forwarding rule interface
export interface ForwardingRule {
  id: number;
  agent_host_id: number;
  name: string;
  protocol: "tcp" | "udp" | "both";
  listen_port: number;
  target_address: string;
  target_port: number;
  enabled: boolean;
  priority: number;
  remark: string;
  version: number;
  created_at: number;
  updated_at: number;
}

export interface ForwardingRulesResponse {
  rules: ForwardingRule[];
  version: number;
}

export interface CreateForwardingRuleRequest {
  agent_host_id: number;
  name: string;
  protocol: "tcp" | "udp" | "both";
  listen_port: number;
  target_address: string;
  target_port: number;
  enabled: boolean;
  priority: number;
  remark: string;
}

export type UpdateForwardingRuleRequest = Partial<CreateForwardingRuleRequest>;

export interface ForwardingRuleLog {
  id: number;
  rule_id?: number;
  agent_host_id: number;
  action: "create" | "update" | "delete" | "apply" | "fail";
  operator_id?: number;
  detail: string;
  created_at: number;
}

export interface ForwardingRuleLogsResponse {
  logs: ForwardingRuleLog[];
  total: number;
}

export interface ForwardingRuleLogsParams {
  agent_host_id: number;
  rule_id?: number;
  start_at?: number;
  end_at?: number;
  limit?: number;
  offset?: number;
}

export interface AccessLogEntry {
  id: number;
  user_id?: number;
  user_email: string;
  agent_host_id: number;
  source_ip: string;
  target_domain: string;
  target_ip: string;
  target_port: number;
  protocol: string;
  upload: number;
  download: number;
  connection_start?: number;
  connection_end?: number;
  created_at: number;
}

export interface AccessLogQueryParams {
  limit?: number;
  offset?: number;
  user_id?: number;
  agent_host_id?: number;
  target_domain?: string;
  source_ip?: string;
  protocol?: string;
  start_at?: number;
  end_at?: number;
}

export interface AccessLogListResponse {
  logs: AccessLogEntry[];
  total: number;
}

export interface AccessLogStats {
  total_count: number;
  total_upload: number;
  total_download: number;
}

export interface AccessLogStatsParams {
  user_id?: number;
  agent_host_id?: number;
  start_at?: number;
  end_at?: number;
}
