import { useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useMutation } from "@tanstack/react-query";
import { Mail, Lock, Eye, EyeOff } from "lucide-react";
import { Card, CardContent, CardFooter, CardHeader } from "@/components/ui";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { login } from "@/api/auth";
import { useAuth } from "@/providers/AuthProvider";
import { ROUTES } from "@/lib/constants";
import { AuthShell } from "@/components/auth/AuthShell";

const sanitizeNextPath = (next: string | null): string => {
  if (!next) {
    return ROUTES.DASHBOARD;
  }

  let decoded = "";
  try {
    decoded = decodeURIComponent(next).trim();
  } catch {
    return ROUTES.DASHBOARD;
  }

  if (!decoded.startsWith("/")) {
    return ROUTES.DASHBOARD;
  }
  if (decoded.startsWith("//") || /^[a-zA-Z][a-zA-Z\d+\-.]*:/.test(decoded)) {
    return ROUTES.DASHBOARD;
  }
  if (/[\r\n\t]/.test(decoded)) {
    return ROUTES.DASHBOARD;
  }

  return decoded;
};

export default function Login() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { login: authLogin } = useAuth();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [rememberMe, setRememberMe] = useState(true);

  const loginMutation = useMutation({
    mutationFn: () => login({ email, password }),
    onSuccess: (data) => {
      authLogin(data.token);
      const next = sanitizeNextPath(searchParams.get("next"));
      navigate(next);
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    loginMutation.mutate();
  };

  return (
    <AuthShell>
      <Card className="border border-border/80">
        <CardHeader className="items-center space-y-2 pt-8 text-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-md bg-primary/10 text-primary font-semibold">
            X
          </div>
          <div className="space-y-1">
            <h1 className="text-2xl font-semibold tracking-tight">
              {t("auth.login")}
            </h1>
            <p className="text-sm text-muted-foreground">{t("auth.welcomeBack")}</p>
          </div>
        </CardHeader>

        <CardContent className="px-6 pb-6 pt-4">
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <label
                htmlFor="login-email"
                className="text-sm font-medium text-foreground"
              >
                {t("auth.emailOrUsername")}
              </label>
              <div className="relative">
                <Mail className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="login-email"
                  type="text"
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
                htmlFor="login-password"
                className="text-sm font-medium text-foreground"
              >
                {t("auth.password")}
              </label>
              <div className="relative">
                <Lock className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="login-password"
                  type={showPassword ? "text" : "password"}
                  placeholder="••••••••"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  required
                  autoComplete="current-password"
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

            <div className="flex items-center justify-between text-sm">
              <label className="flex items-center gap-2 text-muted-foreground">
                <input
                  type="checkbox"
                  className="h-4 w-4 rounded border border-input text-primary focus:ring-2 focus:ring-primary"
                  checked={rememberMe}
                  onChange={(event) => setRememberMe(event.target.checked)}
                />
                <span>{t("auth.rememberMe")}</span>
              </label>
              <Link to={ROUTES.FORGOT_PASSWORD} className="text-primary hover:underline">
                {t("auth.forgotPassword")}
              </Link>
            </div>

            {loginMutation.error && (
              <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                {loginMutation.error.message}
              </div>
            )}

            <Button
              type="submit"
              className="h-11 w-full"
              disabled={loginMutation.isPending}
            >
              {loginMutation.isPending ? t("auth.loggingIn") : t("auth.login")}
            </Button>
          </form>
        </CardContent>

        <CardFooter className="flex justify-center pb-8 pt-2 text-sm text-muted-foreground">
          {t("auth.noAccount")} {" "}
          <Link to={ROUTES.REGISTER} className="text-primary hover:underline">
            {t("auth.register")}
          </Link>
        </CardFooter>
      </Card>
    </AuthShell>
  );
}
