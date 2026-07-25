import { lazy, Suspense } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router-dom";
import { Loading, Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui";

const GeneralTab = lazy(() => import("./tabs/GeneralTab"));
const SubscriptionTab = lazy(() => import("./tabs/SubscriptionTab"));
const NodeTab = lazy(() => import("./tabs/NodeTab"));
const EmailTab = lazy(() => import("./tabs/EmailTab"));
const RoutingTab = lazy(() => import("./tabs/RoutingTab"));
const MCPTab = lazy(() => import("./tabs/MCPTab"));

const SYSTEM_SETTINGS_TABS = ["general", "subscription", "node", "email", "routing", "mcp"] as const;
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
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">{t("admin.system.settings.title")}</h1>
        <p className="text-sm text-muted-foreground">{t("admin.system.settings.description")}</p>
      </div>

      <Tabs value={activeTab} onValueChange={handleTabChange}>
        <TabsList className="flex w-full flex-wrap justify-start">
          <TabsTrigger value="general">{t("admin.system.settings.tabs.general")}</TabsTrigger>
          <TabsTrigger value="subscription">{t("admin.system.settings.tabs.subscription")}</TabsTrigger>
          <TabsTrigger value="node">{t("admin.system.settings.tabs.node")}</TabsTrigger>
          <TabsTrigger value="email">{t("admin.system.settings.tabs.email")}</TabsTrigger>
          <TabsTrigger value="routing">{t("admin.system.settings.tabs.routing")}</TabsTrigger>
          <TabsTrigger value="mcp">{t("admin.system.settings.tabs.mcp")}</TabsTrigger>
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
      </Tabs>
    </div>
  );
}
