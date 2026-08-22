/**
 * InboundFallbackFields — Xray VLESS fallback configuration.
 * Stored in _raw.fallbacks array through the semantic spec.
 */
import { useCallback, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Button, Input } from "@/components/ui";

interface InboundFallbackFieldsProps {
  protocol: string;
  values: Array<Record<string, unknown>> | undefined | null;
  onChange: (fallbacks: Array<Record<string, unknown>> | undefined) => void;
  readOnly?: boolean;
}

export function InboundFallbackFields({
  protocol,
  values,
  onChange,
  readOnly = false,
}: InboundFallbackFieldsProps) {
  const { t } = useTranslation();

  const list = useMemo(() => values ?? [], [values]);

  const updateEntry = useCallback(
    (index: number, partial: Record<string, unknown>) => {
      const next = [...list];
      next[index] = { ...next[index], ...partial };
      onChange(next.length > 0 ? next : undefined);
    },
    [list, onChange],
  );

  const removeEntry = useCallback(
    (index: number) => {
      const next = list.filter((_, i) => i !== index);
      onChange(next.length > 0 ? next : undefined);
    },
    [list, onChange],
  );

  const addEntry = useCallback(() => {
    onChange([...list, { dest: "" }]);
  }, [list, onChange]);

  // Only for Xray VLESS
  if (protocol !== "vless") return null;

  return (
    <div className="space-y-3 rounded-md border bg-muted/20 p-4" data-testid="inbound-fallback-fields">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold">{t("admin.configCenter.inbound.fallbacks", "Fallbacks 回落")}</h3>
        {!readOnly && (
          <Button type="button" size="sm" variant="outline" onClick={addEntry}>
            + {t("admin.configCenter.inbound.addFallback", "添加回落")}
          </Button>
        )}
      </div>

      {list.length === 0 && (
        <p className="text-xs text-muted-foreground">
          {t("admin.configCenter.inbound.noFallbacks", "暂无回落配置")}
        </p>
      )}

      {list.map((entry, i) => (
        <div
          key={i}
          className="space-y-3 rounded-md border border-border/60 bg-background p-3"
        >
          <div className="flex items-start justify-between gap-2">
            <span className="text-xs font-medium text-muted-foreground">#{i + 1}</span>
            {!readOnly && (
              <Button
                type="button"
                size="sm"
                variant="ghost"
                className="text-destructive h-auto px-2 py-1 text-xs"
                onClick={() => removeEntry(i)}
              >
                {t("common.delete")}
              </Button>
            )}
          </div>

          <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
            <div className="space-y-1">
              <label className="text-xs text-muted-foreground">{t("admin.configCenter.inbound.dest", "目的地")}</label>
              <Input
                value={String(entry.dest ?? "")}
                onChange={(e) => updateEntry(i, { dest: e.target.value || undefined })}
                placeholder="80"
                disabled={readOnly}
                className="h-8 text-xs"
              />
            </div>
            <div className="space-y-1">
              <label className="text-xs text-muted-foreground">{t("admin.configCenter.inbound.port")}</label>
              <Input
                type="number"
                min={0}
                max={65535}
                value={entry.port != null ? String(entry.port) : ""}
                onChange={(e) => updateEntry(i, { port: e.target.value ? parseInt(e.target.value, 10) : undefined })}
                placeholder="8080"
                disabled={readOnly}
                className="h-8 text-xs"
              />
            </div>
            <div className="space-y-1">
              <label className="text-xs text-muted-foreground">{t("admin.configCenter.inbound.path")}</label>
              <Input
                value={String(entry.path ?? "")}
                onChange={(e) => updateEntry(i, { path: e.target.value || undefined })}
                placeholder="/websocket"
                disabled={readOnly}
                className="h-8 text-xs font-mono"
              />
            </div>
            <div className="space-y-1">
              <label className="text-xs text-muted-foreground">{t("admin.configCenter.inbound.xver")}</label>
              <Input
                type="number"
                min={0}
                value={entry.xver != null ? String(entry.xver) : ""}
                onChange={(e) => updateEntry(i, { xver: e.target.value ? parseInt(e.target.value, 10) : undefined })}
                disabled={readOnly}
                className="h-8 text-xs"
              />
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
