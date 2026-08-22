import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { getAgentHosts } from "@/api/admin/agentHost";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui";
import { QUERY_KEYS } from "@/lib/constants";
import type { ConfigCenterCoreType, CoreConfigType } from "@/types";
import { CoreConfigTab } from "./CoreConfigTab";

const CORE_OPTIONS: ConfigCenterCoreType[] = ["sing-box", "xray"];

export interface CoreConfigManagerProps {
  configType: CoreConfigType;
  titleKey: string;
  descriptionKey: string;
}

/**
 * 独立的核心配置管理页面骨架。
 * 提供 host + core 选择器，并渲染对应的 CoreConfigTab。
 * 用于从配置中心解耦出来的 outbound / routing / dns / core_settings 页面。
 */
export function CoreConfigManager({
  configType,
  titleKey,
  descriptionKey,
}: CoreConfigManagerProps) {
  const { t } = useTranslation();
  const [selectedHostId, setSelectedHostId] = useState<number | null>(null);
  const [selectedCoreType, setSelectedCoreType] =
    useState<ConfigCenterCoreType>("sing-box");

  const hostQuery = useQuery({
    queryKey: [QUERY_KEYS.ADMIN_AGENTS],
    queryFn: () => getAgentHosts({ page: 1, page_size: 100 }),
  });

  const hostOptions = useMemo(() => hostQuery.data?.data ?? [], [hostQuery.data]);

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>{t(titleKey)}</CardTitle>
          <CardDescription>{t(descriptionKey)}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap items-center gap-3">
            {/* Host selector */}
            <div className="w-48">
              <Select
                value={selectedHostId ? String(selectedHostId) : "all"}
                onValueChange={(value) =>
                  setSelectedHostId(value !== "all" ? Number(value) : null)
                }
              >
                <SelectTrigger>
                  <SelectValue
                    placeholder={t("admin.configCenter.placeholders.selectHost")}
                  />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">
                    {t("admin.configCenter.allHosts")}
                  </SelectItem>
                  {hostOptions.map((host) => (
                    <SelectItem key={host.id} value={String(host.id)}>
                      {host.name || host.host}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {/* Core selector */}
            <div className="w-36">
              <Select
                value={selectedCoreType}
                onValueChange={(value) =>
                  setSelectedCoreType(value as ConfigCenterCoreType)
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {CORE_OPTIONS.map((opt) => (
                    <SelectItem key={opt} value={opt}>
                      {opt}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardContent>
      </Card>

      <CoreConfigTab
        configType={configType}
        selectedHostId={selectedHostId}
        selectedCoreType={selectedCoreType}
      />
    </div>
  );
}
