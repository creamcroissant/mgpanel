import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { Mail, ShieldCheck, Lock, Eye, EyeOff } from "lucide-react";
import { Card, CardContent, CardFooter, CardHeader } from "@/components/ui";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { sendEmailVerify, forgotPassword } from "@/api/auth";
import { ROUTES } from "@/lib/constants";
import ThemeToggle from "@/components/ThemeToggle";
import LanguageSwitcher from "@/components/LanguageSwitcher";

export default function ForgotPassword() {
  const { t } = useTranslation();
  const navigate = useNavigate();

  const [email, setEmail] = useState("");
  const [emailCode, setEmailCode] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);

  const sendCodeMutation = useMutation({
    mutationFn: () => sendEmailVerify(email.trim()),
    onSuccess: () => {
      toast.success(t("common.success"), {
        description: t("auth.forgotPasswordCodeSent"),
      });
    },
    onError: (error: Error) => {
      toast.error(t("common.error"), {
        description: error.message,
      });
    },
  });

  const resetPasswordMutation = useMutation({
    mutationFn: () => forgotPassword(email.trim(), password, emailCode.trim()),
    onSuccess: () => {
      toast.success(t("common.success"), {
        description: t("auth.forgotPasswordResetSuccess"),
      });
      navigate(ROUTES.LOGIN, { replace: true });
    },
    onError: (error: Error) => {
      toast.error(t("common.error"), {
        description: error.message,
      });
    },
  });

  const passwordMismatch = Boolean(confirmPassword) && password !== confirmPassword;

  const handleSendCode = () => {
    if (!email.trim()) {
      toast.warning(t("common.error"), {
        description: t("auth.forgotPasswordEmailRequired"),
      });
      return;
    }
    sendCodeMutation.mutate();
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    if (!email.trim()) {
      toast.warning(t("common.error"), {
        description: t("auth.forgotPasswordEmailRequired"),
      });
      return;
    }

    if (!emailCode.trim()) {
      toast.warning(t("common.error"), {
        description: t("auth.forgotPasswordCodeRequired"),
      });
      return;
    }

    if (!password) {
      toast.warning(t("common.error"), {
        description: t("auth.forgotPasswordPasswordRequired"),
      });
      return;
    }

    if (password !== confirmPassword) {
      toast.warning(t("common.error"), {
        description: t("install.passwordMismatch"),
      });
      return;
    }

    resetPasswordMutation.mutate();
  };

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
              {t("auth.forgotPassword")}
            </h1>
            <p className="text-sm text-muted-foreground">{t("auth.forgotPasswordSubtitle")}</p>
          </div>
        </CardHeader>

        <CardContent className="px-6 pb-6 pt-4">
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <label htmlFor="forgot-email" className="text-sm font-medium text-foreground">
                {t("auth.email")}
              </label>
              <div className="relative">
                <Mail className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="forgot-email"
                  type="email"
                  placeholder="you@example.com"
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  autoComplete="email"
                  className="h-11 pl-10"
                />
              </div>
            </div>

            <div className="space-y-2">
              <label htmlFor="forgot-email-code" className="text-sm font-medium text-foreground">
                {t("auth.forgotPasswordCode")}
              </label>
              <div className="flex gap-2">
                <div className="relative flex-1">
                  <ShieldCheck className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    id="forgot-email-code"
                    type="text"
                    placeholder={t("auth.forgotPasswordCodePlaceholder")}
                    value={emailCode}
                    onChange={(event) => setEmailCode(event.target.value)}
                    className="h-11 pl-10"
                  />
                </div>
                <Button
                  type="button"
                  variant="outline"
                  className="h-11 shrink-0"
                  onClick={handleSendCode}
                  disabled={sendCodeMutation.isPending}
                >
                  {sendCodeMutation.isPending
                    ? t("auth.forgotPasswordSending")
                    : t("auth.forgotPasswordSendCode")}
                </Button>
              </div>
            </div>

            <div className="space-y-2">
              <label htmlFor="forgot-password" className="text-sm font-medium text-foreground">
                {t("auth.password")}
              </label>
              <div className="relative">
                <Lock className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="forgot-password"
                  type={showPassword ? "text" : "password"}
                  placeholder="••••••••"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
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
              <label htmlFor="forgot-confirm-password" className="text-sm font-medium text-foreground">
                {t("auth.confirmPassword")}
              </label>
              <div className="relative">
                <Lock className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="forgot-confirm-password"
                  type={showPassword ? "text" : "password"}
                  placeholder="••••••••"
                  value={confirmPassword}
                  onChange={(event) => setConfirmPassword(event.target.value)}
                  autoComplete="new-password"
                  className="h-11 pl-10"
                />
              </div>
              {passwordMismatch && (
                <p className="text-xs text-destructive">{t("install.passwordMismatch")}</p>
              )}
            </div>

            {resetPasswordMutation.error && (
              <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                {resetPasswordMutation.error.message}
              </div>
            )}

            <Button
              type="submit"
              className="h-11 w-full"
              disabled={resetPasswordMutation.isPending || passwordMismatch}
            >
              {resetPasswordMutation.isPending
                ? t("auth.forgotPasswordSubmitting")
                : t("auth.forgotPasswordSubmit")}
            </Button>
          </form>
        </CardContent>

        <CardFooter className="flex justify-center pb-8 pt-2 text-sm text-muted-foreground">
          <Link to={ROUTES.LOGIN} className="text-primary hover:underline">
            {t("auth.login")}
          </Link>
        </CardFooter>
      </Card>
    </div>
  );
}
