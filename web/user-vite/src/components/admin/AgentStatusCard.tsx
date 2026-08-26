import { useTranslation } from "react-i18next";
import { ArrowDown, ArrowUp, Clock, Cpu, FileText, HardDrive, MemoryStick, Pencil, RotateCw } from "lucide-react";
import { AgentStatus, type AgentHost } from "@/types";
import ResourceGauge from "./ResourceGauge";
import { formatBytes } from "@/lib/format";
import { Card, CardContent, CardHeader } from "@/components/ui";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

interface AgentStatusCardProps {
  agent: AgentHost;
  onClick?: () => void;
  onEdit?: () => void;
  onViewConfig?: () => void;
}

function formatRelativeTime(
  timestamp: number,
  t: (key: string, options?: Record<string, unknown>) => string
): string {
  if (!timestamp) return t("admin.agents.never");

  const now = Math.floor(Date.now() / 1000);
  const diff = now - timestamp;

  if (diff < 0) return t("admin.agents.justNow");
  if (diff < 60) return t("admin.agents.secondsAgo", { count: diff });
  if (diff < 3600) return t("admin.agents.minutesAgo", { count: Math.floor(diff / 60) });
  if (diff < 86400) return t("admin.agents.hoursAgo", { count: Math.floor(diff / 3600) });
  return t("admin.agents.daysAgo", { count: Math.floor(diff / 86400) });
}

function getStatusConfig(status: AgentStatus, t: (key: string) => string) {
  switch (status) {
    case AgentStatus.Online:
      return { variant: "success" as const, label: t("admin.agents.status.online") };
    case AgentStatus.Warning:
      return { variant: "warning" as const, label: t("admin.agents.status.warning") };
    case AgentStatus.Offline:
    default:
      return { variant: "danger" as const, label: t("admin.agents.status.offline") };
  }
}

const hasRealtimeReport = (agent: AgentHost): boolean => Boolean(agent.last_realtime_report_at && agent.last_realtime_report_at > 0);

const formatRate = (value: number | undefined, hasRealtime: boolean, t: (key: string) => string): string => {
  if (!hasRealtime || value === undefined) {
    return t("admin.agents.unknown");
  }
  return `${formatBytes(value)}/s`;
};

const normalizeLabel = (value: string | undefined, fallback: string): string => {
  const trimmed = value?.trim();
  return trimmed || fallback;
};

