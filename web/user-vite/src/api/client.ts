import axios, { type AxiosInstance, type InternalAxiosRequestConfig } from "axios";
import { getToken, clearToken, isSamePath, redirectToLogin } from "@/lib/auth";
import { API_VERSION, ROUTES } from "@/lib/constants";

// Get base URL from runtime settings or environment
const getBaseURL = (): string => {
  const settings = window?.settings;
  const baseURL = settings?.base_url || import.meta.env.VITE_API_BASE_URL || "";
  return baseURL.replace(/\/$/, "") + API_VERSION;
};

// Main API instance (requires authentication)
export const api: AxiosInstance = axios.create({
  baseURL: getBaseURL(),
  timeout: 15000,
  withCredentials: true,
});

// Passport API (for login/register, no auth required)
export const passportApi: AxiosInstance = axios.create({
  baseURL: getBaseURL(),
  timeout: 15000,
  withCredentials: true,
});

// Request interceptor: inject token
api.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const token = getToken();
  if (token) {
    config.headers = config.headers || {};
    config.headers.Authorization = token.startsWith("Bearer") ? token : `Bearer ${token}`;
  }
  return config;
});

// Response interceptor: handle errors
api.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error?.response?.status;

    // Needs bootstrap/installation
    if (status === 428 || error?.response?.data?.needs_bootstrap) {
      // Avoid redirect loop if already on install page
      if (!isSamePath(window.location.pathname, ROUTES.INSTALL)) {
        window.location.href = ROUTES.INSTALL;
      }
      return Promise.reject(error);
    }

    // Unauthorized, clear token and redirect to login
    if (status === 401) {
      clearToken();
      redirectToLogin(ROUTES.LOGIN);
      return Promise.reject(error);
    }

    const message = error?.response?.data?.message || error.message || "Request failed";
    return Promise.reject(new Error(message));
  }
);
