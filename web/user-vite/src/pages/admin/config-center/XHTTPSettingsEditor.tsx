import { useTranslation } from "react-i18next";
import type { ConfigCenterXHTTPMode, ConfigCenterXrayTransport } from "@/types";
import {
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Textarea,
} from "@/components/ui";
import type { XHTTPJSONDraftState } from "./xhttpDraft";

type XHTTPSettingsEditorProps = {
  transportOptions: ConfigCenterXrayTransport[];
  modeOptions: ConfigCenterXHTTPMode[];
  xrayTransport: ConfigCenterXrayTransport;
  xhttpMode: ConfigCenterXHTTPMode;
  xhttpSettings: Record<string, unknown>;
  xhttpJsonDraft: XHTTPJSONDraftState;
  isXHTTPSelected: boolean;
  hasHostHeader: boolean;
  hasDownloadSettingsConflict: boolean;
  onTransportChange: (value: string) => void;
  onStringChange: (field: "host" | "path", value: string) => void;
  onModeChange: (value: string) => void;
  onDraftChange: (field: keyof XHTTPJSONDraftState, value: string) => void;
  onDraftBlur: (field: keyof XHTTPJSONDraftState) => void;
};

const FIELD_IDS = {
  transport: "config-center-xray-transport-trigger",
  transportLabel: "config-center-xray-transport-label",
  host: "config-center-xhttp-host-input",
  path: "config-center-xhttp-path-input",
  mode: "config-center-xhttp-mode-trigger",
  modeLabel: "config-center-xhttp-mode-label",
  headers: "config-center-xhttp-headers-input",
  extra: "config-center-xhttp-extra-input",
  xmux: "config-center-xhttp-xmux-input",
  downloadSettings: "config-center-xhttp-download-settings-input",
};

function stringValue(value: unknown): string {
  return typeof value === "string" ? value : "";
}

