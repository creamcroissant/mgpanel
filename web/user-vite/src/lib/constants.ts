import type { LucideIcon } from "lucide-react";
import {
  BarChart3,
  Bell,
  BookOpen,
  Cog,
  CreditCard,
  GitCompare,
  Globe,
  LayoutDashboard,
  ListChecks,
  MonitorDot,
  Package,
  Rss,
  Server,
  Settings,
  Shield,
  Shuffle,
  Users,
} from "lucide-react";

export const API_VERSION = "/api/v1";


const normalizePath = (value: string | undefined, fallback: string): string => {
  const normalized = (value || "").trim();
  if (!normalized) {
    return fallback;
  }
  const withLeadingSlash = normalized.startsWith("/") ? normalized : `/${normalized}`;
  return withLeadingSlash.replace(/\/+$/, "") || fallback;
};

const runtimeSecurePath = normalizePath(window?.settings?.secure_path, "/admin");
const runtimeRouterBase = normalizePath(window?.settings?.router_base, "/");

export const ADMIN_SECURE_PATH = runtimeSecurePath;
export const ADMIN_API_VERSION = `/api/v2${runtimeSecurePath}`;
export const ADMIN_ROUTE_BASE = runtimeRouterBase;

const normalizeRoute = (route: string): string => {
  if (ADMIN_ROUTE_BASE === "/") {
    return route;
  }
  return `${ADMIN_ROUTE_BASE}${route}`;
};

export const QUERY_KEYS = {
  USER: ["user"],
  USER_INFO: ["user", "info"],
  SERVERS: ["user", "servers"],
  PLANS: ["user", "plans"],
  TRAFFIC: ["user", "traffic"],
  KNOWLEDGE: ["user", "knowledge"],
  SHORT_LINKS: ["user", "shortlinks"],
  USER_NOTICE: ["user", "notice"],
  // Admin query keys
  ADMIN_AGENTS: ["admin", "agents"],
  ADMIN_USERS: ["admin", "users"],
  ADMIN_PLANS: ["admin", "plans"],
  ADMIN_NOTICES: ["admin", "notices"],
  ADMIN_KNOWLEDGE: ["admin", "knowledge"],
  ADMIN_SYSTEM: ["admin", "system"],
  ADMIN_SYSTEM_QUEUE: ["admin", "system", "queue"],
  ADMIN_FORWARDING: ["admin", "forwarding"],
  ADMIN_FORWARDING_LOGS: ["admin", "forwarding", "logs"],
  ADMIN_ACCESS_LOGS: ["admin", "access-logs"],
  ADMIN_ACCESS_LOG_STATS: ["admin", "access-logs", "stats"],
  ADMIN_AGENT_CORES: ["admin", "agents", "cores"],
  ADMIN_AGENT_CORE_INSTANCES: ["admin", "agents", "core-instances"],
  ADMIN_AGENT_CORE_SWITCH_LOGS: ["admin", "agents", "core-switch-logs"],
  ADMIN_AGENT_CORE_OPERATIONS: ["admin", "agents", "core-operations"],
  ADMIN_OPERATION_LOGS: ["admin", "operation-logs"],
  ADMIN_AGENT_BINARY_VERSIONS: ["admin", "agents", "binary-versions"],
  ADMIN_AGENT_LIFECYCLE_OPERATIONS: ["admin", "agents", "lifecycle-operations"],
  ADMIN_AGENT_TRAFFIC_POLICY: ["admin", "agents", "traffic-policy"],
  ADMIN_AGENT_TRAFFIC_STATUS: ["admin", "agents", "traffic-status"],
  ADMIN_SUBSCRIPTION_FILTER_REASONS: ["admin", "subscription", "filter-reasons"],
  ADMIN_SUBSCRIPTION_FILTER_SUMMARY: ["admin", "subscription", "filter-summary"],
  ADMIN_SUBSCRIPTION_SOURCES: ["admin", "subscription", "sources"],
  ADMIN_CONFIG_CENTER_SPECS: ["admin", "config-center", "specs"],
  ADMIN_CONFIG_CENTER_SPEC_HISTORY: ["admin", "config-center", "specs", "history"],
  ADMIN_CONFIG_CENTER_ARTIFACTS: ["admin", "config-center", "artifacts"],
  ADMIN_CONFIG_CENTER_DIFF_TEXT: ["admin", "config-center", "diff", "text"],
  ADMIN_CONFIG_CENTER_DIFF_SEMANTIC: ["admin", "config-center", "diff", "semantic"],
  ADMIN_CONFIG_CENTER_APPLY_RUNS: ["admin", "config-center", "apply-runs"],
  ADMIN_CONFIG_CENTER_SNAPSHOT: ["admin", "config-center", "snapshot"],
  ADMIN_CONFIG_CENTER_DRIFT: ["admin", "config-center", "drift"],
  ADMIN_CONFIG_CENTER_RECOVER: ["admin", "config-center", "recover"],
  ADMIN_CONFIG_CENTER_CORE_CONFIGS: ["admin", "config-center", "core-configs"],
  ADMIN_CDN_SITES: ["admin", "cdn", "sites"],
} as const;

export const ROUTES = {
  INSTALL: normalizeRoute("/install"),
  LOGIN: normalizeRoute("/login"),
  REGISTER: normalizeRoute("/register"),
  FORGOT_PASSWORD: normalizeRoute("/forgot-password"),
  DASHBOARD: normalizeRoute("/dashboard"),
  SUBSCRIPTION: normalizeRoute("/subscription"),
  SERVERS: normalizeRoute("/servers"),
  PLANS: normalizeRoute("/plans"),
  TRAFFIC: normalizeRoute("/traffic"),
  KNOWLEDGE: normalizeRoute("/knowledge"),
  SETTINGS: normalizeRoute("/settings"),
} as const;

