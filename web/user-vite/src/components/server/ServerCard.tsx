import { useTranslation } from "react-i18next";
import type { ServerNode } from "@/types";
import { Badge } from "@/components/ui";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { getCountryDisplayName, getCountryFlag, getServerStatus } from "@/lib/country";

interface ServerCardProps {
  server: ServerNode;
  className?: string;
  onClick?: () => void;
}

export function ServerCard({ server, className, onClick }: ServerCardProps) {
  const { t } = useTranslation();
  const flag = getCountryFlag(server.country, server.name);
  const location = getCountryDisplayName(server.country, server.region, t);
  const status = getServerStatus(server);

  const statusConfig = {
    1: {
      label: t("servers.status.online"),
      dotClass: "bg-emerald-500 shadow-[0_0_6px_rgba(16,185,129,0.5)]",
      textClass: "text-emerald-600 dark:text-emerald-400",
    },
    2: {
      label: t("servers.status.degraded"),
      dotClass: "bg-amber-500 shadow-[0_0_6px_rgba(245,158,11,0.5)]",
      textClass: "text-amber-600 dark:text-amber-400",
    },
    0: {
      label: t("servers.status.offline"),
      dotClass: "bg-rose-500 shadow-[0_0_6px_rgba(244,63,94,0.5)]",
      textClass: "text-rose-600 dark:text-rose-400",
    },
  }[status];

  return (
    <div
      data-testid="server-compact-card"
      onClick={onClick}
      className={`group relative flex items-center justify-between gap-3 rounded-none border border-border border-border bg-card p-3 shadow-sm transition-all duration-200 hover:border-primary/40 hover:bg-muted/40 hover:shadow-md ${
        className || ""
      }`}
    >
      {/* Left: Flag Emoji */}
      <div className="flex h-10 w-10 shrink-0 select-none items-center justify-center rounded-none bg-muted/60 text-2xl shadow-inner transition-transform group-hover:scale-105">
        <span role="img" aria-label="flag">
          {flag}
        </span>
      </div>

      {/* Center: Server Info */}
      <div className="min-w-0 flex-1">
        {/* Row 1: Server Name */}
        <div className="flex items-center gap-1.5">
          <span
            className="truncate text-sm font-semibold tracking-tight text-foreground"
            title={server.name}
          >
            {server.name}
          </span>
        </div>

        {/* Row 2: Location, Protocol, Rate */}
        <div className="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
          {location && (
            <span className="max-w-[110px] truncate text-[11px] font-medium text-muted-foreground/90">
              {location}
            </span>
          )}
          {location && <span className="text-[10px] text-muted-foreground/40">•</span>}
          <Badge
            variant="outline"
            className="h-4 px-1.5 py-0 text-[10px] font-medium uppercase tracking-wider text-muted-foreground"
          >
            {server.type}
          </Badge>
          <span className="text-[11px] font-medium text-muted-foreground/80">
            {server.rate}x
          </span>
        </div>
      </div>

      {/* Right: Status Indicator Dot + Tooltip */}
      <TooltipProvider delayDuration={200}>
        <Tooltip>
          <TooltipTrigger asChild>
            <div className="flex shrink-0 items-center justify-center p-1">
              <span className="relative flex h-2.5 w-2.5">
                {status === 1 && (
                  <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75" />
                )}
                <span
                  className={`relative inline-flex h-2.5 w-2.5 rounded-full ${statusConfig.dotClass}`}
                />
              </span>
            </div>
          </TooltipTrigger>
          <TooltipContent side="left" className="text-xs">
            <span className={`font-medium ${statusConfig.textClass}`}>
              {statusConfig.label}
            </span>
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>
    </div>
  );
}
