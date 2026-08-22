import axios, { type AxiosInstance, type InternalAxiosRequestConfig } from "axios";
import { getToken, clearToken, redirectToLogin } from "@/lib/auth";
import { ADMIN_API_VERSION, ADMIN_AUTH_ROUTES, ROUTES } from "@/lib/constants";

// Get base URL for admin API
export const getAdminBaseURL = (): string => {
  const settings = window?.settings;
  const baseURL = settings?.base_url || import.meta.env.VITE_API_BASE_URL || "";
  return baseURL.replace(/\/$/, "") + ADMIN_API_VERSION;
};

export function getAdminAuthorizationHeader(): string | undefined {
  const token = getToken();
  if (!token) {
    return undefined;
  }
  return token.startsWith("Bearer") ? token : `Bearer ${token}`;
}

export function getAdminFetchHeaders(init?: HeadersInit): Headers {
  const headers = new Headers(init);
  const authorization = getAdminAuthorizationHeader();
  if (authorization) {
    headers.set("Authorization", authorization);
  }
  return headers;
}

export function getAdminApiURL(path: string): string {
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  return `${getAdminBaseURL()}${normalizedPath}`;
}

type AdminApiErrorPayload = {
  error?: string;
  message?: string;
  action?: string;
  details?: unknown;
  error_str?: string;
};

export class AdminApiError extends Error {
  status?: number;
  action?: string;
  details?: unknown;
  error_str?: string;

  constructor(
    message: string,
    options: {
      status?: number;
      action?: string;
      details?: unknown;
  error_str?: string;
    } = {}
  ) {
    super(message);
    this.name = "AdminApiError";
    this.status = options.status;
    this.action = options.action;
    this.details = options.details;
    this.error_str = options.error_str;
    Object.setPrototypeOf(this, AdminApiError.prototype);
  }
}

export function isAdminApiError(error: unknown): error is AdminApiError {
  return error instanceof AdminApiError;
}

// Admin API instance (requires admin authentication)
export const adminApi: AxiosInstance = axios.create({
  baseURL: getAdminBaseURL(),
  timeout: 15000,
  withCredentials: true,
});

// Request interceptor: inject token
adminApi.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const token = getToken();
  if (token) {
    config.headers = config.headers || {};
    config.headers.Authorization = token.startsWith("Bearer") ? token : `Bearer ${token}`;
  }
  return config;
});

// Response interceptor: handle errors
adminApi.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error?.response?.status;

    // Unauthorized or Forbidden, redirect to login
    if (status === 401) {
      clearToken();
      redirectToLogin(ADMIN_AUTH_ROUTES.LOGIN);
      return Promise.reject(error);
    }

    // Forbidden (not admin)
    if (status === 403) {
      window.location.href = ROUTES.DASHBOARD;
      return Promise.reject(error);
    }

    const payload = error?.response?.data as AdminApiErrorPayload | undefined;
    const message = payload?.error || payload?.message || error.message || "Request failed";
    return Promise.reject(
      new AdminApiError(message, {
        status,
        action: payload?.action,
        details: payload?.details,
        error_str: payload?.error_str,
      })
    );
  }
);
