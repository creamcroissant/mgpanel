import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Activity, FileText, Pencil, RefreshCw } from "lucide-react";
import { getAgentConfigYAML, reportAgentConfig } from "@/api/admin";
import { updateAgentConfig } from "@/api/admin/agentConfig";
import { Loading, Textarea } from "@/components/ui";
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

// After PUT, the agent restarts and reports the new config.
// Poll GET /config until content changes: every 3s, up to 60s.
const POLL_INTERVAL_MS = 3000;
const POLL_TIMEOUT_MS = 60000;

export default function AgentConfigEditor({
  agentHostId,
  agentName,
  open,
  onOpenChange,
}: AgentConfigEditorProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [isPolling, setIsPolling] = useState(false);
  const pollCancelledRef = useRef(false);

  const { data: configYAML, isLoading, error, refetch, isRefetching } = useQuery({
    queryKey: ["agent-config-yaml", agentHostId],
    queryFn: () => getAgentConfigYAML(agentHostId),
    enabled: open,
  });

  // Reset edit state when switching to another agent host.
  useEffect(() => {
    setEditing(false);
    setDraft("");
    setConfirmOpen(false);
  }, [agentHostId]);

  // Stop polling when the dialog unmounts.
  useEffect(() => {
    return () => {
      pollCancelledRef.current = true;
    };
  }, []);

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

  const pollForChange = async (before: string) => {
    setIsPolling(true);
    pollCancelledRef.current = false;
    const deadline = Date.now() + POLL_TIMEOUT_MS;
    try {
      while (Date.now() < deadline) {
        await new Promise((resolve) => setTimeout(resolve, POLL_INTERVAL_MS));
        if (pollCancelledRef.current) return;
        let latest: string;
        try {
          latest = await getAgentConfigYAML(agentHostId);
        } catch {
          // Transient failure: keep polling until timeout.
          continue;
        }
        if (pollCancelledRef.current) return;
        if (latest !== before) {
          queryClient.invalidateQueries({ queryKey: ["agent-config-yaml", agentHostId] });
          setEditing(false);
          toast.success(t("admin.agents.config.saveSuccess"));
          return;
        }
      }
      toast.error(t("admin.agents.config.pollTimeout"));
    } finally {
      if (!pollCancelledRef.current) setIsPolling(false);
    }
  };

  const saveMutation = useMutation({
    mutationFn: (yaml: string) => updateAgentConfig(agentHostId, yaml),
    onSuccess: () => {
      setConfirmOpen(false);
      void pollForChange(configYAML ?? "");
    },
    onError: (err: Error) => {
      setConfirmOpen(false);
      toast.error(t("admin.agents.config.saveError"), { description: err.message });
    },
  });

  const isEmpty = !configYAML || configYAML.trim() === "";
  const busy = saveMutation.isPending || isPolling;

  const startEditing = () => {
    setDraft(configYAML ?? "");
    setEditing(true);
  };

  const cancelEditing = () => {
    setDraft("");
    setEditing(false);
  };

  const handleOpenChange = (next: boolean) => {
    if (!next) {
      pollCancelledRef.current = true;
      setIsPolling(false);
      setEditing(false);
      setDraft("");
      setConfirmOpen(false);
    }
    onOpenChange(next);
  };

  return (
    <>
    <Dialog open={open} onOpenChange={handleOpenChange}>
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
          {!editing ? (
            <Button
              size="sm"
              variant="outline"
              onClick={startEditing}
              disabled={isLoading || !!error}
            >
              <Pencil className="mr-1 h-3.5 w-3.5" />
              {t("admin.agents.config.edit")}
            </Button>
          ) : (
            <>
              <Button size="sm" variant="outline" onClick={cancelEditing} disabled={busy}>
                {t("admin.agents.config.cancel")}
              </Button>
              <Button size="sm" onClick={() => setConfirmOpen(true)} disabled={busy}>
                {busy ? t("common.loading") : t("admin.agents.config.save")}
              </Button>
            </>
          )}
          <Button
            size="sm"
            variant="outline"
            onClick={() => refreshMutation.mutate()}
            disabled={isLoading || isRefetching || refreshMutation.isPending || busy}
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

        {!isLoading && !error && !editing && isEmpty && (
          <div className="flex flex-col items-center gap-3 py-16 text-muted-foreground">
            <FileText className="h-12 w-12 opacity-30" />
            <p className="text-sm">{t("admin.agents.config.noReport")}</p>
            <p className="text-xs opacity-60">{t("admin.agents.config.noReportHint")}</p>
          </div>
        )}

        {!isLoading && !error && editing && (
          <Textarea
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            disabled={busy}
            spellCheck={false}
            className="max-h-[65vh] min-h-[40vh] font-mono text-xs leading-relaxed"
          />
        )}

        {!isLoading && !error && !editing && !isEmpty && (
          <pre className="w-full max-h-[65vh] overflow-auto rounded-none border border-border bg-background p-3 font-mono text-xs leading-relaxed whitespace-pre">
            {configYAML}
          </pre>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={() => handleOpenChange(false)}>
            {t("common.close")}
          </Button>
        </DialogFooter>
      </DialogContent>
      </Dialog>

      <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("admin.agents.config.save")}</DialogTitle>
            <DialogDescription>{t("admin.agents.config.restartWarn")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setConfirmOpen(false)}
              disabled={saveMutation.isPending}
            >
              {t("admin.agents.config.cancel")}
            </Button>
            <Button
              onClick={() => saveMutation.mutate(draft)}
              disabled={saveMutation.isPending}
            >
              {saveMutation.isPending ? t("common.loading") : t("admin.agents.config.save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