export default function AgentStatusCard({ agent, onClick, onEdit, onViewConfig }: AgentStatusCardProps) {
  const { t } = useTranslation();
  const statusConfig = getStatusConfig(agent.status, t);
  const hostLabel = agent.port ? `${agent.host}:${agent.port}` : agent.host;
  const hasRealtime = hasRealtimeReport(agent);
  const realtimeLabel = hasRealtime ? t("admin.agents.realtimeCurrent") : t("admin.agents.lastKnown");
  const agentVersion = normalizeLabel(agent.agent_version, t("admin.agents.unknown"));
  const coreLabel = normalizeLabel(agent.current_core_type, t("admin.agents.unknown"));
  const coreVersion = normalizeLabel(agent.core_version, "");
  const coreSummary = coreVersion ? `${coreLabel} ${coreVersion}` : coreLabel;

  return (
    <Card
      className="transition-colors hover:border-primary/30"
      onClick={onClick}
      role={onClick ? "button" : undefined}
      tabIndex={onClick ? 0 : undefined}
      data-testid={onClick ? "admin-agent-card" : undefined}
    >
      <CardHeader className="gap-4 pb-0">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0 space-y-2">
            <div className="flex items-center gap-2">
              <span
                className={`h-2.5 w-2.5 rounded-full ${
                  agent.status === AgentStatus.Online
                    ? "bg-success"
                    : agent.status === AgentStatus.Warning
                      ? "bg-warning"
                      : "bg-muted-foreground"
                }`}
              />
              <span className="truncate text-base font-semibold text-foreground">{agent.name}</span>
            </div>
            <span className="block truncate text-sm text-muted-foreground">{hostLabel}</span>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {onViewConfig && (
              <Button
                type="button"
                size="icon"
                variant="ghost"
                className="h-8 w-8"
                aria-label={t("admin.agents.config.title")}
                onClick={(event) => {
                  event.stopPropagation();
                  onViewConfig();
                }}
              >
                <FileText className="h-4 w-4" />
              </Button>
            )}
            {onEdit && (
              <Button
                type="button"
                size="icon"
                variant="ghost"
                className="h-8 w-8"
                aria-label={t("common.edit")}
                onClick={(event) => {
                  event.stopPropagation();
                  onEdit();
                }}
              >
                <Pencil className="h-4 w-4" />
              </Button>
            )}
            <Badge variant={statusConfig.variant}>{statusConfig.label}</Badge>
          </div>
        </div>
      </CardHeader>

      <CardContent className="space-y-5 pt-5">
        <div className="space-y-4">
          <div className="flex items-start gap-3 rounded-md bg-muted/40 p-3">
            <Cpu className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
            <div className="min-w-0 flex-1">
              <ResourceGauge label={t("admin.agents.cpu")} used={agent.cpu_used} total={100} unit="%" showPercentage={false} />
            </div>
          </div>

          <div className="flex items-start gap-3 rounded-md bg-muted/40 p-3">
            <MemoryStick className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
            <div className="min-w-0 flex-1">
              <ResourceGauge label={t("admin.agents.memory")} used={agent.mem_used} total={agent.mem_total} unit="bytes" />
            </div>
          </div>

          <div className="flex items-start gap-3 rounded-md bg-muted/40 p-3">
            <HardDrive className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
            <div className="min-w-0 flex-1">
              <ResourceGauge label={t("admin.agents.disk")} used={agent.disk_used} total={agent.disk_total} unit="bytes" />
            </div>
          </div>
        </div>

        <div className="space-y-3 rounded-md border bg-muted/20 p-3">
          <div className="flex items-center justify-between gap-3">
            <span className="text-xs font-medium text-muted-foreground">{t("admin.agents.realtimeTraffic")}</span>
            <Badge variant={hasRealtime ? "success" : "outline"}>{realtimeLabel}</Badge>
          </div>
          <div className="grid grid-cols-2 gap-3 text-sm">
            <div className="rounded-md border bg-background p-3">
              <div className="flex items-center gap-2 text-success">
                <ArrowUp className="h-4 w-4" />
                <span className="text-xs font-medium uppercase tracking-[0.08em]">{t("traffic.upload")}</span>
              </div>
              <p className="mt-2 font-semibold text-foreground">{formatRate(agent.upload_rate_bps, hasRealtime, t)}</p>
              <p className="mt-1 text-xs text-muted-foreground">{t("admin.agents.totalTraffic", { value: formatBytes(agent.upload_total) })}</p>
            </div>
            <div className="rounded-md border bg-background p-3">
              <div className="flex items-center gap-2 text-primary">
                <ArrowDown className="h-4 w-4" />
                <span className="text-xs font-medium uppercase tracking-[0.08em]">{t("traffic.download")}</span>
              </div>
              <p className="mt-2 font-semibold text-foreground">{formatRate(agent.download_rate_bps, hasRealtime, t)}</p>
              <p className="mt-1 text-xs text-muted-foreground">{t("admin.agents.totalTraffic", { value: formatBytes(agent.download_total) })}</p>
            </div>
          </div>
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <Clock className="h-3.5 w-3.5" />
            <span>
              {t("admin.agents.lastRealtimeReport")}: {formatRelativeTime(agent.last_realtime_report_at ?? 0, t)}
            </span>
          </div>
        </div>

        <div className="grid gap-2 rounded-md border bg-background p-3 text-sm sm:grid-cols-2">
          <div>
            <p className="text-xs text-muted-foreground">{t("admin.agents.agentVersion")}</p>
            <p className="mt-1 truncate font-medium text-foreground">{agentVersion}</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">{t("admin.agents.currentCore")}</p>
            <p className="mt-1 truncate font-medium text-foreground">{coreSummary}</p>
          </div>
        </div>

        {agent.last_restart_at != null && agent.last_restart_at > 0 && (
          <div className="flex items-center gap-2 rounded-md border border-warning/30 bg-warning/10 p-3 text-xs text-warning-foreground/90 dark:text-warning">
            <RotateCw className="h-3.5 w-3.5" />
            <span>{t("admin.agents.restartDetected", { time: formatRelativeTime(agent.last_restart_at, t) })}</span>
          </div>
        )}

        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <Clock className="h-3.5 w-3.5" />
          <span>
            {t("admin.agents.lastHeartbeat")}: {formatRelativeTime(agent.last_heartbeat_at, t)}
          </span>
        </div>
      </CardContent>
    </Card>
  );
}
