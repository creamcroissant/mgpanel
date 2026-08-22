import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Route, RouteOff } from "lucide-react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui";
import { CoreConfigManager } from "@/pages/admin/config-center/CoreConfigManager";
import { RoutingPoliciesSection } from "./RoutingPoliciesSection";

type TabKey = "routing" | "policies";

export default function RoutingPage() {
  const { t } = useTranslation();
  const [tab, setTab] = useState<TabKey>("routing");

  return (
    <div className="space-y-4">
      <Tabs value={tab} onValueChange={(v) => setTab(v as TabKey)}>
        <TabsList>
          <TabsTrigger value="routing">
            <Route className="mr-2 h-4 w-4" />
            {t("admin.configCenter.configTypes.routing")}
          </TabsTrigger>
          <TabsTrigger value="policies">
            <RouteOff className="mr-2 h-4 w-4" />
            {t("admin.nav.routingPolicies")}
          </TabsTrigger>
        </TabsList>
        <TabsContent value="routing">
          <CoreConfigManager
            configType="routing"
            titleKey="admin.configCenter.configTypes.routing"
            descriptionKey="admin.configCenter.coreConfig.descriptions.routing"
          />
        </TabsContent>
        <TabsContent value="policies">
          <RoutingPoliciesSection />
        </TabsContent>
      </Tabs>
    </div>
  );
}