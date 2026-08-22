/** Spec editor dialog (create/edit), generator confirmation, and import dialog extracted from ConfigCenterPage */
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import type { ConfigCenterSpec } from "@/types/configCenter";
import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Switch,
  Textarea,
} from "@/components/ui";
import type { InboundSemanticSpec } from "@/types/configCenterInbound";
import { parseSemanticSpec, serializeSemanticSpec } from "@/types/configCenterInbound";
import {
  generateCompactUUID,
  generateHexString,
  generateRealityKeyPair,
  normalizeXrayTransport,
  hasMeaningfulValue,
} from "./configCenterPageUtils";
import {
  ensureObjectField,
  isRecord,
  parseJSONRecord,
  parseXHTTPJSONDraftState,
  pickCoreSpecificScope,
  prettyJSON,
  prettyXHTTPJSONDraftState,
  safeParseJSON,
  applyXHTTPJSONDraftToCoreSpecific,
  xhttpJSONDraftFromCoreSpecific,
  xrayStreamSettingsFromCoreSpecific,
  type XHTTPJSONDraftState,
} from "./xhttpDraft";
import { QUERY_KEYS } from "@/lib/constants";
import { importConfigCenterSpecsFromApplied } from "@/api/admin";
import {
  type CoreTypeOption,
  type SpecFormState,
  type SpecJSONErrors,
  type SpecJSONField,
  type GeneratorOverwriteAction,
  type ImportFormState,
  CORE_OPTIONS,
  GENERATOR_OVERWRITE_MESSAGE_KEYS,
  defaultImportFormState,
} from "./configCenterPageTypes";
import { defaultXHTTPJSONDraftState } from "./xhttpDraft";
import { ConfigCenterInboundEditor } from "./ConfigCenterInboundEditor";
import type { ScopeObject } from "@/types/protocol-config";

export interface SpecEditorDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  selectedSpec: ConfigCenterSpec | null;
  specForm: SpecFormState;
  onSpecFormChange: (form: SpecFormState) => void;
  hostOptions: Array<{ id: number; name: string }>;
  onSave: (payload?: { core_specific?: string }) => void;
  isSaving: boolean;
  selectedHostId?: number | null;
  selectedCoreType?: CoreTypeOption;
}

