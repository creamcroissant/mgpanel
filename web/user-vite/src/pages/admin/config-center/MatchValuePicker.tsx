import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Badge, Input } from "@/components/ui";
import { cn } from "@/lib/utils";
import { searchCandidates } from "./presets/matchValuePresets";
import type { MatchFieldKind } from "./presets/matchValuePresets";

/**
 * 匹配值搜索选择器
 *
 * 输入即搜索（匹配 value / descZh / descEn），点击候选填入；
 * 已选值以 badge 展示，可单独移除。支持回车添加自定义值。
 */
interface MatchValuePickerProps {
  kind: MatchFieldKind;
  values: string[];
  onChange: (next: string[]) => void;
  placeholder?: string;
  /** 允许用户手动输入任意值（回车添加） */
  allowCustom?: boolean;
}

export function MatchValuePicker({ kind, values, onChange, placeholder, allowCustom = true }: MatchValuePickerProps) {
  const { t } = useTranslation();
  const [keyword, setKeyword] = useState("");
  const [focused, setFocused] = useState(false);

  const candidates = searchCandidates(kind, keyword);

  const addValue = (v: string) => {
    const trimmed = v.trim();
    if (!trimmed) return;
    if (values.includes(trimmed)) {
      setKeyword("");
      return;
    }
    onChange([...values, trimmed]);
    setKeyword("");
  };

  const removeValue = (v: string) => {
    onChange(values.filter((x) => x !== v));
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      // 有候选时优先取第一个候选，否则添加输入原文
      const matched = candidates.filter((c) => !values.includes(c.value));
      if (matched.length > 0 && kind !== "inboundTag") {
        addValue(matched[0].value);
      } else {
        addValue(keyword);
      }
      return;
    }
    if (e.key === "Backspace" && !keyword && values.length > 0) {
      removeValue(values[values.length - 1]);
    }
  };

  return (
    <div className="space-y-2">
      {/* 已选值 */}
      <div className="flex flex-wrap gap-1">
        {values.length === 0 && (
          <span className="text-xs text-muted-foreground">{t("admin.configCenter.matchValue.empty")}</span>
        )}
        {values.map((v) => (
          <Badge key={v} variant="secondary" className="gap-1 pr-1">
            <span className="max-w-[200px] truncate font-mono">{v}</span>
            <button
              type="button"
              className="ml-0.5 rounded-none px-0.5 text-muted-foreground hover:bg-muted hover:text-foreground"
              onClick={() => removeValue(v)}
              aria-label={t("common.delete")}
            >
              ×
            </button>
          </Badge>
        ))}
      </div>

      {/* 搜索框 */}
      <Input
        value={keyword}
        onChange={(e) => setKeyword(e.target.value)}
        onFocus={() => setFocused(true)}
        onBlur={() => setTimeout(() => setFocused(false), 150)}
        onKeyDown={onKeyDown}
        placeholder={placeholder || t("admin.configCenter.matchValue.search")}
      />

      {/* 候选列表 */}
      {focused && candidates.length > 0 && (
        <div className="max-h-48 overflow-y-auto rounded-none border border-border border-border bg-background shadow-sm">
          {candidates.map((c) => {
            const selected = values.includes(c.value);
            return (
              <button
                key={c.value}
                type="button"
                disabled={selected}
                onMouseDown={(e) => {
                  e.preventDefault();
                  addValue(c.value);
                }}
                className={cn(
                  "flex w-full items-center justify-between gap-2 px-3 py-1.5 text-left text-sm hover:bg-muted/40",
                  selected && "opacity-50"
                )}
              >
                <span className="flex items-center gap-2">
                  <span className="font-mono text-xs">{c.value}</span>
                </span>
                <span className="truncate text-xs text-muted-foreground">{c.descZh ?? c.descEn ?? ""}</span>
              </button>
            );
          })}
          {allowCustom && keyword.trim() && !values.includes(keyword.trim()) && (
            <button
              type="button"
              onMouseDown={(e) => {
                e.preventDefault();
                addValue(keyword);
              }}
              className="flex w-full items-center gap-2 border-t border-border px-3 py-1.5 text-left text-sm text-primary hover:bg-muted/40"
            >
              <span className="font-mono text-xs">+ {keyword.trim()}</span>
              <span className="text-xs text-muted-foreground">{t("admin.configCenter.matchValue.custom")}</span>
            </button>
          )}
        </div>
      )}
    </div>
  );
}