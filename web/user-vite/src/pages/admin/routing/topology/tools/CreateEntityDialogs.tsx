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

const STRATEGIES = [
  { value: "round_robin", label: "轮询 (round_robin)" },
  { value: "weighted_random", label: "加权随机 (weighted_random)" },
  { value: "least_ping", label: "最低延迟 (least_ping)" },
];

/** 新建出口集弹窗：初始化信息一次填完，确认后创建 */
export function CreateExitSetDialog({
  open,
  busy = false,
  onConfirm,
  onCancel,
}: {
  open: boolean;
  busy?: boolean;
  onConfirm: (v: { name: string; description: string; strategy: string; enabled: boolean }) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [strategy, setStrategy] = useState("round_robin");
  const [enabled, setEnabled] = useState(true);

  useEffect(() => {
    if (open) {
      setName("");
      setDescription("");
      setStrategy("round_robin");
      setEnabled(true);
    }
  }, [open]);

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onCancel()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>新建出口集</DialogTitle>
          <DialogDescription>创建后可在画布中向集合添加成员服务器</DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div>
            <label className="mb-1 block text-xs text-muted-foreground">名称</label>
            <Input value={name} onChange={(e) => setName(e.target.value)} autoFocus />
          </div>
          <div>
            <label className="mb-1 block text-xs text-muted-foreground">描述（可选）</label>
            <Input value={description} onChange={(e) => setDescription(e.target.value)} />
          </div>
          <div>
            <label className="mb-1 block text-xs text-muted-foreground">负载策略</label>
            <Select value={strategy} onValueChange={setStrategy}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {STRATEGIES.map((s) => (
                  <SelectItem key={s.value} value={s.value}>
                    {s.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} className="h-4 w-4" />
            创建后立即启用
          </label>
        </div>
        <DialogFooter>
          <Button variant="outline" size="sm" onClick={onCancel} disabled={busy}>取消</Button>
          <Button size="sm" onClick={() => onConfirm({ name, description, strategy, enabled })} disabled={busy || !name.trim()}>
            创建出口集
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

const MATCH_TYPES = [
  { value: "geosite", label: "geosite（站点分类）" },
  { value: "domain", label: "domain（域名）" },
  { value: "ip_cidr", label: "ip_cidr（IP 段）" },
];

/** 新建分流规则弹窗：匹配类型/值 + 目标出口集，一次填完 */
export function CreatePolicyDialog({
  open,
  sets,
  busy = false,
  onConfirm,
  onCancel,
}: {
  open: boolean;
  sets: { id: number; name: string }[];
  busy?: boolean;
  onConfirm: (v: {
    name: string;
    match_type: string;
    match_value: string;
    priority: number;
    enabled: boolean;
    target_set_id: number;
  }) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState("");
  const [matchType, setMatchType] = useState("geosite");
  const [matchValue, setMatchValue] = useState("");
  const [priority, setPriority] = useState(100);
  const [enabled, setEnabled] = useState(true);
  const [targetSetId, setTargetSetId] = useState<string>("");

  useEffect(() => {
    if (open) {
      setName("");
      setMatchType("geosite");
      setMatchValue("");
      setPriority(100);
      setEnabled(true);
      setTargetSetId(sets[0] != null ? String(sets[0].id) : "");
    }
  }, [open, sets]);

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onCancel()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>新建分流规则</DialogTitle>
          <DialogDescription>命中的流量将转发到所选出口集</DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div>
            <label className="mb-1 block text-xs text-muted-foreground">规则名称</label>
            <Input value={name} onChange={(e) => setName(e.target.value)} autoFocus />
          </div>
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
            <div>
              <label className="mb-1 block text-xs text-muted-foreground">匹配类型</label>
              <Select value={matchType} onValueChange={setMatchType}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {MATCH_TYPES.map((m) => (
                    <SelectItem key={m.value} value={m.value}>
                      {m.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <label className="mb-1 block text-xs text-muted-foreground">优先级</label>
              <Input
                type="number"
                value={priority}
                onChange={(e) => setPriority(Number(e.target.value) || 0)}
              />
            </div>
          </div>
          <div>
            <label className="mb-1 block text-xs text-muted-foreground">
              匹配值（多个用逗号分隔）
            </label>
            <Input
              value={matchValue}
              onChange={(e) => setMatchValue(e.target.value)}
              placeholder="例如 netflix 或 example.com,foo.bar"
            />
          </div>
          <div>
            <label className="mb-1 block text-xs text-muted-foreground">目标出口集</label>
            <Select value={targetSetId} onValueChange={setTargetSetId}>
              <SelectTrigger>
                <SelectValue placeholder={sets.length ? "选择出口集" : "暂无出口集"} />
              </SelectTrigger>
              <SelectContent>
                {sets.map((s) => (
                  <SelectItem key={s.id} value={String(s.id)}>
                    {s.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
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
            onClick={() =>
              onConfirm({
                name,
                match_type: matchType,
                match_value: matchValue,
                priority,
                enabled,
                target_set_id: Number(targetSetId),
              })
            }
            disabled={busy || !name.trim() || !matchValue.trim() || !targetSetId}
          >
            创建规则
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
