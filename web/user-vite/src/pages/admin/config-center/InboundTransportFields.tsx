import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Input, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, Textarea } from "@/components/ui";
import type { InboundTransportSpec, TransportType } from "@/types/configCenterInbound";
import { TRANSPORT_OPTIONS } from "@/types/configCenterInbound";

interface InboundTransportFieldsProps {
  value: InboundTransportSpec | null | undefined;
  onChange: (transport: InboundTransportSpec | null) => void;
  readOnly?: boolean;
}

const TRANSPORT_HAS_PATH = new Set(["ws", "grpc", "http", "kcp", "httpupgrade", "xhttp", "quic"]);
const TRANSPORT_HAS_HOST = new Set(["ws", "http", "xhttp"]);
const TRANSPORT_HAS_SERVICE_NAME = new Set(["grpc"]);
const TRANSPORT_HAS_MODE = new Set(["xhttp", "httpupgrade"]);
const TRANSPORT_HAS_SEED = new Set(["kcp"]);
const TRANSPORT_HAS_CONGESTION = new Set(["kcp"]);
const TRANSPORT_HAS_PACKET_ENCODING = new Set(["kcp"]);
const TRANSPORT_HAS_EARLY_DATA = new Set(["ws", "httpupgrade"]);
const TRANSPORT_HAS_HEADERS = new Set(["ws", "http", "grpc"]);

