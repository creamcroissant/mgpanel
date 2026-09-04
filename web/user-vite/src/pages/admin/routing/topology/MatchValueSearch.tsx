import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Badge, Input } from "@/components/ui";
import { cn } from "@/lib/utils";
import { searchTopologyMatchCandidates } from "../../config-center/presets/matchValuePresets";
import type { TopologyMatchType } from "../../config-center/presets/matchValuePresets";

/**
 * 拓扑规则匹配值搜索选择器（受控纯组件，不依赖 API/react-query）。
 *
 * 输入即搜索（匹配 value / descZh / descEn），点击候选填入；
 * 已选值以 badge 展示，可单独移除。支持回车添加自定义值。
 * 对外契约保持与拓扑一致：value 是逗号分隔的裸值字符串（如 "netflix,disney"）。
 */
interface MatchValueSearchProps {
  matchType: TopologyMatchType;
  /** 逗号分隔的裸值字符串 */
  value: string;
  onChange: (next: string) => void;
  placeholder?: string;
}

function parseParts(value: string): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const seg of value.split(",")) {
    const v = seg.trim();
    if (!v || seen.has(v)) continue;
    seen.add(v);
    out.push(v);
  }
  return out;
}

export function MatchValueSearch({ matchType, value, onChange, placeholder }: MatchValueSearchProps) {
  const { t } = useTranslation();
  const [keyword, setKeyword] = useState("");
  const [focused, setFocused] = useState(false);

  const parts = parseParts(value);
  const candidates = searchTopologyMatchCandidates(matchType, keyword).filter((c) => !parts.includes(c.value));

  const commit = (next: string[]) => onChange(next.join(","));

  const addValue = (v: string) => {
    const trimmed = v.trim();
    if (!trimmed || parts.includes(trimmed)) {
      setKeyword("");
      return;
    }
    commit([...parts, trimmed]);
    setKeyword("");
  };

  const removeValue = (v: string) => {
    commit(parts.filter((x) => x !== v));
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      if (candidates.length > 0) {
        addValue(candidates[0].value);
      } else {
        addValue(keyword);
      }
      return;
    }
    if (e.key === "Backspace" && !keyword && parts.length > 0) {
      removeValue(parts[parts.length - 1]);
    }
  };

  return (
    <div className="space-y-2">
      {/* 已选值 */}
      <div className="flex flex-wrap gap-1">
        {parts.length === 0 && (
          <span className="text-xs text-muted-foreground">{t("admin.configCenter.matchValue.empty")}</span>
        )}
        {parts.map((v) => (
          <Badge key={v} variant="secondary" className="gap-1 pr-1">
            <span className="max-w-[200px] truncate font-mono">{v}</span>
            <button
              type="button"
              className="ml-0.5 rounded px-0.5 text-muted-foreground hover:bg-muted hover:text-foreground"
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
        <div className="max-h-48 overflow-y-auto rounded-md border border-border bg-background shadow-sm">
          {candidates.map((c) => (
            <button
              key={c.value}
              type="button"
              onMouseDown={(e) => {
                e.preventDefault();
                addValue(c.value);
              }}
              className={cn(
                "flex w-full items-center justify-between gap-2 px-3 py-1.5 text-left text-sm hover:bg-muted/40"
              )}
            >
              <span className="flex items-center gap-2">
                <span className="font-mono text-xs">{c.value}</span>
              </span>
              <span className="truncate text-xs text-muted-foreground">{c.descZh ?? c.descEn ?? ""}</span>
            </button>
          ))}
          {keyword.trim() && !parts.includes(keyword.trim()) && (
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
