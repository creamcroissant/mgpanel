
import { CoreConfigManager } from "@/pages/admin/config-center/CoreConfigManager";

export default function CoreSettingsPage() {
  return (
    <CoreConfigManager
      configType="core_settings"
      titleKey="admin.configCenter.configTypes.coreSettings"
      descriptionKey="admin.configCenter.coreConfig.descriptions.coreSettings"
    />
  );
}