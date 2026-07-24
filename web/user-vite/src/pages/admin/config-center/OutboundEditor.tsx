import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button, Input, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, Textarea } from "@/components/ui";
import { InboundTLSFields } from "./InboundTLSFields";
import { InboundTransportFields } from "./InboundTransportFields";
import { InboundMultiplexFields } from "./InboundMultiplexFields";
import type { InboundTLSSpec, InboundTransportSpec, InboundMultiplexSpec } from "@/types/configCenterInbound";
import type { ConfigCenterCoreType } from "@/types/configCenter";
import type { VNextServer, ProxySettingsPeers } from "@/types/protocol-config";

// sing-box supported outbound protocols
const SINGBOX_OUTBOUND_PROTOCOLS = [
  "freedom", "blackhole", "socks", "http", "shadowsocks",
  "trojan", "vless", "vmess", "wireguard", "dns", "loopback",
  "direct", "block",
] as const;

// xray supported outbound protocols
const XRAY_OUTBOUND_PROTOCOLS = [
  "freedom", "blackhole", "socks", "http", "shadowsocks",
  "trojan", "vless", "vmess", "dns",
] as const;

const ALL_OUTBOUND_PROTOCOLS = [
  ...new Set([...SINGBOX_OUTBOUND_PROTOCOLS, ...XRAY_OUTBOUND_PROTOCOLS]),
] as const;

type OutboundProtocol = (typeof ALL_OUTBOUND_PROTOCOLS)[number];

interface OutboundConfigData {
  protocol?: string;
  settings?: Record<string, unknown>;
  tag?: string;
  streamSettings?: Record<string, unknown>;
  mux?: Record<string, unknown>;
  [key: string]: unknown;
}

interface OutboundEditorProps {
  value: string;  // JSON string of config_data
  onChange: (jsonString: string) => void;
  coreType?: ConfigCenterCoreType;
}

