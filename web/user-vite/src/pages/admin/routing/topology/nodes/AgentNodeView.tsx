import { memo } from "react";
import { Handle, Position } from "@xyflow/react";
import { Server, Cpu } from "lucide-react";
import type { NodeProps } from "@xyflow/react";
import type { TopoNodeData } from "@/lib/topology/types";
import { NODE_SHELL, disabledClass } from "./shared";

/** 画布节点组件共享的 props 数据类型（兄弟节点视图均从此处导入） */
export type FlowNodeData = TopoNodeData;

/**
 * Agent 物理机节点（物理层波次实体化）：
 * - 主机名 + IP 行 + 在线状态点（绿=在线/灰=离线）
 * - active core 徽章行：运行中的核心高亮 primary，未激活置灰
 * - 详情走原生 title tooltip（点击行为暂不接 Drawer）
 */
function AgentNodeViewInner({ data }: NodeProps) {
  const d = data as FlowNodeData;
  const cores = Array.isArray(d.cores) ? d.cores : [];
  const detail = [
    `主机: ${String(d.label)}`,
    `地址: ${String(d.host ?? "未知")}`,
    ...(d.wgIp ? [`Mesh: ${String(d.wgIp)}`] : []),
    `状态: ${d.online ? "在线" : "离线"}`,
    ...(cores.length > 0
      ? [`核心: ${cores.map((c) => `${c.core_type}${c.active ? "(运行中)" : ""}`).join(", ")}`]
      : []),
  ].join("\n");

  return (
    <div
      className={`${NODE_SHELL} relative w-[208px] border-warning/60 ${disabledClass(d.enabled)}`}
      title={detail}
    >
      {/* 中继链路入口角标：N 个入站绑定此链路（画布C） */}
      {d.relayBoundCount != null && d.relayBoundCount > 0 && (
        <span
          className="absolute -top-2 right-1.5 rounded-full bg-primary px-1.5 py-px text-[10px] font-medium leading-tight text-primary-foreground"
          title={`${d.relayBoundCount} 个入站绑定此链路`}
        >
          ⛓ {d.relayBoundCount}
        </span>
      )}

      <div className="flex items-center gap-1.5">
        <Server className="h-3.5 w-3.5 shrink-0 text-warning" aria-hidden />
        <span className="truncate text-sm font-medium" title={String(d.label)}>
          {d.label}
        </span>
        <span
          className={`ml-auto h-2 w-2 shrink-0 rounded-full ${
            d.online ? "bg-success" : "bg-muted-foreground"
          }`}
          title={d.online ? "在线" : "离线"}
        />
      </div>
      <p className="mt-1 truncate font-mono text-xs text-muted-foreground" title={String(d.host ?? "")}>
        {d.host}
      </p>
      {d.wgIp != null && (
        <p className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground/80" title={`Mesh: ${String(d.wgIp)}`}>
          <span aria-hidden>⛓</span> {d.wgIp}
        </p>
      )}
      {/* 中继链路连线锚点：左入右出（服务器链路画布拖线用） */}
      <Handle
        type="target"
        position={Position.Left}
        className="!h-2.5 !w-2.5 !border-2 !border-background !bg-warning"
        isConnectable
      />
      <Handle
        type="source"
        position={Position.Right}
        className="!h-2.5 !w-2.5 !border-2 !border-background !bg-success"
        isConnectable
      />
      {cores.length > 0 && (
        <div className="mt-1.5 flex flex-wrap items-center gap-1">
          <Cpu className="h-3 w-3 shrink-0 text-muted-foreground" aria-hidden />
          {cores.map((c) => (
            <span
              key={c.core_type}
              className={`rounded-sm px-1 py-px text-[10px] leading-tight ${
                c.active
                  ? "bg-primary/10 font-medium text-primary"
                  : "bg-muted text-muted-foreground"
              }`}
              title={c.active ? `${c.core_type} 运行中` : `${c.core_type} 未激活`}
            >
              {c.core_type}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}


/** 拖拽性能：memo 化避免 React Flow 每帧重绘全部节点 */
export const AgentNodeView = memo(AgentNodeViewInner);
