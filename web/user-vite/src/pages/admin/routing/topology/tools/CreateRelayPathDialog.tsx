import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

/** 连线确认后弹出的链路初始化信息（名称/描述/启停；端点由连线决定不可改）；
 *  也支持手动模式（agents 给定、端点未定时展示两个下拉选择端点） */
export function CreateRelayPathDialog({
  open,
  sourceName,
  targetName,
  agents = [],
  fixed = null,
  busy = false,
  onConfirm,
  onCancel,
}: {
  open: boolean;
  sourceName: string;
  targetName: string;
  /** 手动模式候选（连线模式可不传） */
  agents?: { id: number; name: string }[];
  /** 连线模式的固定端点；null = 手动选 */
  fixed?: { srcId: number; dstId: number } | null;
  busy?: boolean;
  onConfirm: (v: { name: string; description: string; enabled: boolean; srcId: number; dstId: number }) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [enabled, setEnabled] = useState(true);
  const [srcId, setSrcId] = useState<number | null>(null);
  const [dstId, setDstId] = useState<number | null>(null);

  useEffect(() => {
    if (open) {
      setName(fixed && sourceName && targetName ? `${sourceName}-${targetName}` : "");
      setDescription("");
      setEnabled(true);
      setSrcId(fixed?.srcId ?? null);
      setDstId(fixed?.dstId ?? null);
    }
  }, [open, fixed?.srcId, fixed?.dstId, sourceName, targetName]);

  const manual = !fixed;
  const ready = name.trim() !== "" && (!manual || (srcId != null && dstId != null && srcId !== dstId));

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onCancel()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>新建服务器中继链路</DialogTitle>
          <DialogDescription>
            {manual
              ? "选择流量经过的入口与出口服务器，经 mesh 隧道转发"
              : `流量将按 ${sourceName} → ${targetName} 方向经 mesh 隧道转发`}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          {manual ? (
            <>
              <div>
                <label className="mb-1 block text-xs text-muted-foreground">入口服务器</label>
                <Select value={srcId != null ? String(srcId) : ""} onValueChange={(v) => setSrcId(Number(v))}>
                  <SelectTrigger><SelectValue placeholder="选择入口" /></SelectTrigger>
                  <SelectContent>
                    {agents.filter((a) => a.id !== dstId).map((a) => (
                      <SelectItem key={a.id} value={String(a.id)}>{a.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div>
                <label className="mb-1 block text-xs text-muted-foreground">出口服务器</label>
                <Select value={dstId != null ? String(dstId) : ""} onValueChange={(v) => setDstId(Number(v))}>
                  <SelectTrigger><SelectValue placeholder="选择出口" /></SelectTrigger>
                  <SelectContent>
                    {agents.filter((a) => a.id !== srcId).map((a) => (
                      <SelectItem key={a.id} value={String(a.id)}>{a.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </>
          ) : (
            <div className="flex items-center gap-2 rounded-md border bg-muted/40 px-3 py-2 text-sm">
              <span className="font-medium">{sourceName}</span>
              <span aria-hidden>→</span>
              <span className="font-medium">{targetName}</span>
            </div>
          )}
          <div>
            <label className="mb-1 block text-xs text-muted-foreground">链路名称</label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="例如 jp-to-hk" autoFocus />
          </div>
          <div>
            <label className="mb-1 block text-xs text-muted-foreground">描述（可选）</label>
            <Input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="用途说明" />
          </div>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} className="h-4 w-4" />
            创建后立即启用
          </label>
        </div>
        <DialogFooter>
          <Button variant="outline" size="sm" onClick={onCancel} disabled={busy}>取消</Button>
          <Button
            size="sm"
            disabled={busy || !ready}
            onClick={() =>
              onConfirm({
                name,
                description,
                enabled,
                srcId: (fixed?.srcId ?? srcId) as number,
                dstId: (fixed?.dstId ?? dstId) as number,
              })
            }
          >
            创建链路
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
