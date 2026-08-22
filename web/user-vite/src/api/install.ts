import axios from "axios";

// Install API uses a separate base URL without /api/v1 prefix
const getInstallBaseURL = (): string => {
  const settings = window?.settings;
  const baseURL = settings?.base_url || import.meta.env.VITE_API_BASE_URL || "";
  return baseURL.replace(/\/$/, "") + "/api/install";
};

const installApi = axios.create({
  baseURL: getInstallBaseURL(),
  timeout: 15000,
  withCredentials: true,
});

export interface InstallStatus {
  needs_bootstrap: boolean;
}

export interface CreateAdminRequest {
  email?: string;
  username?: string;
  password: string;
}

export interface CreateAdminResponse {
  id: number;
  email: string;
  username: string;
}

/**
 * Check if the system needs bootstrap/installation
 */
export async function getInstallStatus(): Promise<InstallStatus> {
  const response = await installApi.get<{ data: InstallStatus }>("/status");
  return response.data.data || response.data;
}

/**
 * Create the first admin account
 */
export async function createAdmin(data: CreateAdminRequest): Promise<CreateAdminResponse> {
  const response = await installApi.post<{ data: CreateAdminResponse }>("/", data);
  return response.data.data || response.data;
}