const adminRoute = (path: string): string => normalizeRoute(`${ADMIN_SECURE_PATH}${path}`);

export const ADMIN_ROUTES = {
  AGENTS: adminRoute("/agents"),
  USERS: adminRoute("/users"),
  PLANS: adminRoute("/plans"),
  NOTICES: adminRoute("/notices"),
  KNOWLEDGE: adminRoute("/knowledge"),
  SYSTEM: adminRoute("/system"),
  FORWARDING: adminRoute("/forwarding"),
  ACCESS_LOGS: adminRoute("/access-logs"),
  CONFIG_CENTER: adminRoute("/config-center"),
  CDN: adminRoute("/cdn"),
  MCP_KEYS: adminRoute("/mcp-keys"),
} as const;

export const USER_HOME_ROUTE = ROUTES.DASHBOARD;
export const ADMIN_HOME_ROUTE = ADMIN_ROUTES.USERS;

export const ADMIN_AUTH_ROUTES = {
  LOGIN: adminRoute("/login"),
  REGISTER: adminRoute("/register"),
  FORGOT_PASSWORD: adminRoute("/forgot-password"),
} as const;

export interface NavigationItemMeta {
  to: string;
  labelKey: string;
  icon: LucideIcon;
  sidebar: boolean;
}

export interface NavigationGroupMeta {
  labelKey: string;
  items: NavigationItemMeta[];
}

export const USER_NAV_ITEMS: NavigationItemMeta[] = [
  { to: ROUTES.DASHBOARD, labelKey: "nav.dashboard", icon: LayoutDashboard, sidebar: true },
  { to: ROUTES.SUBSCRIPTION, labelKey: "nav.subscription", icon: Rss, sidebar: true },
  { to: ROUTES.SERVERS, labelKey: "nav.servers", icon: Server, sidebar: true },
  { to: ROUTES.PLANS, labelKey: "nav.plans", icon: CreditCard, sidebar: true },
  { to: ROUTES.TRAFFIC, labelKey: "nav.traffic", icon: BarChart3, sidebar: true },
  { to: ROUTES.KNOWLEDGE, labelKey: "nav.knowledge", icon: BookOpen, sidebar: true },
  { to: ROUTES.SETTINGS, labelKey: "nav.settings", icon: Settings, sidebar: true },
];

export const ADMIN_NAV_GROUPS: NavigationGroupMeta[] = [
  {
    labelKey: "admin.nav.groups.operations",
    items: [
      { to: ADMIN_ROUTES.USERS, labelKey: "admin.nav.users", icon: Users, sidebar: true },
      { to: ADMIN_ROUTES.PLANS, labelKey: "admin.nav.plans", icon: Package, sidebar: true },
      { to: ADMIN_ROUTES.NOTICES, labelKey: "admin.nav.notices", icon: Bell, sidebar: true },
      { to: ADMIN_ROUTES.KNOWLEDGE, labelKey: "admin.nav.knowledge", icon: BookOpen, sidebar: true },
    ],
  },
  {
    labelKey: "admin.nav.groups.nodesDeployment",
    items: [
      { to: ADMIN_ROUTES.AGENTS, labelKey: "admin.nav.agents", icon: MonitorDot, sidebar: true },
      { to: ADMIN_ROUTES.FORWARDING, labelKey: "admin.nav.forwarding", icon: Shuffle, sidebar: true },
      { to: ADMIN_ROUTES.CDN, labelKey: "admin.nav.cdn", icon: Globe, sidebar: true },
    ],
  },
  {
    labelKey: "admin.nav.groups.configuration",
    items: [{ to: ADMIN_ROUTES.CONFIG_CENTER, labelKey: "admin.nav.configCenter", icon: GitCompare, sidebar: true }],
  },
  {
    labelKey: "admin.nav.groups.securityLogs",
    items: [{ to: ADMIN_ROUTES.ACCESS_LOGS, labelKey: "admin.nav.accessLogs", icon: ListChecks, sidebar: true }],
  },
  {
    labelKey: "admin.nav.groups.system",
    items: [
      { to: ADMIN_ROUTES.MCP_KEYS, labelKey: "admin.nav.mcpKeys", icon: Shield, sidebar: true },
      { to: ADMIN_ROUTES.SYSTEM, labelKey: "admin.nav.system", icon: Cog, sidebar: true },
    ],
  },
];

export const ADMIN_NAV_ITEMS = ADMIN_NAV_GROUPS.flatMap((group) => group.items);

export function isAdminPath(pathname: string): boolean {
  return ADMIN_NAV_ITEMS.some((item) => pathname === item.to || pathname.startsWith(`${item.to}/`));
}

export const APP_NAV_ITEMS = [...USER_NAV_ITEMS, ...ADMIN_NAV_ITEMS] as const;

export function getRouteLabelKey(pathname: string): string {
  const exact = APP_NAV_ITEMS.find((item) => item.to === pathname);
  if (exact) {
    return exact.labelKey;
  }

  const prefix = APP_NAV_ITEMS.filter((item) => pathname.startsWith(item.to))
    .sort((a, b) => b.to.length - a.to.length)
    .at(0);

  return prefix?.labelKey ?? "";
}
