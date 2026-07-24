/**
 * ProtocolUserFields — visual editor for protocol-specific user accounts.
 * Supports: VLESS (UUID), Shadowsocks (method/password), VMESS (UUID/security),
 * Trojan (password), Hysteria2 (password), SOCKS/HTTP (username/password).
 */
import { useCallback, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Button, Input, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui";
import { generateCompactUUID } from "./configCenterPageUtils";

/** Internal user entry with typed fields */
interface UserEntry {
  uuid?: string;
  password?: string;
  username?: string;
  flow?: string;
  encryption?: string;
  security?: string;
  alter_id?: number;
  level?: number;
  email?: string;
  method?: string;
}

interface ProtocolUserFieldsProps {
  protocol: string;
  users: Array<Record<string, unknown>> | undefined | null;
  onChange: (users: Array<Record<string, unknown>> | undefined) => void;
  readOnly?: boolean;
}

/** Protocols that use UUID-based authentication */
const UUID_PROTOCOLS = new Set(["vless", "vmess"]);

/** Protocols that use password-based authentication */
const PASSWORD_PROTOCOLS = new Set(["trojan", "shadowsocks", "hysteria2", "hysteria", "tuic"]);

/** Protocols that use username/password authentication */
const USERNAME_PASSWORD_PROTOCOLS = new Set(["socks", "http", "mixed"]);

function isUUIDProtocol(proto: string): boolean {
  return UUID_PROTOCOLS.has(proto);
}

function isPasswordProtocol(proto: string): boolean {
  return PASSWORD_PROTOCOLS.has(proto);
}

function isUsernamePasswordProtocol(proto: string): boolean {
  return USERNAME_PASSWORD_PROTOCOLS.has(proto);
}

function hasUsers(proto: string): boolean {
  return isUUIDProtocol(proto) || isPasswordProtocol(proto) || isUsernamePasswordProtocol(proto);
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
export { hasUsers };

export function ProtocolUserFields({
  protocol,
  users,
  onChange,
  readOnly = false,
}: ProtocolUserFieldsProps) {
  const { t } = useTranslation();

  const rawList = useMemo(() => users ?? [], [users]);

  const updateUser = useCallback(
    (index: number, partial: Partial<Record<string, unknown>>) => {
      const next = [...rawList];
      next[index] = { ...next[index], ...partial };
      onChange(next.length > 0 ? next : undefined);
    },
    [rawList, onChange],
  );

  const removeUser = useCallback(
    (index: number) => {
      const next = rawList.filter((_, i) => i !== index);
      onChange(next.length > 0 ? next : undefined);
    },
    [rawList, onChange],
  );

  const addUser = useCallback(() => {
    const newUser: Record<string, unknown> = {};
    if (isUUIDProtocol(protocol)) {
      newUser.uuid = generateCompactUUID();
    }
    if (protocol === "vless") {
      newUser.flow = "";
    }
    if (protocol === "vmess") {
      newUser.security = "auto";
      newUser.alter_id = 0;
    }
    if (protocol === "shadowsocks") {
      newUser.method = "2022-blake3-aes-128-gcm";
    }
    onChange([...rawList, newUser]);
  }, [protocol, rawList, onChange]);

  if (!hasUsers(protocol)) {
    return null;
  }

  return (
    <div className="space-y-3 rounded-md border bg-muted/20 p-4" data-testid="protocol-user-fields">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold">{t("admin.configCenter.inbound.users")}</h3>
        <Button type="button" size="sm" variant="outline" onClick={addUser} disabled={readOnly}>
          + {t("admin.configCenter.inbound.addUser")}
        </Button>
      </div>

      {rawList.length === 0 && (
        <p className="text-xs text-muted-foreground">
          {t("admin.configCenter.inbound.noUsers")}
        </p>
      )}

      {rawList.map((rawUser, i) => {
        const user = rawUser as UserEntry;
        return (
        <div
          key={i}
          className="space-y-3 rounded-md border border-border/60 bg-background p-3"
          data-testid={`protocol-user-${i}`}
        >
          <div className="flex items-start justify-between gap-2">
            <span className="text-xs font-medium text-muted-foreground">
              #{i + 1}
            </span>
            {!readOnly && (
              <Button
                type="button"
                size="sm"
                variant="ghost"
                className="text-destructive h-auto px-2 py-1 text-xs"
                onClick={() => removeUser(i)}
              >
                {t("common.delete")}
              </Button>
            )}
          </div>

          <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
            {/* UUID fields: VLESS, VMESS */}
            {isUUIDProtocol(protocol) && (
              <>
                <div className="space-y-1 md:col-span-2">
                  <label className="text-xs text-muted-foreground">
                    {protocol === "vless" ? "UUID" : "ID"}
                  </label>
                  <div className="flex gap-2">
                    <Input
                      value={user.uuid ?? ""}
                      onChange={(e) => updateUser(i, { uuid: e.target.value || undefined })}
                      placeholder={generateCompactUUID()}
                      disabled={readOnly}
                      className="font-mono text-xs"
                    />
                    {!readOnly && (
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        onClick={() => updateUser(i, { uuid: generateCompactUUID() })}
                      >
                        {t("admin.configCenter.generator.actions.generateUUID")}
                      </Button>
                    )}
                  </div>
                </div>
              </>
            )}

            {/* VLESS flow */}
            {protocol === "vless" && (
              <div className="space-y-1">
                <label className="text-xs text-muted-foreground">{t("admin.configCenter.inbound.flow")}</label>
                <Select
                  value={user.flow ?? ""}
                  onValueChange={(v) => updateUser(i, { flow: v || undefined })}
                  disabled={readOnly}
                >
                  <SelectTrigger className="h-8 text-xs"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {VLESS_FLOWS.map((f) => (
                      <SelectItem key={f || "none"} value={f}>{f || t("admin.configCenter.inbound.vlessNoFlow")}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}

            {/* VMESS security + alterId */}
            {protocol === "vmess" && (
              <>
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">{t("admin.configCenter.inbound.encryption")}</label>
                  <Select
                    value={user.security ?? "auto"}
                    onValueChange={(v) => updateUser(i, { security: v })}
                    disabled={readOnly}
                  >
                    <SelectTrigger className="h-8 text-xs"><SelectValue /></SelectTrigger>
                    <SelectContent>
                      {VMESS_SECURITIES.map((s) => (
                        <SelectItem key={s} value={s}>{s}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">{t("admin.configCenter.inbound.alterId")}</label>
                  <Input
                    type="number"
                    min={0}
                    max={65535}
                    value={user.alter_id ?? 0}
                    onChange={(e) => updateUser(i, { alter_id: e.target.value ? parseInt(e.target.value, 10) : 0 })}
                    disabled={readOnly}
                    className="h-8 text-xs"
                  />
                </div>
              </>
            )}

            {/* Password fields: Trojan, Hysteria2, TUIC */}
            {isPasswordProtocol(protocol) && protocol !== "shadowsocks" && (
              <div className="space-y-1 md:col-span-2">
                <label className="text-xs text-muted-foreground">{t("admin.configCenter.inbound.password")}</label>
                <Input
                  value={user.password ?? ""}
                  onChange={(e) => updateUser(i, { password: e.target.value || undefined })}
                  placeholder="password"
                  disabled={readOnly}
                  className="font-mono text-xs"
                />
              </div>
            )}

            {/* Shadowsocks method + password */}
            {protocol === "shadowsocks" && (
              <>
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">{t("admin.configCenter.inbound.method")}</label>
                  <Select
                    value={user.method ?? "2022-blake3-aes-128-gcm"}
                    onValueChange={(v) => updateUser(i, { method: v })}
                    disabled={readOnly}
                  >
                    <SelectTrigger className="h-8 text-xs"><SelectValue /></SelectTrigger>
                    <SelectContent>
                      {SHADOWSOCKS_METHODS.map((m) => (
                        <SelectItem key={m} value={m}>{m}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">{t("admin.configCenter.inbound.password")}</label>
                  <Input
                    value={user.password ?? ""}
                    onChange={(e) => updateUser(i, { password: e.target.value || undefined })}
                    placeholder="password"
                    disabled={readOnly}
                    className="font-mono text-xs"
                  />
                </div>
              </>
            )}

            {/* Username/Password: SOCKS, HTTP */}
            {isUsernamePasswordProtocol(protocol) && (
              <>
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">{t("admin.configCenter.inbound.username")}</label>
                  <Input
                    value={user.username ?? ""}
                    onChange={(e) => updateUser(i, { username: e.target.value || undefined })}
                    placeholder="username"
                    disabled={readOnly}
                    className="text-xs"
                  />
                </div>
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">{t("admin.configCenter.inbound.password")}</label>
                  <Input
                    value={user.password ?? ""}
                    onChange={(e) => updateUser(i, { password: e.target.value || undefined })}
                    placeholder="password"
                    disabled={readOnly}
                    className="font-mono text-xs"
                  />
                </div>
              </>
            )}

            {/* Email (optional, common across all) */}
            <div className="space-y-1">
              <label className="text-xs text-muted-foreground">{t("admin.configCenter.inbound.email")}</label>
              <Input
                value={user.email ?? ""}
                onChange={(e) => updateUser(i, { email: e.target.value || undefined })}
                placeholder="user@example.com"
                disabled={readOnly}
                className="text-xs"
              />
            </div>

            {/* Level (optional) */}
            <div className="space-y-1">
              <label className="text-xs text-muted-foreground">{t("admin.configCenter.inbound.userLevel")}</label>
              <Input
                type="number"
                min={0}
                value={user.level ?? ""}
                onChange={(e) => updateUser(i, { level: e.target.value ? parseInt(e.target.value, 10) : undefined })}
                disabled={readOnly}
                className="h-8 text-xs"
              />
            </div>
          </div>
        </div>
      );
    })}
    </div>
  );
}
