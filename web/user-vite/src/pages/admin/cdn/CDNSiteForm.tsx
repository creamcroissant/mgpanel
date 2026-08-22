import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import type { CDNSite, CreateCDNSiteRequest } from "@/api/admin/cdn";
import {
  Button,
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
  Switch,
} from "@/components/ui";

const originTypeOptions = ["ip", "domain", "s3"] as const;
const sslModeOptions = ["off", "flexible", "full", "full_strict"] as const;

const defaultFormData: CreateCDNSiteRequest = {
  name: "",
  domain: "",
  origin_type: "domain",
  origin_url: "",
  cache_ttl: 3600,
  ssl_mode: "full",
  enabled: true,
};

interface CDNSiteFormProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editingSite: CDNSite | null;
  onSubmit: (data: CreateCDNSiteRequest) => void;
  isPending: boolean;
}

export default function CDNSiteForm({
  open,
  onOpenChange,
  editingSite,
  onSubmit,
  isPending,
}: CDNSiteFormProps) {
  const { t } = useTranslation();
  const [formData, setFormData] = useState<CreateCDNSiteRequest>(defaultFormData);

  useEffect(() => {
    if (open) {
      if (editingSite) {
        setFormData({
          name: editingSite.name,
          domain: editingSite.domain,
          origin_type: editingSite.origin_type,
          origin_url: editingSite.origin_url,
          cache_ttl: editingSite.cache_ttl,
          ssl_mode: editingSite.ssl_mode,
          enabled: editingSite.enabled,
        });
      } else {
        setFormData(defaultFormData);
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, editingSite?.id]);

  const handleDialogChange = (open: boolean) => {
    if (!open) {
      setFormData(defaultFormData);
    }
    onOpenChange(open);
  };

  const handleSubmit = () => {
    if (!formData.domain || !formData.origin_url) {
      return;
    }
    onSubmit(formData);
  };

  return (
    <Dialog open={open} onOpenChange={handleDialogChange}>
      <DialogContent className="sm:max-w-lg" data-testid="cdn-site-form-dialog">
        <DialogHeader>
          <DialogTitle>
            {editingSite ? t("admin.cdn.editTitle") : t("admin.cdn.addTitle")}
          </DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <label className="text-sm font-medium" data-testid="cdn-form-name-label">
              {t("admin.cdn.name")}
            </label>
            <Input
              data-testid="cdn-form-name"
              placeholder={t("admin.cdn.namePlaceholder")}
              value={formData.name}
              onChange={(event) => setFormData({ ...formData, name: event.target.value })}
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium" data-testid="cdn-form-domain-label">
              {t("admin.cdn.domain")} <span className="text-destructive">*</span>
            </label>
            <Input
              data-testid="cdn-form-domain"
              placeholder={t("admin.cdn.domainPlaceholder")}
              value={formData.domain}
              onChange={(event) => setFormData({ ...formData, domain: event.target.value })}
              required
            />
          </div>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <label className="text-sm font-medium" data-testid="cdn-form-origin-type-label">
                {t("admin.cdn.originType")}
              </label>
              <Select
                value={formData.origin_type}
                onValueChange={(value) => setFormData({ ...formData, origin_type: value })}
              >
                <SelectTrigger data-testid="cdn-form-origin-type">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {originTypeOptions.map((option) => (
                    <SelectItem key={option} value={option}>
                      {t(`admin.cdn.originType${option.charAt(0).toUpperCase() + option.slice(1)}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium" data-testid="cdn-form-origin-url-label">
                {t("admin.cdn.originUrl")} <span className="text-destructive">*</span>
              </label>
              <Input
                data-testid="cdn-form-origin-url"
                placeholder={t("admin.cdn.originUrlPlaceholder")}
                value={formData.origin_url}
                onChange={(event) => setFormData({ ...formData, origin_url: event.target.value })}
                required
              />
            </div>
          </div>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <label className="text-sm font-medium" data-testid="cdn-form-cache-ttl-label">
                {t("admin.cdn.cacheTtl")}
              </label>
              <Input
                data-testid="cdn-form-cache-ttl"
                type="number"
                placeholder="3600"
                value={formData.cache_ttl ? String(formData.cache_ttl) : ""}
                onChange={(event) =>
                  setFormData({
                    ...formData,
                    cache_ttl: parseInt(event.target.value, 10) || 0,
                  })
                }
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium" data-testid="cdn-form-ssl-mode-label">
                {t("admin.cdn.sslMode")}
              </label>
              <Select
                value={formData.ssl_mode}
                onValueChange={(value) => setFormData({ ...formData, ssl_mode: value })}
              >
                <SelectTrigger data-testid="cdn-form-ssl-mode">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {sslModeOptions.map((option) => (
                    <SelectItem key={option} value={option}>
                      {t(`admin.cdn.sslMode${option
                        .split("_")
                        .map((s) => s.charAt(0).toUpperCase() + s.slice(1))
                        .join("")}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <label className="flex items-center gap-2 text-sm" data-testid="cdn-form-enabled-label">
            <Switch
              data-testid="cdn-form-enabled"
              checked={formData.enabled}
              onCheckedChange={(value) => setFormData({ ...formData, enabled: value })}
            />
            {t("admin.cdn.enabled")}
          </label>
        </div>
        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => handleDialogChange(false)}
            data-testid="cdn-form-cancel"
          >
            {t("common.cancel")}
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={isPending || !formData.domain || !formData.origin_url}
            data-testid="cdn-form-submit"
          >
            {isPending
              ? t("common.loading")
              : editingSite
              ? t("common.save")
              : t("common.create")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
