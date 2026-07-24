const TOKEN_KEY = "xboard-token";
const REFRESH_TOKEN_KEY = "xboard-refresh-token";

// In-memory store — primary token holder, cleared on tab close.
// sessionStorage is a fallback so tokens survive page refresh (SPA reload)
// but not cross-tab or after full browser close (mitigates XSS persistence).
const memoryStore = new Map<string, string>();

function ssGet(key: string): string | null {
  try {
    return sessionStorage.getItem(key);
  } catch {
    return null;
  }
}

function ssSet(key: string, value: string): void {
  try {
    sessionStorage.setItem(key, value);
  } catch {
    // quota exceeded or blocked — memory-only is fine
  }
}

function ssRemove(key: string): void {
  try {
    sessionStorage.removeItem(key);
  } catch {
    // ignore
  }
}

export function getToken(): string | null {
  return memoryStore.get(TOKEN_KEY) ?? ssGet(TOKEN_KEY);
}

export function setToken(token: string): void {
  memoryStore.set(TOKEN_KEY, token);
  ssSet(TOKEN_KEY, token);
}

export function clearToken(): void {
  memoryStore.delete(TOKEN_KEY);
  memoryStore.delete(REFRESH_TOKEN_KEY);
  ssRemove(TOKEN_KEY);
  ssRemove(REFRESH_TOKEN_KEY);
}

export function getRefreshToken(): string | null {
  return memoryStore.get(REFRESH_TOKEN_KEY) ?? ssGet(REFRESH_TOKEN_KEY);
}

export function setRefreshToken(token: string): void {
  memoryStore.set(REFRESH_TOKEN_KEY, token);
  ssSet(REFRESH_TOKEN_KEY, token);
}

const normalizePathname = (path: string): string => {
  if (!path) {
    return "/";
  }
  const trimmed = path.replace(/\/+$/, "");
  return trimmed || "/";
};

export function isSamePath(pathname: string, targetPath: string): boolean {
  return normalizePathname(pathname) === normalizePathname(targetPath);
}

export function redirectToLogin(loginPath: string): void {
  const returnUrl = encodeURIComponent(window.location.pathname + window.location.search);
  if (!isSamePath(window.location.pathname, loginPath)) {
    window.location.href = `${loginPath}?next=${returnUrl}`;
  }
}
