import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { Mail, User, Lock, Eye, EyeOff } from "lucide-react";
import { Card, CardContent, CardHeader } from "@/components/ui";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { createAdmin, getInstallStatus } from "@/api/install";
import { ROUTES } from "@/lib/constants";
import ThemeToggle from "@/components/ThemeToggle";
import LanguageSwitcher from "@/components/LanguageSwitcher";

export default function Install() {
  const { t } = useTranslation();
  const navigate = useNavigate();

  const [email, setEmail] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);

  const createAdminMutation = useMutation({
    mutationFn: () =>
      createAdmin({
        email: email.trim() || undefined,
        username: username.trim() || undefined,
        password,
      }),
    onSuccess: () => {
      toast.success(t("common.success"), {
        description: t("install.success"),
      });
      // Redirect to login after 2 seconds
      setTimeout(() => {
        navigate(ROUTES.LOGIN);
      }, 2000);
    },
    onError: (error: Error) => {
      toast.error(t("common.error"), {
        description: error.message,
      });
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    // Validate at least one identifier
    if (!email.trim() && !username.trim()) {
      toast.warning(t("common.error"), {
        description: t("install.identifierRequired"),
      });
      return;
    }

    // Validate password
    if (!password) {
      toast.warning(t("common.error"), {
        description: t("install.passwordRequired"),
      });
      return;
    }

    if (password.length < 8) {
      toast.warning(t("common.error"), {
        description: t("install.passwordTooShort"),
      });
      return;
    }

    // Validate password match
    if (password !== confirmPassword) {
      toast.warning(t("common.error"), {
        description: t("install.passwordMismatch"),
      });
      return;
    }

    createAdminMutation.mutate();
  };

  const passwordMismatch = Boolean(confirmPassword) && password !== confirmPassword;

  useEffect(() => {
    let isMounted = true;

    const checkStatus = async () => {
      try {
        const status = await getInstallStatus();
        if (isMounted && status && !status.needs_bootstrap) {
          navigate(ROUTES.LOGIN, { replace: true });
        }
      } catch {
        // If status check fails, stay on install page and allow manual retry.
      }
    };

    void checkStatus();

    return () => {
      isMounted = false;
    };
  }, [navigate]);

  return (
    <div className="relative flex min-h-screen items-center justify-center bg-background px-4 py-10">
      <div className="pointer-events-none absolute -left-10 -top-10 h-40 w-40 rounded-full bg-primary/10 blur-2xl" />
      <div className="pointer-events-none absolute bottom-0 right-0 h-48 w-48 rounded-full bg-primary/5 blur-3xl" />

      <div className="absolute right-4 top-4 flex items-center gap-2">
        <LanguageSwitcher />
        <ThemeToggle />
      </div>

      <Card className="w-full max-w-md border border-border/80 shadow-sm">
        <CardHeader className="items-center space-y-2 pt-8 text-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-md bg-primary/10 text-primary font-semibold">
            X
          </div>
          <div className="space-y-1">
            <h1 className="text-2xl font-semibold tracking-tight">
              {t("install.title")}
            </h1>
            <p className="text-sm text-muted-foreground">{t("install.subtitle")}</p>
          </div>
        </CardHeader>

        <CardContent className="px-6 pb-8 pt-4">
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <label
                htmlFor="install-email"
                className="text-sm font-medium text-foreground"
              >
                {t("install.email")}
              </label>
              <div className="relative">
                <Mail className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="install-email"
                  type="email"
                  placeholder={t("install.emailPlaceholder")}
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  autoComplete="email"
                  className="h-11 pl-10"
                />
              </div>
            </div>

            <div className="space-y-2">
              <label
                htmlFor="install-username"
                className="text-sm font-medium text-foreground"
              >
                {t("install.username")}
              </label>
              <div className="relative">
                <User className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="install-username"
                  type="text"
                  placeholder={t("install.usernamePlaceholder")}
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                  autoComplete="username"
                  className="h-11 pl-10"
                />
              </div>
            </div>

            <div className="-mt-2 space-y-1 text-xs leading-5 text-muted-foreground">
              <p>{t("install.hint")}</p>
              <p>{t("install.agentDeployHint")}</p>
            </div>

            <div className="rounded-md border bg-muted/30 px-3 py-2 text-xs leading-5 text-muted-foreground">
              <p className="font-medium text-foreground">{t("install.agentDiagnosticsTitle")}</p>
              <p className="mt-1">{t("install.agentDiagnosticsHint")}</p>
            </div>

            <div className="space-y-2">
              <label
                htmlFor="install-password"
                className="text-sm font-medium text-foreground"
              >
                {t("install.password")}
              </label>
              <div className="relative">
                <Lock className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="install-password"
                  type={showPassword ? "text" : "password"}
                  placeholder={t("install.passwordPlaceholder")}
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  required
                  autoComplete="new-password"
                  className="h-11 pl-10 pr-10"
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="absolute right-1 top-1/2 h-8 w-8 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                  onClick={() => setShowPassword((current) => !current)}
                >
                  {showPassword ? <EyeOff size={18} /> : <Eye size={18} />}
                </Button>
              </div>
            </div>

            <div className="space-y-2">
              <label
                htmlFor="install-confirm-password"
                className="text-sm font-medium text-foreground"
              >
                {t("install.confirmPassword")}
              </label>
              <div className="relative">
                <Lock className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="install-confirm-password"
                  type={showPassword ? "text" : "password"}
                  placeholder={t("install.confirmPasswordPlaceholder")}
                  value={confirmPassword}
                  onChange={(event) => setConfirmPassword(event.target.value)}
                  required
                  autoComplete="new-password"
                  className={`h-11 pl-10${
                    passwordMismatch
                      ? " border-destructive focus-visible:ring-destructive"
                      : ""
                  }`}
                />
              </div>
              {passwordMismatch && (
                <p className="text-xs text-destructive">
                  {t("install.passwordMismatch")}
                </p>
              )}
            </div>

            {createAdminMutation.error && (
              <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                {createAdminMutation.error.message}
              </div>
            )}

            <Button
              type="submit"
              className="h-11 w-full"
              disabled={createAdminMutation.isPending || passwordMismatch}
            >
              {createAdminMutation.isPending
                ? `${t("install.submit")}...`
                : t("install.submit")}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
