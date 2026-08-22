/**
 * 入站配置前端类型 — 由 Go struct 生成 + 前端专用扩展。
 *
 * 运行 `make gen-types` 从 Go 代码重新生成基础类型。
 * 生成源: internal/template/unified.go, internal/template/types.go
 */
import type {
  UnifiedTransport,
  UnifiedTLS,
  UnifiedReality,
  UnifiedMultiplex,
  UnifiedBrutal,
  UnifiedSniffing,
} from "./_generated/inbound";

// Re-export generated types
export type { UnifiedTransport, UnifiedTLS, UnifiedReality, UnifiedMultiplex, UnifiedBrutal, UnifiedSniffing };

// ---- Backward-compatible alias names ----
export type InboundTLSSpec = UnifiedTLS;
export type InboundRealitySpec = UnifiedReality;
export type InboundMultiplexSpec = UnifiedMultiplex;
export type InboundBrutalSpec = UnifiedBrutal;
export type InboundSniffingSpec = UnifiedSniffing;

/** Frontend-enhanced transport spec (extends generated type) */
export interface InboundTransportSpec extends UnifiedTransport {
  early_data_header_name?: string;
  max_early_data?: number;
  seed?: string;
  congestion_control?: string;
  packet_encoding?: string;
}

/** Frontend InboundSemanticSpec */
export interface InboundSemanticSpec {
  tag?: string;
  protocol: string;
  listen: string;
  port: number;
  tls?: InboundTLSSpec | null;
  transport?: InboundTransportSpec | null;
  multiplex?: InboundMultiplexSpec | null;
  sniffing?: InboundSniffingSpec | null;
  /** Raw users array — passed through without field-level editing */
  users?: Record<string, unknown>[];
  /** Raw options map — passed through without field-level editing */
  options?: Record<string, unknown>;
  /** Raw fields not explicitly modeled, preserved across parse/serialize */
  _raw?: Record<string, unknown>;
}

// ---- 协议常量 ----
export type SingBoxInboundProtocol =
  | "direct" | "block" | "socks" | "http"
  | "shadowsocks" | "vmess" | "trojan" | "vless"
  | "hysteria" | "hysteria2" | "tuic" | "anytls" | "shadowtls"
  | "dns" | "loopback" | "mixed" | "naive"
  | "tun" | "redirect" | "tproxy" | "wireguard" | "mtproto";

export type XrayInboundProtocol =
  | "socks" | "http"
  | "shadowsocks" | "vmess" | "trojan" | "vless"
  | "hysteria2" | "tuic"
  | "dokodemo-door" | "mixed";

export type InboundProtocol = SingBoxInboundProtocol | XrayInboundProtocol;
export type TransportType = "tcp" | "ws" | "grpc" | "http" | "kcp" | "httpupgrade" | "xhttp" | "quic" | "hysteria";

export const PROTOCOL_OPTIONS: InboundProtocol[] = [
  "vless", "vmess", "trojan", "shadowsocks",
  "socks", "http", "mixed",
  "wireguard", "hysteria", "hysteria2", "tuic",
  "direct", "block", "anytls", "shadowtls", "naive",
  "dns", "loopback",
  "tun", "redirect", "tproxy", "mtproto",
  "dokodemo-door",
];

export const SINGBOX_INBOUND_PROTOCOLS: SingBoxInboundProtocol[] = [
  "direct", "block", "socks", "http",
  "shadowsocks", "vmess", "trojan", "vless",
  "hysteria", "hysteria2", "tuic", "anytls", "shadowtls",
  "dns", "loopback", "mixed", "naive",
  "tun", "redirect", "tproxy", "wireguard", "mtproto",
];

export const XRAY_INBOUND_PROTOCOLS: XrayInboundProtocol[] = [
  "socks", "http",
  "shadowsocks", "vmess", "trojan", "vless",
  "hysteria2", "tuic",
  "dokodemo-door", "mixed",
];

export const TRANSPORT_OPTIONS: TransportType[] = ["tcp", "ws", "grpc", "http", "kcp", "httpupgrade", "xhttp", "quic", "hysteria"];

// ---- Parse / Serialize ----

