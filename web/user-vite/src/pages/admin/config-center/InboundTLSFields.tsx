import { useTranslation } from "react-i18next";
import { Input, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui";
import type { InboundTLSSpec, InboundRealitySpec } from "@/types/configCenterInbound";

interface InboundTLSFieldsProps {
  value: InboundTLSSpec | null | undefined;
  onChange: (tls: InboundTLSSpec | null) => void;
  readOnly?: boolean;
}

type TLSCertMode = "none" | "self-signed" | "reality";

const REALITY_FINGERPRINTS = ["chrome", "firefox", "safari", "random", "randomized"];

export function InboundTLSFields({ value, onChange, readOnly }: InboundTLSFieldsProps) {
  const { t } = useTranslation();
  const enabled = value?.enabled === true || value?.reality?.enabled === true;
  const isReality = value?.reality?.enabled === true;

  const tlsCertMode: TLSCertMode = isReality ? "reality" : enabled ? "self-signed" : "none";

  const handleModeChange = (mode: TLSCertMode) => {
    if (mode === "none") {
      onChange(null);
      return;
    }
    if (mode === "reality") {
      onChange({ enabled: true, reality: { enabled: true } });
      return;
    }
    onChange({ enabled: true });
  };

  const setReality = (partial: Partial<InboundRealitySpec>) => {
    if (!value) return;
    onChange({
      ...value,
      reality: { enabled: value.reality?.enabled ?? true, ...value.reality, ...partial },
    });
  };

  return (
    <div className="space-y-3 rounded-md border bg-muted/20 p-4" data-testid="inbound-tls-fields">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold">{t("admin.configCenter.inbound.security")}</h3>
        <Select value={tlsCertMode} onValueChange={handleModeChange as (v: string) => void} disabled={readOnly}>
          <SelectTrigger className="w-44" data-testid="inbound-tls-mode">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="none">{t("admin.configCenter.inbound.tlsNone")}</SelectItem>
            <SelectItem value="self-signed">{t("admin.configCenter.inbound.tlsSelfSigned")}</SelectItem>
            <SelectItem value="reality">{t("admin.configCenter.inbound.reality")}</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {enabled && !isReality && (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.configCenter.inbound.serverName")}</label>
            <Input
              value={value?.server_name ?? ""}
              onChange={(e) => onChange({ ...(value as InboundTLSSpec), server_name: e.target.value || undefined })}
              placeholder="example.com"
              disabled={readOnly}
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.configCenter.inbound.alpn")}</label>
            <Input
              value={value?.alpn?.join(", ") ?? ""}
              onChange={(e) =>
                onChange({
                  ...(value as InboundTLSSpec),
                  alpn: e.target.value
                    .split(",")
                    .map((s) => s.trim())
                    .filter(Boolean),
                })
              }
              placeholder="h2, http/1.1"
              disabled={readOnly}
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.configCenter.inbound.certPath")}</label>
            <Input
              value={value?.cert_path ?? ""}
              onChange={(e) => onChange({ ...(value as InboundTLSSpec), cert_path: e.target.value || undefined })}
              placeholder="/etc/ssl/cert.pem"
              disabled={readOnly}
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.configCenter.inbound.keyPath")}</label>
            <Input
              value={value?.key_path ?? ""}
              onChange={(e) => onChange({ ...(value as InboundTLSSpec), key_path: e.target.value || undefined })}
              placeholder="/etc/ssl/key.pem"
              disabled={readOnly}
            />
          </div>
        </div>
      )}

      {isReality && value?.reality && (
        <div className="space-y-3 pl-3 border-l-2 border-primary/30">
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.serverNames")}</label>
              <Input
                value={value.reality.server_names?.join(", ") ?? ""}
                onChange={(e) =>
                  setReality({
                    server_names: e.target.value
                      .split(",")
                      .map((s) => s.trim())
                      .filter(Boolean),
                  })
                }
                placeholder="www.example.com"
                disabled={readOnly}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.handshakeServer")}</label>
              <Input
                value={value.reality.handshake_server ?? ""}
                onChange={(e) => setReality({ handshake_server: e.target.value || undefined })}
                placeholder="www.cloudflare.com"
                disabled={readOnly}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.handshakePort")}</label>
              <Input
                type="number"
                value={value.reality.handshake_port ?? ""}
                onChange={(e) =>
                  setReality({
                    handshake_port: e.target.value ? parseInt(e.target.value, 10) : undefined,
                  })
                }
                placeholder="443"
                disabled={readOnly}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.fingerprint")}</label>
              <Select
                value={value.reality.fingerprint ?? ""}
                onValueChange={(v) => setReality({ fingerprint: v || undefined })}
                disabled={readOnly}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t("admin.configCenter.inbound.selectFingerprint")} />
                </SelectTrigger>
                <SelectContent>
                  {REALITY_FINGERPRINTS.map((f) => (
                    <SelectItem key={f} value={f}>
                      {f}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
