import { lazy, Suspense } from "react";
import { Navigate, Outlet, Route, Routes } from "react-router-dom";
import { useAuth } from "@/providers/AuthProvider";
import { AppLayout } from "@/components/layout";
import { Loading } from "@/components/ui";
import { ROUTES, ADMIN_ROUTES, ADMIN_AUTH_ROUTES } from "@/lib/constants";

// Lazy load pages
const Login = lazy(() => import("@/pages/auth/Login"));
const Register = lazy(() => import("@/pages/auth/Register"));
const ForgotPassword = lazy(() => import("@/pages/auth/ForgotPassword"));
const Install = lazy(() => import("@/pages/install"));
const Dashboard = lazy(() => import("@/pages/dashboard"));
const Subscription = lazy(() => import("@/pages/subscription"));
const Servers = lazy(() => import("@/pages/servers"));
const Plans = lazy(() => import("@/pages/plans"));
const TrafficStats = lazy(() => import("@/pages/traffic"));
const Knowledge = lazy(() => import("@/pages/knowledge"));
const Settings = lazy(() => import("@/pages/settings"));
const NotFound = lazy(() => import("@/pages/NotFound"));

// Admin pages
const AdminAgents = lazy(() => import("@/pages/admin/agents"));
const AdminUsers = lazy(() => import("@/pages/admin/users"));
const AdminPlans = lazy(() => import("@/pages/admin/plans"));
const AdminNotices = lazy(() => import("@/pages/admin/notices"));
const AdminKnowledge = lazy(() => import("@/pages/admin/knowledge"));
const AdminSystem = lazy(() => import("@/pages/admin/system"));
const AdminForwarding = lazy(() => import("@/pages/admin/forwarding"));
const AdminAccessLogs = lazy(() => import("@/pages/admin/access-logs"));
const AdminConfigCenter = lazy(() => import("@/pages/admin/config-center"));
const AdminCDN = lazy(() => import("@/pages/admin/cdn"));
const AdminMCPKeys = lazy(() => import("@/pages/admin/mcp-keys"));

const adminAuthAliases = [
  { adminPath: ADMIN_AUTH_ROUTES.LOGIN, defaultPath: ROUTES.LOGIN, component: Login },
  { adminPath: ADMIN_AUTH_ROUTES.REGISTER, defaultPath: ROUTES.REGISTER, component: Register },
  {
    adminPath: ADMIN_AUTH_ROUTES.FORGOT_PASSWORD,
    defaultPath: ROUTES.FORGOT_PASSWORD,
    component: ForgotPassword,
  },
];

// Route guard for authenticated routes
function RequireAuth() {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) {
    return <Loading fullScreen />;
  }

  if (!isAuthenticated) {
    return <Navigate to={ROUTES.LOGIN} replace />;
  }

  return (
    <AppLayout>
      <Suspense fallback={<Loading />}>
        <Outlet />
      </Suspense>
    </AppLayout>
  );
}

// Route guard for public routes (redirect if already logged in)
function PublicRoute() {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) {
    return <Loading fullScreen />;
  }

  if (isAuthenticated) {
    return <Navigate to={ROUTES.DASHBOARD} replace />;
  }

  return (
    <Suspense fallback={<Loading fullScreen />}>
      <Outlet />
    </Suspense>
  );
}

// Route guard for admin-only routes
function RequireAdminAuth() {
  const { isAuthenticated, isAdmin, isLoading } = useAuth();

  if (isLoading) {
    return <Loading fullScreen />;
  }

  if (!isAuthenticated) {
    return <Navigate to={ADMIN_AUTH_ROUTES.LOGIN} replace />;
  }

  if (!isAdmin) {
    return <Navigate to={ROUTES.DASHBOARD} replace />;
  }

  return (
    <AppLayout>
      <Suspense fallback={<Loading />}>
        <Outlet />
      </Suspense>
    </AppLayout>
  );
}

export default function App() {
  return (
    <Routes>
      {/* Install route - no auth required */}
      <Route
        path={ROUTES.INSTALL}
        element={
          <Suspense fallback={<Loading fullScreen />}>
            <Install />
          </Suspense>
        }
      />

      {/* Public routes */}
      <Route element={<PublicRoute />}>
        {adminAuthAliases.map(({ defaultPath, component: Component }) => (
          <Route key={defaultPath} path={defaultPath} element={<Component />} />
        ))}
        {adminAuthAliases.map(
          ({ adminPath, defaultPath, component: Component }) =>
            adminPath !== defaultPath && <Route key={adminPath} path={adminPath} element={<Component />} />
        )}
      </Route>

      {/* Protected routes */}
      <Route element={<RequireAuth />}>
        <Route index element={<Navigate to={ROUTES.DASHBOARD} replace />} />
        <Route path={ROUTES.DASHBOARD} element={<Dashboard />} />
        <Route path={ROUTES.SUBSCRIPTION} element={<Subscription />} />
        <Route path={ROUTES.SERVERS} element={<Servers />} />
        <Route path={ROUTES.PLANS} element={<Plans />} />
        <Route path={ROUTES.TRAFFIC} element={<TrafficStats />} />
        <Route path={ROUTES.KNOWLEDGE} element={<Knowledge />} />
        <Route path={ROUTES.SETTINGS} element={<Settings />} />
      </Route>

      {/* Admin routes */}
      <Route element={<RequireAdminAuth />}>
        <Route path={ADMIN_ROUTES.AGENTS} element={<AdminAgents />} />
        <Route path={ADMIN_ROUTES.USERS} element={<AdminUsers />} />
        <Route path={ADMIN_ROUTES.PLANS} element={<AdminPlans />} />
        <Route path={ADMIN_ROUTES.NOTICES} element={<AdminNotices />} />
        <Route path={ADMIN_ROUTES.KNOWLEDGE} element={<AdminKnowledge />} />
        <Route path={ADMIN_ROUTES.SYSTEM} element={<AdminSystem />} />
        <Route path={ADMIN_ROUTES.FORWARDING} element={<AdminForwarding />} />
        <Route path={ADMIN_ROUTES.MCP_KEYS} element={<AdminMCPKeys />} />
        <Route path={ADMIN_ROUTES.ACCESS_LOGS} element={<AdminAccessLogs />} />
        <Route path={ADMIN_ROUTES.CDN} element={<AdminCDN />} />
        <Route path={ADMIN_ROUTES.CONFIG_CENTER} element={<AdminConfigCenter />} />
      </Route>

      {/* 404 */}
      <Route path="*" element={<NotFound />} />
    </Routes>
  );
}
