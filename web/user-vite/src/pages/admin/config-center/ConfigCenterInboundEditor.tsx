import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Input, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui";
import type { InboundSemanticSpec } from "@/types/configCenterInbound";
import { PROTOCOL_OPTIONS, SINGBOX_INBOUND_PROTOCOLS, XRAY_INBOUND_PROTOCOLS } from "@/types/configCenterInbound";
import type { ConfigCenterCoreType } from "@/types/configCenter";
import { InboundTLSFields } from "./InboundTLSFields";
import { InboundTransportFields } from "./InboundTransportFields";
import { InboundMultiplexFields } from "./InboundMultiplexFields";

interface ConfigCenterInboundEditorProps {
  value: InboundSemanticSpec;
  onChange: (spec: InboundSemanticSpec) => void;
  readOnly?: boolean;
  coreType?: ConfigCenterCoreType;
}

const LISTEN_OPTIONS = ["::", "0.0.0.0", "127.0.0.1"];

export function ConfigCenterInboundEditor({
  value,
  onChange,
  readOnly = false,
  coreType,
}: ConfigCenterInboundEditorProps) {
  const { t } = useTranslation();

  // Filter protocols based on selected core type
  const filteredProtocols = useMemo(() => {
    if (!coreType) return PROTOCOL_OPTIONS;
    const supported = coreType === "xray" ? XRAY_INBOUND_PROTOCOLS : SINGBOX_INBOUND_PROTOCOLS;
    return PROTOCOL_OPTIONS.filter((p) => (supported as readonly string[]).includes(p));
  }, [coreType]);

  const set = (partial: Partial<InboundSemanticSpec>) => onChange({ ...value, ...partial });

  const editTransport = (transport: InboundSemanticSpec["transport"]) => set({ transport });
  const editTLS = (tls: InboundSemanticSpec["tls"]) => set({ tls });
  const editMultiplex = (multiplex: InboundSemanticSpec["multiplex"]) => set({ multiplex });

  return (
    <div className="space-y-5" data-testid="config-center-inbound-editor">
      {/* Protocol + Tag row */}
      <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
        <div className="space-y-2">
          <label className="text-sm font-medium">{t("admin.configCenter.inbound.protocol")}</label>
          <Select
            value={value.protocol}
            onValueChange={(v) => set({ protocol: v, transport: null })}
            disabled={readOnly}
          >
            <SelectTrigger data-testid="inbound-protocol-select">
              <SelectValue placeholder={t("admin.configCenter.inbound.selectProtocol")} />
            </SelectTrigger>
            <SelectContent>
              {filteredProtocols.map((p) => (
                <SelectItem key={p} value={p}>
                  {p}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="space-y-2">
          <label className="text-sm font-medium">{t("admin.configCenter.fields.tag")}</label>
          <Input
            value={value.tag ?? ""}
            onChange={(e) => set({ tag: e.target.value || undefined })}
            placeholder="inbound-tag"
            disabled={readOnly}
          />
        </div>

        <div className="space-y-2">
          <label className="text-sm font-medium">{t("admin.configCenter.inbound.listen")}</label>
          <Select
            value={value.listen}
            onValueChange={(v) => set({ listen: v })}
            disabled={readOnly}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {LISTEN_OPTIONS.map((l) => (
                <SelectItem key={l} value={l}>
                  {l}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      {/* Port */}
      <div className="space-y-2">
        <label className="text-sm font-medium">{t("admin.configCenter.inbound.port")}</label>
        <Input
          type="number"
          className="w-full max-w-xs"
          value={value.port || ""}
          onChange={(e: React.ChangeEvent<HTMLInputElement>) => set({ port: e.target.value ? parseInt(e.target.value, 10) : 0 })}
          min={1}
          max={65535}
          placeholder="443"
          disabled={readOnly}
        />
      </div>

      {/* Transport section */}
      <InboundTransportFields
        value={value.transport}
        onChange={editTransport}
        readOnly={readOnly}
      />

      {/* TLS / Reality section */}
      <InboundTLSFields
        value={value.tls}
        onChange={editTLS}
        readOnly={readOnly}
      />

      {/* Multiplex section */}
      <InboundMultiplexFields
        value={value.multiplex}
        onChange={editMultiplex}
        readOnly={readOnly}
      />
    </div>
  );
}
