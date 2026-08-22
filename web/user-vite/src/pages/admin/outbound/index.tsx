import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowRightLeft, Network } from "lucide-react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui";
import { CoreConfigManager } from "@/pages/admin/config-center/CoreConfigManager";
import { ExitNodeSetsSection } from "./ExitNodeSetsSection";

type TabKey = "outbounds" | "exitSets";

export default function OutboundPage() {
  const { t } = useTranslation();
  const [tab, setTab] = useState<TabKey>("outbounds");

  return (
    <div className="space-y-4">
      <Tabs value={tab} onValueChange={(v) => setTab(v as TabKey)}>
        <TabsList>
          <TabsTrigger value="outbounds">
            <ArrowRightLeft className="mr-2 h-4 w-4" />
            {t("admin.configCenter.configTypes.outbound")}
          </TabsTrigger>
          <TabsTrigger value="exitSets">
            <Network className="mr-2 h-4 w-4" />
            {t("admin.nav.exitNodeSets")}
          </TabsTrigger>
        </TabsList>
        <TabsContent value="outbounds">
          <CoreConfigManager
            configType="outbound"
            titleKey="admin.configCenter.configTypes.outbound"
            descriptionKey="admin.configCenter.coreConfig.descriptions.outbound"
          />
        </TabsContent>
        <TabsContent value="exitSets">
          <ExitNodeSetsSection />
        </TabsContent>
      </Tabs>
    </div>
  );
}