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

interface DnsConfigEditorProps {
  value: string;  // JSON string of dns config_data
  onChange: (jsonString: string) => void;
}

interface DnsServer {
  address: string;
  port?: number;
  tag?: string;
  clientIP?: string;
  domains?: string[];
  expectedIPs?: string[];
  queryStrategy?: string;
  strategy?: string;
  skipFallback?: boolean;
  disableCache?: boolean;
  serveStale?: boolean;
}

interface DnsRule {
  domain?: string[];
  rule_set?: string[];
  server?: string;
  action?: string;
  query_type?: string;
}

const STRATEGY_OPTIONS = ["UseIP", "UseIPv4", "UseIPv6", "prefer_ipv4", "prefer_ipv6", "ipv4_only", "ipv6_only"];

function arrFromString(val: unknown): string[] {
  if (Array.isArray(val)) return val.map(String);
  if (typeof val === "string" && val.trim()) return val.split(",").map((s) => s.trim()).filter(Boolean);
  return [];
}

function stringFromArr(val: unknown): string {
  if (Array.isArray(val)) return val.join(", ");
  return "";
}

export function DnsConfigEditor({ value, onChange }: DnsConfigEditorProps) {
  const { t } = useTranslation();
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [serverDialogOpen, setServerDialogOpen] = useState(false);
  const [editServerIdx, setEditServerIdx] = useState<number | null>(null);
  const [editServer, setEditServer] = useState<DnsServer | null>(null);
  const [ruleDialogOpen, setRuleDialogOpen] = useState(false);
  const [editRuleIdx, setEditRuleIdx] = useState<number | null>(null);
  const [editRule, setEditRule] = useState<DnsRule | null>(null);

  const config = useMemo<Record<string, unknown>>(() => {
    try { return JSON.parse(value) as Record<string, unknown>; }
    catch { return {}; }
  }, [value]);

  const servers = useMemo(() => (config.servers as DnsServer[]) ?? [], [config]);
  const rules = useMemo(() => (config.rules as DnsRule[]) ?? [], [config]);

  const updateConfig = (partial: Record<string, unknown>) => {
    const next = { ...config, ...partial };
    onChange(JSON.stringify(next, null, 2));
  };

  // Server dialog
  const openAddServer = () => { setEditServerIdx(null); setEditServer({ address: "", port: 53 }); setServerDialogOpen(true); };
  const openEditServer = (idx: number) => { setEditServerIdx(idx); setEditServer({ ...servers[idx] }); setServerDialogOpen(true); };
  const closeServerDialog = () => { setServerDialogOpen(false); setEditServerIdx(null); setEditServer(null); };
  const saveServer = () => {
    if (!editServer || !editServer.address.trim()) return;
    const next = [...servers];
    if (editServerIdx != null) next[editServerIdx] = { ...editServer };
    else next.push({ ...editServer });
    updateConfig({ ...config, servers: next });
    closeServerDialog();
  };
  const deleteServer = (idx: number) => updateConfig({ ...config, servers: servers.filter((_, i) => i !== idx) });

  // Rule dialog
  const openAddRule = () => { setEditRuleIdx(null); setEditRule({ domain: [], server: "" }); setRuleDialogOpen(true); };
  const openEditRule = (idx: number) => { setEditRuleIdx(idx); setEditRule({ ...rules[idx] }); setRuleDialogOpen(true); };
  const closeRuleDialog = () => { setRuleDialogOpen(false); setEditRuleIdx(null); setEditRule(null); };
  const saveRule = () => {
    if (!editRule) return;
    const next = [...rules];
    if (editRuleIdx != null) next[editRuleIdx] = { ...editRule };
    else next.push({ ...editRule });
    updateConfig({ ...config, rules: next });
    closeRuleDialog();
  };
  const deleteRule = (idx: number) => updateConfig({ ...config, rules: rules.filter((_, i) => i !== idx) });

  // Global toggles
  const boolFields = ["disableCache", "disableFallback", "disableFallbackIfMatch"] as const;

  return (
    <div className="space-y-4">
      {/* Global Settings */}
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <label className="text-sm font-medium">{t("admin.configCenter.inbound.strategy")}</label>
          <Select value={(config.queryStrategy as string) || (config.strategy as string) || ""} onValueChange={(v) => updateConfig({ queryStrategy: v || undefined, strategy: v || undefined })}>
            <SelectTrigger><SelectValue placeholder={t("admin.configCenter.inbound.selectMode")} /></SelectTrigger>
            <SelectContent>
              {STRATEGY_OPTIONS.map((s) => (<SelectItem key={s} value={s}>{s}</SelectItem>))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <label className="text-sm font-medium">Final / Fallback</label>
          <Input value={(config.final as string) ?? ""} onChange={(e) => updateConfig({ final: e.target.value || undefined })} placeholder="dns-main" />
        </div>
      </div>
      <div className="flex flex-wrap gap-4">
        {boolFields.map((field) => (
          <label key={field} className="flex items-center gap-2 text-sm">
            <Switch checked={((config[field] as boolean) ?? false)} onCheckedChange={(value) => updateConfig({ [field]: value || undefined })} />
            {field}
          </label>
        ))}
      </div>

      {/* Servers */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <label className="text-sm font-semibold">{t("admin.configCenter.dns.servers")} ({servers.length})</label>
          <Button size="sm" onClick={openAddServer}>{t("admin.configCenter.dns.addServer")}</Button>
        </div>
        {servers.length === 0 ? (
          <div className="rounded-md border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
            {t("admin.configCenter.dns.noServers")}
          </div>
        ) : (
          <div className="space-y-1">
            {servers.map((srv, idx) => (
              <div key={idx} className="flex items-center gap-2 rounded-md border border-border p-2 hover:bg-muted/20">
                <div className="flex-1 min-w-0">
                  <span className="text-xs font-mono truncate block">
                    {srv.address}{srv.port ? `:${srv.port}` : ""} {srv.tag ? `[${srv.tag}]` : ""}
                  </span>
                </div>
                <Button size="sm" variant="outline" onClick={() => openEditServer(idx)}>{t("common.edit")}</Button>
                <Button size="sm" variant="destructive" onClick={() => deleteServer(idx)}>{t("common.delete")}</Button>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* DNS Rules */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <label className="text-sm font-semibold">{t("admin.configCenter.dns.rules")} ({rules.length})</label>
          <Button size="sm" onClick={openAddRule}>{t("admin.configCenter.dns.addRule")}</Button>
        </div>
        {rules.length === 0 ? (
          <div className="rounded-md border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
            {t("admin.configCenter.dns.noRules")}
          </div>
        ) : (
          <div className="space-y-1">
            {rules.map((rule, idx) => (
              <div key={idx} className="flex items-center gap-2 rounded-md border border-border p-2 hover:bg-muted/20">
                <div className="flex-1 min-w-0">
                  <span className="text-xs font-mono truncate block">
                    {rule.domain?.slice(0, 2).join(", ")}{(rule.domain?.length ?? 0) > 2 ? "..." : ""} → {rule.server || rule.action || "-"}
                  </span>
                </div>
                <Button size="sm" variant="outline" onClick={() => openEditRule(idx)}>{t("common.edit")}</Button>
                <Button size="sm" variant="destructive" onClick={() => deleteRule(idx)}>{t("common.delete")}</Button>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Server Dialog */}
      {serverDialogOpen && editServer && (
        <div className="rounded-md border border-border p-4 space-y-4">
          <h4 className="text-sm font-semibold">{editServerIdx != null ? t("common.edit") : t("admin.configCenter.dns.addServer")}</h4>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.dns.address")}</label>
              <Input value={editServer.address} onChange={(e) => setEditServer({ ...editServer, address: e.target.value })} placeholder="1.1.1.1 / https://..." />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.port")}</label>
              <Input type="number" value={editServer.port ?? ""} onChange={(e) => setEditServer({ ...editServer, port: parseInt(e.target.value) || undefined })} min={1} max={65535} />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.tag")}</label>
              <Input value={editServer.tag ?? ""} onChange={(e) => setEditServer({ ...editServer, tag: e.target.value || undefined })} placeholder="dns-main" />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.dns.clientIP")}</label>
              <Input value={editServer.clientIP ?? ""} onChange={(e) => setEditServer({ ...editServer, clientIP: e.target.value || undefined })} />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.strategy")}</label>
              <Select value={editServer.queryStrategy ?? editServer.strategy ?? ""} onValueChange={(v) => setEditServer({ ...editServer, queryStrategy: v || undefined, strategy: v || undefined })}>
                <SelectTrigger><SelectValue placeholder={t("admin.configCenter.inbound.selectMode")} /></SelectTrigger>
                <SelectContent>
                  {STRATEGY_OPTIONS.map((s) => (<SelectItem key={s} value={s}>{s}</SelectItem>))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.domain")}</label>
              <Input value={stringFromArr(editServer.domains)} onChange={(e) => setEditServer({ ...editServer, domains: arrFromString(e.target.value) })} placeholder="domain:example.com" />
            </div>
          </div>
          <div className="flex gap-2 justify-end">
            <Button variant="outline" onClick={closeServerDialog}>{t("common.cancel")}</Button>
            <Button onClick={saveServer}>{t("common.save")}</Button>
          </div>
        </div>
      )}

      {/* DNS Rule Dialog */}
      {ruleDialogOpen && editRule && (
        <div className="rounded-md border border-border p-4 space-y-4">
          <h4 className="text-sm font-semibold">{editRuleIdx != null ? t("common.edit") : t("admin.configCenter.dns.addRule")}</h4>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.domain")}</label>
              <Input value={stringFromArr(editRule.domain)} onChange={(e) => setEditRule({ ...editRule, domain: arrFromString(e.target.value) })} placeholder="example.com, geosite:cn" />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.dns.server")}</label>
              <Input value={editRule.server ?? ""} onChange={(e) => setEditRule({ ...editRule, server: e.target.value || undefined })} placeholder="dns-main" />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.dns.queryType")}</label>
              <Input value={editRule.query_type ?? ""} onChange={(e) => setEditRule({ ...editRule, query_type: e.target.value || undefined })} placeholder="A, AAAA" />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.dns.action")}</label>
              <Input value={editRule.action ?? ""} onChange={(e) => setEditRule({ ...editRule, action: e.target.value || undefined })} placeholder="route, block" />
            </div>
          </div>
          <div className="flex gap-2 justify-end">
            <Button variant="outline" onClick={closeRuleDialog}>{t("common.cancel")}</Button>
            <Button onClick={saveRule}>{t("common.save")}</Button>
          </div>
        </div>
      )}

      {/* Raw JSON Fallback */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <label className="text-sm font-medium">{t("admin.configCenter.advancedJson.title")}</label>
          <Button type="button" size="sm" variant="outline" onClick={() => setShowAdvanced((p) => !p)}>
            {showAdvanced ? t("admin.configCenter.advancedJson.hide") : t("admin.configCenter.advancedJson.show")}
          </Button>
        </div>
        {showAdvanced && (
          <Textarea
            className="min-h-[150px] font-mono text-xs"
            value={value}
            onChange={(e) => onChange(e.target.value)}
            placeholder="{}"
          />
        )}
      </div>
    </div>
  );
}
