import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Globe, Plus, RefreshCw, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { AdminPageShell } from "@/components/admin";
import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
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
import { QUERY_KEYS } from "@/lib/constants";
import {
  createCDNSite,
  deleteCDNSite,
  deployCDNSite,
  fetchCDNSites,
  updateCDNSite,
} from "@/api/admin/cdn";
import type { CDNSite, CreateCDNSiteRequest } from "@/api/admin/cdn";
import CDNSiteForm from "./CDNSiteForm";

export default function CDNSiteList() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const queryKey = QUERY_KEYS.ADMIN_CDN_SITES;

  const [formOpen, setFormOpen] = useState(false);
  const [editingSite, setEditingSite] = useState<CDNSite | null>(null);

  const { data: fetched, isLoading } = useQuery({
    queryKey,
    queryFn: () => fetchCDNSites(),
  });
  const sites: CDNSite[] = fetched?.sites ?? [];

  const createMutation = useMutation({
    mutationFn: (data: CreateCDNSiteRequest) => createCDNSite(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
      setFormOpen(false);
      toast.success(t("admin.cdn.messages.siteCreated"));
    },
  });

  const updateMutation = useMutation({
    mutationFn: (data: CreateCDNSiteRequest & { id: number }) => updateCDNSite(data.id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
      setFormOpen(false);
      toast.success(t("admin.cdn.messages.siteUpdated"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteCDNSite(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
      toast.success(t("admin.cdn.messages.siteDeleted"));
    },
  });

  const deployMutation = useMutation({
    mutationFn: (id: number) => deployCDNSite(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
      toast.success(t("admin.cdn.messages.deployed"));
    },
  });

  const handleAdd = () => {
    setEditingSite(null);
    setFormOpen(true);
  };

  const handleEdit = (site: CDNSite) => {
    setEditingSite(site);
    setFormOpen(true);
  };

  const handleSubmit = (data: CreateCDNSiteRequest) => {
    if (editingSite) {
      updateMutation.mutate({ ...data, id: editingSite.id });
    } else {
      createMutation.mutate(data);
    }
  };

  const handleDeploy = (id: number) => {
    deployMutation.mutate(id);
  };

  const renderStatusBadge = (site: CDNSite) => (
    <Badge
      variant={
        site.status === "active"
          ? "success"
          : site.status === "error"
            ? "destructive"
            : "secondary"
      }
    >
      {site.status || "pending"}
    </Badge>
  );

  const renderEnabledBadge = (site: CDNSite) =>
    site.enabled ? (
      <Badge variant="success">{t("admin.cdn.enabled")}</Badge>
    ) : (
      <Badge variant="secondary">{t("admin.cdn.disabled")}</Badge>
    );

  const renderSiteActions = (site: CDNSite, layout: "desktop" | "mobile") => (
    <div
      className={layout === "mobile" ? "mt-4 grid grid-cols-2 gap-2" : "flex justify-end gap-1"}
    >
      <Button
        variant="ghost"
        size={layout === "mobile" ? "default" : "sm"}
        onClick={() => handleEdit(site)}
        data-testid={`cdn-edit-${site.id}`}
      >
        {t("common.edit")}
      </Button>
      <Button
        variant="ghost"
        size={layout === "mobile" ? "default" : "sm"}
        onClick={() => handleDeploy(site.id)}
        data-testid={`cdn-deploy-${site.id}`}
      >
        {t("admin.cdn.deploy")}
      </Button>
      <Button
        variant="ghost"
        size={layout === "mobile" ? "default" : "sm"}
        className={
          layout === "mobile"
            ? "col-span-2 text-destructive hover:text-destructive"
            : "text-destructive hover:text-destructive"
        }
        onClick={() => deleteMutation.mutate(site.id)}
        data-testid={`cdn-delete-${site.id}`}
        aria-label={t("admin.cdn.deleteSite")}
      >
        <Trash2 className={layout === "mobile" ? "mr-2 h-4 w-4" : "h-4 w-4"} />
        {layout === "mobile" ? t("admin.cdn.deleteSite") : null}
      </Button>
    </div>
  );

  return (
    <AdminPageShell title={t("admin.cdn.title")} data-testid="cdn-site-list-page">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle data-testid="cdn-list-title">{t("admin.cdn.title")}</CardTitle>
            <CardDescription>{t("admin.cdn.description")}</CardDescription>
          </div>
          <Button onClick={handleAdd} data-testid="cdn-add-site-button">
            <Plus className="mr-2 h-4 w-4" />
            {t("admin.cdn.addSite")}
          </Button>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="flex items-center justify-center py-8">
              <RefreshCw className="h-5 w-5 animate-spin text-muted-foreground" />
            </div>
          ) : sites.length === 0 ? (
            <div className="flex flex-col items-center gap-2 py-8 text-muted-foreground">
              <Globe className="h-10 w-10" />
              <p>{t("admin.cdn.empty")}</p>
            </div>
          ) : (
            <>
              <div className="hidden md:block">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("admin.cdn.name")}</TableHead>
                      <TableHead>{t("admin.cdn.domain")}</TableHead>
                      <TableHead>{t("admin.cdn.originType")}</TableHead>
                      <TableHead>{t("admin.cdn.provider")}</TableHead>
                      <TableHead>{t("admin.cdn.deployStatus")}</TableHead>
                      <TableHead>{t("admin.cdn.enabled")}</TableHead>
                      <TableHead className="text-right">{t("common.actions")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {sites.map((site) => (
                      <TableRow key={site.id} data-testid={`cdn-site-row-${site.id}`}>
                        <TableCell className="font-medium">{site.name || "-"}</TableCell>
                        <TableCell>{site.domain}</TableCell>
                        <TableCell>{site.origin_type || "-"}</TableCell>
                        <TableCell>{site.provider || "-"}</TableCell>
                        <TableCell>{renderStatusBadge(site)}</TableCell>
                        <TableCell>{renderEnabledBadge(site)}</TableCell>
                        <TableCell className="text-right">{renderSiteActions(site, "desktop")}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>

              <ResponsiveList label={t("admin.cdn.mobileListLabel")}>
                {sites.map((site) => (
                  <ResponsiveListItem key={site.id}>
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0 space-y-1">
                        <div className="truncate font-medium text-foreground">{site.name || site.domain}</div>
                        <div className="break-all text-sm text-muted-foreground">{site.domain}</div>
                      </div>
                      {renderStatusBadge(site)}
                    </div>

                    <dl className="mt-4 grid grid-cols-2 gap-3">
                      <ResponsiveListField label={t("admin.cdn.domain")} className="col-span-2">
                        <span className="break-all">{site.domain}</span>
                      </ResponsiveListField>
                      <ResponsiveListField label={t("admin.cdn.originType")}>
                        {site.origin_type || "-"}
                      </ResponsiveListField>
                      <ResponsiveListField label={t("admin.cdn.deployStatus")}>
                        {renderStatusBadge(site)}
                      </ResponsiveListField>
                      <ResponsiveListField label={t("admin.cdn.provider")}>
                        {site.provider || "-"}
                      </ResponsiveListField>
                      <ResponsiveListField label={t("admin.cdn.enabled")}>
                        {renderEnabledBadge(site)}
                      </ResponsiveListField>
                    </dl>

                    {renderSiteActions(site, "mobile")}
                  </ResponsiveListItem>
                ))}
              </ResponsiveList>
            </>
          )}
        </CardContent>
      </Card>

      <CDNSiteForm
        open={formOpen}
        onOpenChange={setFormOpen}
        editingSite={editingSite}
        onSubmit={handleSubmit}
        isPending={createMutation.isPending || updateMutation.isPending}
      />
    </AdminPageShell>
  );
}
