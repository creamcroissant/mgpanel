import { Home, Menu, LogOut, Shield, User } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate } from "react-router-dom";
import ThemeToggle from "../ThemeToggle";
import LanguageSwitcher from "../LanguageSwitcher";
import { useAuth } from "@/providers/AuthProvider";
import { ADMIN_HOME_ROUTE, getRouteLabelKey, isAdminPath, ROUTES, USER_HOME_ROUTE } from "@/lib/constants";
import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui";
import { cn } from "@/lib/utils";

interface HeaderProps {
  onMenuClick: () => void;
}

function getInitial(email?: string) {
  if (!email) {
    return "?";
  }
  return email.trim().charAt(0).toUpperCase() || "?";
}

export default function Header({ onMenuClick }: HeaderProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const { user, isAdmin, logout } = useAuth();
  const currentLabelKey = getRouteLabelKey(location.pathname);
  const isAdminRoute = isAdminPath(location.pathname);
  const portalLabelKey = isAdminRoute ? "admin.nav.portal" : "nav.portal";
  const portalSwitchLabelKey = isAdminRoute ? "nav.backToUser" : "nav.adminPortal";
  const PortalSwitchIcon = isAdminRoute ? Home : Shield;

  const handleLogout = async () => {
    await logout();
    navigate(ROUTES.LOGIN);
  };

  const handlePortalSwitch = () => {
    navigate(isAdminRoute ? USER_HOME_ROUTE : ADMIN_HOME_ROUTE);
  };

  return (
    <header className="relative z-30 h-[var(--header-height)] shrink-0 border-b bg-background">
      <div className="flex h-full items-center justify-between gap-4 px-4 md:px-6 lg:px-8">
        <div className="flex min-w-0 items-center gap-3">
          <Button
            variant="outline"
            size="icon"
            className="lg:hidden"
            onClick={onMenuClick}
            aria-label={t("nav.dashboard")}
          >
            <Menu size={18} />
          </Button>
          {currentLabelKey && (
            <nav className="min-w-0" aria-label={t("common.breadcrumb")}>
              <ol className="flex min-w-0 items-center gap-1.5 text-sm">
                <li className="hidden max-w-[130px] truncate text-muted-foreground sm:block">{t(portalLabelKey)}</li>
                <li className="hidden text-muted-foreground/70 sm:block" aria-hidden="true">
                  /
                </li>
                <li
                  className="block max-w-[180px] truncate text-base font-semibold text-foreground sm:max-w-[300px] md:max-w-[420px] lg:max-w-[560px]"
                  aria-current="page"
                >
                  {t(currentLabelKey)}
                </li>
              </ol>
            </nav>
          )}
        </div>

        <div className="flex items-center gap-2">
          <LanguageSwitcher />
          <ThemeToggle />

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" className="h-10 gap-2 px-2.5 shadow-none" data-testid="user-menu-trigger">
                <span
                  className={cn(
                    "inline-flex h-8 w-8 items-center justify-center rounded-full",
                    "bg-primary/10 text-xs font-semibold text-primary"
                  )}
                >
                  {getInitial(user?.email)}
                </span>
                <span className="hidden max-w-[140px] truncate text-sm md:inline">
                  {user?.email ?? "-"}
                </span>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              {isAdmin && (
                <>
                  <DropdownMenuItem onSelect={handlePortalSwitch}>
                    <PortalSwitchIcon className="mr-2 h-4 w-4" />
                    {t(portalSwitchLabelKey)}
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                </>
              )}
              <DropdownMenuItem onSelect={() => navigate(ROUTES.SETTINGS)}>
                <User className="mr-2 h-4 w-4" />
                {t("nav.settings")}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem className="gap-2 text-destructive focus:text-destructive" onSelect={handleLogout}>
                <LogOut className="h-4 w-4" />
                {t("nav.logout")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
    </header>
  );
}