export function InboundTransportFields({ value, onChange, readOnly }: InboundTransportFieldsProps) {
  const { t } = useTranslation();
  const [showAdvanced, setShowAdvanced] = useState(false);

  const enabled = !!value;
  const transportType = value?.type ?? "tcp";

  const handleToggle = (v: boolean) => {
    if (!v) { onChange(null); return; }
    onChange({ type: "tcp" });
  };

  const set = (partial: Partial<InboundTransportSpec>) => {
    if (!value) return;
    onChange({ ...value, ...partial });
  };

  const showPath = TRANSPORT_HAS_PATH.has(transportType);
  const showHost = TRANSPORT_HAS_HOST.has(transportType);
  const showServiceName = TRANSPORT_HAS_SERVICE_NAME.has(transportType);
  const showMode = TRANSPORT_HAS_MODE.has(transportType);
  const showSeed = TRANSPORT_HAS_SEED.has(transportType);
  const showCongestion = TRANSPORT_HAS_CONGESTION.has(transportType);
  const showPacketEncoding = TRANSPORT_HAS_PACKET_ENCODING.has(transportType);
  const showEarlyData = TRANSPORT_HAS_EARLY_DATA.has(transportType);
  const showHeaders = TRANSPORT_HAS_HEADERS.has(transportType);

  const headersJSON = value?.headers ? JSON.stringify(value.headers, null, 2) : "{}";

  const onHeadersChange = (raw: string) => {
    try {
      const parsed = JSON.parse(raw);
      if (typeof parsed === "object" && !Array.isArray(parsed)) {
        const clean: Record<string, string> = {};
        for (const [k, v] of Object.entries(parsed)) {
          if (typeof v === "string") clean[k] = v;
        }
        set({ headers: Object.keys(clean).length ? clean : undefined });
      }
    } catch {
      // invalid JSON — keep previous value
    }
  };

  return (
    <div className="space-y-3 rounded-md border bg-muted/20 p-4" data-testid="inbound-transport-fields">
      <label className="flex items-center gap-2 text-sm font-medium cursor-pointer" onClick={() => !readOnly && handleToggle(!enabled)}>
        <input type="checkbox" checked={enabled} onChange={(e) => handleToggle(e.target.checked)} disabled={readOnly} className="h-4 w-4 rounded border-input" />
        {t("admin.configCenter.inbound.enableTransport")}
      </label>

      {enabled && (
        <div className="space-y-3 pl-1">
          <div className="grid grid-cols-1 gap-3 md:grid-cols-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.transportType")}</label>
              <Select value={transportType} onValueChange={(v) => set({ type: v as TransportType })} disabled={readOnly}>
                <SelectTrigger data-testid="inbound-transport-type"><SelectValue /></SelectTrigger>
                <SelectContent>{TRANSPORT_OPTIONS.map((o) => (<SelectItem key={o} value={o}>{o}</SelectItem>))}</SelectContent>
              </Select>
            </div>
            {showPath && (<div className="space-y-2"><label className="text-sm font-medium">{t("admin.configCenter.inbound.path")}</label><Input value={value?.path ?? ""} onChange={(e) => set({ path: e.target.value || undefined })} placeholder={transportType === "grpc" ? "/ServiceName" : transportType === "xhttp" ? "/xhttp" : "/ws"} disabled={readOnly} /></div>)}
            {showHost && (<div className="space-y-2"><label className="text-sm font-medium">{t("admin.configCenter.inbound.host")}</label><Input value={value?.host ?? ""} onChange={(e) => set({ host: e.target.value || undefined })} placeholder="example.com" disabled={readOnly} /></div>)}
            {showServiceName && (<div className="space-y-2"><label className="text-sm font-medium">{t("admin.configCenter.inbound.serviceName")}</label><Input value={value?.service_name ?? ""} onChange={(e) => set({ service_name: e.target.value || undefined })} placeholder="ServiceName" disabled={readOnly} /></div>)}
            {showMode && (<div className="space-y-2"><label className="text-sm font-medium">{t("admin.configCenter.inbound.mode")}</label><Select value={value?.mode ?? ""} onValueChange={(v) => set({ mode: v || undefined })} disabled={readOnly}><SelectTrigger><SelectValue placeholder={t("admin.configCenter.inbound.selectMode")} /></SelectTrigger><SelectContent>{(transportType === "xhttp" ? [{ value: "auto", label: "auto" }, { value: "packet-up", label: "packet-up" }, { value: "stream-up", label: "stream-up" }, { value: "stream-one", label: "stream-one" }] : [{ value: "connect", label: "connect" }, { value: "h2", label: "h2" }]).map((o) => (<SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>))}</SelectContent></Select></div>)}
          </div>

          {/* KCP fields */}
          {showSeed && (
            <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
              <div className="space-y-2"><label className="text-sm font-medium">{t("admin.configCenter.inbound.kcpSeed")}</label><Input value={value?.seed ?? ""} onChange={(e) => set({ seed: e.target.value || undefined })} disabled={readOnly} /></div>
              {showCongestion && (<div className="space-y-2"><label className="text-sm font-medium">{t("admin.configCenter.inbound.congestionControl")}</label><Input value={value?.congestion_control ?? ""} onChange={(e) => set({ congestion_control: e.target.value || undefined })} placeholder="bbr" disabled={readOnly} /></div>)}
              {showPacketEncoding && (<div className="space-y-2"><label className="text-sm font-medium">{t("admin.configCenter.inbound.packetEncoding")}</label><Input value={value?.packet_encoding ?? ""} onChange={(e) => set({ packet_encoding: e.target.value || undefined })} disabled={readOnly} /></div>)}
            </div>
          )}

          {/* Advanced toggle */}
          <button type="button" className="text-xs text-muted-foreground hover:text-foreground underline underline-offset-2" onClick={() => setShowAdvanced(!showAdvanced)}>
            {showAdvanced ? t("admin.configCenter.inbound.hideAdvancedTransport") : t("admin.configCenter.inbound.showAdvancedTransport")}
          </button>

          {showAdvanced && (
            <div className="space-y-3 border-l-2 border-border/40 pl-3">
              {showHeaders && (
                <div className="space-y-2">
                  <label className="text-sm font-medium">{t("admin.configCenter.inbound.headers")}</label>
                  <Textarea className="min-h-[80px] font-mono text-xs" value={headersJSON} onChange={(e) => onHeadersChange(e.target.value)} placeholder='{ "Host": "example.com" }' disabled={readOnly} />
                </div>
              )}
              {showEarlyData && (
                <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                  <div className="space-y-2"><label className="text-sm font-medium">{t("admin.configCenter.inbound.earlyDataHeader")}</label><Input value={value?.early_data_header_name ?? ""} onChange={(e) => set({ early_data_header_name: e.target.value || undefined })} placeholder="X-Early-Data" disabled={readOnly} /></div>
                  <div className="space-y-2"><label className="text-sm font-medium">{t("admin.configCenter.inbound.maxEarlyData")}</label><Input type="number" value={value?.max_early_data ?? ""} onChange={(e) => set({ max_early_data: e.target.value ? parseInt(e.target.value, 10) : undefined })} min={0} disabled={readOnly} /></div>
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
