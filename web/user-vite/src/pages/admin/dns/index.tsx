
import { CoreConfigManager } from "@/pages/admin/config-center/CoreConfigManager";

export default function DnsPage() {
  return (
    <CoreConfigManager
      configType="dns"
      titleKey="admin.configCenter.configTypes.dns"
      descriptionKey="admin.configCenter.coreConfig.descriptions.dns"
    />
  );
}