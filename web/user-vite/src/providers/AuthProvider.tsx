/* eslint-disable react-refresh/only-export-components */
import {
  createContext,
  useContext,
  useState,
  useCallback,
  type ReactNode,
} from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { getToken, setToken, clearToken } from "@/lib/auth";
import { fetchUserInfo } from "@/api/user";
import { logout as apiLogout } from "@/api/auth";
import { QUERY_KEYS } from "@/lib/constants";
import type { UserProfile } from "@/types";

interface AuthContextType {
  isAuthenticated: boolean;
  isLoading: boolean;
  isAdmin: boolean;
  user: UserProfile | null;
  login: (token: string) => void;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

interface AuthProviderProps {
  children: ReactNode;
}

export function AuthProvider({ children }: AuthProviderProps) {
  const queryClient = useQueryClient();
  const [token, setTokenState] = useState<string | null>(getToken());

  const { data: user, isLoading } = useQuery<UserProfile, Error>({
    queryKey: QUERY_KEYS.USER_INFO,
    queryFn: fetchUserInfo,
    enabled: !!token,
    retry: false,
  });

  const login = useCallback(
    (newToken: string) => {
      setToken(newToken);
      setTokenState(newToken);
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.USER });
    },
    [queryClient]
  );

  const logout = useCallback(async () => {
    try {
      await apiLogout();
    } catch {
      // Ignore logout errors
    }
    clearToken();
    setTokenState(null);
    queryClient.clear();
  }, [queryClient]);

  return (
    <AuthContext.Provider
      value={{
        isAuthenticated: !!token,
        isLoading: isLoading && !!token,
        isAdmin: user?.is_admin ?? false,
        user: user ?? null,
        login,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within AuthProvider");
  }
  return context;
}