export function XHTTPSettingsEditor({
  transportOptions,
  modeOptions,
  xrayTransport,
  xhttpMode,
  xhttpSettings,
  xhttpJsonDraft,
  isXHTTPSelected,
  hasHostHeader,
  hasDownloadSettingsConflict,
  onTransportChange,
  onStringChange,
  onModeChange,
  onDraftChange,
  onDraftBlur,
}: XHTTPSettingsEditorProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-4 rounded-md border bg-muted/20 p-4" data-testid="config-center-xhttp-editor">
      <div className="space-y-1">
        <h3 className="text-sm font-semibold">{t("admin.configCenter.xhttp.title")}</h3>
        <p className="text-xs text-muted-foreground">{t("admin.configCenter.xhttp.description")}</p>
      </div>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-4">
        <div className="space-y-2">
          <label id={FIELD_IDS.transportLabel} htmlFor={FIELD_IDS.transport} className="text-sm font-medium">
            {t("admin.configCenter.xhttp.fields.transport")}
          </label>
          <Select value={xrayTransport} onValueChange={onTransportChange}>
            <SelectTrigger
              id={FIELD_IDS.transport}
              aria-labelledby={FIELD_IDS.transportLabel}
              data-testid="config-center-xray-transport"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {transportOptions.map((item) => (
                <SelectItem key={item} value={item}>
                  {t(`admin.configCenter.xhttp.transportOptions.${item}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <label htmlFor={FIELD_IDS.host} className="text-sm font-medium">
            {t("admin.configCenter.xhttp.fields.host")}
          </label>
          <Input
            id={FIELD_IDS.host}
            data-testid="config-center-xhttp-host"
            value={stringValue(xhttpSettings.host)}
            onChange={(event) => onStringChange("host", event.target.value)}
            placeholder={t("admin.configCenter.xhttp.placeholders.host")}
            disabled={!isXHTTPSelected}
          />
        </div>
        <div className="space-y-2">
          <label htmlFor={FIELD_IDS.path} className="text-sm font-medium">
            {t("admin.configCenter.xhttp.fields.path")}
          </label>
          <Input
            id={FIELD_IDS.path}
            data-testid="config-center-xhttp-path"
            value={stringValue(xhttpSettings.path)}
            onChange={(event) => onStringChange("path", event.target.value)}
            placeholder={t("admin.configCenter.xhttp.placeholders.path")}
            disabled={!isXHTTPSelected}
          />
        </div>
        <div className="space-y-2">
          <label id={FIELD_IDS.modeLabel} htmlFor={FIELD_IDS.mode} className="text-sm font-medium">
            {t("admin.configCenter.xhttp.fields.mode")}
          </label>
          <Select value={xhttpMode} onValueChange={onModeChange} disabled={!isXHTTPSelected}>
            <SelectTrigger
              id={FIELD_IDS.mode}
              aria-labelledby={FIELD_IDS.modeLabel}
              data-testid="config-center-xhttp-mode"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {modeOptions.map((item) => (
                <SelectItem key={item} value={item}>
                  {t(`admin.configCenter.xhttp.modeOptions.${item}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      {!isXHTTPSelected && (
        <p className="rounded-md border bg-background px-3 py-2 text-xs text-muted-foreground">
          {t("admin.configCenter.xhttp.disabledHint")}
        </p>
      )}

      <div className="space-y-2">
        <label htmlFor={FIELD_IDS.headers} className="text-sm font-medium">
          {t("admin.configCenter.xhttp.fields.headers")}
        </label>
        <Textarea
          id={FIELD_IDS.headers}
          data-testid="config-center-xhttp-headers"
          className="min-h-[84px] font-mono text-xs"
          value={xhttpJsonDraft.headers}
          onChange={(event) => onDraftChange("headers", event.target.value)}
          onBlur={() => onDraftBlur("headers")}
          placeholder={t("admin.configCenter.xhttp.placeholders.json")}
          disabled={!isXHTTPSelected}
        />
        <p className="text-xs text-muted-foreground">{t("admin.configCenter.xhttp.headersHint")}</p>
        {hasHostHeader && (
          <p className="text-xs text-destructive">{t("admin.configCenter.xhttp.hostHeaderWarning")}</p>
        )}
      </div>

      <div className="space-y-2">
        <div>
          <p className="text-sm font-medium">{t("admin.configCenter.xhttp.advancedTitle")}</p>
          <p className="text-xs text-muted-foreground">{t("admin.configCenter.xhttp.advancedDescription")}</p>
        </div>
        <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
          <div className="space-y-2">
            <label htmlFor={FIELD_IDS.extra} className="text-xs font-medium text-muted-foreground">
              {t("admin.configCenter.xhttp.fields.extra")}
            </label>
            <Textarea
              id={FIELD_IDS.extra}
              data-testid="config-center-xhttp-extra"
              className="min-h-[96px] font-mono text-xs"
              value={xhttpJsonDraft.extra}
              onChange={(event) => onDraftChange("extra", event.target.value)}
              onBlur={() => onDraftBlur("extra")}
              placeholder={t("admin.configCenter.xhttp.placeholders.json")}
              disabled={!isXHTTPSelected}
            />
          </div>
          <div className="space-y-2">
            <label htmlFor={FIELD_IDS.xmux} className="text-xs font-medium text-muted-foreground">
              {t("admin.configCenter.xhttp.fields.xmux")}
            </label>
            <Textarea
              id={FIELD_IDS.xmux}
              data-testid="config-center-xhttp-xmux"
              className="min-h-[96px] font-mono text-xs"
              value={xhttpJsonDraft.xmux}
              onChange={(event) => onDraftChange("xmux", event.target.value)}
              onBlur={() => onDraftBlur("xmux")}
              placeholder={t("admin.configCenter.xhttp.placeholders.json")}
              disabled={!isXHTTPSelected}
            />
          </div>
          <div className="space-y-2">
            <label htmlFor={FIELD_IDS.downloadSettings} className="text-xs font-medium text-muted-foreground">
              {t("admin.configCenter.xhttp.fields.downloadSettings")}
            </label>
            <Textarea
              id={FIELD_IDS.downloadSettings}
              data-testid="config-center-xhttp-download-settings"
              className="min-h-[96px] font-mono text-xs"
              value={xhttpJsonDraft.downloadSettings}
              onChange={(event) => onDraftChange("downloadSettings", event.target.value)}
              onBlur={() => onDraftBlur("downloadSettings")}
              placeholder={t("admin.configCenter.xhttp.placeholders.json")}
              disabled={!isXHTTPSelected}
            />
          </div>
        </div>
        {hasDownloadSettingsConflict && (
          <p className="text-xs text-destructive">
            {t("admin.configCenter.xhttp.streamOneDownloadWarning")}
          </p>
        )}
      </div>
    </div>
  );
}
