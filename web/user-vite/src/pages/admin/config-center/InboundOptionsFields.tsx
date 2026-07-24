/**
 * InboundOptionsFields — protocol-aware options editor.
 * Shows relevant fields based on selected protocol,
 * with a key-value fallback for protocol-specific extras.
 */
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Input, Textarea } from "@/components/ui";

interface InboundOptionsFieldsProps {
  protocol: string;
  value: Record<string, unknown> | undefined | null;
  onChange: (options: Record<string, unknown> | undefined) => void;
  readOnly?: boolean;
}

export function InboundOptionsFields({
  protocol,
  value,
  onChange,
  readOnly = false,
}: InboundOptionsFieldsProps) {
  const { t } = useTranslation();
  const [showAllOptions, setShowAllOptions] = useState(false);

  const opts = value ?? {};

  const set = (key: string, val: unknown) => {
    const next = { ...opts };
    if (val === undefined || val === "" || val === null) {
      delete next[key];
    } else {
      next[key] = val;
    }
    onChange(Object.keys(next).length > 0 ? next : undefined);
  };

  const extrasJSON = JSON.stringify(opts, null, 2);
  const [extrasDraft, setExtrasDraft] = useState(extrasJSON);
  const syncExtras = () => {
    try {
      const parsed = JSON.parse(extrasDraft);
      if (typeof parsed === "object" && !Array.isArray(parsed)) {
        onChange(Object.keys(parsed).length > 0 ? parsed : undefined);
      }
    } catch {
      // keep draft
    }
  };

  return (
    <div className="space-y-3 rounded-md border bg-muted/20 p-4" data-testid="inbound-options-fields">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold">{t("admin.configCenter.inbound.options", "协议选项")}</h3>
      </div>

      <div className="space-y-3">
        {/* VLESS-specific: decryption */}
        {protocol === "vless" && (
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.decryption")}</label>
              <Input
                value={(opts.decryption as string) ?? ""}
                onChange={(e) => set("decryption", e.target.value || undefined)}
                placeholder="none"
                disabled={readOnly}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.encryption")}</label>
              <Input
                value={(opts.encryption as string) ?? ""}
                onChange={(e) => set("encryption", e.target.value || undefined)}
                placeholder="none"
                disabled={readOnly}
              />
            </div>
          </div>
        )}

        {/* VMESS-specific */}
        {protocol === "vmess" && (
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.configCenter.inbound.encryption")}</label>
            <Input
              value={(opts.encryption as string) ?? ""}
              onChange={(e) => set("encryption", e.target.value || undefined)}
              placeholder="auto"
              disabled={readOnly}
            />
          </div>
        )}
      </div>

      {/* Toggle for all key-value editor */}
      <button
        type="button"
        className="text-xs text-muted-foreground hover:text-foreground underline underline-offset-2"
        onClick={() => setShowAllOptions(!showAllOptions)}
      >
        {showAllOptions
          ? t("admin.configCenter.inbound.hideAllOptions", "收起全部选项")
          : t("admin.configCenter.inbound.showAllOptions", "显示全部选项")}
      </button>

      {showAllOptions && (
        <div className="space-y-2 border-l-2 border-border/40 pl-3">
          <label className="text-sm font-medium">{t("admin.configCenter.inbound.extraOptions", "附加选项 JSON")}</label>
          <Textarea
            className="min-h-[100px] font-mono text-xs"
            value={extrasDraft}
            onChange={(e) => setExtrasDraft(e.target.value)}
            onBlur={syncExtras}
            disabled={readOnly}
          />
        </div>
      )}
    </div>
  );
}
