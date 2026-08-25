import { memo } from "react";

/**
 * 物理层泳道底板：纯装饰性分组背景（半透明色带 + 虚线框），
 * 置于 agent 节点后方形成"逻辑层 / 物理层"视觉区隔。
 * 不可选中/拖拽/连线，布局尺寸由 layout.ts 计算。
 */
function LaneNodeViewInner() {
  return (
    <div
      className="h-full w-full rounded-lg border border-dashed border-border/60 bg-muted/25"
      title="物理层：agent 主机与 mesh 组网链路"
    />
  );
}


/** 拖拽性能：memo 化避免 React Flow 每帧重绘全部节点 */
export const LaneNodeView = memo(LaneNodeViewInner);
