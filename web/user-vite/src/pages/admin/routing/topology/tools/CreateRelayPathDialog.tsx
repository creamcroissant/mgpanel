import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

/** 连线确认后弹出的链路初始化信息（名称/描述/启停；端点由连线决定不可改） */
export function CreateRelayPathDialog({
  open,
  sourceName,
  targetName,
  busy = false,
  onConfirm,
  onCancel,
}: {
  open: boolean;
  sourceName: string;
  targetName: string;
  busy?: boolean;
  onConfirm: (v: { name: string; description: string; enabled: boolean }) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [enabled, setEnabled] = useState(true);

  useEffect(() => {
    if (open) {
      setName(`${sourceName}-${targetName}`);
      setDescription("");
      setEnabled(true);
    }
  }, [open, sourceName, targetName]);

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onCancel()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>新建服务器中继链路</DialogTitle>
          <DialogDescription>
            流量将按 {sourceName} → {targetName} 方向经 mesh 隧道转发
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div className="flex items-center gap-2 rounded-md border bg-muted/40 px-3 py-2 text-sm">
            <span className="font-medium">{sourceName}</span>
            <span aria-hidden>→</span>
            <span className="font-medium">{targetName}</span>
          </div>
          <div>
            <label className="mb-1 block text-xs text-muted-foreground">链路名称</label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="例如 jp-to-hk"
              autoFocus
            />
          </div>
          <div>
            <label className="mb-1 block text-xs text-muted-foreground">描述（可选）</label>
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="用途说明"
            />
          </div>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
              className="h-4 w-4"
            />
            创建后立即启用
          </label>
        </div>
        <DialogFooter>
          <Button variant="outline" size="sm" onClick={onCancel} disabled={busy}>
            取消
          </Button>
          <Button size="sm" onClick={() => onConfirm({ name, description, enabled })} disabled={busy || !name.trim()}>
            创建链路
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
