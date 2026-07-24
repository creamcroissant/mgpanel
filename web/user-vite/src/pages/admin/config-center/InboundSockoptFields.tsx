/**
 * InboundSockoptFields — socket option settings for inbound transport.
 * Stored in _raw.sockopt through the semantic spec.
 */
import { useTranslation } from "react-i18next";
import { Input, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui";

interface InboundSockoptFieldsProps {
  value: Record<string, unknown> | undefined | null;
  onChange: (sockopt: Record<string, unknown> | undefined) => void;
  readOnly?: boolean;
}

const TPROXY_OPTIONS = ["redirect", "tproxy", "off"];

export function InboundSockoptFields({
  value,
  onChange,
  readOnly = false,
}: InboundSockoptFieldsProps) {
  const { t } = useTranslation();
  const enabled = !!value;

  const handleToggle = (v: boolean) => {
    if (!v) { onChange(undefined); return; }
    onChange({});
  };

  const set = (key: string, val: unknown) => {
    const next = { ...(value ?? {}) };
    if (val === undefined || val === "" || val === null) {
      delete next[key];
    } else {
      next[key] = val;
    }
    onChange(Object.keys(next).length > 0 ? next : undefined);
  };

  return (
    <div className="space-y-3 rounded-md border bg-muted/20 p-4" data-testid="inbound-sockopt-fields">
      <label
        className="flex items-center gap-2 text-sm font-medium cursor-pointer"
        onClick={() => !readOnly && handleToggle(!enabled)}
      >
        <input
          type="checkbox"
          checked={enabled}
          onChange={(e) => handleToggle(e.target.checked)}
          disabled={readOnly}
          className="h-4 w-4 rounded border-input"
        />
        {t("admin.configCenter.inbound.sockopt", "套接字选项")}
      </label>

      {enabled && (
        <div className="space-y-3 pl-1">
          <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.tproxy")}</label>
              <Select
                value={(value?.tproxy as string) ?? ""}
                onValueChange={(v) => set("tproxy", v || undefined)}
                disabled={readOnly}
              >
                <SelectTrigger>
                  <SelectValue placeholder="off" />
                </SelectTrigger>
                <SelectContent>
                  {TPROXY_OPTIONS.map((opt) => (
                    <SelectItem key={opt} value={opt}>{opt}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.mark")}</label>
              <Input
                type="number"
                min={0}
                value={(value?.mark as number) ?? ""}
                onChange={(e) => set("mark", e.target.value ? parseInt(e.target.value, 10) : undefined)}
                placeholder="0"
                disabled={readOnly}
              />
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.interface")}</label>
              <Input
                value={(value?.interface as string) ?? ""}
                onChange={(e) => set("interface", e.target.value || undefined)}
                placeholder="eth0"
                disabled={readOnly}
              />
            </div>

            <div className="flex items-end pb-2">
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={(value?.tcp_fast_open as boolean) === true}
                  onChange={(e) => set("tcp_fast_open", e.target.checked || undefined)}
                  disabled={readOnly}
                  className="h-4 w-4 rounded border-input"
                />
                {t("admin.configCenter.inbound.tcpFastOpen")}
              </label>
            </div>

            <div className="flex items-end pb-2">
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={(value?.tcp_mptcp as boolean) === true}
                  onChange={(e) => set("tcp_mptcp", e.target.checked || undefined)}
                  disabled={readOnly}
                  className="h-4 w-4 rounded border-input"
                />
                {t("admin.configCenter.inbound.tcpMptcp")}
              </label>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
