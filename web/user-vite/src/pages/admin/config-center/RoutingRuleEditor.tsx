import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button, Input, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, Switch, Textarea } from "@/components/ui";

interface RoutingRuleEditorProps {
  value: string;  // JSON string of routing config_data
  onChange: (jsonString: string) => void;
}

interface RoutingRule {
  enabled?: boolean;
  domain?: string[];
  ip?: string[];
  port?: string;
  sourcePort?: string;
  network?: string;
  protocol?: string[];
  inboundTag?: string[];
  outboundTag?: string;
  balancerTag?: string;
  rule_set?: string[];
  action?: string;
  type?: string;
}

interface RoutingConfig {
  domainStrategy?: string;
  final?: string;
  rules?: RoutingRule[];
}

const DOMAIN_STRATEGIES = ["AsIs", "IPIfNonMatch", "IPOnDemand", "prefer_ipv4", "prefer_ipv6", "ipv4_only", "ipv6_only"];

function emptyRule(): RoutingRule {
  return { enabled: true, action: "route" };
}

function arrFromString(val: unknown): string[] {
  if (Array.isArray(val)) return val.map(String);
  if (typeof val === "string" && val.trim()) return val.split(",").map((s) => s.trim()).filter(Boolean);
  return [];
}

function stringFromArr(val: unknown): string {
  if (Array.isArray(val)) return val.join(", ");
  return "";
}

