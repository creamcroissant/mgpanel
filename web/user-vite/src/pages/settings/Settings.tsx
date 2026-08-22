import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { CalendarDays, Eye, EyeOff, Key, RefreshCw, ShieldCheck, SlidersHorizontal, UserRound } from "lucide-react";
import { changePassword, fetchUserInfo, resetSubscribeToken } from "@/api/user";
import {
  Badge,
  Button,
  CopyField,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  ErrorBanner,
  Input,
  KeyValueGrid,
  Loading,
  PageShell,
  ResourceCard,
  ResponsiveGrid,
  SectionCard,
  type KeyValueItem,
} from "@/components/ui";
import { QUERY_KEYS } from "@/lib/constants";
import { formatBytes, formatDate } from "@/lib/format";

function maskToken(token?: string): string {
  if (!token) return "-";
  if (token.length <= 8) return "••••";
  return `•••• ${token.slice(-6)}`;
}

export default function Settings() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPasswords, setShowPasswords] = useState(false);
  const [isDialogOpen, setIsDialogOpen] = useState(false);

  const {
    data: user,
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: QUERY_KEYS.USER_INFO,
    queryFn: fetchUserInfo,
  });

  const changePasswordMutation = useMutation({
    mutationFn: () => changePassword(currentPassword, newPassword),
    onSuccess: () => {
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
      toast.success(t("common.success"), {
        description: t("settings.passwordChanged"),
      });
    },
    onError: (error) => {
      toast.error(t("common.error"), {
        description: error.message,
      });
    },
  });

  const resetTokenMutation = useMutation({
    mutationFn: resetSubscribeToken,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.USER_INFO });
      setIsDialogOpen(false);
      toast.success(t("common.success"), {
        description: t("settings.tokenReset"),
      });
    },
    onError: (error) => {
      toast.error(t("common.error"), {
        description: error.message,
      });
    },
  });

  const handleChangePassword = (e: React.FormEvent) => {
    e.preventDefault();
    if (newPassword !== confirmPassword) return;
    changePasswordMutation.mutate();
  };

  const passwordMismatch = confirmPassword && newPassword !== confirmPassword;

  if (isLoading) return <Loading />;
  if (error || !user) return <ErrorBanner message={t("error.loadProfile")} onRetry={() => refetch()} />;

  const transferEnable = user.transfer_enable ?? 0;
  const transferUsed = Math.max(user.transfer_used ?? user.u + user.d, 0);
  const remainingTraffic = transferEnable > 0 ? Math.max(transferEnable - transferUsed, 0) : 0;
  const statusText = user.banned
    ? t("settings.statusBanned")
    : user.status === 1
      ? t("settings.statusActive")
      : t("settings.statusInactive");
  const accountItems: KeyValueItem[] = [
    {
      label: t("settings.email"),
      value: user.email,
      hint: t("settings.emailHint"),
    },
    {
      label: t("settings.username"),
      value: user.username || "-",
      hint: t("settings.usernameHint"),
    },
    {
      label: t("settings.accountId"),
      value: `#${user.id}`,
      hint: t("settings.accountIdHint"),
    },
    {
      label: t("settings.accountStatus"),
      value: statusText,
      hint: t("settings.accountStatusHint"),
    },
    {
      label: t("dashboard.currentPlan"),
      value: user.plan?.name ?? t("dashboard.noPlan"),
      hint: t("settings.planHint"),
    },
    {
      label: t("dashboard.remainingTraffic"),
      value: transferEnable > 0 ? formatBytes(remainingTraffic) : t("plans.unlimited"),
      hint: t("settings.remainingTrafficHint"),
    },
    {
      label: t("plans.expiresAt"),
      value: user.expired_at ? formatDate(user.expired_at) : t("dashboard.never"),
      hint: t("settings.expiresAtHint"),
    },
    {
      label: t("settings.createdAt"),
      value: user.created_at ? formatDate(user.created_at) : "-",
      hint: t("settings.createdAtHint"),
    },
  ];

  return (
    <PageShell
      data-testid="account-center"
      title={t("settings.title")}
      description={t("settings.subtitle")}
      actions={
        <Button variant="outline" className="gap-2" onClick={() => refetch()}>
          <RefreshCw className="h-4 w-4" />
          {t("common.refresh")}
        </Button>
      }
    >
      <SectionCard
        title={t("settings.accountOverview")}
        description={t("settings.accountOverviewHint")}
        actions={<Badge variant={user.banned ? "destructive" : "secondary"}>{statusText}</Badge>}
      >
        <KeyValueGrid items={accountItems} className="lg:grid-cols-4" />
      </SectionCard>

      <SectionCard title={t("settings.changePassword")} description={t("settings.changePasswordHint")}>
        <form onSubmit={handleChangePassword} className="max-w-xl space-y-4">
          <div className="space-y-2">
            <label htmlFor="settings-current-password" className="text-sm font-medium">
              {t("settings.currentPassword")}
            </label>
            <div className="relative">
              <Input
                id="settings-current-password"
                type={showPasswords ? "text" : "password"}
                value={currentPassword}
                onChange={(event) => setCurrentPassword(event.target.value)}
                required
                autoComplete="current-password"
                className="h-11 pr-11"
              />
              <button
                type="button"
                onClick={() => setShowPasswords((value) => !value)}
                className="absolute right-2 top-1/2 flex h-8 w-8 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                aria-label={showPasswords ? t("settings.hidePasswords") : t("settings.showPasswords")}
              >
                {showPasswords ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </button>
            </div>
          </div>

          <div className="space-y-2">
            <label htmlFor="settings-new-password" className="text-sm font-medium">
              {t("settings.newPassword")}
            </label>
            <Input
              id="settings-new-password"
              type={showPasswords ? "text" : "password"}
              value={newPassword}
              onChange={(event) => setNewPassword(event.target.value)}
              required
              autoComplete="new-password"
              className="h-11"
            />
          </div>

          <div className="space-y-2">
            <label htmlFor="settings-confirm-password" className="text-sm font-medium">
              {t("settings.confirmPassword")}
            </label>
            <Input
              id="settings-confirm-password"
              type={showPasswords ? "text" : "password"}
              value={confirmPassword}
              onChange={(event) => setConfirmPassword(event.target.value)}
              required
              autoComplete="new-password"
              aria-invalid={!!passwordMismatch}
              aria-describedby={passwordMismatch ? "settings-password-mismatch" : undefined}
              className="h-11"
            />
            {passwordMismatch && (
              <p id="settings-password-mismatch" role="alert" className="text-sm text-destructive">
                {t("settings.passwordMismatch")}
              </p>
            )}
          </div>

          {changePasswordMutation.error && (
            <div role="alert" className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
              {changePasswordMutation.error.message}
            </div>
          )}

          <Button type="submit" disabled={!!passwordMismatch || changePasswordMutation.isPending}>
            {changePasswordMutation.isPending ? t("common.loading") : t("common.save")}
          </Button>
        </form>
      </SectionCard>

      <SectionCard
        title={t("settings.subscriptionSecurity")}
        description={t("settings.subscriptionSecurityHint")}
        actions={
          <Button variant="outline" className="gap-2" onClick={() => setIsDialogOpen(true)}>
            <RefreshCw className="h-4 w-4" />
            {t("settings.resetToken")}
          </Button>
        }
      >
        <div className="space-y-4">
          <ResponsiveGrid minColWidth={220} gap={12}>
            <ResourceCard
              icon={<ShieldCheck className="h-5 w-5" />}
              title={t("settings.tokenStatus")}
              description={t("settings.tokenProtected")}
              meta={<Badge variant="secondary">{maskToken(user.token)}</Badge>}
            />
            <ResourceCard
              icon={<Key className="h-5 w-5" />}
              title={t("settings.resetToken")}
              description={t("settings.resetTokenDescription")}
              meta={<Badge variant="outline">{t("settings.requiresClientRefresh")}</Badge>}
            />
          </ResponsiveGrid>
          <CopyField
            label={t("dashboard.subscribeUrl")}
            value={user.subscribe_url}
            emptyLabel={t("dashboard.noSubscription")}
            copyLabel={t("common.copy")}
            copiedLabel={t("common.copied")}
            helperText={t("settings.subscribeUrlHint")}
          />
        </div>
      </SectionCard>

      <SectionCard title={t("settings.preferences")} description={t("settings.preferencesHint")}>
        <ResponsiveGrid minColWidth={220} gap={12}>
          <ResourceCard
            icon={<SlidersHorizontal className="h-5 w-5" />}
            title={t("settings.portalPreferences")}
            description={t("settings.portalPreferencesHint")}
            status={<Badge variant="outline">{t("plans.readonly")}</Badge>}
          />
          <ResourceCard
            icon={<CalendarDays className="h-5 w-5" />}
            title={t("settings.renewalReminder")}
            description={t("settings.renewalReminderHint")}
            status={<Badge variant="secondary">{user.expired_at ? formatDate(user.expired_at) : t("dashboard.never")}</Badge>}
          />
          <ResourceCard
            icon={<UserRound className="h-5 w-5" />}
            title={t("settings.supportProfile")}
            description={t("settings.supportProfileHint")}
            status={<Badge variant="outline">{user.email}</Badge>}
          />
        </ResponsiveGrid>
      </SectionCard>

      <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("settings.resetConfirmTitle")}</DialogTitle>
            <DialogDescription>{t("settings.resetConfirmDescription")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setIsDialogOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button
              type="button"
              variant="destructive"
              disabled={resetTokenMutation.isPending}
              onClick={() => resetTokenMutation.mutate()}
            >
              {resetTokenMutation.isPending ? t("common.loading") : t("common.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </PageShell>
  );
}
