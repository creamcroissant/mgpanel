import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Plus, MoreVertical, Pencil, Trash2, Package, RefreshCw } from "lucide-react";
import { QUERY_KEYS } from "@/lib/constants";
import { getPlans, createPlan, updatePlan, deletePlan } from "@/api/admin";
import type { AdminPlan, CreatePlanRequest } from "@/types";
import { AdminPageShell, formatBytes } from "@/components/admin";
import {
  Badge,
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
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
} from "@/components/ui";

export default function PlanList() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [editingPlan, setEditingPlan] = useState<AdminPlan | null>(null);
  const [formData, setFormData] = useState<CreatePlanRequest>({
    name: "",
    transfer_enable: 107374182400,
    show: true,
    sell: true,
    renew: true,
  });

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: QUERY_KEYS.ADMIN_PLANS,
    queryFn: () => getPlans({ page: 1, page_size: 100 }),
  });

  const createMutation = useMutation({
    mutationFn: createPlan,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_PLANS });
      handleDialogChange(false);
      toast.success(t("admin.plans.createSuccess"));
    },
    onError: (err: Error) => {
      toast.error(t("admin.plans.createError"), { description: err.message });
    },
  });

  const updateMutation = useMutation({
    mutationFn: updatePlan,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_PLANS });
      handleDialogChange(false);
      toast.success(t("admin.plans.updateSuccess"));
    },
    onError: (err: Error) => {
      toast.error(t("admin.plans.updateError"), { description: err.message });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deletePlan,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.ADMIN_PLANS });
      toast.success(t("admin.plans.deleteSuccess"));
    },
  });

  const handleDialogChange = (open: boolean) => {
    setIsDialogOpen(open);
    if (!open) {
      setEditingPlan(null);
      setFormData({
        name: "",
        transfer_enable: 107374182400,
        show: true,
        sell: true,
        renew: true,
      });
    }
  };

  const handleEdit = (plan: AdminPlan) => {
    setEditingPlan(plan);
    setFormData({
      name: plan.name,
      content: plan.content,
      transfer_enable: plan.transfer_enable,
      speed_limit: plan.speed_limit,
      device_limit: plan.device_limit,
      show: plan.show,
      sell: plan.sell,
      renew: plan.renew,
    });
    setIsDialogOpen(true);
  };

  const handleSubmit = () => {
    if (!formData.name) {
      toast.warning(t("admin.plans.nameRequired"));
      return;
    }
    if (editingPlan) {
      updateMutation.mutate({ id: editingPlan.id, ...formData });
    } else {
      createMutation.mutate(formData);
    }
  };

  const plans: AdminPlan[] = data?.data || [];

  const actions = (
    <>
      <Button variant="outline" onClick={() => refetch()} disabled={isLoading}>
        <RefreshCw className="mr-2 h-4 w-4" />
        {t("common.refresh")}
      </Button>
      <Button data-testid="admin-plans-add-button" onClick={() => setIsDialogOpen(true)}>
        <Plus className="mr-2 h-4 w-4" />
        {t("admin.plans.add")}
      </Button>
    </>
  );

  let content = <Loading />;

  if (error) {
    content = (
      <EmptyState
        title={t("admin.plans.loadError")}
        action={
          <Button variant="outline" onClick={() => refetch()}>
            {t("common.retry")}
          </Button>
        }
      />
    );
  } else if (!isLoading && plans.length === 0) {
    content = (
      <EmptyState
        icon={<Package className="h-full w-full" />}
        title={t("admin.plans.empty")}
        description={t("admin.plans.emptyDescription")}
        action={
          <Button data-testid="admin-plans-add-button-empty" onClick={() => setIsDialogOpen(true)}>
            <Plus className="mr-2 h-4 w-4" />
            {t("admin.plans.add")}
          </Button>
        }
      />
    );
  } else if (!isLoading) {
    content = (
      <div className="overflow-x-auto rounded-lg border border-border">
        <Table aria-label={t("admin.plans.title")}>
          <TableHeader>
            <TableRow className="bg-muted/40">
              <TableHead>{t("admin.plans.name")}</TableHead>
              <TableHead>{t("admin.plans.traffic")}</TableHead>
              <TableHead>{t("admin.plans.speedLimit")}</TableHead>
              <TableHead>{t("admin.plans.deviceLimit")}</TableHead>
              <TableHead>{t("admin.plans.status")}</TableHead>
              <TableHead>{t("common.actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {plans.map((plan) => (
              <TableRow key={plan.id} className="h-12">
                <TableCell className="font-medium">{plan.name}</TableCell>
                <TableCell>{formatBytes(plan.transfer_enable)}</TableCell>
                <TableCell>
                  {plan.speed_limit ? `${plan.speed_limit} Mbps` : t("admin.plans.unlimited")}
                </TableCell>
                <TableCell>
                  {plan.device_limit ? plan.device_limit : t("admin.plans.unlimited")}
                </TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-1">
                    {plan.show && <Badge variant="success">{t("admin.plans.visible")}</Badge>}
                    {plan.sell && <Badge variant="secondary">{t("admin.plans.selling")}</Badge>}
                    {plan.renew && <Badge variant="default">{t("admin.plans.renewable")}</Badge>}
                  </div>
                </TableCell>
                <TableCell>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" size="icon" aria-label={t("common.actions")}>
                        <MoreVertical className="h-4 w-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem className="gap-2" onSelect={() => handleEdit(plan)}>
                        <Pencil className="h-4 w-4" />
                        {t("common.edit")}
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        className="gap-2 text-destructive focus:text-destructive"
                        onSelect={() => deleteMutation.mutate(plan.id)}
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
        title={t("admin.plans.title")}
        description={t("admin.plans.total", { count: plans.length })}
        actions={actions}
      >
        {content}
      </AdminPageShell>

      <Dialog open={isDialogOpen} onOpenChange={handleDialogChange}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>
              {editingPlan ? t("admin.plans.editTitle") : t("admin.plans.addTitle")}
            </DialogTitle>
            <DialogDescription>{t("admin.plans.dialogDescription")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.plans.name")}</label>
              <Input
                placeholder={t("admin.plans.namePlaceholder")}
                value={formData.name}
                onChange={(event) => setFormData({ ...formData, name: event.target.value })}
                required
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.plans.trafficGB")}</label>
              <Input
                type="number"
                placeholder="100"
                value={String(formData.transfer_enable / 1073741824)}
                onChange={(event) =>
                  setFormData({
                    ...formData,
                    transfer_enable: parseInt(event.target.value, 10) * 1073741824 || 0,
                  })
                }
              />
            </div>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.plans.speedLimit")}</label>
                <Input
                  type="number"
                  placeholder={t("admin.plans.unlimited")}
                  value={formData.speed_limit ? String(formData.speed_limit) : ""}
                  onChange={(event) =>
                    setFormData({
                      ...formData,
                      speed_limit: parseInt(event.target.value, 10) || undefined,
                    })
                  }
                />
              </div>
              <div className="space-y-2">
                <label className="text-sm font-medium">{t("admin.plans.deviceLimit")}</label>
                <Input
                  type="number"
                  placeholder={t("admin.plans.unlimited")}
                  value={formData.device_limit ? String(formData.device_limit) : ""}
                  onChange={(event) =>
                    setFormData({
                      ...formData,
                      device_limit: parseInt(event.target.value, 10) || undefined,
                    })
                  }
                />
              </div>
            </div>
            <div className="flex flex-wrap gap-4">
              <label className="flex items-center gap-2 text-sm">
                <Switch
                  checked={formData.show}
                  onCheckedChange={(value) => setFormData({ ...formData, show: value })}
                />
                {t("admin.plans.visible")}
              </label>
              <label className="flex items-center gap-2 text-sm">
                <Switch
                  checked={formData.sell}
                  onCheckedChange={(value) => setFormData({ ...formData, sell: value })}
                />
                {t("admin.plans.selling")}
              </label>
              <label className="flex items-center gap-2 text-sm">
                <Switch
                  checked={formData.renew}
                  onCheckedChange={(value) => setFormData({ ...formData, renew: value })}
                />
                {t("admin.plans.renewable")}
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
                : editingPlan
                  ? t("common.save")
                  : t("common.create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
