import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { AlertTriangle } from "lucide-react";
import {
  Button,
  Checkbox,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Switch,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui";

type FieldType = "number" | "bool" | "string";

/** 白名单字段：path 必须与后端 agentBatchConfigWhitelist 一致。 */
const CONFIG_FIELDS: { path: string; type: FieldType; options?: string[] }[] = [
  { path: "interval.sync", type: "number" },
  { path: "interval.report", type: "number" },
  { path: "log.max_days", type: "number" },
  { path: "log.upload.enabled", type: "bool" },
  { path: "log.upload.max_lines", type: "number" },
  { path: "log.upload.interval_seconds", type: "number" },
  { path: "mesh.enabled", type: "bool" },
  { path: "unlock.enabled", type: "bool" },
  { path: "unlock.interval_hours", type: "number" },
  { path: "forwarding.enabled", type: "bool" },
  { path: "traffic.type", type: "string", options: ["netio", "none", "dummy", "xray_api"] },
  { path: "traffic.interface", type: "string" },
];

interface AgentBatchConfigDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  count: number;
  agentNames: string[];
  busy: boolean;
  result: {
    success: number[];
    failed: { agent_id: number; error: string }[];
  } | null;
  onConfirm: (fields: Record<string, string | number | boolean>) => void;
}

export default function AgentBatchConfigDialog({
  open,
  onOpenChange,
  count,
  agentNames,
  busy,
  result,
  onConfirm,
}: AgentBatchConfigDialogProps) {
  const { t } = useTranslation();
  const [checked, setChecked] = useState<Record<string, boolean>>({});
  const [values, setValues] = useState<Record<string, string>>({});

  const selectedPaths = useMemo(
    () => CONFIG_FIELDS.filter((f) => checked[f.path]).map((f) => f.path),
    [checked],
  );

  const toggle = (path: string) =>
    setChecked((prev) => ({ ...prev, [path]: !prev[path] }));

  const setValue = (path: string, v: string) =>
    setValues((prev) => ({ ...prev, [path]: v }));

  const handleConfirm = () => {
    const fields: Record<string, string | number | boolean> = {};
    for (const f of CONFIG_FIELDS) {
      if (!checked[f.path]) continue;
      const raw = (values[f.path] ?? "").trim();
      if (raw === "") continue; // 留空=不改此字段
      if (f.type === "number") {
        const n = Number(raw);
        if (!Number.isFinite(n)) continue;
        fields[f.path] = n;
      } else if (f.type === "bool") {
        fields[f.path] = raw === "true";
      } else {
        fields[f.path] = raw;
      }
    }
    onConfirm(fields);
  };

  const canConfirm = selectedPaths.some((p) => (values[p] ?? "").trim() !== "") && !busy;

  const handleClose = (next: boolean) => {
    if (!busy) onOpenChange(next);
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-2xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t("admin.agents.batch.configTitle")}</DialogTitle>
          <DialogDescription>
            {t("admin.agents.batch.configDesc", { count })}
            {agentNames.length > 0 && agentNames.length <= 5
              ? ` (${agentNames.join("、")})`
              : ""}
          </DialogDescription>
        </DialogHeader>

        <div className="flex items-start gap-2 rounded-none border border-warning/40 bg-warning/10 p-2.5 text-xs text-muted-foreground">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-warning" />
          <span>{t("admin.agents.batch.configRestartWarn")}</span>
        </div>

        <div className="space-y-2">
          {CONFIG_FIELDS.map((f) => {
            const isOn = !!checked[f.path];
            const labelKey = `admin.agents.batch.configFields.${f.path.replaceAll(".", "_")}`;
            const label = t(labelKey, { defaultValue: f.path });
            return (
              <div key={f.path} className="rounded-none border border-border p-2.5">
                <label className="flex cursor-pointer items-center gap-2 text-sm font-medium">
                  <Checkbox
                    checked={isOn}
                    onCheckedChange={() => toggle(f.path)}
                    aria-label={label}
                  />
                  <span>{label}</span>
                  <span className="font-mono text-xs font-normal text-muted-foreground">
                    {f.path}
                  </span>
                </label>
                {isOn && (
                  <div className="mt-2 pl-6">
                    {f.type === "bool" ? (
                      <div className="flex items-center gap-2">
                        <Switch
                          checked={(values[f.path] ?? "false") === "true"}
                          onCheckedChange={(v) => setValue(f.path, v ? "true" : "false")}
                          aria-label={label}
                        />
                        <span className="text-xs text-muted-foreground">
                          {(values[f.path] ?? "false") === "true"
                            ? t("common.enabled")
                            : t("common.disabled")}
                        </span>
                      </div>
                    ) : f.options ? (
                      <Select
                        value={values[f.path] ?? ""}
                        onValueChange={(v) => setValue(f.path, v)}
                      >
                        <SelectTrigger className="w-full">
                          <SelectValue placeholder={t("admin.agents.batch.configLeaveBlank")} />
                        </SelectTrigger>
                        <SelectContent>
                          {f.options.map((o) => (
                            <SelectItem key={o} value={o}>
                              {o}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    ) : (
                      <Input
                        value={values[f.path] ?? ""}
                        onChange={(e) => setValue(f.path, e.target.value)}
                        placeholder={t("admin.agents.batch.configLeaveBlank")}
                        inputMode={f.type === "number" ? "numeric" : undefined}
                      />
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>

        {result && (
          <div className="rounded-none border border-border bg-muted/30 p-2.5 text-xs">
            <div className="font-medium">
              {t("admin.agents.batch.configPartial", {
                ok: result.success.length,
                fail: result.failed.length,
              })}
            </div>
            {result.failed.length > 0 && (
              <ul className="mt-1 list-disc space-y-0.5 pl-4 text-destructive">
                {result.failed.slice(0, 5).map((f) => (
                  <li key={f.agent_id}>
                    {agentNames[f.agent_id] ?? f.agent_id}: {f.error}
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={() => handleClose(false)} disabled={busy}>
            {t("common.cancel")}
          </Button>
          <Button onClick={handleConfirm} disabled={!canConfirm}>
            {busy ? t("common.loading") : t("admin.agents.batch.configConfirm", { count })}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