export function parseSemanticSpec(raw: Record<string, unknown>): InboundSemanticSpec {
  const knownKeys = new Set(["tag", "protocol", "listen", "port", "tls", "transport", "multiplex", "sniffing", "users", "options"]);
  const extras: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(raw)) {
    if (!knownKeys.has(k)) extras[k] = v;
  }
  return {
    tag: typeof raw.tag === "string" ? raw.tag : undefined,
    protocol: String(raw.protocol ?? ""),
    listen: String(raw.listen || "::"),
    port: typeof raw.port === "number" ? raw.port : 0,
    tls: raw.tls && typeof raw.tls === "object" ? parseTLSSpec(raw.tls as Record<string, unknown>) : null,
    transport: raw.transport && typeof raw.transport === "object" ? parseTransportSpec(raw.transport as Record<string, unknown>) : null,
    multiplex: raw.multiplex && typeof raw.multiplex === "object" ? parseMultiplexSpec(raw.multiplex as Record<string, unknown>) : null,
    users: Array.isArray(raw.users) ? raw.users.filter((u): u is Record<string, unknown> => typeof u === "object" && u !== null) : undefined,
    options: raw.options && typeof raw.options === "object" && !Array.isArray(raw.options) ? raw.options as Record<string, unknown> : undefined,
    _raw: Object.keys(extras).length > 0 ? extras : undefined,
  };
}

function parseTLSSpec(raw: Record<string, unknown>): InboundTLSSpec {
  return {
    enabled: raw.enabled !== false,
    server_name: typeof raw.server_name === "string" ? raw.server_name : undefined,
    alpn: Array.isArray(raw.alpn) ? raw.alpn.map(String) : undefined,
    cert_path: typeof raw.cert_path === "string" ? raw.cert_path : undefined,
    key_path: typeof raw.key_path === "string" ? raw.key_path : undefined,
    reality: raw.reality && typeof raw.reality === "object" ? parseRealitySpec(raw.reality as Record<string, unknown>) : null,
  };
}

function parseRealitySpec(raw: Record<string, unknown>): InboundRealitySpec {
  return {
    enabled: raw.enabled !== false,
    private_key: typeof raw.private_key === "string" ? raw.private_key : undefined,
    public_key: typeof raw.public_key === "string" ? raw.public_key : undefined,
    short_ids: Array.isArray(raw.short_ids) ? raw.short_ids.map(String) : undefined,
    server_names: Array.isArray(raw.server_names) ? raw.server_names.map(String) : undefined,
    handshake_server: typeof raw.handshake_server === "string" ? raw.handshake_server : undefined,
    handshake_port: typeof raw.handshake_port === "number" ? raw.handshake_port : undefined,
    fingerprint: typeof raw.fingerprint === "string" ? raw.fingerprint : undefined,
  };
}

function parseTransportSpec(raw: Record<string, unknown>): InboundTransportSpec {
  return {
    type: String(raw.type ?? "tcp"),
    path: typeof raw.path === "string" ? raw.path : undefined,
    host: typeof raw.host === "string" ? raw.host : undefined,
    service_name: typeof raw.service_name === "string" ? raw.service_name : undefined,
    headers: raw.headers && typeof raw.headers === "object" ? raw.headers as Record<string, string> : undefined,
    mode: typeof raw.mode === "string" ? raw.mode : undefined,
    early_data_header_name: typeof raw.early_data_header_name === "string" ? raw.early_data_header_name : undefined,
    max_early_data: typeof raw.max_early_data === "number" ? raw.max_early_data : undefined,
    seed: typeof raw.seed === "string" ? raw.seed : undefined,
    congestion_control: typeof raw.congestion_control === "string" ? raw.congestion_control : undefined,
    packet_encoding: typeof raw.packet_encoding === "string" ? raw.packet_encoding : undefined,
  };
}

function parseMultiplexSpec(raw: Record<string, unknown>): InboundMultiplexSpec {
  return {
    enabled: raw.enabled !== false,
    protocol: typeof raw.protocol === "string" ? raw.protocol : undefined,
    max_streams: typeof raw.max_streams === "number" ? raw.max_streams : undefined,
    padding: typeof raw.padding === "boolean" ? raw.padding : undefined,
    brutal: raw.brutal && typeof raw.brutal === "object" ? {
      enabled: (raw.brutal as Record<string, unknown>).enabled !== false,
      up_mbps: typeof (raw.brutal as Record<string, unknown>).up_mbps === "number" ? (raw.brutal as Record<string, unknown>).up_mbps as number : undefined,
      down_mbps: typeof (raw.brutal as Record<string, unknown>).down_mbps === "number" ? (raw.brutal as Record<string, unknown>).down_mbps as number : undefined,
    } : null,
  };
}