export function OutboundEditor({ value, onChange, coreType }: OutboundEditorProps) {
  const { t } = useTranslation();
  const [showAdvanced, setShowAdvanced] = useState(false);

  // Filter protocols based on selected core type
  const outboundProtocols = useMemo(() => {
    if (!coreType) return ALL_OUTBOUND_PROTOCOLS;
    return (coreType === "xray" ? XRAY_OUTBOUND_PROTOCOLS : SINGBOX_OUTBOUND_PROTOCOLS) as readonly string[];
  }, [coreType]);

  const parsed = useMemo<OutboundConfigData>(() => {
    try {
      return JSON.parse(value) as OutboundConfigData;
    } catch {
      return {};
    }
  }, [value]);

  const protocol = (outboundProtocols.includes(parsed.protocol as OutboundProtocol)
    ? parsed.protocol
    : "") as OutboundProtocol | "";

  const update = (partial: Partial<OutboundConfigData>) => {
    const next = {
      ...parsed,
      ...partial,
      // Ensure tag stays synced from the form-level tag field — we handle it in CoreConfigTab
    };
    // Remove undefined keys
    for (const key of Object.keys(next)) {
      if (next[key] === undefined) delete next[key];
    }
    // Clean empty settings
    if (next.settings && typeof next.settings === "object" && Object.keys(next.settings).length === 0) {
      delete next.settings;
    }
    onChange(JSON.stringify(next, null, 2));
  };

  const setProtocol = (p: OutboundProtocol) => {
    const settings = getDefaultSettings(p);
    const streamSettings = ["trojan", "vless", "vmess", "shadowsocks", "socks", "http"].includes(p)
      ? (parsed.streamSettings ?? {})
      : undefined;
    update({ protocol: p, settings, streamSettings });
  };

  const setSettingsField = (key: string, value: unknown) => {
    update({ settings: { ...(parsed.settings ?? {}), [key]: value } });
  };


  const parsedTlsSpec = useMemo<InboundTLSSpec | null>(() => {
    if (!parsed.streamSettings) return null;
    const tls = parsed.streamSettings.tlsSettings as Record<string, unknown> | undefined;
    const reality = parsed.streamSettings.realitySettings as Record<string, unknown> | undefined;
    if (tls || reality) {
      return {
        enabled: true,
        server_name: (tls?.serverName ?? tls?.server_name ?? reality?.serverName ?? "") as string,
        alpn: (tls?.alpn ?? []) as string[],
        reality: reality
          ? {
              enabled: true,
              private_key: reality.privateKey as string,
              public_key: reality.public_key as string,
              short_ids: (reality.shortIds ?? reality.shortId ? [reality.shortId as string] : []) as string[],
              server_names: (reality.serverNames ?? []) as string[],
              handshake_server: reality.handshakeServer as string,
              handshake_port: reality.handshakePort as number,
              fingerprint: reality.fingerprint as string,
            }
          : null,
      };
    }
    return null;
  }, [parsed.streamSettings]);

  const handleTlsChange = (tls: InboundTLSSpec | null) => {
    const ss = { ...(parsed.streamSettings ?? {}) };
    if (!tls) {
      delete ss.tlsSettings;
      delete ss.realitySettings;
    } else if (tls.reality?.enabled) {
      ss.realitySettings = {
        serverName: tls.reality.server_names?.join(", ") || tls.server_name,
        fingerprint: tls.reality.fingerprint,
        privateKey: tls.reality.private_key,
        publicKey: tls.reality.public_key,
        shortIds: tls.reality.short_ids,
        handshakeServer: tls.reality.handshake_server,
        handshakePort: tls.reality.handshake_port,
      };
      delete ss.tlsSettings;
    } else {
      ss.tlsSettings = {
        serverName: tls.server_name,
        alpn: tls.alpn,
      };
      delete ss.realitySettings;
    }
    update({ streamSettings: Object.keys(ss).length > 0 ? ss : undefined });
  };

  const parsedTransportSpec = useMemo(() => {
    if (!parsed.streamSettings?.network) return null;
    const ws = parsed.streamSettings.wsSettings as Record<string, unknown> | undefined;
    const grpc = parsed.streamSettings.grpcSettings as Record<string, unknown> | undefined;
    const xhttp = parsed.streamSettings.xhttpSettings as Record<string, unknown> | undefined;
    return {
      type: parsed.streamSettings.network as string,
      path: (ws?.path ?? grpc?.serviceName) as string | undefined,
      host: ws?.host as string | undefined,
      service_name: grpc?.serviceName as string | undefined,
      headers: ws?.headers as Record<string, string> | undefined,
      mode: xhttp?.mode as string | undefined,
    };
  }, [parsed.streamSettings]);

  const handleTransportChange = (tp: InboundTransportSpec | null) => {
    const ss = { ...(parsed.streamSettings ?? {}) };
    if (!tp) {
      delete ss.network;
      delete ss.wsSettings;
      delete ss.grpcSettings;
      delete ss.xhttpSettings;
      delete ss.httpupgradeSettings;
    } else {
      ss.network = tp.type;
      // Clear previous transport settings
      delete ss.wsSettings;
      delete ss.grpcSettings;
      delete ss.xhttpSettings;
      delete ss.httpupgradeSettings;
      if (tp.type === "ws") {
        ss.wsSettings = { path: tp.path, host: tp.host, headers: tp.headers };
      } else if (tp.type === "grpc") {
        ss.grpcSettings = { serviceName: tp.service_name };
      } else if (tp.type === "xhttp") {
        ss.xhttpSettings = { path: tp.path, host: tp.host, mode: tp.mode };
      } else if (tp.type === "httpupgrade") {
        ss.httpupgradeSettings = { path: tp.path, host: tp.host };
      }
    }
    update({ streamSettings: Object.keys(ss).length > 0 ? ss : undefined });
  };

  const parsedMuxSpec = useMemo(() => {
    if (!parsed.mux?.enabled) return null;
    return {
      enabled: true,
      protocol: parsed.mux.protocol as string | undefined,
      max_streams: parsed.mux.concurrency as number | undefined,
    };
  }, [parsed.mux]);

  const handleMuxChange = (mp: InboundMultiplexSpec | null) => {
    if (!mp?.enabled) {
      update({ mux: undefined });
    } else {
      update({ mux: { enabled: true, concurrency: mp.max_streams, protocol: mp.protocol } });
    }
  };

  // Show protocol-specific settings form
  const renderSettings = () => {
    if (!protocol) return null;
    switch (protocol) {
      case "freedom":
        return <FreedomSettings settings={parsed.settings ?? {}} onChange={setSettingsField} />;
      case "blackhole":
        return <BlackholeSettings settings={parsed.settings ?? {}} onChange={setSettingsField} />;
      case "socks":
      case "http":
        return <AuthSettings protocol={protocol} settings={parsed.settings ?? {}} onChange={setSettingsField} />;
      case "shadowsocks":
        return <ShadowSocksSettings settings={parsed.settings ?? {}} onChange={setSettingsField} />;
      case "trojan":
        return <TrojanSettings settings={parsed.settings ?? {}} onChange={setSettingsField} />;
      case "vless":
        return <VlessSettings settings={parsed.settings ?? {}} onChange={setSettingsField} />;
      case "vmess":
        return <VmessSettings settings={parsed.settings ?? {}} onChange={setSettingsField} />;
      case "wireguard":
        return <WireGuardSettings settings={parsed.settings ?? {}} onChange={setSettingsField} />;
      case "dns":
        return <DnsOutSettings settings={parsed.settings ?? {}} onChange={setSettingsField} />;
      case "loopback":
        return <LoopbackSettings settings={parsed.settings ?? {}} onChange={setSettingsField} />;
    }
  };

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <label className="text-sm font-medium">{t("admin.configCenter.inbound.protocol")}</label>
          <Select value={protocol} onValueChange={(v) => setProtocol(v as OutboundProtocol)}>
            <SelectTrigger>
              <SelectValue placeholder={t("admin.configCenter.inbound.selectProtocol")} />
            </SelectTrigger>
            <SelectContent>
              {outboundProtocols.map((p) => (
                <SelectItem key={p} value={p}>{p}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      {protocol && (
        <>
          <div className="rounded-md border bg-muted/20 p-4 space-y-4">
            <h3 className="text-sm font-semibold">{t("admin.configCenter.inbound.settings")}</h3>
            {renderSettings()}
          </div>

          {["trojan", "vless", "vmess", "shadowsocks", "socks", "http"].includes(protocol) && (
            <>
              <InboundTLSFields value={parsedTlsSpec} onChange={handleTlsChange} />
              <InboundTransportFields value={parsedTransportSpec} onChange={handleTransportChange} />
              <InboundMultiplexFields value={parsedMuxSpec} onChange={handleMuxChange} />
            </>
          )}
        </>
      )}

      {/* Fallback raw JSON */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <label className="text-sm font-medium">{t("admin.configCenter.advancedJson.title")}</label>
          <Button type="button" size="sm" variant="outline" onClick={() => setShowAdvanced((p) => !p)}>
            {showAdvanced ? t("admin.configCenter.advancedJson.hide") : t("admin.configCenter.advancedJson.show")}
          </Button>
        </div>
        {showAdvanced && (
          <Textarea
            className="min-h-[200px] font-mono text-xs"
            value={value}
            onChange={(e) => onChange(e.target.value)}
            placeholder="{}"
          />
        )}
      </div>
    </div>
  );
}

// ---------- Protocol sub-forms ----------

function ServerTargetFields({ settings, onChange }: { settings: Record<string, unknown>; onChange: (k: string, v: unknown) => void }) {
  const { t } = useTranslation();
  const s = settings as Record<string, unknown>;
  // Support both flat and vnext/server format
  const address = typeof s.address === "string" ? s.address : ((s.vnext as VNextServer[] | undefined)?.[0]?.address ?? "");
  const port = typeof s.port === "number" ? s.port : ((s.vnext as VNextServer[] | undefined)?.[0]?.port ?? 443);

  return (
    <div className="grid grid-cols-2 gap-4">
      <div className="space-y-2">
        <label className="text-sm font-medium">{t("admin.configCenter.inbound.address")}</label>
        <Input value={address} onChange={(e) => onChange("address", e.target.value)} placeholder="example.com" />
      </div>
      <div className="space-y-2">
        <label className="text-sm font-medium">{t("admin.configCenter.inbound.port")}</label>
        <Input type="number" value={port} onChange={(e) => onChange("port", parseInt(e.target.value) || 443)} min={1} max={65535} />
      </div>
    </div>
  );
}

function FreedomSettings({ settings, onChange }: { settings: Record<string, unknown>; onChange: (k: string, v: unknown) => void }) {
  const { t } = useTranslation();
  return (
    <div className="grid grid-cols-2 gap-4">
      <div className="space-y-2">
        <label className="text-sm font-medium">{t("admin.configCenter.inbound.strategy")}</label>
        <Select value={(settings.domainStrategy as string) ?? "AsIs"} onValueChange={(v) => onChange("domainStrategy", v)}>
          <SelectTrigger><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="AsIs">AsIs</SelectItem>
            <SelectItem value="UseIP">UseIP</SelectItem>
            <SelectItem value="UseIPv4">UseIPv4</SelectItem>
            <SelectItem value="UseIPv6">UseIPv6</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-2">
        <label className="text-sm font-medium">{t("admin.configCenter.inbound.redirect")}</label>
        <Input value={(settings.redirect as string) ?? ""} onChange={(e) => onChange("redirect", e.target.value || undefined)} placeholder="127.0.0.1:1234" />
      </div>
    </div>
  );
}

function BlackholeSettings({ settings, onChange }: { settings: Record<string, unknown>; onChange: (k: string, v: unknown) => void }) {
  const { t } = useTranslation();
  return (
    <div className="grid grid-cols-2 gap-4">
      <div className="space-y-2">
        <label className="text-sm font-medium">{t("admin.configCenter.inbound.response")}</label>
        <Select value={(settings.type as string) ?? "none"} onValueChange={(v) => onChange("type", v)}>
          <SelectTrigger><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="none">{t("admin.configCenter.inbound.tlsNone")}</SelectItem>
            <SelectItem value="http">HTTP</SelectItem>
          </SelectContent>
        </Select>
      </div>
    </div>
  );
}

function AuthSettings({ settings, onChange }: { protocol: string; settings: Record<string, unknown>; onChange: (k: string, v: unknown) => void }) {
  const { t } = useTranslation();
  return (
    <>
      <ServerTargetFields settings={settings} onChange={onChange} />
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <label className="text-sm font-medium">{t("admin.configCenter.inbound.username")}</label>
          <Input value={(settings.user as string) ?? ""} onChange={(e) => onChange("user", e.target.value || undefined)} />
        </div>
        <div className="space-y-2">
          <label className="text-sm font-medium">{t("admin.configCenter.inbound.password")}</label>
          <Input type="password" value={(settings.pass as string) ?? ""} onChange={(e) => onChange("pass", e.target.value || undefined)} />
        </div>
      </div>
    </>
  );
}

function ShadowSocksSettings({ settings, onChange }: { settings: Record<string, unknown>; onChange: (k: string, v: unknown) => void }) {
  const { t } = useTranslation();
  return (
    <>
      <ServerTargetFields settings={settings} onChange={onChange} />
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <label className="text-sm font-medium">{t("admin.configCenter.inbound.method")}</label>
          <Select value={(settings.method as string) ?? "aes-256-gcm"} onValueChange={(v) => onChange("method", v)}>
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              {["aes-256-gcm", "aes-128-gcm", "chacha20-ietf-poly1305", "xchacha20-ietf-poly1305", "2022-blake3-aes-256-gcm", "2022-blake3-aes-128-gcm", "none"].map((m) => (
                <SelectItem key={m} value={m}>{m}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <label className="text-sm font-medium">{t("admin.configCenter.inbound.password")} / Key</label>
          <Input value={(settings.password as string) ?? ""} onChange={(e) => onChange("password", e.target.value || undefined)} />
        </div>
      </div>
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" checked={(settings.uot as boolean) ?? false} onChange={(e) => onChange("uot", e.target.checked || undefined)} className="h-4 w-4" />
        UDP over TCP
      </label>
    </>
  );
}

function TrojanSettings({ settings, onChange }: { settings: Record<string, unknown>; onChange: (k: string, v: unknown) => void }) {
  const { t } = useTranslation();
  return (
    <>
      <ServerTargetFields settings={settings} onChange={onChange} />
      <div className="space-y-2">
        <label className="text-sm font-medium">{t("admin.configCenter.inbound.password")}</label>
        <Input value={(settings.password as string) ?? ""} onChange={(e) => onChange("password", e.target.value || undefined)} placeholder="trojan-password" />
      </div>
    </>
  );
}

function VlessSettings({ settings, onChange }: { settings: Record<string, unknown>; onChange: (k: string, v: unknown) => void }) {
  const { t } = useTranslation();
  const s = settings as Record<string, unknown>;
  const vnext = (s.vnext as VNextServer[] | undefined) ?? [];
  const user = vnext[0]?.users?.[0] ?? {};
  return (
    <>
      <ServerTargetFields settings={settings} onChange={onChange} />
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <label className="text-sm font-medium">{t("admin.configCenter.outbound.uuid")}</label>
          <Input value={(user.id as string) ?? ""} onChange={(e) => {
            const users = [{ ...user, id: e.target.value || undefined }];
            onChange("vnext", [{ ...vnext[0], users }]);
          }} placeholder="uuid" />
        </div>
        <div className="space-y-2">
          <label className="text-sm font-medium">{t("admin.configCenter.inbound.encryption")}</label>
          <Select value={(user.encryption as string) ?? "none"} onValueChange={(v) => {
            const users = [{ ...user, encryption: v }];
            onChange("vnext", [{ ...vnext[0], users }]);
          }}>
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="none">none</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
    </>
  );
}

function VmessSettings({ settings, onChange }: { settings: Record<string, unknown>; onChange: (k: string, v: unknown) => void }) {
  const { t } = useTranslation();
  const s = settings as Record<string, unknown>;
  const vnext = (s.vnext as VNextServer[] | undefined) ?? [];
  const user = vnext[0]?.users?.[0] ?? {};
  return (
    <>
      <ServerTargetFields settings={settings} onChange={onChange} />
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <label className="text-sm font-medium">{t("admin.configCenter.outbound.uuid")}</label>
          <Input value={(user.id as string) ?? ""} onChange={(e) => {
            const users = [{ ...user, id: e.target.value || undefined }];
            onChange("vnext", [{ ...vnext[0], users }]);
          }} placeholder="uuid" />
        </div>
        <div className="space-y-2">
          <label className="text-sm font-medium">{t("admin.configCenter.inbound.security")}</label>
          <Select value={(user.security as string) ?? "auto"} onValueChange={(v) => {
            const users = [{ ...user, security: v }];
            onChange("vnext", [{ ...vnext[0], users }]);
          }}>
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              {["auto", "aes-128-gcm", "chacha20-poly1305", "none", "zero"].map((m) => (
                <SelectItem key={m} value={m}>{m}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
    </>
  );
}

function WireGuardSettings({ settings, onChange }: { settings: Record<string, unknown>; onChange: (k: string, v: unknown) => void }) {
  const { t } = useTranslation();
  const peers = (settings.peers as ProxySettingsPeers[] | undefined) ?? [];
  const peer = peers[0] ?? {};
  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <label className="text-sm font-medium">{t("admin.configCenter.inbound.address")}</label>
          <Input value={(settings.address as string) ?? ""} onChange={(e) => onChange("address", e.target.value || undefined)} placeholder="10.0.0.1/24" />
        </div>
        <div className="space-y-2">
          <label className="text-sm font-medium">{t("admin.configCenter.inbound.privateKey")}</label>
          <Input value={(settings.secretKey as string) ?? ""} onChange={(e) => onChange("secretKey", e.target.value || undefined)} />
        </div>
      </div>
      <div className="rounded-md border border-border p-3 space-y-3">
        <h4 className="text-sm font-medium">{t("admin.configCenter.outbound.peer")}</h4>
        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.configCenter.inbound.endpoint")}</label>
            <Input value={(peer.endpoint as string) ?? ""} onChange={(e) => {
              const newPeers = [{ ...peer, endpoint: e.target.value || undefined }];
              onChange("peers", newPeers);
            }} placeholder="example.com:51820" />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.configCenter.inbound.publicKey")}</label>
            <Input value={(peer.public_key as string) ?? ""} onChange={(e) => {
              const newPeers = [{ ...peer, public_key: e.target.value || undefined }];
              onChange("peers", newPeers);
            }} />
          </div>
        </div>
      </div>
    </div>
  );
}

function DnsOutSettings({ settings, onChange }: { settings: Record<string, unknown>; onChange: (k: string, v: unknown) => void }) {
  const { t } = useTranslation();
  return (
    <div className="grid grid-cols-2 gap-4">
      <div className="space-y-2">
        <label className="text-sm font-medium">{t("admin.configCenter.inbound.rewriteAddress")}</label>
        <Input value={(settings.rewriteAddress as string) ?? ""} onChange={(e) => onChange("rewriteAddress", e.target.value || undefined)} placeholder="1.1.1.1" />
      </div>
      <div className="space-y-2">
        <label className="text-sm font-medium">{t("admin.configCenter.inbound.rewritePort")}</label>
        <Input type="number" value={(settings.rewritePort as number) ?? ""} onChange={(e) => onChange("rewritePort", parseInt(e.target.value) || undefined)} />
      </div>
    </div>
  );
}

function LoopbackSettings({ settings, onChange }: { settings: Record<string, unknown>; onChange: (k: string, v: unknown) => void }) {
  const { t } = useTranslation();
  return (
    <div className="space-y-2">
      <label className="text-sm font-medium">{t("admin.configCenter.inbound.inboundTag")}</label>
      <Input value={(settings.inboundTag as string) ?? ""} onChange={(e) => onChange("inboundTag", e.target.value || undefined)} placeholder="inbound-tag-name" />
    </div>
  );
}

function getDefaultSettings(protocol: string): Record<string, unknown> | undefined {
  switch (protocol) {
    case "freedom":
      return { domainStrategy: "AsIs" };
    case "blackhole":
      return { type: "none" };
    case "socks":
    case "http":
      return { address: "", port: 1080 };
    case "shadowsocks":
      return { address: "", port: 443, method: "aes-256-gcm", password: "" };
    case "trojan":
      return { address: "", port: 443, password: "" };
    case "vless":
      return { vnext: [{ address: "", port: 443, users: [{ id: "", encryption: "none" }] }] };
    case "vmess":
      return { vnext: [{ address: "", port: 443, users: [{ id: "", security: "auto" }] }] };
    case "wireguard":
      return { address: "", secretKey: "", peers: [{ endpoint: "", publicKey: "" }] };
    case "dns":
      return {};
    case "loopback":
      return { inboundTag: "" };
    default:
      return {};
  }
}
