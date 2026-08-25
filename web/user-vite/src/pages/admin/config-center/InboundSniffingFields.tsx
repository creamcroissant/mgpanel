/**
 * InboundSniffingFields — visual editor for sniffing settings.
 * Controls: enabled, dest_override, metadata_only, domains_excluded, route_only.
 */
import { useTranslation } from "react-i18next";
import {
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Switch,
} from "@/components/ui";
import type { UnifiedSniffing } from "@/types/_generated/inbound";

interface InboundSniffingFieldsProps {
  value: UnifiedSniffing | null | undefined;
  onChange: (sniffing: UnifiedSniffing | null) => void;
  readOnly?: boolean;
}

const DEST_OVERRIDE_OPTIONS = ["http", "tls", "frag"];

export function InboundSniffingFields({ value, onChange, readOnly }: InboundSniffingFieldsProps) {
  const { t } = useTranslation();
  const enabled = !!value;

  const handleToggle = (v: boolean) => {
    if (!v) { onChange(null); return; }
    onChange({ enabled: true });
  };

  const set = (partial: Partial<UnifiedSniffing>) => {
    if (!value) return;
    onChange({ ...value, ...partial });
  };

  return (
    <div className="space-y-3 rounded-md border bg-muted/20 p-4" data-testid="inbound-sniffing-fields">
      <label
        className="flex items-center gap-2 text-sm font-medium cursor-pointer"
        onClick={() => !readOnly && handleToggle(!enabled)}
      >
        <Switch checked={(enabled)} onCheckedChange={(value) => handleToggle(value)} disabled={readOnly} />
        {t("admin.configCenter.inbound.enableSniffing", "启用嗅探")}
      </label>

      {enabled && (
        <div className="space-y-3 pl-1">
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.destOverride", "目的地覆写")}</label>
              <Select
                value={value?.dest_override?.join(",") ?? ""}
                onValueChange={(v) => set({ dest_override: v ? v.split(",") : undefined })}
                disabled={readOnly}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t("admin.configCenter.inbound.selectDestination", "选择覆写类型")} />
                </SelectTrigger>
                <SelectContent>
                  {DEST_OVERRIDE_OPTIONS.map((opt) => (
                    <SelectItem key={opt} value={opt}>
                      {opt}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.domainsExcluded", "排除域名")}</label>
              <Input
                value={value?.domains_excluded?.join(", ") ?? ""}
                onChange={(e) =>
                  set({
                    domains_excluded: e.target.value
                      .split(",")
                      .map((s) => s.trim())
                      .filter(Boolean),
                  })
                }
                placeholder="example.com, *.cn"
                disabled={readOnly}
              />
            </div>
          </div>

          <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
            <label className="flex items-center gap-2 text-sm">
              <Switch checked={value?.metadata_only === true} onCheckedChange={(value) => set({ metadata_only: value || undefined })} disabled={readOnly} />
              {t("admin.configCenter.inbound.metadataOnly", "仅元数据")}
            </label>
            <label className="flex items-center gap-2 text-sm">
              <Switch checked={value?.route_only === true} onCheckedChange={(value) => set({ route_only: value || undefined })} disabled={readOnly} />
              {t("admin.configCenter.inbound.routeOnly", "仅路由")}
            </label>
          </div>
        </div>
      )}
    </div>
  );
}
