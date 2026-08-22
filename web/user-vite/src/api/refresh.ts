import { passportApi } from "./client";
import { getRefreshToken, setToken, setRefreshToken, clearToken } from "@/lib/auth";

let refreshing: Promise<boolean> | null = null;

export async function refreshToken(): Promise<boolean> {
  if (refreshing) {
    return refreshing;
  }

  refreshing = (async () => {
    const refreshTokenValue = getRefreshToken();
    if (!refreshTokenValue) {
      return false;
    }

    try {
      const response = await passportApi.post("/passport/auth/refresh", {
        refresh_token: refreshTokenValue,
      });
      const { token, refresh_token } = response.data.data;
      setToken(token);
      if (refresh_token) {
        setRefreshToken(refresh_token);
      }
      return true;
    } catch {
      clearToken();
      return false;
    } finally {
      refreshing = null;
    }
  })();

  return refreshing;
}