export function serializeSemanticSpec(spec: InboundSemanticSpec): Record<string, unknown> {
  const result: Record<string, unknown> = {
    protocol: spec.protocol,
    listen: spec.listen,
    port: spec.port,
  };
  if (spec.tag) result.tag = spec.tag;
  if (spec.tls) result.tls = serializeTLSSpec(spec.tls);
  if (spec.transport) result.transport = serializeTransportSpec(spec.transport);
  if (spec.multiplex) result.multiplex = serializeMultiplexSpec(spec.multiplex);
  if (spec.users) result.users = spec.users;
  if (spec.options) result.options = spec.options;
  if (spec._raw) {
    for (const [k, v] of Object.entries(spec._raw)) {
      result[k] = v;
    }
  }
  return result;
}

function serializeTLSSpec(tls: InboundTLSSpec): Record<string, unknown> | undefined {
  const r: Record<string, unknown> = {};
  r.enabled = tls.enabled !== false;
  if (tls.server_name) r.server_name = tls.server_name;
  if (tls.alpn?.length) r.alpn = tls.alpn;
  if (tls.cert_path) r.cert_path = tls.cert_path;
  if (tls.key_path) r.key_path = tls.key_path;
  if (tls.reality) r.reality = serializeRealitySpec(tls.reality);
  return Object.keys(r).length > 1 ? r : undefined;
}

function serializeRealitySpec(rl: InboundRealitySpec): Record<string, unknown> | undefined {
  const r: Record<string, unknown> = {};
  r.enabled = rl.enabled !== false;
  if (rl.private_key) r.private_key = rl.private_key;
  if (rl.public_key) r.public_key = rl.public_key;
  if (rl.short_ids?.length) r.short_ids = rl.short_ids;
  if (rl.server_names?.length) r.server_names = rl.server_names;
  if (rl.handshake_server) r.handshake_server = rl.handshake_server;
  if (rl.handshake_port) r.handshake_port = rl.handshake_port;
  if (rl.fingerprint) r.fingerprint = rl.fingerprint;
  return r;
}

function serializeTransportSpec(tp: InboundTransportSpec): Record<string, unknown> | undefined {
  const r: Record<string, unknown> = { type: tp.type };
  if (tp.path) r.path = tp.path;
  if (tp.host) r.host = tp.host;
  if (tp.service_name) r.service_name = tp.service_name;
  if (tp.headers && Object.keys(tp.headers).length) r.headers = tp.headers;
  if (tp.mode) r.mode = tp.mode;
  if (tp.early_data_header_name) r.early_data_header_name = tp.early_data_header_name;
  if (tp.max_early_data) r.max_early_data = tp.max_early_data;
  if (tp.seed) r.seed = tp.seed;
  if (tp.congestion_control) r.congestion_control = tp.congestion_control;
  if (tp.packet_encoding) r.packet_encoding = tp.packet_encoding;
  return Object.keys(r).length > 1 ? r : undefined;
}

function serializeMultiplexSpec(mp: InboundMultiplexSpec): Record<string, unknown> | undefined {
  const r: Record<string, unknown> = {};
  r.enabled = mp.enabled !== false;
  if (mp.protocol) r.protocol = mp.protocol;
  if (mp.max_streams) r.max_streams = mp.max_streams;
  if (mp.padding === true) r.padding = true;
  if (mp.brutal) {
    const br: Record<string, unknown> = {};
    br.enabled = mp.brutal.enabled !== false;
    if (mp.brutal.up_mbps) br.up_mbps = mp.brutal.up_mbps;
    if (mp.brutal.down_mbps) br.down_mbps = mp.brutal.down_mbps;
    if (Object.keys(br).length) r.brutal = br;
  }
  return Object.keys(r).length ? r : undefined;
}
