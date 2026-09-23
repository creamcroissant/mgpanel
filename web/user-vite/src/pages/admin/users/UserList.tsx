import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Plus, Search, MoreVertical, Ban, Trash2, Pencil } from "lucide-react";
import { QUERY_KEYS } from "@/lib/constants";
import {
  getUsers,
  createUser,
  updateUser,
  toggleUserBan,
  deleteUser,
  getServerGroups,
  getAdminServerNodes,
} from "@/api/admin";
import type { AdminUser, AdminServerGroup, AdminServerNode, CreateUserRequest, UpdateUserRequest } from "@/types";
import { AdminPageShell, formatBytes } from "@/components/admin";
import { useAuth } from "@/providers/AuthProvider";
import {
  Badge,
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  EmptyState,
  Input,
  Loading,
  Pagination,
  ResponsiveList,
  ResponsiveListField,
  ResponsiveListItem,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui";
import UserFormDialog from "./UserFormDialog";

export default function UserList() {
  const { t } = useTranslation();
  const { user: currentUser } = useAuth();
  const queryClient = useQueryClient();
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [dialogMode, setDialogMode] = useState<"create" | "edit">("create");
  const [editingUser, setEditingUser] = useState<AdminUser | undefined>(undefined);
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: [...QUERY_KEYS.ADMIN_USERS, page, search],
    queryFn: () => getUsers({ page, page_size: 20, search: search || undefined }),
  });

  const { data: groups } = useQuery({
    queryKey: ["admin", "server-groups"],
    queryFn: getServerGroups,
    staleTime: 60_000,
  });
  const groupOptions: AdminServerGroup[] = groups ?? [];

  const { data: nodes } = useQuery({
    queryKey: ["admin", "server-nodes"],
    queryFn: getAdminServerNodes,
    staleTime: 60_000,
  });
  const serverOptions: AdminServerNode[] = nodes ?? [];

  const createMutation = useMutation({
    mutationFn: createUser,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_USERS });
      setIsDialogOpen(false);
      toast.success(t("admin.users.createSuccess"));
    },
    onError: (err: Error) => {
      toast.error(t("admin.users.createError"), { description: err.message });
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: Record<string, unknown> }) =>
      updateUser({ ...(payload as unknown as Omit<UpdateUserRequest, "id">), id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_USERS });
      setIsDialogOpen(false);
      toast.success(t("admin.users.updateSuccess"));
    },
    onError: (err: Error) => {
      toast.error(t("admin.users.updateSuccess"), { description: err.message });
    },
  });

  const banMutation = useMutation({
    mutationFn: ({ id, banned }: { id: number; banned: boolean }) => toggleUserBan(id, banned),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_USERS });
      toast.success(t("admin.users.updateSuccess"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteUser,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_USERS });
      toast.success(t("admin.users.deleteSuccess"));
    },
  });

  const users: AdminUser[] = data?.data || [];
  const total = data?.total || 0;
  const totalPages = Math.ceil(total / 20);

  const formatDate = (timestamp?: number) => {
    if (!timestamp) return "-";
    return new Date(timestamp * 1000).toLocaleDateString();
  };

  const groupNameOf = (user: AdminUser): string =>
    user.group?.name || String(user.group_id ?? "");

  const isSelf = (user: AdminUser): boolean => !!currentUser && currentUser.id === user.id;

  const openCreate = () => {
    setDialogMode("create");
    setEditingUser(undefined);
    setIsDialogOpen(true);
  };

  const openEdit = (user: AdminUser) => {
    setDialogMode("edit");
    setEditingUser(user);
    setIsDialogOpen(true);
  };

  const handleDialogChange = (open: boolean) => {
    setIsDialogOpen(open);
    if (!open) {
      setEditingUser(undefined);
    }
  };

  const handleSubmit = (data: {
    create?: Record<string, unknown>;
    edit?: { id: number; payload: Record<string, unknown> };
  }) => {
    if (data.create) {
      createMutation.mutate(data.create as unknown as CreateUserRequest);
      return;
    }
    if (data.edit) {
      updateMutation.mutate({ id: data.edit.id, payload: data.edit.payload });
    }
  };

  const renderUserStatus = (user: AdminUser) => (
    <Badge
      variant={user.banned ? "danger" : user.status === 1 ? "success" : "default"}
    >
      {user.banned
        ? t("admin.users.banned")
        : user.status === 1
          ? t("admin.users.active")
          : t("admin.users.inactive")}
    </Badge>
  );

  const renderUserActions = (user: AdminUser) => {
    const self = isSelf(user);
    return (
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon" aria-label={t("common.actions")}>
            <MoreVertical className="h-4 w-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem className="gap-2" onSelect={() => openEdit(user)}>
            <Pencil className="h-4 w-4" />
            {t("common.edit")}
          </DropdownMenuItem>
          {!self ? (
            <>
              <DropdownMenuItem
                className="gap-2"
                onSelect={() => banMutation.mutate({ id: user.id, banned: !user.banned })}
              >
                <Ban className="h-4 w-4" />
                {user.banned ? t("admin.users.unban") : t("admin.users.ban")}
              </DropdownMenuItem>
              <DropdownMenuItem
                className="gap-2 text-destructive focus:text-destructive"
                onSelect={() => deleteMutation.mutate(user.id)}
              >
                <Trash2 className="h-4 w-4" />
                {t("common.delete")}
              </DropdownMenuItem>
            </>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>
    );
  };

  const toolbar = (
    <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
      <div className="relative w-full sm:w-64">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          className="h-10 pl-9"
          placeholder={t("admin.users.searchPlaceholder")}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          onKeyDown={(event) => event.key === "Enter" && refetch()}
        />
      </div>
      <Button onClick={openCreate}>
        <Plus className="mr-2 h-4 w-4" />
        {t("admin.users.add")}
      </Button>
    </div>
  );

  let content = <Loading />;

  if (error) {
    content = (
      <EmptyState
        title={t("admin.users.loadError")}
        description={t("common.retry")}
        action={
          <Button variant="outline" onClick={() => refetch()}>
            {t("common.retry")}
          </Button>
        }
      />
    );
  } else if (!isLoading) {
    content = (
      <div className="space-y-4">
        <div className="hidden md:block">
          <Table aria-label={t("admin.users.title")}>
            <TableHeader>
              <TableRow>
                <TableHead>{t("admin.users.email")}</TableHead>
                <TableHead>{t("admin.users.group")}</TableHead>
                <TableHead className="text-right">{t("admin.users.traffic")}</TableHead>
                <TableHead>{t("admin.users.expiredAt")}</TableHead>
                <TableHead>{t("admin.users.status")}</TableHead>
                <TableHead>{t("common.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {users.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} className="p-0">
                    <EmptyState
                      title={t("admin.users.empty")}
                      description={t("admin.users.searchPlaceholder")}
                      size="sm"
                    />
                  </TableCell>
                </TableRow>
              ) : (
                users.map((user) => (
                  <TableRow key={user.id}>
                    <TableCell>
                      <div className="flex flex-col gap-1">
                        <span className="font-medium text-foreground">
                          {user.email || user.username || "-"}
                        </span>
                        <span className="flex flex-wrap gap-1">
                          {user.is_admin && (
                            <Badge variant="warning">{t("admin.users.admin")}</Badge>
                          )}
                          {isSelf(user) && <Badge variant="default">{t("admin.users.self")}</Badge>}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell>{groupNameOf(user) || "-"}</TableCell>
                    <TableCell className="text-right tabular-nums">
                      {formatBytes(user.u + user.d)} / {formatBytes(user.transfer_enable)}
                    </TableCell>
                    <TableCell>{formatDate(user.expired_at)}</TableCell>
                    <TableCell>{renderUserStatus(user)}</TableCell>
                    <TableCell>{renderUserActions(user)}</TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>

        {users.length === 0 ? (
          <div className="md:hidden rounded-none border border-border bg-card">
            <EmptyState
              title={t("admin.users.empty")}
              description={t("admin.users.searchPlaceholder")}
              size="sm"
            />
          </div>
        ) : (
          <ResponsiveList label={t("admin.users.mobileListLabel")}>
            {users.map((user) => (
              <ResponsiveListItem key={user.id}>
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0 space-y-1">
                    <div className="truncate font-medium text-foreground">
                      {user.email || user.username || "-"}
                    </div>
                    <span className="flex flex-wrap gap-1">
                      {user.is_admin && <Badge variant="warning">{t("admin.users.admin")}</Badge>}
                      {isSelf(user) && <Badge variant="default">{t("admin.users.self")}</Badge>}
                    </span>
                  </div>
                  {renderUserActions(user)}
                </div>

                <dl className="mt-4 grid grid-cols-2 gap-3">
                  <ResponsiveListField label={t("admin.users.group")}>
                    {groupNameOf(user) || "-"}
                  </ResponsiveListField>
                  <ResponsiveListField label={t("admin.users.status")}>
                    {renderUserStatus(user)}
                  </ResponsiveListField>
                  <ResponsiveListField label={t("admin.users.traffic")} className="col-span-2">
                    {formatBytes(user.u + user.d)} / {formatBytes(user.transfer_enable)}
                  </ResponsiveListField>
                  <ResponsiveListField label={t("admin.users.expiredAt")}>
                    {formatDate(user.expired_at)}
                  </ResponsiveListField>
                </dl>
              </ResponsiveListItem>
            ))}
          </ResponsiveList>
        )}

        {totalPages > 1 && (
          <div className="flex justify-center">
            <Pagination page={page} totalPages={totalPages} onPageChange={setPage} />
          </div>
        )}
      </div>
    );
  }

  return (
    <>
      <AdminPageShell
        title={t("admin.users.title")}
        description={t("admin.users.total", { count: total })}
        toolbar={toolbar}
      >
        {content}
      </AdminPageShell>

      <UserFormDialog
        open={isDialogOpen}
        mode={dialogMode}
        initial={editingUser}
        groups={groupOptions}
        servers={serverOptions}
        saving={createMutation.isPending || updateMutation.isPending}
        onOpenChange={handleDialogChange}
        onSubmit={handleSubmit}
      />
    </>
  );
}
