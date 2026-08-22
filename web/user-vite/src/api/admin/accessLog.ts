import { adminApi } from "./client";
import type {
  AccessLogEntry,
  AccessLogListResponse,
  AccessLogQueryParams,
  AccessLogStats,
  AccessLogStatsParams,
} from "@/types/admin";

type AccessLogPayload = {
  ID: number;
  UserID: number | null;
  UserEmail: string;
  AgentHostID: number;
  SourceIP: string;
  TargetDomain: string;
  TargetIP: string;
  TargetPort: number;
  Protocol: string;
  Upload: number;
  Download: number;
  ConnectionStart: number | null;
  ConnectionEnd: number | null;
  CreatedAt: number;
};

type AccessLogStatsPayload = {
  TotalCount: number;
  TotalUpload: number;
  TotalDownload: number;
};

const mapAccessLog = (log: AccessLogPayload): AccessLogEntry => ({
  id: log.ID,
  user_id: log.UserID ?? undefined,
  user_email: log.UserEmail,
  agent_host_id: log.AgentHostID,
  source_ip: log.SourceIP,
  target_domain: log.TargetDomain,
  target_ip: log.TargetIP,
  target_port: log.TargetPort,
  protocol: log.Protocol,
  upload: log.Upload,
  download: log.Download,
  connection_start: log.ConnectionStart ?? undefined,
  connection_end: log.ConnectionEnd ?? undefined,
  created_at: log.CreatedAt,
});

const mapAccessLogStats = (stats: AccessLogStatsPayload): AccessLogStats => ({
  total_count: stats.TotalCount,
  total_upload: stats.TotalUpload,
  total_download: stats.TotalDownload,
});

/**
 * Fetch access logs with filters.
 */
export async function fetchAccessLogs(
  params?: AccessLogQueryParams
): Promise<AccessLogListResponse> {
  const response = await adminApi.get<{ total: number; data: AccessLogPayload[] | null }>(
    "/access-logs/fetch",
    { params }
  );
  return {
    logs: (response.data.data || []).map(mapAccessLog),
    total: response.data.total,
  };
}

/**
 * Fetch access log statistics.
 */
export async function getAccessLogStats(params?: AccessLogStatsParams): Promise<AccessLogStats> {
  const response = await adminApi.get<AccessLogStatsPayload>("/access-logs/stats", {
    params,
  });
  return mapAccessLogStats(response.data);
}

/**
 * Cleanup access logs by retention days.
 */
export async function cleanupAccessLogs(): Promise<number> {
  const response = await adminApi.post<{ count: number }>("/access-logs/cleanup");
  return response.data.count;
}
