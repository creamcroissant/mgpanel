import { adminApi } from "./client";

export type SettingsMap = Record<string, string>;

export interface SettingsResponse {
  data: SettingsMap;
}

export interface SaveSettingsRequest {
  category: string;
  settings: SettingsMap;
}

export interface SaveSettingsResponse {
  message: string;
}

export interface SMTPTestRequest {
  host: string;
  port: number;
  encryption: string;
  username: string;
  password: string;
  from_address: string;
  to_address: string;
}

export interface SMTPTestResponse {
  message: string;
  data: {
    encryption: string;
    elapsed_ms: number;
  };
}

export interface KeyResponse {
  key: string;
  masked: boolean;
  has_value: boolean;
  impact?: string;
  generated?: boolean;
  last_modified?: number;
}

/**
 * Fetch settings by category
 */
export async function fetchSettings(category: string): Promise<SettingsMap> {
  const response = await adminApi.get<SettingsResponse>("/system/settings", {
    params: { category },
  });
  return response.data.data;
}

/**
 * Save settings by category
 */
export async function saveSettings(
  category: string,
  settings: SettingsMap
): Promise<SaveSettingsResponse> {
  const response = await adminApi.post<SaveSettingsResponse>("/system/settings", {
    category,
    settings,
  });
  return response.data;
}

/**
 * Test SMTP configuration
 */
export async function testSMTP(config: SMTPTestRequest): Promise<SMTPTestResponse> {
  const response = await adminApi.post<SMTPTestResponse>("/system/smtp/test", config);
  return response.data;
}

/**
 * Get masked communication key
 */
export async function getKey(): Promise<KeyResponse> {
  const response = await adminApi.get<KeyResponse>("/system/key");
  return response.data;
}

/**
 * Reveal communication key
 */
export async function revealKey(): Promise<KeyResponse> {
  const response = await adminApi.post<KeyResponse>("/system/key/reveal", {
    confirm: true,
  });
  return response.data;
}

/**
 * Reset communication key
 */
export async function resetKey(): Promise<KeyResponse> {
  const response = await adminApi.post<KeyResponse>("/system/key/reset");
  return response.data;
}
