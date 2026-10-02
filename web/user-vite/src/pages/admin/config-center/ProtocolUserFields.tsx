/**
 * ProtocolUserFields — spec 级用户默认值（不存逐用户 UUID/密码）。
 * 真实用户由面板用户表经 sync_users 下发（每用户全局唯一 UUID，天然不重复）；
 * 这里只配通用默认值（VLESS flow / VMESS security / SS method），存 options，
 * 后端投射全局用户时套用（没配走渲染侧 vision 保底）。
 */
import { useTranslation } from "react-i18next";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui";

interface ProtocolUserFieldsProps {
  protocol: string;
  value: Record<string, unknown> | undefined | null;
  onChange: (options: Record<string, unknown> | undefined) => void;
  readOnly?: boolean;
}

const SHADOWSOCKS_METHODS = [
  "aes-128-gcm", "aes-256-gcm", "chacha20-ieft-poly1305",
  "xchacha20-ieft-poly1305", "aes-128-ctr", "aes-256-ctr",
  "2022-blake3-aes-128-gcm", "2022-blake3-aes-256-gcm",
  "2022-blake3-chacha20-poly1305", "none",
];

const VLESS_FLOWS = ["", "xtls-rprx-vision", "xtls-rprx-vision-udp443"];

const VMESS_SECURITIES = ["auto", "aes-128-gcm", "chacha20-poly1305", "none", "zero"];

// eslint-disable-next-line react-refresh/only-export-components
export function hasUsers(proto: string): boolean {
  return proto === "vless" || proto === "vmess" || proto === "shadowsocks";
}

export function ProtocolUserFields({
  protocol,
  value,
  onChange,
  readOnly = false,
}: ProtocolUserFieldsProps) {
  const { t } = useTranslation();
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

  if (!hasUsers(protocol)) {
    return null;
  }

  return (
    <div className="space-y-3 rounded-none border border-border bg-muted/20 p-4" data-testid="protocol-user-fields">
      <h3 className="text-sm font-semibold">{t("admin.configCenter.inbound.userDefaults")}</h3>
      <p className="text-xs text-muted-foreground">
        {t("admin.configCenter.inbound.userDefaultsHint")}
      </p>
      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
        {protocol === "vless" && (
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.configCenter.inbound.flow")}</label>
            <Select
              value={(opts.default_flow as string) || "xtls-rprx-vision"}
              onValueChange={(v) => set("default_flow", v)}
              disabled={readOnly}
            >
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {VLESS_FLOWS.filter(Boolean).map((f) => (
                  <SelectItem key={f} value={f}>{f}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}
        {protocol === "vmess" && (
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.configCenter.inbound.encryption")}</label>
            <Select
              value={(opts.default_security as string) ?? "auto"}
              onValueChange={(v) => set("default_security", v)}
              disabled={readOnly}
            >
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {VMESS_SECURITIES.map((s) => (
                  <SelectItem key={s} value={s}>{s}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}
        {protocol === "shadowsocks" && (
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.configCenter.inbound.method")}</label>
            <Select
              value={(opts.default_method as string) ?? "2022-blake3-aes-128-gcm"}
              onValueChange={(v) => set("default_method", v)}
              disabled={readOnly}
            >
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {SHADOWSOCKS_METHODS.map((m) => (
                  <SelectItem key={m} value={m}>{m}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}
      </div>
    </div>
  );
}
