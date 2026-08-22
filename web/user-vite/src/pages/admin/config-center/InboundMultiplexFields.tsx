import { useTranslation } from "react-i18next";
import { Input, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui";
import type { InboundMultiplexSpec } from "@/types/configCenterInbound";

interface InboundMultiplexFieldsProps {
  value: InboundMultiplexSpec | null | undefined;
  onChange: (mp: InboundMultiplexSpec | null) => void;
  readOnly?: boolean;
}

const PROTOCOL_OPTIONS = ["h2mux", "smux", "yamux", "hysteria"];

export function InboundMultiplexFields({ value, onChange, readOnly }: InboundMultiplexFieldsProps) {
  const { t } = useTranslation();
  const enabled = !!value;
  const brutalEnabled = !!value?.brutal;

  const handleToggle = (v: boolean) => {
    if (!v) { onChange(null); return; }
    onChange({ enabled: true });
  };

  const set = (partial: Partial<InboundMultiplexSpec>) => {
    if (!value) return;
    onChange({ ...value, ...partial });
  };

  const setBrutalEnabled = (v: boolean) => {
    if (!value) return;
    if (v) {
      onChange({ ...value, brutal: { enabled: true } });
    } else {
      const rest = { ...value };
      delete rest.brutal;
      onChange(rest);
    }
  };

  const setBrutalField = (field: "up_mbps" | "down_mbps", val: string) => {
    if (!value) return;
    const parsed = val ? parseInt(val, 10) : undefined;
    onChange({ ...value, brutal: { enabled: value.brutal?.enabled ?? true, ...value.brutal, [field]: parsed } });
  };

  return (
    <div className="space-y-3 rounded-md border bg-muted/20 p-4" data-testid="inbound-multiplex-fields">
      <label className="flex items-center gap-2 text-sm font-medium cursor-pointer" onClick={() => !readOnly && handleToggle(!enabled)}>
        <input type="checkbox" checked={enabled} onChange={(e) => handleToggle(e.target.checked)} disabled={readOnly} className="h-4 w-4 rounded border-input" />
        {t("admin.configCenter.inbound.enableMultiplex")}
      </label>

      {enabled && (
        <div className="space-y-3 pl-1">
          <div className="grid grid-cols-1 gap-3 md:grid-cols-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.mpProtocol")}</label>
              <Select value={value?.protocol ?? ""} onValueChange={(v) => set({ protocol: v || undefined })} disabled={readOnly}>
                <SelectTrigger><SelectValue placeholder={t("admin.configCenter.inbound.selectProtocol")} /></SelectTrigger>
                <SelectContent>{PROTOCOL_OPTIONS.map((p) => (<SelectItem key={p} value={p}>{p}</SelectItem>))}</SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.mpMaxStreams")}</label>
              <Input type="number" value={value?.max_streams ?? ""} onChange={(e) => set({ max_streams: e.target.value ? parseInt(e.target.value, 10) : undefined })} min={1} disabled={readOnly} />
            </div>
            <div className="flex items-end pb-2">
              <label className="flex items-center gap-2 text-sm font-medium">
                <input type="checkbox" checked={value?.padding === true} onChange={(e) => set({ padding: e.target.checked || undefined })} disabled={readOnly} className="h-4 w-4 rounded border-input" />
                {t("admin.configCenter.inbound.mpPadding")}
              </label>
            </div>
          </div>

          {/* Brutal congestion control */}
          <div className="space-y-3 rounded-md border border-border/60 bg-background p-3">
            <label className="flex items-center gap-2 text-sm font-medium">
              <input type="checkbox" checked={brutalEnabled} onChange={(e) => setBrutalEnabled(e.target.checked)} disabled={readOnly} className="h-4 w-4 rounded border-input" />
              {t("admin.configCenter.inbound.mpBrutal")}
            </label>
            {brutalEnabled && (
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2 pl-6">
                <div className="space-y-2">
                  <label className="text-xs font-medium text-muted-foreground">{t("admin.configCenter.inbound.mpBrutalUp")}</label>
                  <Input type="number" value={value?.brutal?.up_mbps ?? ""} onChange={(e) => setBrutalField("up_mbps", e.target.value)} min={1} disabled={readOnly} />
                </div>
                <div className="space-y-2">
                  <label className="text-xs font-medium text-muted-foreground">{t("admin.configCenter.inbound.mpBrutalDown")}</label>
                  <Input type="number" value={value?.brutal?.down_mbps ?? ""} onChange={(e) => setBrutalField("down_mbps", e.target.value)} min={1} disabled={readOnly} />
                </div>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
