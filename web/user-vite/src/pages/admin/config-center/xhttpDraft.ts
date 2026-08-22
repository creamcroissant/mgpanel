import type { ConfigCenterCoreType } from "@/types";

export type XHTTPJSONDraftState = {
  headers: string;
  extra: string;
  xmux: string;
  downloadSettings: string;
};

export type ParsedXHTTPJSONDraftState = Record<keyof XHTTPJSONDraftState, Record<string, unknown>>;

const XHTTP_JSON_DRAFT_FIELDS: Array<keyof XHTTPJSONDraftState> = [
  "headers",
  "extra",
  "xmux",
  "downloadSettings",
];

export const defaultXHTTPJSONDraftState: XHTTPJSONDraftState = {
  headers: "{}",
  extra: "{}",
  xmux: "{}",
  downloadSettings: "{}",
};

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function safeParseJSON(input: string, fallback: unknown = {}): unknown {
  const text = input.trim();
  if (!text) return fallback;
  return JSON.parse(text);
}

export function prettyJSON(input: unknown): string {
  try {
    return JSON.stringify(input ?? {}, null, 2);
  } catch {
    return "{}";
  }
}

export function ensureObjectField(target: Record<string, unknown>, key: string): Record<string, unknown> {
  const current = target[key];
  if (isRecord(current)) {
    return current;
  }
  const next: Record<string, unknown> = {};
  target[key] = next;
  return next;
}

export function pickCoreSpecificScope(
  coreSpecific: Record<string, unknown>,
  coreType: ConfigCenterCoreType
): Record<string, unknown> {
  if (coreType === "xray") {
    if (isRecord(coreSpecific.xray)) {
      return coreSpecific.xray;
    }
    return coreSpecific;
  }

  const singBoxScoped = coreSpecific["sing-box"];
  if (isRecord(singBoxScoped)) {
    return singBoxScoped;
  }
  if (isRecord(coreSpecific.singbox)) {
    return coreSpecific.singbox;
  }
  return coreSpecific;
}

export function parseJSONRecord(input: string): Record<string, unknown> | null {
  try {
    const parsed = safeParseJSON(input, {});
    return isRecord(parsed) ? parsed : {};
  } catch {
    return null;
  }
}

export function xrayStreamSettingsFromCoreSpecific(
  coreSpecific: Record<string, unknown> | null,
  coreType: ConfigCenterCoreType
): Record<string, unknown> | null {
  if (!coreSpecific || coreType !== "xray") {
    return null;
  }
  const scope = pickCoreSpecificScope(coreSpecific, coreType);
  return isRecord(scope.streamSettings) ? scope.streamSettings : {};
}

export function xhttpSettingsFromStreamSettings(streamSettings: Record<string, unknown> | null): Record<string, unknown> {
  if (!streamSettings || !isRecord(streamSettings.xhttpSettings)) {
    return {};
  }
  return streamSettings.xhttpSettings;
}

export function xhttpJSONDraftFromCoreSpecific(
  coreSpecific: Record<string, unknown> | null,
  coreType: ConfigCenterCoreType
): XHTTPJSONDraftState {
  const xhttpSettings = xhttpSettingsFromStreamSettings(
    xrayStreamSettingsFromCoreSpecific(coreSpecific, coreType)
  );
  return {
    headers: prettyJSON(isRecord(xhttpSettings.headers) ? xhttpSettings.headers : {}),
    extra: prettyJSON(isRecord(xhttpSettings.extra) ? xhttpSettings.extra : {}),
    xmux: prettyJSON(isRecord(xhttpSettings.xmux) ? xhttpSettings.xmux : {}),
    downloadSettings: prettyJSON(
      isRecord(xhttpSettings.downloadSettings) ? xhttpSettings.downloadSettings : {}
    ),
  };
}

export function parseJSONDraftRecord(input: string): Record<string, unknown> | null {
  const parsed = parseJSONRecord(input);
  if (!parsed) {
    return null;
  }
  return { ...parsed };
}

export function parseXHTTPJSONDraftState(
  draft: XHTTPJSONDraftState
): ParsedXHTTPJSONDraftState | null {
  const parsed = {} as ParsedXHTTPJSONDraftState;
  for (const field of XHTTP_JSON_DRAFT_FIELDS) {
    const value = parseJSONDraftRecord(draft[field]);
    if (!value) {
      return null;
    }
    parsed[field] = value;
  }
  return parsed;
}

export function applyXHTTPJSONDraftToCoreSpecific(
  coreSpecific: Record<string, unknown>,
  coreType: ConfigCenterCoreType,
  draft: ParsedXHTTPJSONDraftState
) {
  const scope = pickCoreSpecificScope(coreSpecific, coreType);
  const streamSettings = ensureObjectField(scope, "streamSettings");
  streamSettings.network = "xhttp";
  const xhttpSettings = ensureObjectField(streamSettings, "xhttpSettings");
  for (const field of XHTTP_JSON_DRAFT_FIELDS) {
    if (Object.keys(draft[field]).length > 0) {
      xhttpSettings[field] = draft[field];
    } else {
      delete xhttpSettings[field];
    }
  }
}

export function prettyXHTTPJSONDraftState(
  draft: ParsedXHTTPJSONDraftState
): XHTTPJSONDraftState {
  return {
    headers: prettyJSON(draft.headers),
    extra: prettyJSON(draft.extra),
    xmux: prettyJSON(draft.xmux),
    downloadSettings: prettyJSON(draft.downloadSettings),
  };
}

export function hasJSONDraftKeys(input: string): boolean {
  const parsed = parseJSONDraftRecord(input);
  return Boolean(parsed && Object.keys(parsed).length > 0);
}

export function hasHostHeader(input: string): boolean {
  const parsed = parseJSONDraftRecord(input);
  return Boolean(parsed && Object.keys(parsed).some((key) => key.trim().toLowerCase() === "host"));
}
