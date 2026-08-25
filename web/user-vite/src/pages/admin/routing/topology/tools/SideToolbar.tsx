import { MousePointer2, Spline, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";

export type ToolMode = "select" | "connect" | "delete";

interface SideToolbarProps {
  mode: ToolMode;
  onChange: (mode: ToolMode) => void;
  /** connect 模式下已选入口的提示文案（空=未选） */
  hint?: string;
}

/** 服务器链路画布左侧工具栏：选择 / 连线 / 删除（点击式交互，替代锚点拖拽） */
export function SideToolbar({ mode, onChange, hint }: SideToolbarProps) {
  const { t } = useTranslation();
  const items: { key: ToolMode; icon: typeof MousePointer2; label: string }[] = [
    { key: "select", icon: MousePointer2, label: t("admin.topology.tools.select") },
    { key: "connect", icon: Spline, label: t("admin.topology.tools.connect") },
    { key: "delete", icon: Trash2, label: t("admin.topology.tools.delete") },
  ];
  return (
    <div className="flex w-14 flex-col items-center gap-1 rounded-md border bg-card py-2">
      {items.map(({ key, icon: Icon, label }) => (
        <button
          key={key}
          type="button"
          title={`${label} — ${t(`admin.topology.tools.hint.${key}`)}`}
          onClick={() => onChange(key)}
          className={`flex h-9 w-9 items-center justify-center rounded transition-colors ${
            mode === key
              ? "bg-primary text-primary-foreground"
              : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
          }`}
        >
          <Icon className="h-4 w-4" />
        </button>
      ))}
      <div className="mt-1 h-px w-8 bg-border" />
      <p className="mt-1 w-full px-1 text-center text-[10px] leading-tight text-muted-foreground">
        {hint || t(`admin.topology.tools.hint.${mode}`)}
      </p>
    </div>
  );
}
