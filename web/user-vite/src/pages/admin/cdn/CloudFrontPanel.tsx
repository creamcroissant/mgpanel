import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Eye, EyeOff, Globe, Settings } from "lucide-react";
import { toast } from "sonner";
import {
  fetchCloudFrontDistributions,
  getCloudFrontCredentials,
  setCloudFrontCredentials,
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

const FULLY_MASKED_VALUE = "••••••••••••••••";

export default function CloudFrontPanel() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [credentialOpen, setCredentialOpen] = useState(false);

  // credential form
  const [accessKeyId, setAccessKeyId] = useState("");
  const [secretAccessKey, setSecretAccessKey] = useState("");
  const [region, setRegion] = useState("");
  const [showSecret, setShowSecret] = useState(false);

  const zonesQueryKey = [...QUERY_KEYS.ADMIN_CDN_SITES, "cloudfront"];
  const credQueryKey = [...QUERY_KEYS.ADMIN_CDN_SITES, "cloudfront", "credentials"];

  const { data: distributions, isLoading: distLoading, error: distError, refetch: refetchDist } = useQuery({
    queryKey: zonesQueryKey,
    queryFn: fetchCloudFrontDistributions,
  });

  const { data: credentials, isLoading: credLoading, error: credError } = useQuery({
    queryKey: credQueryKey,
    queryFn: getCloudFrontCredentials,
  });

  const setCredMutation = useMutation({
    mutationFn: () =>
      setCloudFrontCredentials({
        access_key_id: accessKeyId,
        secret_access_key: secretAccessKey,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: credQueryKey });
      setCredentialOpen(false);
      resetCredForm();
      toast.success(t("common.success"));
    },
    onError: (err: Error) => {
      toast.error(t("common.error"), { description: err.message });
    },
  });

  const hasCredentials = credentials !== null && credentials !== undefined;
  const maskedSecretDisplay = hasCredentials
    ? credentials.access_key_id.slice(0, 4) + FULLY_MASKED_VALUE.slice(4)
    : "";

  function resetCredForm() {
    setAccessKeyId("");
    setSecretAccessKey("");
    setRegion("");
    setShowSecret(false);
  }

  function handleSaveCredentials() {
    if (!accessKeyId.trim() || !secretAccessKey.trim()) {
      toast.warning(t("common.error"), {
        description: t("admin.cdn.cloudfront.credentialRequired"),
      });
      return;
    }
    setCredMutation.mutate();
  }

  function getDistStatusVariant(status: string) {
    switch (status) {
      case "Deployed":
        return "success";
      case "InProgress":
        return "warning";
      case "Failed":
        return "danger";
      default:
        return "secondary";
    }
  }

  if (distLoading || credLoading) return <Loading />;
  if (distError || credError) {
    return (
      <ErrorBanner
        message={t("common.error")}
        onRetry={() => { refetchDist(); }}
      />
    );
  }

  return (
    <>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle>CloudFront</CardTitle>
            <CardDescription>{t("admin.cdn.cloudfront.description")}</CardDescription>
          </div>
          <Button onClick={() => setCredentialOpen(true)} data-testid="cdn-cloudfront-credential-button">
            <Settings className="mr-2 h-4 w-4" />
            {t("admin.cdn.cloudfront.configureCredentials")}
          </Button>
        </CardHeader>
        <CardContent className="space-y-6">
          {/* Credential status */}
          <div className="flex items-center gap-2 rounded-md border px-4 py-3">
            <span className="text-sm font-medium">{t("admin.cdn.cloudfront.credentialStatus")}:</span>
            {hasCredentials ? (
              <>
                <Badge variant="success" data-testid="cdn-cloudfront-credential-status-set">
                  {t("admin.cdn.cloudfront.credentialSet")}
                </Badge>
                <span className="ml-2 font-mono text-xs text-muted-foreground" data-testid="cdn-cloudfront-credential-key-preview">
                  {maskedSecretDisplay}
                </span>
              </>
            ) : (
              <Badge variant="secondary" data-testid="cdn-cloudfront-credential-status-unset">
                {t("admin.cdn.cloudfront.credentialNotSet")}
              </Badge>
            )}
          </div>

          {/* Distribution list */}
          <div>
            <h3 className="mb-3 text-sm font-semibold">{t("admin.cdn.cloudfront.distributions")}</h3>
            {!distributions || distributions.length === 0 ? (
              <EmptyState
                icon={<Globe className="h-10 w-10" />}
                title={t("admin.cdn.cloudfront.noDistributions")}
                description={t("admin.cdn.cloudfront.noDistributionsDescription")}
                size="sm"
              />
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("admin.cdn.cloudfront.distributionId")}</TableHead>
                    <TableHead>{t("admin.cdn.cloudfront.domain")}</TableHead>
                    <TableHead>{t("admin.cdn.cloudfront.originDomain")}</TableHead>
                    <TableHead>{t("admin.cdn.cloudfront.status")}</TableHead>
                    <TableHead>{t("admin.cdn.cloudfront.enabled")}</TableHead>
                    <TableHead>{t("admin.cdn.cloudfront.createdAt")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {distributions.map((dist) => (
                    <TableRow key={dist.id} data-testid={`cdn-cloudfront-dist-row-${dist.id}`}>
                      <TableCell className="font-mono text-xs font-medium">{dist.distribution_id}</TableCell>
                      <TableCell className="font-mono text-xs">{dist.domain}</TableCell>
                      <TableCell className="font-mono text-xs">{dist.origin_domain}</TableCell>
                      <TableCell>
                        <Badge variant={getDistStatusVariant(dist.status)}>
                          {dist.status}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        {dist.enabled ? (
                          <Badge variant="success">{t("common.yes")}</Badge>
                        ) : (
                          <Badge variant="secondary">{t("common.no")}</Badge>
                        )}
                      </TableCell>
                      <TableCell>{formatDateTime(dist.created_at)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Credential Settings Dialog */}
      <Dialog
        open={credentialOpen}
        onOpenChange={(open) => { setCredentialOpen(open); if (!open) resetCredForm(); }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("admin.cdn.cloudfront.configureCredentials")}</DialogTitle>
            <DialogDescription>{t("admin.cdn.cloudfront.credentialDescription")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">Access Key ID</label>
              <Input
                value={accessKeyId}
                onChange={(e) => setAccessKeyId(e.target.value)}
                placeholder="AKIAIOSFODNN7EXAMPLE"
                data-testid="cdn-cloudfront-credential-access-key"
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Secret Access Key</label>
              <div className="relative">
                <Input
                  type={showSecret ? "text" : "password"}
                  value={secretAccessKey}
                  onChange={(e) => setSecretAccessKey(e.target.value)}
                  placeholder={t("admin.cdn.cloudfront.secretKeyPlaceholder")}
                  className="pr-11"
                  data-testid="cdn-cloudfront-credential-secret-key"
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="absolute right-0 top-0 h-10 w-10 text-muted-foreground hover:text-foreground"
                  onClick={() => setShowSecret((prev) => !prev)}
                  data-testid="cdn-cloudfront-toggle-secret-visibility"
                >
                  {showSecret ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </Button>
              </div>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t("admin.cdn.cloudfront.region")}</label>
              <Input
                value={region}
                onChange={(e) => setRegion(e.target.value)}
                placeholder="us-east-1"
                data-testid="cdn-cloudfront-credential-region"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setCredentialOpen(false); resetCredForm(); }}>
              {t("common.cancel")}
            </Button>
            <Button
              onClick={handleSaveCredentials}
              disabled={setCredMutation.isPending}
              data-testid="cdn-cloudfront-credential-save"
            >
              {setCredMutation.isPending ? t("common.loading") : t("common.save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
