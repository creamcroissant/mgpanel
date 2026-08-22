import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import type { AdminKnowledgeDetail, AdminKnowledgeSaveRequest } from "@/types";
import {
  Button,
  Dialog,
  DialogContent,
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

const defaultFormState: AdminKnowledgeSaveRequest = {
  language: "zh-CN",
  category: "",
  title: "",
  body: "",
  show: true,
  sort: 0,
};

type KnowledgeFormProps = {
  open: boolean;
  categories: string[];
  languages: string[];
  initialData?: AdminKnowledgeDetail | null;
  onSubmit: (payload: AdminKnowledgeSaveRequest) => void;
  onCancel: () => void;
  isSubmitting?: boolean;
};

type KnowledgeFormFieldsProps = {
  categories: string[];
  languages: string[];
  initialFormState: AdminKnowledgeSaveRequest;
  initialCustomCategory: string;
  initialData?: AdminKnowledgeDetail | null;
  onSubmit: (payload: AdminKnowledgeSaveRequest) => void;
  onCancel: () => void;
  isSubmitting?: boolean;
};

function KnowledgeFormFields({
  categories,
  languages,
  initialFormState,
  initialCustomCategory,
  initialData,
  onSubmit,
  onCancel,
  isSubmitting,
}: KnowledgeFormFieldsProps) {
  const { t } = useTranslation();
  const [formState, setFormState] = useState<AdminKnowledgeSaveRequest>(initialFormState);
  const [customCategory, setCustomCategory] = useState(initialCustomCategory);
  const isEdit = Boolean(initialData?.id);

  const sortedCategories = useMemo(() => {
    const unique = new Set(categories.map((item) => item.trim()).filter(Boolean));
    return Array.from(unique).sort((a, b) => a.localeCompare(b));
  }, [categories]);

  const sortedLanguages = useMemo(() => {
    const unique = new Set(languages.map((item) => item.trim()).filter(Boolean));
    if (unique.size === 0) {
      return ["zh-CN", "en-US"];
    }
    return Array.from(unique);
  }, [languages]);

  const categoryInList = sortedCategories.includes(formState.category);
  const selectedCategoryValue = categoryInList ? formState.category : "__custom__";
  const resolvedCategory = formState.category.trim() || customCategory.trim();

  const handleSubmit = () => {
    if (!formState.title.trim() || !formState.body.trim()) {
      toast.warning(t("admin.knowledge.fieldsRequired"));
      return;
    }
    if (!resolvedCategory) {
      toast.warning(t("admin.knowledge.categoryRequired"));
      return;
    }
    if (!formState.language.trim()) {
      toast.warning(t("admin.knowledge.languageRequired"));
      return;
    }
    onSubmit({
      ...formState,
      category: resolvedCategory,
      language: formState.language.trim(),
    });
  };

  return (
    <>
      <div className="space-y-4 py-2">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.knowledge.language")}</label>
            <Select
              value={formState.language}
              onValueChange={(value) => setFormState({ ...formState, language: value })}
            >
              <SelectTrigger>
                <SelectValue placeholder={t("admin.knowledge.languagePlaceholder")} />
              </SelectTrigger>
              <SelectContent>
                {sortedLanguages.map((lang) => (
                  <SelectItem key={lang} value={lang}>
                    {lang}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.knowledge.category")}</label>
            <Select
              value={selectedCategoryValue}
              onValueChange={(value) => {
                if (value === "__custom__") {
                  setFormState({ ...formState, category: "" });
                  return;
                }
                setFormState({ ...formState, category: value });
              }}
            >
              <SelectTrigger>
                <SelectValue placeholder={t("admin.knowledge.categoryPlaceholder")} />
              </SelectTrigger>
              <SelectContent>
                {sortedCategories.map((category) => (
                  <SelectItem key={category} value={category}>
                    {category}
                  </SelectItem>
                ))}
                <SelectItem value="__custom__">{t("admin.knowledge.categoryCustom")}</SelectItem>
              </SelectContent>
            </Select>
            {!categoryInList && (
              <Input
                placeholder={t("admin.knowledge.categoryPlaceholder")}
                value={customCategory}
                onChange={(event) => setCustomCategory(event.target.value)}
              />
            )}
          </div>
        </div>

        <div className="space-y-2">
          <label className="text-sm font-medium">{t("admin.knowledge.titleLabel")}</label>
          <Input
            placeholder={t("admin.knowledge.titlePlaceholder")}
            value={formState.title}
            onChange={(event) => setFormState({ ...formState, title: event.target.value })}
            required
          />
        </div>

        <div className="space-y-2">
          <label className="text-sm font-medium">{t("admin.knowledge.content")}</label>
          <Textarea
            placeholder={t("admin.knowledge.contentPlaceholder")}
            value={formState.body}
            onChange={(event) => setFormState({ ...formState, body: event.target.value })}
            rows={10}
            required
          />
          <p className="text-xs text-muted-foreground">{t("admin.knowledge.contentHint")}</p>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.knowledge.sort")}</label>
            <Input
              type="number"
              placeholder={t("admin.knowledge.sortPlaceholder")}
              value={String(formState.sort ?? 0)}
              onChange={(event) =>
                setFormState({
                  ...formState,
                  sort: Number.isNaN(Number(event.target.value)) ? 0 : Number(event.target.value),
                })
              }
            />
          </div>
          <div className="flex items-center gap-2 pt-6">
            <Switch
              checked={formState.show ?? true}
              onCheckedChange={(value) => setFormState({ ...formState, show: value })}
            />
            <span className="text-sm">{t("admin.knowledge.visible")}</span>
          </div>
        </div>
      </div>
      <DialogFooter>
        <Button variant="outline" onClick={onCancel}>
          {t("common.cancel")}
        </Button>
        <Button onClick={handleSubmit} disabled={isSubmitting}>
          {isSubmitting
            ? t("common.loading")
            : isEdit
            ? t("common.save")
            : t("common.create")}
        </Button>
      </DialogFooter>
    </>
  );
}

export default function KnowledgeForm({
  open,
  categories,
  languages,
  initialData,
  onSubmit,
  onCancel,
  isSubmitting,
}: KnowledgeFormProps) {
  const { t } = useTranslation();
  const initialFormState = useMemo<AdminKnowledgeSaveRequest>(
    () =>
      initialData
        ? {
            id: initialData.id,
            language: initialData.language,
            category: initialData.category,
            title: initialData.title,
            body: initialData.body,
            sort: initialData.sort,
            show: initialData.show,
          }
        : defaultFormState,
    [initialData]
  );
  const initialCustomCategory = initialData?.category || "";
  const draftKey = `${open ? "open" : "closed"}:${initialData?.id ?? "new"}`;
  const isEdit = Boolean(initialData?.id);

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onCancel()}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>
            {isEdit ? t("admin.knowledge.editTitle") : t("admin.knowledge.addTitle")}
          </DialogTitle>
        </DialogHeader>
        <KnowledgeFormFields
          key={draftKey}
          categories={categories}
          languages={languages}
          initialFormState={initialFormState}
          initialCustomCategory={initialCustomCategory}
          initialData={initialData}
          onSubmit={onSubmit}
          onCancel={onCancel}
          isSubmitting={isSubmitting}
        />
      </DialogContent>
    </Dialog>
  );
}
