import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Button,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Switch,
  Textarea,
} from "@/components/ui";
import type { PolicyConfig, ExperimentalConfig, NTPConfig, LogConfig, ApiConfig } from "@/types/xray-config";

interface CoreSettingsEditorProps {
  value: string;  // JSON string
  onChange: (jsonString: string) => void;
}

export function CoreSettingsEditor({ value, onChange }: CoreSettingsEditorProps) {
  const { t } = useTranslation();
  const [showAdvanced, setShowAdvanced] = useState(false);

  const parsed = useMemo<Record<string, unknown>>(() => {
    try { return JSON.parse(value) as Record<string, unknown>; }
    catch { return {}; }
  }, [value]);

  const setSection = (section: string, data: unknown) => {
    const next = { ...parsed };
    if (data === undefined || data === null || (typeof data === "object" && Object.keys(data as Record<string, unknown>).length === 0)) {
      delete next[section];
    } else {
      next[section] = data;
    }
    onChange(JSON.stringify(next, null, 2));
  };

  const logConfig = parsed.log as LogConfig | undefined;
  const apiConfig = parsed.api as ApiConfig | undefined;
  const experimentalConfig = parsed.experimental as ExperimentalConfig | undefined;
  const ntpConfig = parsed.ntp as NTPConfig | undefined;
  const statsConfig = parsed.stats as Record<string, unknown> | undefined;
  const policyConfig = parsed.policy as PolicyConfig | undefined;

  return (
    <div className="space-y-4">
      {/* Log */}
      <details className="rounded-md border border-border">
        <summary className="cursor-pointer px-4 py-2 text-sm font-semibold hover:bg-muted/30">
          {t("admin.configCenter.inbound.log")} — Log
        </summary>
        <div className="space-y-3 p-4 border-t border-border">
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.configCenter.inbound.level")}</label>
            <Select
              value={(logConfig?.loglevel as string) ?? (logConfig?.level as string) ?? "info"}
              onValueChange={(v) => setSection("log", { ...(logConfig ?? {}), level: v })}
            >
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="debug">debug</SelectItem>
                <SelectItem value="info">info</SelectItem>
                <SelectItem value="warning">warning</SelectItem>
                <SelectItem value="error">error</SelectItem>
                <SelectItem value="panic">panic</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.configCenter.inbound.output")}</label>
            <Input
              value={(logConfig?.output as string) ?? ""}
              onChange={(e) => setSection("log", { ...(logConfig ?? {}), output: e.target.value || undefined })}
              placeholder="/var/log/xray/access.log"
            />
          </div>
          <label className="flex items-center gap-2 text-sm">
            <Switch checked={((logConfig?.disabled as boolean) ?? false)} onCheckedChange={(value) => setSection("log", { ...(logConfig ?? {}), disabled: value || undefined })} />
            {t("admin.configCenter.inbound.disabled")}
          </label>
        </div>
      </details>

      {/* API — Xray only */}
      <details className="rounded-md border border-border">
        <summary className="cursor-pointer px-4 py-2 text-sm font-semibold hover:bg-muted/30">
          API
        </summary>
        <div className="space-y-3 p-4 border-t border-border">
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.configCenter.inbound.tag")}</label>
            <Input value={(apiConfig?.tag as string) ?? ""}
              onChange={(e) => setSection("api", { ...(apiConfig ?? {}), tag: e.target.value || undefined })}
              placeholder="api" />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.configCenter.inbound.listen")}</label>
            <Input value={(apiConfig?.listen as string) ?? ""}
              onChange={(e) => setSection("api", { ...(apiConfig ?? {}), listen: e.target.value || undefined })}
              placeholder="127.0.0.1:8080" />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.configCenter.inbound.services")}</label>
            <Input value={((apiConfig?.services as string[]) ?? []).join(", ")}
              onChange={(e) => setSection("api", {
                ...(apiConfig ?? {}),
                services: e.target.value.split(",").map((s) => s.trim()).filter(Boolean),
              })}
              placeholder={t("admin.configCenter.coreSettings.placeholder")} />
          </div>
        </div>
      </details>

      {/* Stats — Xray only */}
      <details className="rounded-md border border-border">
        <summary className="cursor-pointer px-4 py-2 text-sm font-semibold hover:bg-muted/30">
          {t("admin.configCenter.coreSettings.stats")}
        </summary>
        <div className="space-y-3 p-4 border-t border-border">
          <label className="flex items-center gap-2 text-sm">
            <Switch checked={((statsConfig !== undefined))} onCheckedChange={(value) => setSection("stats", value ? {} : undefined)} />
            {t("admin.configCenter.inbound.enabled")}
          </label>
        </div>
      </details>

      {/* Policy — Xray only */}
      <details className="rounded-md border border-border">
        <summary className="cursor-pointer px-4 py-2 text-sm font-semibold hover:bg-muted/30">
          {t("admin.configCenter.coreSettings.policy")}
        </summary>
        <div className="space-y-3 p-4 border-t border-border">
          <div className="space-y-2">
            <label className="text-sm font-medium">connIdle (s)</label>
            <Input type="number" value={String(policyConfig?.levels?.["0"]?.connIdle ?? "")}
              onChange={(e) => setSection("policy", {
                ...(policyConfig ?? {}),
                levels: { "0": { ...(policyConfig?.levels?.["0"] ?? {}), connIdle: parseInt(e.target.value) || undefined } },
              })} placeholder="300" />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">handshake (s)</label>
            <Input type="number" value={policyConfig?.levels?.["0"]?.handshake ?? ""}
              onChange={(e) => setSection("policy", {
                ...(policyConfig ?? {}),
                levels: { "0": { ...(policyConfig?.levels?.["0"] ?? {}), handshake: parseInt(e.target.value) || undefined } },
              })} placeholder="4" />
          </div>
          <label className="flex items-center gap-2 text-sm">
            <Switch checked={(policyConfig?.system?.statsInboundUplink ?? false)} onCheckedChange={(value) => setSection("policy", {
                ...(policyConfig ?? {}),
                system: { ...(policyConfig?.system ?? {}), statsInboundUplink: value || undefined },
              })} />
            statsInboundUplink
          </label>
          <label className="flex items-center gap-2 text-sm">
            <Switch checked={(policyConfig?.system?.statsInboundDownlink ?? false)} onCheckedChange={(value) => setSection("policy", {
                ...(policyConfig ?? {}),
                system: { ...(policyConfig?.system ?? {}), statsInboundDownlink: value || undefined },
              })} />
            statsInboundDownlink
          </label>
        </div>
      </details>

      {/* Experimental — sing-box only */}
      <details className="rounded-md border border-border">
        <summary className="cursor-pointer px-4 py-2 text-sm font-semibold hover:bg-muted/30">
          Experimental
        </summary>
        <div className="space-y-3 p-4 border-t border-border">
          <label className="flex items-center gap-2 text-sm">
            <Switch checked={(experimentalConfig?.cache_file?.enabled ?? false)} onCheckedChange={(value) => setSection("experimental", {
                ...(experimentalConfig ?? {}),
                cache_file: { enabled: value, path: experimentalConfig?.cache_file?.path },
              })} />
            {t("admin.configCenter.inbound.enabled")}
          </label>
          {experimentalConfig?.cache_file?.enabled && (
            <div className="space-y-2">
              <label className="text-sm font-medium">Cache Path</label>
              <Input value={experimentalConfig?.cache_file?.path ?? ""}
                onChange={(e) => setSection("experimental", {
                  ...(experimentalConfig ?? {}),
                  cache_file: { enabled: true, path: e.target.value || undefined },
                })} placeholder="/etc/xray/cache.db" />
            </div>
          )}
        </div>
      </details>

      {/* NTP — sing-box only */}
      <details className="rounded-md border border-border">
        <summary className="cursor-pointer px-4 py-2 text-sm font-semibold hover:bg-muted/30">
          {t("admin.configCenter.coreSettings.ntp")}
        </summary>
        <div className="space-y-3 p-4 border-t border-border">
          <label className="flex items-center gap-2 text-sm">
            <Switch checked={((ntpConfig?.enabled as boolean) ?? false)} onCheckedChange={(value) => setSection("ntp", { enabled: value, server: ntpConfig?.server })} />
            {t("admin.configCenter.inbound.enabled")}
          </label>
          {!!ntpConfig?.enabled && (
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.coreSettings.server")}</label>
              <Input value={(ntpConfig?.server as string) ?? ""}
                onChange={(e) => setSection("ntp", { ...(ntpConfig ?? {}), server: e.target.value || undefined })}
                placeholder="time.apple.com" />
            </div>
          )}
        </div>
      </details>

      {/* Raw JSON fallback */}
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
