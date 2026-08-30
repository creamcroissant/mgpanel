import { lazy, Suspense } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router-dom";
import { Loading, Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui";
import AdminPageShell from "@/components/admin/AdminPageShell";

const GeneralTab = lazy(() => import("./tabs/GeneralTab"));
const DataTab = lazy(() => import("./tabs/DataTab"));
const SubscriptionTab = lazy(() => import("./tabs/SubscriptionTab"));
const NodeTab = lazy(() => import("./tabs/NodeTab"));
const EmailTab = lazy(() => import("./tabs/EmailTab"));
const RoutingTab = lazy(() => import("./tabs/RoutingTab"));
const MCPTab = lazy(() => import("./tabs/MCPTab"));
const NamingTab = lazy(() => import("./tabs/NamingTab"));

const SYSTEM_SETTINGS_TABS = ["general", "subscription", "node", "email", "routing", "mcp", "data", "naming"] as const;
type SystemSettingsTab = (typeof SYSTEM_SETTINGS_TABS)[number];

function parseSystemSettingsTab(value: string | null): SystemSettingsTab {
  if (SYSTEM_SETTINGS_TABS.includes(value as SystemSettingsTab)) {
    return value as SystemSettingsTab;
  }
  return "general";
}

export default function SystemSettings() {
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const activeTab = parseSystemSettingsTab(searchParams.get("tab"));

  const handleTabChange = (value: string) => {
    const nextTab = parseSystemSettingsTab(value);
    const nextParams = new URLSearchParams(searchParams);
    nextParams.set("tab", nextTab);
    setSearchParams(nextParams);
  };

  return (
    <AdminPageShell
      title={t("admin.system.settings.title")}
      description={t("admin.system.settings.description")}
    >
    <Tabs value={activeTab} onValueChange={handleTabChange}>
        <TabsList className="flex w-full flex-wrap justify-start">
          <TabsTrigger value="general">{t("admin.system.settings.tabs.general")}</TabsTrigger>
          <TabsTrigger value="subscription">{t("admin.system.settings.tabs.subscription")}</TabsTrigger>
          <TabsTrigger value="node">{t("admin.system.settings.tabs.node")}</TabsTrigger>
          <TabsTrigger value="email">{t("admin.system.settings.tabs.email")}</TabsTrigger>
          <TabsTrigger value="routing">{t("admin.system.settings.tabs.routing")}</TabsTrigger>
          <TabsTrigger value="mcp">{t("admin.system.settings.tabs.mcp")}</TabsTrigger>
          <TabsTrigger value="data">{t("admin.system.retention.title")}</TabsTrigger>
          <TabsTrigger value="naming">{t("admin.system.settings.tabs.naming")}</TabsTrigger>
        </TabsList>

        <TabsContent value="general">
          <Suspense fallback={<Loading />}>
            <GeneralTab />
          </Suspense>
        </TabsContent>
        <TabsContent value="subscription">
          <Suspense fallback={<Loading />}>
            <SubscriptionTab />
          </Suspense>
        </TabsContent>
        <TabsContent value="node">
          <Suspense fallback={<Loading />}>
            <NodeTab />
          </Suspense>
        </TabsContent>
        <TabsContent value="email">
          <Suspense fallback={<Loading />}>
            <EmailTab />
          </Suspense>
        </TabsContent>
        <TabsContent value="routing">
          <Suspense fallback={<Loading />}>
            <RoutingTab />
          </Suspense>
        </TabsContent>
        <TabsContent value="mcp">
          <Suspense fallback={<Loading />}>
            <MCPTab />
          </Suspense>
        </TabsContent>
        <TabsContent value="data">
          <Suspense fallback={<Loading />}>
            <DataTab />
          </Suspense>
        </TabsContent>
        <TabsContent value="naming">
          <Suspense fallback={<Loading />}>
            <NamingTab />
          </Suspense>
        </TabsContent>
      </Tabs>
    </AdminPageShell>
  );
}
