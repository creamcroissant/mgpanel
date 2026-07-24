import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import DOMPurify from "dompurify";
import { Button, Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui";
import type { UserNotice } from "@/api/notice";

interface NoticePopupProps {
  notice: UserNotice | null;
  open: boolean;
  onClose: () => void;
}

export default function NoticePopup({ notice, open, onClose }: NoticePopupProps) {
  const { t } = useTranslation();

  const content = useMemo(() => {
    if (!notice?.content) {
      return "";
    }
    return DOMPurify.sanitize(notice.content);
  }, [notice]);

  if (!notice) {
    return null;
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{notice.title || t("notice.popup.title")}</DialogTitle>
        </DialogHeader>
        <div className="space-y-3">
          {notice.img_url ? (
            <div className="overflow-hidden rounded-lg border border-border">
              <img
                src={notice.img_url}
                alt={notice.title || t("notice.popup.imageAlt")}
                className="h-48 w-full object-cover"
                loading="lazy"
              />
            </div>
          ) : null}
          <div
            className="prose prose-sm max-w-none text-foreground dark:prose-invert"
            dangerouslySetInnerHTML={{ __html: content }}
          />
        </div>
        <DialogFooter>
          <Button onClick={onClose}>{t("notice.popup.close")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
