import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Sidebar from "./Sidebar";
import Header from "./Header";
import NoticePopup from "@/components/NoticePopup";
import { fetchUnreadNotice, markNoticeRead } from "@/api/notice";
import { QUERY_KEYS } from "@/lib/constants";
import { useAuth } from "@/providers/AuthProvider";

interface AppLayoutProps {
  children: ReactNode;
}

export default function AppLayout({ children }: AppLayoutProps) {
  const { isAuthenticated } = useAuth();
  const queryClient = useQueryClient();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(() => {
    if (typeof window === "undefined") {
      return false;
    }
    return window.localStorage.getItem("xboard.sidebar.collapsed") === "true";
  });

  const noticeQuery = useQuery({
    queryKey: QUERY_KEYS.USER_NOTICE,
    queryFn: fetchUnreadNotice,
    enabled: isAuthenticated,
    refetchOnWindowFocus: false,
  });

  const markReadMutation = useMutation({
    mutationFn: (id: number) => markNoticeRead(id),
    onSuccess: () => {
      queryClient.setQueryData(QUERY_KEYS.USER_NOTICE, null);
    },
  });

  const notice = noticeQuery.data ?? null;
  const showNotice = Boolean(notice);

  const handleNoticeClose = () => {
    if (!notice) {
      return;
    }
    markReadMutation.mutate(notice.id);
  };

  const containerStyle = useMemo(
    () =>
      ({
        "--sidebar-width": sidebarCollapsed
          ? "var(--sidebar-width-collapsed)"
          : "var(--sidebar-width-expanded)",
      }) as React.CSSProperties,
    [sidebarCollapsed]
  );

  useEffect(() => {
    window.localStorage.setItem("xboard.sidebar.collapsed", String(sidebarCollapsed));
  }, [sidebarCollapsed]);

  return (
    <div className="h-dvh overflow-hidden bg-background" style={containerStyle}>
      <NoticePopup notice={notice} open={showNotice} onClose={handleNoticeClose} />
      <div className="flex h-full min-h-0">
        <Sidebar
          isOpen={sidebarOpen}
          onClose={() => setSidebarOpen(false)}
          isCollapsed={sidebarCollapsed}
          onToggleCollapsed={() => setSidebarCollapsed((previous) => !previous)}
        />

        <div className="flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
          <Header onMenuClick={() => setSidebarOpen(true)} />

          <main className="min-h-0 flex-1 overflow-y-auto overscroll-contain px-4 py-4 md:px-6 md:py-6 lg:px-8 lg:py-8">
            <div className="mx-auto w-full max-w-7xl">{children}</div>
          </main>
        </div>
      </div>
    </div>
  );
}
