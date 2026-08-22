import { useTranslation } from "react-i18next";
import { CoreConfigManager } from "@/pages/admin/config-center/CoreConfigManager";

export default function DnsPage() {
  const { t } = useTranslation();
  return (
    <CoreConfigManager
      configType="dns"
      titleKey="admin.configCenter.configTypes.dns"
      descriptionKey="admin.configCenter.coreConfig.descriptions.dns"
    />
  );
}