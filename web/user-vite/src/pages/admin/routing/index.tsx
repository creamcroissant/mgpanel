import { lazy, Suspense, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router-dom";
import { ArrowLeftRight, Route, RouteOff, Workflow } from "lucide-react";
import { Button, Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui";
import { CoreConfigManager } from "@/pages/admin/config-center/CoreConfigManager";
import { RoutingPoliciesSection } from "./RoutingPoliciesSection";

// 拓扑画布懒加载：@xyflow/react 依赖隔离进独立 chunk，不进路由页主包
const TopologyTab = lazy(() =>
  import("./topology/TopologyTab").then((m) => ({ default: m.TopologyTab }))
);

type TabKey = "topology" | "routing" | "policies";
const TAB_KEYS: TabKey[] = ["topology", "routing", "policies"];

export default function RoutingPage() {
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const urlTab = searchParams.get("tab") as TabKey | null;
  const [tab, setTab] = useState<TabKey>(
    urlTab && TAB_KEYS.includes(urlTab) ? urlTab : "topology"
  );

  // tab 变化同步到 URL（?tab=topology），供外部页（如配置中心）深链直达
  useEffect(() => {
    if (urlTab !== tab) setSearchParams({ tab }, { replace: true });
  }, [tab, urlTab, setSearchParams]);

  return (
    <div className="space-y-4">
      <Tabs value={tab} onValueChange={(v) => setTab(v as TabKey)}>
        <TabsList>
          <TabsTrigger value="topology">
            <Workflow className="mr-2 h-4 w-4" />
            拓扑
          </TabsTrigger>
          <TabsTrigger value="routing">
            <Route className="mr-2 h-4 w-4" />
            {t("admin.configCenter.configTypes.routing")}
          </TabsTrigger>
          <TabsTrigger value="policies">
            <RouteOff className="mr-2 h-4 w-4" />
            {t("admin.nav.routingPolicies")}
          </TabsTrigger>
        </TabsList>
        <TabsContent value="topology">
          <ViewSwitchLink direction="list" onSwitch={() => setTab("routing")} />
          <Suspense
            fallback={
              <div className="h-[560px] w-full animate-pulse rounded-md border bg-muted/40" />
            }
          >
            <TopologyTab />
          </Suspense>
        </TabsContent>
        <TabsContent value="routing">
          <ViewSwitchLink direction="topology" onSwitch={() => setTab("topology")} />
          <CoreConfigManager
            configType="routing"
            titleKey="admin.configCenter.configTypes.routing"
            descriptionKey="admin.configCenter.coreConfig.descriptions.routing"
          />
        </TabsContent>
        <TabsContent value="policies">
          <ViewSwitchLink direction="topology" onSwitch={() => setTab("topology")} />
          <RoutingPoliciesSection />
        </TabsContent>
      </Tabs>
    </div>
  );
}

/** 列表/拓扑双向共存入口 */
function ViewSwitchLink({
  direction,
  onSwitch,
}: {
  direction: "topology" | "list";
  onSwitch: () => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="mb-2 flex justify-end">
      <Button type="button" variant="ghost" size="sm" onClick={onSwitch}>
        <ArrowLeftRight className="mr-1.5 h-3.5 w-3.5" aria-hidden />
        {direction === "topology"
          ? t("admin.topology.switch_to_topology")
          : t("admin.topology.switch_to_list")}
      </Button>
    </div>
  );
}
