import { passportApi } from "./client";

export interface LoginRequest {
  email: string;
  password: string;
}

export interface LoginResponse {
  data: {
    token: string;
    refresh_token?: string;
    is_admin?: boolean;
  };
}

export interface RegisterRequest {
  email: string;
  password: string;
  email_code?: string;
}

export async function login(data: LoginRequest): Promise<LoginResponse["data"]> {
  const response = await passportApi.post<LoginResponse>("/passport/auth/login", data);
  return response.data.data;
}

export async function register(data: RegisterRequest): Promise<LoginResponse["data"]> {
  const response = await passportApi.post<LoginResponse>("/passport/auth/register", data);
  return response.data.data;
}

export async function logout(): Promise<void> {
  try {
    await passportApi.post("/passport/auth/logout");
  } catch {
    // Ignore logout errors
  }
}

export async function sendEmailVerify(email: string): Promise<void> {
  await passportApi.post("/passport/comm/sendEmailVerify", { email });
}

export async function forgotPassword(email: string, password: string, email_code: string): Promise<void> {
  await passportApi.post("/passport/auth/forget", { email, password, email_code });
}
