import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useMutation } from "@tanstack/react-query";
import { Mail, Lock, Eye, EyeOff, Ticket } from "lucide-react";
import { Card, CardContent, CardFooter, CardHeader } from "@/components/ui";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { register } from "@/api/auth";
import { useAuth } from "@/providers/AuthProvider";
import { ROUTES } from "@/lib/constants";
import { AuthShell } from "@/components/auth/AuthShell";

export default function Register() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { login: authLogin } = useAuth();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [, setInviteCode] = useState("");

  const registerMutation = useMutation({
    mutationFn: () =>
      register({
        email,
        password,
      }),
    onSuccess: (data) => {
      authLogin(data.token);
      navigate(ROUTES.DASHBOARD);
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (password !== confirmPassword) {
      return;
    }
    registerMutation.mutate();
  };

  const passwordMismatch = Boolean(confirmPassword) && password !== confirmPassword;

  return (
    <AuthShell>
      <Card className="border border-border/80">
        <CardHeader className="items-center space-y-2 pt-8 text-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-md bg-primary/10 text-primary font-semibold">
            X
          </div>
          <div className="space-y-1">
            <h1 className="text-2xl font-semibold tracking-tight">
              {t("auth.register")}
            </h1>
            <p className="text-sm text-muted-foreground">
              Create your MGPanel account
            </p>
          </div>
        </CardHeader>

        <CardContent className="px-6 pb-6 pt-4">
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <label
                htmlFor="register-email"
                className="text-sm font-medium text-foreground"
              >
                {t("auth.email")}
              </label>
              <div className="relative">
                <Mail className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="register-email"
                  type="email"
                  placeholder="you@example.com"
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  required
                  autoComplete="email"
                  className="h-11 pl-10"
                />
              </div>
            </div>

            <div className="space-y-2">
              <label
                htmlFor="register-password"
                className="text-sm font-medium text-foreground"
              >
                {t("auth.password")}
              </label>
              <div className="relative">
                <Lock className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="register-password"
                  type={showPassword ? "text" : "password"}
                  placeholder="••••••••"
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
                htmlFor="register-confirm-password"
                className="text-sm font-medium text-foreground"
              >
                {t("auth.confirmPassword")}
              </label>
              <div className="relative">
                <Lock className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="register-confirm-password"
                  type={showPassword ? "text" : "password"}
                  placeholder="••••••••"
                  value={confirmPassword}
                  onChange={(event) => setConfirmPassword(event.target.value)}
                  required
                  autoComplete="new-password"
                  className="h-11 pl-10"
                />
              </div>
              {passwordMismatch && (
                <p className="text-xs text-destructive">Passwords do not match</p>
              )}
            </div>

            <div className="space-y-2">
              <label
                htmlFor="register-invite"
                className="text-sm font-medium text-foreground"
              >
              </label>
              <div className="relative">
                <Ticket className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="register-invite"
                  type="text"
                  placeholder="Optional"
                  onChange={(event) => setInviteCode(event.target.value)}
                  className="h-11 pl-10"
                />
              </div>
            </div>

            {registerMutation.error && (
              <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                {registerMutation.error.message}
              </div>
            )}

            <Button
              type="submit"
              className="h-11 w-full"
              disabled={registerMutation.isPending || passwordMismatch}
            >
              {registerMutation.isPending ? "注册中..." : t("auth.register")}
            </Button>
          </form>
        </CardContent>

        <CardFooter className="flex justify-center pb-8 pt-2 text-sm text-muted-foreground">
          {t("auth.hasAccount")} {" "}
          <Link to={ROUTES.LOGIN} className="text-primary hover:underline">
            {t("auth.login")}
          </Link>
        </CardFooter>
      </Card>
    </AuthShell>
  );
}
