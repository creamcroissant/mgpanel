import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui";

/**
 * 画布顶部工具条：新建实体入口 + core_type 切换占位（f4 波次接线）。
 * 点击按钮创建空实体并打开 Drawer——由父层(TopologyTab/f4)通过回调完成，
 * 本组件保持纯展示，便于单测与样式迭代。
 */

interface PaletteProps {
  onCreateRule: () => void;
  onCreateSet: () => void;
  /** 无出口集时创建规则无意义（策略必须指向集合），父层置灰 */
  canCreateRule?: boolean;
  disabled?: boolean;
}

export function Palette({ onCreateRule, onCreateSet, canCreateRule = true, disabled = false }: PaletteProps) {
  const { t } = useTranslation();
  const tf = (key: string, fallback: string) => t(key, fallback) as string;

  return (
    <div className="flex items-center gap-2 border-b border-border bg-card/60 px-3 py-2">
      {/* f4 预留位：core_type 切换 Select 迁移/复用到此处 */}
      <Button type="button" variant="outline" size="sm" onClick={onCreateRule} disabled={disabled || !canCreateRule} title={canCreateRule ? undefined : tf("admin.topology.palette.needSetFirst", "请先创建至少一个出口集")}>
        + {tf("admin.topology.palette.addRule", "规则")}
      </Button>
      <Button type="button" variant="outline" size="sm" onClick={onCreateSet} disabled={disabled}>
        + {tf("admin.topology.palette.addSet", "出口集")}
      </Button>
    </div>
  );
}