export function RoutingRuleEditor({ value, onChange }: RoutingRuleEditorProps) {
  const { t } = useTranslation();
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [editIndex, setEditIndex] = useState<number | null>(null);
  const [editRule, setEditRule] = useState<RoutingRule | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);

  const config = useMemo<RoutingConfig>(() => {
    try { return JSON.parse(value) as RoutingConfig; }
    catch { return {}; }
  }, [value]);

  const { domainStrategy, final, rules = [] } = config;

  const updateConfig = (partial: Partial<RoutingConfig>) => {
    const next = { ...config, ...partial };
    onChange(JSON.stringify(next, null, 2));
  };

  const openAdd = () => {
    setEditIndex(null);
    setEditRule(emptyRule());
    setDialogOpen(true);
  };

  const openEdit = (idx: number) => {
    setEditIndex(idx);
    setEditRule({ ...rules[idx] });
    setDialogOpen(true);
  };

  const closeDialog = () => {
    setDialogOpen(false);
    setEditIndex(null);
    setEditRule(null);
  };

  const saveRule = () => {
    if (!editRule) return;
    const updated = [...rules];
    if (editIndex != null) {
      updated[editIndex] = { ...editRule };
    } else {
      updated.push({ ...editRule });
    }
    updateConfig({ ...config, rules: updated });
    closeDialog();
  };

  const deleteRule = (idx: number) => {
    const updated = rules.filter((_, i) => i !== idx);
    updateConfig({ ...config, rules: updated });
  };

  const moveRule = (idx: number, dir: "up" | "down") => {
    if ((dir === "up" && idx === 0) || (dir === "down" && idx >= rules.length - 1)) return;
    const target = dir === "up" ? idx - 1 : idx + 1;
    const updated = [...rules];
    [updated[idx], updated[target]] = [updated[target], updated[idx]];
    updateConfig({ ...config, rules: updated });
  };

  const formatRuleSummary = (rule: RoutingRule): string => {
    const parts: string[] = [];
    if (rule.domain?.length) parts.push(`domain:${rule.domain.slice(0, 2).join(",")}${rule.domain.length > 2 ? "..." : ""}`);
    if (rule.ip?.length) parts.push(`ip:${rule.ip.slice(0, 1)}`);
    if (rule.port) parts.push(`port:${rule.port}`);
    if (rule.protocol?.length) parts.push(`proto:${rule.protocol.join(",")}`);
    parts.push(rule.outboundTag || rule.balancerTag || rule.action || "route");
    return parts.join(" | ");
  };

  const updateEditRuleField = (key: keyof RoutingRule, value: unknown) => {
    if (!editRule) return;
    setEditRule({ ...editRule, [key]: value });
  };

  return (
    <div className="space-y-4">
      {/* Domain Strategy + Final */}
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <label className="text-sm font-medium">{t("admin.configCenter.routing.domainStrategy")}</label>
          <Select value={domainStrategy ?? "AsIs"} onValueChange={(v) => updateConfig({ domainStrategy: v })}>
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              {DOMAIN_STRATEGIES.map((s) => (<SelectItem key={s} value={s}>{s}</SelectItem>))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <label className="text-sm font-medium">{t("admin.configCenter.routing.final")}</label>
          <Input value={final ?? ""} onChange={(e) => updateConfig({ final: e.target.value || undefined })} placeholder="direct" />
        </div>
      </div>

      {/* Rules List */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <label className="text-sm font-semibold">{t("admin.configCenter.routing.rules")} ({rules.length})</label>
          <Button size="sm" onClick={openAdd}>{t("admin.configCenter.routing.addRule")}</Button>
        </div>
        {rules.length === 0 ? (
          <div className="rounded-md border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
            {t("admin.configCenter.routing.noRules")}
          </div>
        ) : (
          <div className="space-y-2">
            {rules.map((rule, idx) => (
              <div key={idx} className="flex items-center gap-2 rounded-md border border-border p-2 hover:bg-muted/20">
                <div className="flex flex-col gap-0.5">
                  <button className="text-xs text-muted-foreground hover:text-foreground" onClick={() => moveRule(idx, "up")} disabled={idx === 0}>↑</button>
                  <button className="text-xs text-muted-foreground hover:text-foreground" onClick={() => moveRule(idx, "down")} disabled={idx >= rules.length - 1}>↓</button>
                </div>
                <Switch
                  checked={rule.enabled !== false}
                  onCheckedChange={(c) => {
                    const updated = [...rules];
                    updated[idx] = { ...updated[idx], enabled: c };
                    updateConfig({ ...config, rules: updated });
                  }}
                />
                <div className="flex-1 min-w-0">
                  <span className="text-xs font-mono truncate block">{formatRuleSummary(rule)}</span>
                </div>
                <Button size="sm" variant="outline" onClick={() => openEdit(idx)}>{t("common.edit")}</Button>
                <Button size="sm" variant="destructive" onClick={() => deleteRule(idx)}>{t("common.delete")}</Button>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Rule Edit Dialog */}
      {dialogOpen && editRule && (
        <div className="rounded-md border border-border p-4 space-y-4">
          <h4 className="text-sm font-semibold">{editIndex != null ? t("admin.configCenter.routing.editRule") : t("admin.configCenter.routing.addRule")}</h4>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.domain")}</label>
              <Input value={stringFromArr(editRule.domain)} onChange={(e) => updateEditRuleField("domain", arrFromString(e.target.value))} placeholder="example.com, google.com" />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.routing.ip")}</label>
              <Input value={stringFromArr(editRule.ip)} onChange={(e) => updateEditRuleField("ip", arrFromString(e.target.value))} placeholder="10.0.0.0/8, 1.1.1.1" />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.port")}</label>
              <Input value={editRule.port ?? ""} onChange={(e) => updateEditRuleField("port", e.target.value || undefined)} placeholder="80, 443" />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.routing.sourcePort")}</label>
              <Input value={editRule.sourcePort ?? ""} onChange={(e) => updateEditRuleField("sourcePort", e.target.value || undefined)} />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.network")}</label>
              <Select value={editRule.network ?? ""} onValueChange={(v) => updateEditRuleField("network", v || undefined)}>
                <SelectTrigger><SelectValue placeholder={t("admin.configCenter.inbound.selectMode")} /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="tcp">tcp</SelectItem>
                  <SelectItem value="udp">udp</SelectItem>
                  <SelectItem value="tcp,udp">tcp,udp</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.inbound.protocol")}</label>
              <Input value={stringFromArr(editRule.protocol)} onChange={(e) => updateEditRuleField("protocol", arrFromString(e.target.value))} placeholder="http, tls, bittorrent" />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.routing.inboundTag")}</label>
              <Input value={stringFromArr(editRule.inboundTag)} onChange={(e) => updateEditRuleField("inboundTag", arrFromString(e.target.value))} placeholder="api, in-vless" />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.routing.outboundTag")}</label>
              <Input value={editRule.outboundTag ?? ""} onChange={(e) => updateEditRuleField("outboundTag", e.target.value || undefined)} placeholder="direct, block" />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.routing.ruleSet")}</label>
              <Input value={stringFromArr(editRule.rule_set)} onChange={(e) => updateEditRuleField("rule_set", arrFromString(e.target.value))} placeholder="geosite-openai" />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.configCenter.routing.action")}</label>
              <Select value={editRule.action ?? "route"} onValueChange={(v) => updateEditRuleField("action", v)}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="route">route</SelectItem>
                  <SelectItem value="direct">direct</SelectItem>
                  <SelectItem value="block">block</SelectItem>
                  <SelectItem value="sniff">sniff</SelectItem>
                  <SelectItem value="resolve">resolve</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Switch checked={editRule.enabled !== false} onCheckedChange={(c) => updateEditRuleField("enabled", c)} />
            <span className="text-sm">{t("admin.configCenter.fields.enabled")}</span>
          </div>
          <div className="flex gap-2 justify-end">
            <Button variant="outline" onClick={closeDialog}>{t("common.cancel")}</Button>
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