export function SpecEditorDialog({
  open,
  onOpenChange,
  selectedSpec,
  specForm,
  onSpecFormChange,
  hostOptions,
  onSave,
  isSaving,
  selectedHostId = null,
  selectedCoreType,
}: SpecEditorDialogProps) {
  const { t } = useTranslation();

  /* ============ internal state ============ */
  const [pendingGeneratorOverwriteAction, setPendingGeneratorOverwriteAction] =
    useState<GeneratorOverwriteAction | null>(null);
  const [specJSONErrors, setSpecJSONErrors] = useState<SpecJSONErrors>({});
  const [isAdvancedJsonOpen, setIsAdvancedJsonOpen] = useState(false);
  const [xhttpJsonDraft, setXHTTPJsonDraft] = useState<XHTTPJSONDraftState>(defaultXHTTPJSONDraftState);
  const [importOpen, setImportOpen] = useState(false);
  const [importForm, setImportForm] = useState<ImportFormState>(defaultImportFormState);
  const queryClient = useQueryClient();
  const importMutation = useMutation({
    mutationFn: importConfigCenterSpecsFromApplied,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_CONFIG_CENTER_SPECS });
      setImportOpen(false);
      toast.success(t("admin.configCenter.messages.importSuccess"));
    },
    onError: (err: Error) => toast.error(t("admin.configCenter.messages.importFailed"), { description: err.message }),
  });

  /* ============ derived state ============ */
  const parsedCoreSpecific = useMemo(
    () => parseJSONRecord(specForm.core_specific),
    [specForm.core_specific],
  );
  const inboundSpec = useMemo<InboundSemanticSpec>(
    () => parseSemanticSpec(parseJSONRecord(specForm.semantic_spec) ?? {}),
    [specForm.semantic_spec],
  );
  const xrayStreamSettings = useMemo(
    () => xrayStreamSettingsFromCoreSpecific(parsedCoreSpecific, specForm.core_type),
    [parsedCoreSpecific, specForm.core_type],
  );
  const isXHTTPSelected =
    specForm.core_type === "xray" &&
    normalizeXrayTransport(xrayStreamSettings?.network) === "xhttp";

  /* ============ internal helpers ============ */
  const setSpecJSONError = (field: SpecJSONField, message?: string) => {
    const next = { ...specJSONErrors };
    if (message) {
      next[field] = message;
    } else {
      delete next[field];
    }
    setSpecJSONErrors(next);
  };

  const clearSpecJSONErrors = () => setSpecJSONErrors({});

  const formatSpecJSONError = (error: unknown) => {
    const detail =
      error instanceof Error ? error.message : t("error.bad_request");
    return `${t("admin.configCenter.messages.invalidJson")}: ${detail}`;
  };

  const updateSpecJSONField = (
    field: SpecJSONField,
    nextValue: Record<string, unknown>,
  ) => {
    onSpecFormChange({ ...specForm, [field]: prettyJSON(nextValue) });
    setSpecJSONError(field);
  };

  const handleSpecJSONChange = (field: SpecJSONField, value: string) => {
    onSpecFormChange({ ...specForm, [field]: value });
    setSpecJSONError(field);
  };

  const syncXHTTPJsonDraft = (
    coreSpecificInput: string,
    coreType: CoreTypeOption,
  ) => {
    setXHTTPJsonDraft(
      xhttpJSONDraftFromCoreSpecific(parseJSONRecord(coreSpecificInput), coreType),
    );
  };

  const tryParseSpecJSON = (field: SpecJSONField) => {
    try {
      const parsed = safeParseJSON(specForm[field], {});
      setSpecJSONError(field);
      if (isRecord(parsed)) {
        return parsed;
      }
      return {};
    } catch (error) {
      const message =
        error instanceof Error ? error.message : t("error.bad_request");
      setSpecJSONError(field, formatSpecJSONError(error));
      if (!isAdvancedJsonOpen) {
        setIsAdvancedJsonOpen(p => !p);
      }
      toast.error(t("admin.configCenter.messages.invalidJson"), {
        description: message,
      });
      return null;
    }
  };

  const tryParseSpecJSONForSave = (
    field: SpecJSONField,
  ): { ok: true; value: unknown } | { ok: false } => {
    try {
      const parsed = safeParseJSON(specForm[field], {});
      setSpecJSONError(field);
      return { ok: true, value: parsed };
    } catch (error) {
      const message =
        error instanceof Error ? error.message : t("error.bad_request");
      setSpecJSONError(field, formatSpecJSONError(error));
      if (!isAdvancedJsonOpen) {
        setIsAdvancedJsonOpen(p => !p);
      }
      toast.error(t("admin.configCenter.messages.invalidJson"), {
        description: message,
      });
      return { ok: false };
    }
  };

  /* ============ dialog open/close ============ */
  const handleSpecDialogOpenChange = (nextOpen: boolean) => {
    onOpenChange(nextOpen);
    if (!nextOpen) {
      clearSpecJSONErrors();
      setPendingGeneratorOverwriteAction(null);
    }
  };

  /* ============ inbound editor ============ */
  const handleInboundSpecChange = async (next: InboundSemanticSpec) => {
    const serialized = serializeSemanticSpec(next);
    const formUpdate: SpecFormState = {
      ...specForm,
      semantic_spec: prettyJSON(serialized as Record<string, unknown>),
    };

    // Auto-switch core_type based on transport selection
    const transportType = next.transport?.type;
    if (
      transportType === "xhttp" ||
      transportType === "hysteria" ||
      transportType === "quic"
    ) {
      if (formUpdate.core_type !== "xray") {
        formUpdate.core_type = "xray";
      }
    }

    // Auto-generate Reality keys/short ID when Reality is first enabled
    if (next.tls?.reality?.enabled) {
      const cs = parseJSONRecord(specForm.core_specific) ?? {};
      const scope = pickCoreSpecificScope(cs, specForm.core_type);
      const keys = scope as Record<string, unknown>;
      const streamSettings = keys?.streamSettings as Record<string, unknown> | undefined;
      const realitySettings = streamSettings?.realitySettings as Record<string, unknown> | undefined;
      const hasKeys =
        specForm.core_type === "xray"
          ? !!(realitySettings?.privateKey || realitySettings?.publicKey)
          : !!(
              (keys as ScopeObject)?.tls?.reality?.private_key ||
              (keys as ScopeObject)?.tls?.reality?.public_key
            );
      if (!hasKeys) {
        const keyPair = await generateRealityKeyPair();
        const shortId = generateHexString(8);
        if (specForm.core_type === "xray") {
          const streamSettings = ensureObjectField(
            keys,
            "streamSettings",
          );
          const realitySettings = ensureObjectField(
            streamSettings,
            "realitySettings",
          );
          realitySettings.privateKey = keyPair.privateKey;
          realitySettings.publicKey = keyPair.publicKey;
          realitySettings.shortIds = [shortId];
        } else {
          const tls = ensureObjectField(keys, "tls");
          const reality = ensureObjectField(tls, "reality");
          reality.private_key = keyPair.privateKey;
          reality.public_key = keyPair.publicKey;
          reality.short_id = shortId;
        }
        formUpdate.core_specific = prettyJSON(cs);
      }
    }

    onSpecFormChange(formUpdate);
    setSpecJSONError("semantic_spec");
    setSpecJSONError("core_specific");
  };

  /* ============ generator handlers ============ */
  const requestGeneratorOverwrite = (action: GeneratorOverwriteAction) => {
    setPendingGeneratorOverwriteAction(action);
  };

  const handleGenerateUUID = (skipOverwriteConfirm = false) => {
    const semantic = tryParseSpecJSON("semantic_spec");
    if (!semantic) {
      return;
    }

    const rawUsers = Array.isArray(semantic.users) ? semantic.users : [];
    const users = rawUsers.filter(
      (item): item is Record<string, unknown> => isRecord(item),
    );

    const hasExistingUUID = users.some((user) =>
      hasMeaningfulValue(user.uuid),
    );
    if (hasExistingUUID && !skipOverwriteConfirm) {
      requestGeneratorOverwrite("uuid");
      return;
    }

    if (users.length === 0) {
      semantic.users = [{ uuid: generateCompactUUID() }];
    } else {
      semantic.users = users.map((user) => ({
        ...user,
        uuid: generateCompactUUID(),
      }));
    }

    updateSpecJSONField("semantic_spec", semantic);
    toast.success(t("admin.configCenter.generator.messages.uuidGenerated"));
  };

  const handleGenerateShortID = (skipOverwriteConfirm = false) => {
    const coreSpecific = tryParseSpecJSON("core_specific");
    if (!coreSpecific) {
      return;
    }

    const scope = pickCoreSpecificScope(coreSpecific, specForm.core_type);

    if (specForm.core_type === "xray") {
      const streamSettings = ensureObjectField(scope, "streamSettings");
      const realitySettings = ensureObjectField(
        streamSettings,
        "realitySettings",
      );
      const existing = realitySettings.shortIds;
      const hasExistingShortID =
        (Array.isArray(existing) && existing.length > 0) ||
        (typeof existing === "string" && existing.trim().length > 0);

      if (hasExistingShortID && !skipOverwriteConfirm) {
        requestGeneratorOverwrite("shortId");
        return;
      }

      realitySettings.shortIds = [generateHexString(8)];
    } else {
      const tls = ensureObjectField(scope, "tls");
      const reality = ensureObjectField(tls, "reality");
      if (hasMeaningfulValue(reality.short_id) && !skipOverwriteConfirm) {
        requestGeneratorOverwrite("shortId");
        return;
      }
      reality.short_id = generateHexString(8);
    }

    updateSpecJSONField("core_specific", coreSpecific);
    toast.success(t("admin.configCenter.generator.messages.shortIdGenerated"));
  };

  const handleGenerateRealityKeyPair = async (
    skipOverwriteConfirm = false,
  ) => {
    const coreSpecific = tryParseSpecJSON("core_specific");
    if (!coreSpecific) {
      return;
    }

    const scope = pickCoreSpecificScope(coreSpecific, specForm.core_type);

    if (specForm.core_type === "xray") {
      const streamSettings = ensureObjectField(scope, "streamSettings");
      const realitySettings = ensureObjectField(
        streamSettings,
        "realitySettings",
      );
      const hasExistingKey =
        hasMeaningfulValue(realitySettings.privateKey) ||
        hasMeaningfulValue(realitySettings.publicKey);
      if (hasExistingKey && !skipOverwriteConfirm) {
        requestGeneratorOverwrite("realityKey");
        return;
      }
    } else {
      const tls = ensureObjectField(scope, "tls");
      const reality = ensureObjectField(tls, "reality");
      const hasExistingKey =
        hasMeaningfulValue(reality.private_key) ||
        hasMeaningfulValue(reality.public_key);
      if (hasExistingKey && !skipOverwriteConfirm) {
        requestGeneratorOverwrite("realityKey");
        return;
      }
    }

    try {
      const generated = await generateRealityKeyPair();

      if (specForm.core_type === "xray") {
        const streamSettings = ensureObjectField(scope, "streamSettings");
        const realitySettings = ensureObjectField(
          streamSettings,
          "realitySettings",
        );
        realitySettings.privateKey = generated.privateKey;
        realitySettings.publicKey = generated.publicKey;
      } else {
        const tls = ensureObjectField(scope, "tls");
        const reality = ensureObjectField(tls, "reality");
        reality.private_key = generated.privateKey;
        reality.public_key = generated.publicKey;
      }

      updateSpecJSONField("core_specific", coreSpecific);
      toast.success(
        t("admin.configCenter.generator.messages.realityKeyGenerated"),
      );
    } catch {
      toast.error(
        t("admin.configCenter.generator.messages.webCryptoUnsupported"),
      );
    }
  };

  const handleConfirmGeneratorOverwrite = () => {
    const action = pendingGeneratorOverwriteAction;
    setPendingGeneratorOverwriteAction(null);

    if (action === "uuid") {
      handleGenerateUUID(true);
    } else if (action === "realityKey") {
      void handleGenerateRealityKeyPair(true);
    } else if (action === "shortId") {
      handleGenerateShortID(true);
    }
  };

  /* ============ save / import ============ */
  const handleSaveSpec = () => {
    try {
      if (!specForm.tag.trim()) {
        toast.warning(t("admin.configCenter.messages.requiredFields"));
        return;
      }
      if (!specForm.is_template && !specForm.agent_host_id) {
        toast.warning(t("admin.configCenter.messages.selectHostFirst"));
        return;
      }

      const semanticParse = tryParseSpecJSONForSave("semantic_spec");
      if (!semanticParse.ok) {
        return;
      }
      const coreSpecificParse = tryParseSpecJSONForSave("core_specific");
      if (!coreSpecificParse.ok) {
        return;
      }

      const semanticSpec = semanticParse.value;
      let coreSpecific = coreSpecificParse.value;
      let nextXHTTPJsonDraft: XHTTPJSONDraftState | null = null;

      if (isXHTTPSelected) {
        const parsedDraft = parseXHTTPJSONDraftState(xhttpJsonDraft);
        if (!parsedDraft) {
          toast.error(t("admin.configCenter.messages.invalidJson"));
          return;
        }
        const coreSpecificRecord = isRecord(coreSpecific)
          ? coreSpecific
          : {};
        applyXHTTPJSONDraftToCoreSpecific(
          coreSpecificRecord,
          specForm.core_type,
          parsedDraft,
        );
        coreSpecific = coreSpecificRecord;
        nextXHTTPJsonDraft = prettyXHTTPJSONDraftState(parsedDraft);
      }

      // Ensure semantic spec tag matches form tag
      if (typeof semanticSpec === "object" && semanticSpec !== null) {
        (semanticSpec as Record<string, unknown>).tag = specForm.tag.trim();
      }

      if (nextXHTTPJsonDraft) {
        setXHTTPJsonDraft(nextXHTTPJsonDraft);
        onSpecFormChange({
          ...specForm,
          core_specific: prettyJSON(coreSpecific),
        });
      }

      onSave({ core_specific: prettyJSON(coreSpecific) });
    } catch (error) {
      const message =
        error instanceof Error ? error.message : t("error.bad_request");
      toast.error(t("admin.configCenter.messages.invalidJson"), {
        description: message,
      });
    }
  };

  const handleImport = () => {
    if (!selectedHostId) {
      toast.warning(t("admin.configCenter.messages.selectHostFirst"));
      return;
    }
    importMutation.mutate({
      agent_host_id: selectedHostId as number,
      core_type: selectedCoreType || specForm.core_type,
      source: importForm.source,
      filename: importForm.filename.trim() || undefined,
      tag: importForm.tag.trim() || undefined,
      enabled: importForm.enabled,
      overwrite_existing: importForm.overwrite_existing,
      change_note: importForm.change_note.trim() || undefined,
    });
  };

  /* ================================================================
     JSX
     ================================================================ */
  return (
    <>
      {/* ---- Spec form dialog ---- */}
      <Dialog open={open} onOpenChange={handleSpecDialogOpenChange}>
        <DialogContent className="sm:max-w-4xl">
          <DialogHeader>
            <DialogTitle>
              {selectedSpec
                ? t("admin.configCenter.specs.editTitle")
                : t("admin.configCenter.specs.createTitle")}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            {/* Host + Core type row */}
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <label className="text-sm font-medium">
                  {t("admin.configCenter.fields.agentHost")}
                </label>
                {specForm.is_template ? (
                  <div className="flex h-10 items-center rounded-md border border-border px-3 text-sm text-muted-foreground">
                    {t("admin.configCenter.template.templateSpec")}
                  </div>
                ) : (
                  <Select
                    value={
                      specForm.agent_host_id
                        ? String(specForm.agent_host_id)
                        : undefined
                    }
                    onValueChange={(value) =>
                      onSpecFormChange({
                        ...specForm,
                        agent_host_id: Number(value) || 0,
                      })
                    }
                  >
                    <SelectTrigger>
                      <SelectValue
                        placeholder={t(
                          "admin.configCenter.placeholders.selectHost",
                        )}
                      />
                    </SelectTrigger>
                    <SelectContent>
                      {hostOptions.map((host) => (
                        <SelectItem
                          key={String(host.id)}
                          value={String(host.id)}
                        >
                          {host.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">
                  {t("admin.configCenter.fields.coreType")}
                </label>
                <Select
                  value={specForm.core_type}
                  onValueChange={(value) => {
                    const nextCoreType = value as CoreTypeOption;
                    onSpecFormChange({
                      ...specForm,
                      core_type: nextCoreType,
                    });
                    syncXHTTPJsonDraft(specForm.core_specific, nextCoreType);
                  }}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {CORE_OPTIONS.map((item) => (
                      <SelectItem key={item} value={item}>
                        {item}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            {/* Tag */}
            <div className="space-y-2">
              <label className="text-sm font-medium">
                {t("admin.configCenter.fields.tag")}
              </label>
              <Input
                value={specForm.tag}
                onChange={(event) =>
                  onSpecFormChange({
                    ...specForm,
                    tag: event.target.value,
                  })
                }
                placeholder={t("admin.configCenter.placeholders.specTag")}
              />
            </div>

            {/* Enabled */}
            <label className="flex items-center gap-2 text-sm">
              <Switch
                checked={specForm.enabled}
                onCheckedChange={(checked) =>
                  onSpecFormChange({ ...specForm, enabled: checked })
                }
              />
              {t("admin.configCenter.fields.enabled")}
            </label>

            {/* Template switch */}
            <label className="flex items-center gap-2 text-sm">
              <Switch
                checked={specForm.is_template}
                onCheckedChange={(checked) =>
                  onSpecFormChange({
                    ...specForm,
                    is_template: checked,
                    agent_host_id: checked
                      ? null
                      : specForm.agent_host_id ||
                        selectedSpec?.agent_host_id ||
                        0,
                  })
                }
              />
              {t("admin.configCenter.template.isTemplate")}
            </label>

            {/* ---- Visual Inbound Editor ---- */}
            <ConfigCenterInboundEditor
              value={inboundSpec}
              onChange={handleInboundSpecChange}
              coreType={specForm.core_type}
            />

            {/* ---- Advanced JSON ---- */}
            <div
              className="space-y-4 rounded-md border bg-muted/20 p-4"
              data-testid="config-center-advanced-json"
            >
              <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div className="space-y-1">
                  <p className="text-sm font-semibold">
                    {t("admin.configCenter.advancedJson.title")}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {t("admin.configCenter.advancedJson.description")}
                  </p>
                </div>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  aria-expanded={isAdvancedJsonOpen}
                  aria-controls="config-center-advanced-json-fields"
                  data-testid="config-center-advanced-json-toggle"
                  onClick={() => setIsAdvancedJsonOpen(p => !p)}
                >
                  {isAdvancedJsonOpen
                    ? t("admin.configCenter.advancedJson.hide")
                    : t("admin.configCenter.advancedJson.show")}
                </Button>
              </div>

              <div
                id="config-center-advanced-json-fields"
                className="space-y-4"
              >
                {/* semantic_spec */}
                <div className="space-y-2">
                  <div className="flex items-center justify-between gap-2">
                    <label
                      className="text-sm font-medium"
                      htmlFor="config-center-semantic-json-input"
                    >
                      {t("admin.configCenter.fields.semanticSpec")}
                    </label>
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      onClick={() => handleGenerateUUID()}
                      data-testid="config-center-generate-uuid"
                    >
                      {t("admin.configCenter.generator.actions.generateUUID")}
                    </Button>
                  </div>
                  {specJSONErrors.semantic_spec && (
                    <p role="alert" className="text-xs text-destructive">
                      {specJSONErrors.semantic_spec}
                    </p>
                  )}
                  <Textarea
                    id="config-center-semantic-json-input"
                    data-testid="config-center-semantic-json"
                    hidden={!isAdvancedJsonOpen}
                    className="min-h-[140px] font-mono text-xs"
                    value={specForm.semantic_spec}
                    onChange={(event) =>
                      handleSpecJSONChange(
                        "semantic_spec",
                        event.target.value,
                      )
                    }
                  />
                </div>

                {/* core_specific */}
                <div className="space-y-2">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <label
                      className="text-sm font-medium"
                      htmlFor="config-center-core-specific-json-input"
                    >
                      {t("admin.configCenter.fields.coreSpecific")}
                    </label>
                    <div className="flex flex-wrap items-center gap-2">
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        onClick={() => void handleGenerateRealityKeyPair()}
                        data-testid="config-center-generate-reality-key"
                      >
                        {t(
                          "admin.configCenter.generator.actions.generateRealityKey",
                        )}
                      </Button>
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        onClick={() => handleGenerateShortID()}
                        data-testid="config-center-generate-short-id"
                      >
                        {t(
                          "admin.configCenter.generator.actions.generateShortId",
                        )}
                      </Button>
                    </div>
                  </div>
                  {specJSONErrors.core_specific && (
                    <p role="alert" className="text-xs text-destructive">
                      {specJSONErrors.core_specific}
                    </p>
                  )}
                  <Textarea
                    id="config-center-core-specific-json-input"
                    data-testid="config-center-core-specific-json"
                    hidden={!isAdvancedJsonOpen}
                    className="min-h-[120px] font-mono text-xs"
                    value={specForm.core_specific}
                    onChange={(event) =>
                      handleSpecJSONChange(
                        "core_specific",
                        event.target.value,
                      )
                    }
                    onBlur={() =>
                      syncXHTTPJsonDraft(
                        specForm.core_specific,
                        specForm.core_type,
                      )
                    }
                  />
                </div>
              </div>
            </div>

            {/* Change note */}
            <div className="space-y-2">
              <label className="text-sm font-medium">
                {t("admin.configCenter.fields.changeNote")}
              </label>
              <Input
                value={specForm.change_note}
                onChange={(event) =>
                  onSpecFormChange({
                    ...specForm,
                    change_note: event.target.value,
                  })
                }
                placeholder={t("admin.configCenter.placeholders.optional")}
              />
            </div>
          </div>

          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => handleSpecDialogOpenChange(false)}
            >
              {t("common.cancel")}
            </Button>
            <Button onClick={handleSaveSpec} disabled={isSaving}>
              {isSaving
                ? t("common.loading")
                : selectedSpec
                  ? t("common.save")
                  : t("common.create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ---- Generator overwrite confirmation ---- */}
      <Dialog
        open={pendingGeneratorOverwriteAction !== null}
        onOpenChange={(nextOpen) => {
          if (!nextOpen) {
            setPendingGeneratorOverwriteAction(null);
          }
        }}
      >
        <DialogContent
          className="sm:max-w-md"
          data-testid="config-center-overwrite-confirm-dialog"
        >
          <DialogHeader>
            <DialogTitle>
              {t("admin.configCenter.generator.confirm.title")}
            </DialogTitle>
            <DialogDescription>
              {pendingGeneratorOverwriteAction
                ? t(
                    GENERATOR_OVERWRITE_MESSAGE_KEYS[
                      pendingGeneratorOverwriteAction
                    ],
                  )
                : ""}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setPendingGeneratorOverwriteAction(null)}
            >
              {t("common.cancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={handleConfirmGeneratorOverwrite}
            >
              {t("admin.configCenter.generator.confirm.overwrite")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ---- Import dialog ---- */}
      <Dialog open={importOpen} onOpenChange={setImportOpen}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {t("admin.configCenter.import.title")}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <label className="text-sm font-medium">
                  {t("admin.configCenter.fields.source")}
                </label>
                <Select
                  value={importForm.source}
                  onValueChange={(value) =>
                    setImportForm({
                      ...importForm,
                      source: value as ImportFormState["source"],
                    })
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="legacy">
                      {t("admin.configCenter.source.legacy")}
                    </SelectItem>
                    <SelectItem value="managed">
                      {t("admin.configCenter.source.managed")}
                    </SelectItem>
                    <SelectItem value="merged">
                      {t("admin.configCenter.source.merged")}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">
                  {t("admin.configCenter.fields.enabled")}
                </label>
                <div className="flex h-10 items-center rounded-md border border-border px-3">
                  <Switch
                    checked={importForm.enabled}
                    onCheckedChange={(checked) =>
                      setImportForm({
                        ...importForm,
                        enabled: checked,
                      })
                    }
                  />
                </div>
              </div>
            </div>

            <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <label className="text-sm font-medium">
                  {t("admin.configCenter.fields.filename")}
                </label>
                <Input
                  value={importForm.filename}
                  onChange={(event) =>
                    setImportForm({
                      ...importForm,
                      filename: event.target.value,
                    })
                  }
                  placeholder={t("admin.configCenter.placeholders.optional")}
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">
                  {t("admin.configCenter.fields.tag")}
                </label>
                <Input
                  value={importForm.tag}
                  onChange={(event) =>
                    setImportForm({
                      ...importForm,
                      tag: event.target.value,
                    })
                  }
                  placeholder={t("admin.configCenter.placeholders.optional")}
                />
              </div>
            </div>

            <label className="flex items-center gap-2 text-sm">
              <Switch
                checked={importForm.overwrite_existing}
                onCheckedChange={(checked) =>
                  setImportForm({
                    ...importForm,
                    overwrite_existing: checked,
                  })
                }
              />
              {t("admin.configCenter.import.overwriteExisting")}
            </label>

            <div className="space-y-2">
              <label className="text-sm font-medium">
                {t("admin.configCenter.fields.changeNote")}
              </label>
              <Input
                value={importForm.change_note}
                onChange={(event) =>
                  setImportForm({
                    ...importForm,
                    change_note: event.target.value,
                  })
                }
                placeholder={t("admin.configCenter.placeholders.optional")}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setImportOpen(false)}
            >
              {t("common.cancel")}
            </Button>
            <Button
              onClick={handleImport}
              disabled={importMutation.isPending || !selectedHostId}
            >
              {importMutation.isPending
                ? t("common.loading")
                : t("admin.configCenter.actions.import")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
