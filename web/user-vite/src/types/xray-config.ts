/**
 * Xray / sing-box Core Config Type Definitions
 *
 * Typed interfaces for core config sections (policy, experimental, NTP, etc.)
 * to eliminate `as any` type erasures in the config-center UI.
 */

// ---- Policy ----

export interface PolicyLevelConfig {
  connIdle?: number;
  handshake?: number;
  [key: string]: unknown;
}

export interface PolicySystemConfig {
  statsInboundUplink?: boolean;
  statsInboundDownlink?: boolean;
  [key: string]: unknown;
}

export interface PolicyConfig {
  levels?: Record<string, PolicyLevelConfig>;
  system?: PolicySystemConfig;
  [key: string]: unknown;
}

// ---- Experimental (sing-box) ----

export interface CacheFileConfig {
  enabled?: boolean;
  path?: string;
  [key: string]: unknown;
}

export interface ExperimentalConfig {
  cache_file?: CacheFileConfig;
  [key: string]: unknown;
}

// ---- NTP (sing-box) ----

export interface NTPConfig {
  enabled?: boolean;
  server?: string;
  [key: string]: unknown;
}

// ---- Reality (Xray streamSettings.realitySettings) ----

export interface RealitySettings {
  publicKey?: string;
  privateKey?: string;
  shortIds?: string[];
  serverNames?: string[];
  fingerprint?: string;
  dest?: string;
  [key: string]: unknown;
}

// ---- StreamSettings (Xray) ----

export interface XrayStreamSettings {
  network?: string;
  security?: string;
  tlsSettings?: Record<string, unknown>;
  realitySettings?: RealitySettings;
  [key: string]: unknown;
}

// ---- Log ----

export interface LogConfig {
  loglevel?: string;
  level?: string;
  output?: string;
  disabled?: boolean;
  [key: string]: unknown;
}

// ---- API ----

export interface ApiConfig {
  tag?: string;
  listen?: string;
  services?: string[];
  [key: string]: unknown;
}
