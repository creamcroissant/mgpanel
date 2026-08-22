import { useTranslation } from "react-i18next";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Activity, FileText, RefreshCw } from "lucide-react";
import { getAgentConfigYAML, reportAgentConfig } from "@/api/admin";
import { Loading } from "@/components/ui";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";

interface AgentConfigEditorProps {
  agentHostId: number;
  agentName?: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export default function AgentConfigEditor({
  agentHostId,
  agentName,
  open,
  onOpenChange,
}: AgentConfigEditorProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();

  const { data: configYAML, isLoading, error, refetch, isRefetching } = useQuery({
    queryKey: ["agent-config-yaml", agentHostId],
    queryFn: () => getAgentConfigYAML(agentHostId),
    enabled: open,
  });

  const refreshMutation = useMutation({
    mutationFn: () => reportAgentConfig(agentHostId),
    onSuccess: () => {
      toast.success(t("admin.agents.config.refreshTriggered"));
      // After a short delay, refetch the config from Panel DB
      setTimeout(() => {
        queryClient.invalidateQueries({ queryKey: ["agent-config-yaml", agentHostId] });
        refetch();
      }, 3000);
    },
    onError: (err: Error) => {
      toast.error(t("admin.agents.config.refreshError"), { description: err.message });
    },
  });

  const isEmpty = !configYAML || configYAML.trim() === "";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-5xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Activity className="h-5 w-5" />
            {t("admin.agents.config.title")}
          </DialogTitle>
          <DialogDescription>
            {agentName
              ? t("admin.agents.config.description", { name: agentName })
              : t("admin.agents.config.subtitle")}
          </DialogDescription>
        </DialogHeader>

        <div className="flex items-center justify-end gap-2">
          <Button
            size="sm"
            variant="outline"
            onClick={() => refreshMutation.mutate()}
            disabled={isLoading || isRefetching || refreshMutation.isPending}
          >
            <RefreshCw className={`mr-1 h-3.5 w-3.5 ${refreshMutation.isPending ? "animate-spin" : ""}`} />
            {refreshMutation.isPending ? t("common.loading") : t("admin.agents.config.refreshFromAgent")}
          </Button>
        </div>

        {isLoading && <Loading />}

        {error && (
          <div className="py-10 text-center text-sm text-destructive">
            {t("admin.agents.config.loadError")}
          </div>
        )}

        {!isLoading && !error && isEmpty && (
          <div className="flex flex-col items-center gap-3 py-16 text-muted-foreground">
            <FileText className="h-12 w-12 opacity-30" />
            <p className="text-sm">{t("admin.agents.config.noReport")}</p>
            <p className="text-xs opacity-60">{t("admin.agents.config.noReportHint")}</p>
          </div>
        )}

        {!isLoading && !isEmpty && (
          <pre className="w-full max-h-[65vh] overflow-auto rounded-md border bg-background p-3 font-mono text-xs leading-relaxed whitespace-pre">
            {configYAML}
          </pre>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
