import { Button } from "@/components/ui/button";
import { Globe } from "lucide-react";
import { useTranslation } from "react-i18next";

export default function LanguageSwitcher() {
  const { i18n } = useTranslation();

  const toggle = () => {
    const newLang = i18n.language?.startsWith("zh") ? "en-US" : "zh-CN";
    i18n.changeLanguage(newLang);
  };

  const isZh = i18n.language?.startsWith("zh");

  return (
    <Button
      variant="ghost"
      size="sm"
      onClick={toggle}
      className="min-w-[70px] gap-2"
    >
      <Globe size={16} />
      {isZh ? "EN" : "中文"}
    </Button>
  );
}
