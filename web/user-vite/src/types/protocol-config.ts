// Protocol-specific configuration types for type-safe config-center access.

export interface VNextServer {
  address: string;
  port: number;
  users?: Array<{
    id?: string;
    encryption?: string;
    flow?: string;
    security?: string;
  }>;
}

export interface ProxySettingsPeers {
  server: string;
  public_key?: string;
  pre_shared_key?: string;
  allowed_ips?: string;
  endpoint?: string;
}

export interface TLSConfig {
  enabled?: boolean;
  server_name?: string;
  alpn?: string[];
  reality?: {
    enabled?: boolean;
    private_key?: string;
    public_key?: string;
    short_id?: string;
  };
}

export interface ScopeObject {
  inbound?: Record<string, unknown>;
  outbound?: Record<string, unknown>;
  tls?: TLSConfig;
  transport?: Record<string, unknown>;
  [key: string]: unknown;
}
