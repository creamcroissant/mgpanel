import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Plus, MoreVertical, Pencil, Trash2, Eye, EyeOff, RefreshCw } from "lucide-react";
import { QUERY_KEYS } from "@/lib/constants";
import { getNotices, createNotice, updateNotice, deleteNotice, toggleNoticeVisibility } from "@/api/admin";
import type { AdminNotice, CreateNoticeRequest } from "@/types";
import { AdminPageShell } from "@/components/admin";
import {
  Badge,
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  EmptyState,
  Input,
  Loading,
  Switch,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Textarea,
} from "@/components/ui";

export default function NoticeList() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [editingNotice, setEditingNotice] = useState<AdminNotice | null>(null);
  const [formData, setFormData] = useState<CreateNoticeRequest>({
    title: "",
    content: "",
    show: true,
    popup: false,
  });

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: QUERY_KEYS.ADMIN_NOTICES,
    queryFn: () => getNotices({ page: 1, page_size: 100 }),
  });

  const createMutation = useMutation({
    mutationFn: createNotice,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_NOTICES });
      handleDialogChange(false);
      toast.success(t("admin.notices.createSuccess"));
    },
    onError: (err: Error) => {
      toast.error(t("admin.notices.createError"), { description: err.message });
    },
  });

  const updateMutation = useMutation({
    mutationFn: updateNotice,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_NOTICES });
      handleDialogChange(false);
      toast.success(t("admin.notices.updateSuccess"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteNotice,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_NOTICES });
      toast.success(t("admin.notices.deleteSuccess"));
    },
  });

  const toggleMutation = useMutation({
    mutationFn: ({ id, show }: { id: number; show: boolean }) => toggleNoticeVisibility(id, show),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_NOTICES });
    },
  });

  const handleDialogChange = (open: boolean) => {
    setIsDialogOpen(open);
    if (!open) {
      setEditingNotice(null);
      setFormData({ title: "", content: "", show: true, popup: false });
    }
  };

  const handleEdit = (notice: AdminNotice) => {
    setEditingNotice(notice);
    setFormData({
      title: notice.title,
      content: notice.content,
      img_url: notice.img_url,
      show: notice.show,
      popup: notice.popup,
    });
    setIsDialogOpen(true);
  };

  const handleSubmit = () => {
    if (!formData.title || !formData.content) {
      toast.warning(t("admin.notices.fieldsRequired"));
      return;
    }
    if (editingNotice) {
      updateMutation.mutate({ id: editingNotice.id, ...formData });
    } else {
      createMutation.mutate(formData);
    }
  };

  const formatDate = (timestamp: number) => new Date(timestamp * 1000).toLocaleString();

  const notices: AdminNotice[] = data?.data || [];

  const actions = (
    <>
      <Button variant="outline" onClick={() => refetch()} disabled={isLoading}>
        <RefreshCw className="mr-2 h-4 w-4" />
        {t("common.refresh")}
      </Button>
      <Button onClick={() => setIsDialogOpen(true)}>
        <Plus className="mr-2 h-4 w-4" />
        {t("admin.notices.add")}
      </Button>
    </>
  );

  let content = <Loading />;

  if (error) {
    content = (
      <EmptyState
        title={t("admin.notices.loadError")}
        action={
          <Button variant="outline" onClick={() => refetch()}>
            {t("common.retry")}
          </Button>
        }
      />
    );
  } else if (!isLoading && notices.length === 0) {
    content = <EmptyState title={t("admin.notices.empty")} />;
  } else if (!isLoading) {
    content = (
      <div className="overflow-x-auto rounded-lg border border-border">
        <Table aria-label={t("admin.notices.title")}>
          <TableHeader>
            <TableRow>
              <TableHead>{t("admin.notices.titleCol")}</TableHead>
              <TableHead>{t("admin.notices.createdAt")}</TableHead>
              <TableHead>{t("admin.notices.status")}</TableHead>
              <TableHead>{t("common.actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {notices.map((notice) => (
              <TableRow key={notice.id}>
                <TableCell className="max-w-xs truncate font-medium">{notice.title}</TableCell>
                <TableCell>{formatDate(notice.created_at)}</TableCell>
                <TableCell>
                  <Badge variant={notice.show ? "success" : "secondary"}>
                    {notice.show ? t("admin.notices.visible") : t("admin.notices.hidden")}
                  </Badge>
                </TableCell>
                <TableCell>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" size="icon" aria-label={t("common.actions")}>
                        <MoreVertical className="h-4 w-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem
                        className="gap-2"
                        onSelect={() => toggleMutation.mutate({ id: notice.id, show: !notice.show })}
                      >
                        {notice.show ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                        {notice.show ? t("admin.notices.hide") : t("admin.notices.show")}
                      </DropdownMenuItem>
                      <DropdownMenuItem className="gap-2" onSelect={() => handleEdit(notice)}>
                        <Pencil className="h-4 w-4" />
                        {t("common.edit")}
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        className="gap-2 text-destructive focus:text-destructive"
                        onSelect={() => deleteMutation.mutate(notice.id)}
                      >
                        <Trash2 className="h-4 w-4" />
                        {t("common.delete")}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    );
  }

  return (
    <>
      <AdminPageShell
        title={t("admin.notices.title")}
        description={t("admin.notices.total", { count: notices.length })}
        actions={actions}
      >
        {content}
      </AdminPageShell>

      <Dialog open={isDialogOpen} onOpenChange={handleDialogChange}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {editingNotice ? t("admin.notices.editTitle") : t("admin.notices.addTitle")}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.notices.titleCol")}</label>
              <Input
                placeholder={t("admin.notices.titlePlaceholder")}
                value={formData.title}
                onChange={(event) => setFormData({ ...formData, title: event.target.value })}
                required
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.notices.content")}</label>
              <Textarea
                placeholder={t("admin.notices.contentPlaceholder")}
                value={formData.content}
                onChange={(event) => setFormData({ ...formData, content: event.target.value })}
                rows={6}
                required
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.notices.imageUrl")}</label>
              <Input
                placeholder="https://..."
                value={formData.img_url || ""}
                onChange={(event) => setFormData({ ...formData, img_url: event.target.value })}
              />
            </div>
            <div className="flex items-center gap-6 pt-2">
              <label className="flex items-center gap-2 text-sm">
                <Switch
                  checked={formData.show}
                  onCheckedChange={(value) => setFormData({ ...formData, show: value })}
                />
                {t("admin.notices.visible")}
              </label>
              <label className="flex items-center gap-2 text-sm">
                <Switch
                  checked={formData.popup}
                  onCheckedChange={(value) => setFormData({ ...formData, popup: value })}
                />
                {t("admin.notices.popup")}
              </label>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => handleDialogChange(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={handleSubmit} disabled={createMutation.isPending || updateMutation.isPending}>
              {createMutation.isPending || updateMutation.isPending
                ? t("common.loading")
                : editingNotice
                  ? t("common.save")
                  : t("common.create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
