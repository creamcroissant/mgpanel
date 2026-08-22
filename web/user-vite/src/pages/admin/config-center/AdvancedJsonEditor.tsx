/**
 * Advanced JSON Editor — visual editor for semantic_spec and core_specific JSON fields.
 * Extracted from SpecEditorDialog for maintainability.
 */
import { useTranslation } from "react-i18next";
import { Button, Textarea } from "@/components/ui";
import type { SpecFormState, SpecJSONErrors, SpecJSONField } from "./configCenterPageTypes";

interface AdvancedJsonEditorProps {
  specForm: SpecFormState;
  specJSONErrors: SpecJSONErrors;
  isOpen: boolean;
  onToggleOpen: () => void;
  onJSONChange: (field: SpecJSONField, value: string) => void;
  onGenerateUUID: () => void;
  onGenerateRealityKey: () => void;
  onGenerateShortID: () => void;
}

export function AdvancedJsonEditor({
  specForm,
  specJSONErrors,
  isOpen,
  onToggleOpen,
  onJSONChange,
  onGenerateUUID,
  onGenerateRealityKey,
  onGenerateShortID,
}: AdvancedJsonEditorProps) {
  const { t } = useTranslation();

  return (
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
          aria-expanded={isOpen}
          aria-controls="config-center-advanced-json-fields"
          data-testid="config-center-advanced-json-toggle"
          onClick={onToggleOpen}
        >
          {isOpen
            ? t("admin.configCenter.advancedJson.hide")
            : t("admin.configCenter.advancedJson.show")}
        </Button>
      </div>

      <div id="config-center-advanced-json-fields" className="space-y-4">
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
              onClick={onGenerateUUID}
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
            hidden={!isOpen}
            className="min-h-[140px] font-mono text-xs"
            value={specForm.semantic_spec}
            onChange={(event) => onJSONChange("semantic_spec", event.target.value)}
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
                onClick={onGenerateRealityKey}
                data-testid="config-center-generate-reality-key"
              >
                {t("admin.configCenter.generator.actions.generateRealityKey")}
              </Button>
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={onGenerateShortID}
                data-testid="config-center-generate-short-id"
              >
                {t("admin.configCenter.generator.actions.generateShortId")}
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
            hidden={!isOpen}
            className="min-h-[120px] font-mono text-xs"
            value={specForm.core_specific}
            onChange={(event) => onJSONChange("core_specific", event.target.value)}
          />
        </div>
      </div>
    </div>
  );
}
