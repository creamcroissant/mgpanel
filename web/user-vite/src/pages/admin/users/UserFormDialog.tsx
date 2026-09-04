import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { X } from "lucide-react";
import {
  Badge,
  Button,
  Checkbox,
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
  Textarea,
} from "@/components/ui";
import type { AdminServerGroup, AdminServerNode, AdminUser } from "@/types";

const GB = 1024 * 1024 * 1024;

interface UserFormValue {
  email: string;
  username: string;
  password: string;
  group_id: number | "";
  trafficGb: string;
  expiresDate: string;
  neverExpire: boolean;
  remarks: string;
  bannedServerIds: number[];
}

function bytesToGb(bytes: number): number {
  return bytes > 0 ? Math.floor(bytes / GB) : 0;
}

function gbToBytes(gb: string): number {
  const value = Number(gb);
  if (!Number.isFinite(value) || value <= 0) return 0;
  return Math.round(value * GB);
}

function unixToDateInput(unix?: number): string {
  if (!unix || unix <= 0) return "";
  const date = new Date(unix * 1000);
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000);
  return local.toISOString().slice(0, 10);
}

function dateInputToUnix(value: string): number {
  if (!value) return 0;
  const date = new Date(`${value}T00:00:00`);
  if (Number.isNaN(date.getTime())) return 0;
  return Math.floor(date.getTime() / 1000);
}

function isValidPassword(value: string): boolean {
  if (value.length < 8) return false;
  return /[a-zA-Z]/.test(value) && /\d/.test(value);
}

function diffArrays(a: number[], b: number[]): boolean {
  const sortedA = [...a].sort((x, y) => x - y);
  const sortedB = [...b].sort((x, y) => x - y);
  if (sortedA.length !== sortedB.length) return true;
  return sortedA.some((v, i) => v !== sortedB[i]);
}

function initialState(initial?: AdminUser): UserFormValue {
  const trafficGb = initial && initial.transfer_enable > 0 ? String(bytesToGb(initial.transfer_enable)) : "";
  const hasExpiry = !!initial?.expired_at && initial.expired_at > 0;
  return {
    email: initial?.email ?? "",
    username: initial?.username ?? "",
    password: "",
    group_id: initial?.group_id ? initial.group_id : "",
    trafficGb,
    expiresDate: hasExpiry ? unixToDateInput(initial.expired_at) : "",
    neverExpire: !hasExpiry,
    remarks: initial?.remarks ?? "",
    bannedServerIds: initial?.banned_server_ids ?? [],
  };
}

interface UserFormDialogProps {
  open: boolean;
  mode: "create" | "edit";
  initial?: AdminUser;
  groups: AdminServerGroup[];
  servers: AdminServerNode[];
  saving: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (data: { create?: Record<string, unknown>; edit?: { id: number; payload: Record<string, unknown> } }) => void;
}

