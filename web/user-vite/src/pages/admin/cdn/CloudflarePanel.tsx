import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Eye, EyeOff, Globe, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";
import {
  addCloudflareZone,
  fetchCloudflareZones,
  fetchDNSRecords,
  removeCloudflareZone,
} from "@/api/admin/cdn";
import { QUERY_KEYS } from "@/lib/constants";
import { formatDateTime } from "@/lib/format";
import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Loading,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui";
import EmptyState from "@/components/ui/EmptyState";
import ErrorBanner from "@/components/ui/ErrorBanner";

export default function CloudflarePanel() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [addOpen, setAddOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<number | null>(null);
  const [expandedZone, setExpandedZone] = useState<number | null>(null);
  const [showToken, setShowToken] = useState(false);

  // add zone form
  const [zoneName, setZoneName] = useState("");
  const [zoneId, setZoneId] = useState("");
  const [apiToken, setApiToken] = useState("");

  const queryKey = [...QUERY_KEYS.ADMIN_CDN_SITES, "cloudflare", "zones"];

  const { data: zones, isLoading, error, refetch } = useQuery({
    queryKey,
    queryFn: fetchCloudflareZones,
  });

  const addMutation = useMutation({
    mutationFn: () =>
      addCloudflareZone({ name: zoneName, zone_id: zoneId }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
      setAddOpen(false);
      resetForm();
      toast.success(t("common.success"));
    },
    onError: (err: Error) => {
      toast.error(t("common.error"), { description: err.message });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => removeCloudflareZone(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
      setDeleteTarget(null);
      toast.success(t("common.success"));
    },
    onError: (err: Error) => {
      toast.error(t("common.error"), { description: err.message });
    },
  });

  const dnsQueryKey = [...QUERY_KEYS.ADMIN_CDN_SITES, "cloudflare", "dns", expandedZone].filter(
    (x) => x !== null,
  );

  const { data: dnsRecords, isLoading: dnsLoading } = useQuery({
    queryKey: dnsQueryKey,
    queryFn: () => fetchDNSRecords(expandedZone!),
    enabled: expandedZone !== null,
  });

  function resetForm() {
    setZoneName("");
    setZoneId("");
    setApiToken("");
    setShowToken(false);
  }

  function handleAdd() {
    if (!zoneName.trim() || !zoneId.trim()) {
      toast.warning(t("common.error"), { description: "Zone name and Zone ID are required" });
      return;
    }
    addMutation.mutate();
  }

  function handleDelete(id: number) {
    deleteMutation.mutate(id);
  }

  function toggleExpand(zoneIdNum: number) {
    setExpandedZone((prev) => (prev === zoneIdNum ? null : zoneIdNum));
  }

  function getStatusBadgeVariant(status: string) {
    switch (status) {
      case "active":
        return "success";
      case "pending":
        return "warning";
      case "deactivated":
        return "secondary";
      default:
        return "default";
    }
  }

  const dnsTypeColors: Record<string, string> = {
    A: "text-blue-600 dark:text-blue-400",
    AAAA: "text-indigo-600 dark:text-indigo-400",
    CNAME: "text-green-600 dark:text-green-400",
    MX: "text-orange-600 dark:text-orange-400",
    TXT: "text-purple-600 dark:text-purple-400",
    NS: "text-rose-600 dark:text-rose-400",
    CAA: "text-teal-600 dark:text-teal-400",
    SRV: "text-cyan-600 dark:text-cyan-400",
  };

  if (isLoading) return <Loading />;
  if (error) {
    return (
      <ErrorBanner
        message={t("common.error")}
        onRetry={() => refetch()}
      />
    );
  }

  return (
    <>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle>Cloudflare</CardTitle>
            <CardDescription>{t("admin.cdn.cloudflare.description")}</CardDescription>
          </div>
          <Button onClick={() => setAddOpen(true)} data-testid="cdn-cloudflare-add-zone-button">
            <Plus className="mr-2 h-4 w-4" />
            {t("admin.cdn.cloudflare.addZone")}
          </Button>
        </CardHeader>
        <CardContent>
          {!zones || zones.length === 0 ? (
            <EmptyState
              icon={<Globe className="h-10 w-10" />}
              title={t("admin.cdn.cloudflare.noZones")}
              description={t("admin.cdn.cloudflare.noZonesDescription")}
              size="sm"
            />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("admin.cdn.cloudflare.zoneName")}</TableHead>
                  <TableHead>Zone ID</TableHead>
                  <TableHead>{t("admin.cdn.cloudflare.status")}</TableHead>
                  <TableHead>{t("admin.cdn.cloudflare.plan")}</TableHead>
                  <TableHead>{t("admin.cdn.cloudflare.createdAt")}</TableHead>
                  <TableHead className="w-24">{t("common.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {zones.map((zone) => (
                  <>
                    <TableRow
                      key={zone.id}
                      className="cursor-pointer"
                      onClick={() => toggleExpand(zone.id)}
                      data-testid={`cdn-cloudflare-zone-row-${zone.id}`}
                    >
                      <TableCell className="font-medium">{zone.zone_name}</TableCell>
                      <TableCell className="font-mono text-xs">{zone.zone_id}</TableCell>
                      <TableCell>
                        <Badge variant={getStatusBadgeVariant(zone.status)}>
                          {zone.status}
                        </Badge>
                      </TableCell>
                      <TableCell>{zone.plan}</TableCell>
                      <TableCell>{formatDateTime(zone.created_at)}</TableCell>
                      <TableCell>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={(e) => {
                            e.stopPropagation();
                            setDeleteTarget(zone.id);
                          }}
                          data-testid={`cdn-cloudflare-delete-zone-${zone.id}`}
                        >
                          <Trash2 className="h-4 w-4 text-destructive" />
                        </Button>
                      </TableCell>
                    </TableRow>
                    {expandedZone === zone.id && (
                      <TableRow key={`${zone.id}-dns`} data-testid={`cdn-cloudflare-dns-section-${zone.id}`}>
                        <TableCell colSpan={6} className="bg-muted/20 p-4">
                          <div className="space-y-2">
                            <h4 className="text-sm font-semibold">
                              {t("admin.cdn.cloudflare.dnsRecords")}
                            </h4>
                            {dnsLoading ? (
                              <Loading />
                            ) : !dnsRecords || dnsRecords.length === 0 ? (
                              <p className="text-sm text-muted-foreground">
                                {t("admin.cdn.cloudflare.noDNSRecords")}
                              </p>
                            ) : (
                              <div className="overflow-x-auto rounded-md border">
                                <table className="w-full text-xs">
                                  <thead className="bg-muted/50">
                                    <tr>
                                      <th className="px-3 py-2 text-left font-medium text-muted-foreground">
                                        {t("admin.cdn.cloudflare.dnsType")}
                                      </th>
                                      <th className="px-3 py-2 text-left font-medium text-muted-foreground">
                                        {t("admin.cdn.cloudflare.dnsName")}
                                      </th>
                                      <th className="px-3 py-2 text-left font-medium text-muted-foreground">
                                        {t("admin.cdn.cloudflare.dnsContent")}
                                      </th>
                                      <th className="px-3 py-2 text-left font-medium text-muted-foreground">
                                        TTL
                                      </th>
                                      <th className="px-3 py-2 text-left font-medium text-muted-foreground">
                                        {t("admin.cdn.cloudflare.dnsProxied")}
                                      </th>
                                    </tr>
                                  </thead>
                                  <tbody>
                                    {dnsRecords.map((record, idx) => (
                                      <tr
                                        key={record.id ?? idx}
                                        className="border-t border-border/40"
                                        data-testid={`cdn-cloudflare-dns-record-${idx}`}
                                      >
                                        <td className={`px-3 py-2 font-mono font-medium ${dnsTypeColors[record.type] ?? "text-foreground"}`}>
                                          {record.type}
                                        </td>
                                        <td className="px-3 py-2 font-mono">{record.name}</td>
                                        <td className="max-w-[300px] truncate px-3 py-2 font-mono">
                                          {record.content}
                                        </td>
                                        <td className="px-3 py-2">{record.ttl === 1 ? "Auto" : `${record.ttl}s`}</td>
                                        <td className="px-3 py-2">
                                          {record.proxied === null
                                            ? "-"
                                            : record.proxied
                                              ? t("common.yes")
                                              : t("common.no")}
                                        </td>
                                      </tr>
                                    ))}
                                  </tbody>
                                </table>
                              </div>
                            )}
                          </div>
                        </TableCell>
                      </TableRow>
                    )}
                  </>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Add Zone Dialog */}
      <Dialog open={addOpen} onOpenChange={(open) => { setAddOpen(open); if (!open) resetForm(); }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("admin.cdn.cloudflare.addZone")}</DialogTitle>
            <DialogDescription>{t("admin.cdn.cloudflare.addZoneDescription")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.cdn.cloudflare.zoneName")}</label>
              <Input
                value={zoneName}
                onChange={(e) => setZoneName(e.target.value)}
                placeholder="example.com"
                data-testid="cdn-cloudflare-add-zone-name"
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Zone ID</label>
              <Input
                value={zoneId}
                onChange={(e) => setZoneId(e.target.value)}
                placeholder="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
                data-testid="cdn-cloudflare-add-zone-id"
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.cdn.cloudflare.apiToken")}</label>
              <div className="relative">
                <Input
                  type={showToken ? "text" : "password"}
                  value={apiToken}
                  onChange={(e) => setApiToken(e.target.value)}
                  placeholder={t("admin.cdn.cloudflare.apiTokenPlaceholder")}
                  className="pr-11"
                  data-testid="cdn-cloudflare-add-api-token"
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="absolute right-0 top-0 h-10 w-10 text-muted-foreground hover:text-foreground"
                  onClick={() => setShowToken((prev) => !prev)}
                  data-testid="cdn-cloudflare-toggle-token-visibility"
                >
                  {showToken ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </Button>
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setAddOpen(false); resetForm(); }}>
              {t("common.cancel")}
            </Button>
            <Button onClick={handleAdd} disabled={addMutation.isPending} data-testid="cdn-cloudflare-add-zone-confirm">
              {addMutation.isPending ? t("common.loading") : t("common.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Zone Dialog */}
      <Dialog open={deleteTarget !== null} onOpenChange={(open) => { if (!open) setDeleteTarget(null); }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("common.confirm")}</DialogTitle>
            <DialogDescription>{t("admin.cdn.cloudflare.deleteZoneConfirm")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(null)}>
              {t("common.cancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={() => deleteTarget !== null && handleDelete(deleteTarget)}
              disabled={deleteMutation.isPending}
              data-testid="cdn-cloudflare-delete-zone-confirm"
            >
              {deleteMutation.isPending ? t("common.loading") : t("common.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