export default function UserFormDialog({
  open,
  mode,
  initial,
  groups,
  servers,
  saving,
  onOpenChange,
  onSubmit,
}: UserFormDialogProps) {
  const { t } = useTranslation();
  const isEdit = mode === "edit";
  const [serverSearch, setServerSearch] = useState("");

  useEffect(() => {
    if (open) {
      setForm(initialState(initial));
      setServerSearch("");
    }
  }, [open, mode, initial]);

  const set = <K extends keyof UserFormValue>(key: K, value: UserFormValue[K]) =>
    setForm((prev) => ({ ...prev, [key]: value }));

  const [form, setForm] = useState<UserFormValue>(() => initialState(initial));

  const selectedServerNames = useMemo(() => {
    const byId = new Map(servers.map((s) => [s.id, s.name]));
    return form.bannedServerIds.map((id) => ({ id, name: byId.get(id) ?? String(id) }));
  }, [form.bannedServerIds, servers]);

  const serverCandidates = useMemo(() => {
    const keyword = serverSearch.trim().toLowerCase();
    if (!keyword) return servers;
    return servers.filter((s) => (s.name || "").toLowerCase().includes(keyword));
  }, [servers, serverSearch]);

  const toggleBannedServer = (id: number) => {
    const current = form.bannedServerIds;
    set(
      "bannedServerIds",
      current.includes(id) ? current.filter((x) => x !== id) : [...current, id]
    );
  };

  const removeBannedServer = (id: number) => {
    set("bannedServerIds", form.bannedServerIds.filter((x) => x !== id));
  };

  const handleSubmit = () => {
    if (mode === "create") {
      if (!form.email.trim()) return;
      if (!form.password || !isValidPassword(form.password)) return;
      onSubmit({
        create: {
          email: form.email.trim() || undefined,
          username: form.username.trim() || undefined,
          password: form.password,
          group_id: form.group_id === "" ? undefined : form.group_id,
          transfer_enable: gbToBytes(form.trafficGb) || undefined,
          expired_at: form.neverExpire ? undefined : dateInputToUnix(form.expiresDate) || undefined,
          remarks: form.remarks.trim() || undefined,
        },
      });
      return;
    }
    if (!initial) return;
    if (form.password && !isValidPassword(form.password)) return;
    const payload: Record<string, unknown> = {};
    payload.email = form.email.trim() || undefined;
    payload.username = form.username.trim() || undefined;
    if (form.password) payload.password = form.password;
    const newGroup = form.group_id === "" ? 0 : Number(form.group_id);
    if (newGroup !== (initial.group_id ?? 0)) payload.group_id = newGroup;
    const newBytes = gbToBytes(form.trafficGb);
    if (newBytes !== (initial.transfer_enable ?? 0)) payload.transfer_enable = newBytes;
    const newExp = form.neverExpire ? 0 : dateInputToUnix(form.expiresDate);
    if (newExp !== (initial.expired_at ?? 0)) payload.expired_at = newExp;
    payload.remarks = form.remarks.trim() || undefined;
    const initialBan = initial.banned_server_ids ?? [];
    if (diffArrays(form.bannedServerIds, initialBan)) {
      payload.banned_server_ids = form.bannedServerIds;
    }
    onSubmit({ edit: { id: initial.id, payload } });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {isEdit ? t("admin.users.editTitle") : t("admin.users.addTitle")}
          </DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.users.email")}</label>
            <Input
              placeholder={t("admin.users.emailPlaceholder") || "user@example.com"}
              value={form.email}
              onChange={(e) => set("email", e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.users.username")}</label>
            <Input
              placeholder={t("admin.users.usernamePlaceholder")}
              value={form.username}
              onChange={(e) => set("username", e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.users.password")}</label>
            <Input
              type="password"
              placeholder={t("admin.users.passwordPlaceholder") || "••••••••"}
              value={form.password}
              onChange={(e) => set("password", e.target.value)}
            />
            {isEdit ? (
              <p className="text-xs text-muted-foreground">{t("admin.users.resetPasswordHint")}</p>
            ) : null}
          </div>
          {groups.length > 0 ? (
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.users.group")}</label>
              <Select
                value={form.group_id === "" ? "" : String(form.group_id)}
                onValueChange={(value) =>
                  set("group_id", value === "" ? "" : Number(value))
                }
              >
                <SelectTrigger>
                  <SelectValue placeholder={t("admin.users.group")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="">{t("admin.users.noGroup")}</SelectItem>
                  {groups.map((group) => (
                    <SelectItem key={group.id} value={String(group.id)}>
                      {group.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          ) : null}
          <div className="space-y-2">
            <label className="text-sm font-medium">
              {t("admin.users.trafficLimit")} (GB)
            </label>
            <Input
              type="number"
              min={0}
              step="any"
              placeholder={t("admin.users.trafficLimitPlaceholder")}
              value={form.trafficGb}
              onChange={(e) => set("trafficGb", e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.users.expiredAt")}</label>
            <Input
              type="date"
              value={form.expiresDate}
              disabled={form.neverExpire}
              onChange={(e) => set("expiresDate", e.target.value)}
            />
            <label className="flex items-center gap-2 text-sm text-muted-foreground">
              <Checkbox
                checked={form.neverExpire}
                onCheckedChange={(checked) => set("neverExpire", !!checked)}
              />
              {t("admin.users.neverExpire")}
            </label>
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">{t("admin.users.remarks")}</label>
            <Textarea
              placeholder={t("admin.users.remarksPlaceholder")}
              value={form.remarks}
              onChange={(e) => set("remarks", e.target.value)}
            />
          </div>
          {servers.length > 0 ? (
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.users.bannedServers")}</label>
              <Input
                placeholder={t("admin.users.bannedServersPlaceholder")}
                value={serverSearch}
                onChange={(e) => setServerSearch(e.target.value)}
              />
              {selectedServerNames.length > 0 ? (
                <div className="flex flex-wrap gap-1.5">
                  {selectedServerNames.map(({ id, name }) => (
                    <Badge key={id} variant="secondary" className="gap-1 pr-1">
                      {name}
                      <button
                        type="button"
                        className="rounded p-0.5 hover:bg-muted-foreground/20"
                        onClick={() => removeBannedServer(id)}
                        aria-label={t("common.remove")}
                      >
                        <X className="h-3 w-3" />
                      </button>
                    </Badge>
                  ))}
                </div>
              ) : null}
              <div className="max-h-40 space-y-1 overflow-y-auto rounded-md border p-2">
                {serverCandidates.length === 0 ? (
                  <p className="text-xs text-muted-foreground">{t("admin.users.bannedServersEmpty")}</p>
                ) : (
                  serverCandidates.map((server) => {
                    const checked = form.bannedServerIds.includes(server.id);
                    return (
                      <label
                        key={server.id}
                        className="flex cursor-pointer items-center gap-2 rounded px-1 py-1 text-sm hover:bg-muted"
                      >
                        <Checkbox
                          checked={checked}
                          onCheckedChange={() => toggleBannedServer(server.id)}
                        />
                        <span className="min-w-0 truncate">{server.name}</span>
                      </label>
                    );
                  })
                )}
              </div>
              <p className="text-xs text-muted-foreground">{t("admin.users.bannedServersHint")}</p>
            </div>
          ) : null}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button onClick={handleSubmit} disabled={saving}>
            {saving ? t("common.loading") : isEdit ? t("common.save") : t("common.create")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
