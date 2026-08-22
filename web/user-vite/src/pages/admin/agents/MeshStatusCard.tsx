import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { Network, Wifi, WifiOff, ExternalLink } from "lucide-react";
import { fetchAgentMeshStatus, fetchMeshStatus } from "@/api/admin/mesh";
import { Badge, Card, CardContent, CardHeader, CardTitle } from "@/components/ui";
import { Loading } from "@/components/ui";
import { formatDateTime } from "@/lib/format";

interface MeshStatusCardProps {
  agentHostId: number;
}

export function MeshStatusCard({ agentHostId }: MeshStatusCardProps) {
  const { t } = useTranslation();

  // Fetch this agent's mesh peer status
  const peerQuery = useQuery({
    queryKey: ["agent-mesh-peer", agentHostId],
    queryFn: () => fetchAgentMeshStatus(agentHostId),
  });

  // Fetch all mesh peers for count/overview
  const allPeersQuery = useQuery({
    queryKey: ["agent-mesh-peers", agentHostId],
    queryFn: () => fetchMeshStatus(),
    enabled: peerQuery.data !== null && peerQuery.data !== undefined,
  });

  const peer = peerQuery.data;
  const allPeers = allPeersQuery.data?.data ?? [];
  const totalPeers = allPeers.length;
  const isLoading = peerQuery.isLoading;

  const errorTitle = t("admin.agents.mesh.title");
  const cardClass = "border border-border shadow-none";

  if (isLoading) {
    return (
      <Card className={cardClass}>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-sm font-semibold">
            <Network className="h-4 w-4" />
            {errorTitle}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <Loading />
        </CardContent>
      </Card>
    );
  }

  if (peerQuery.error) {
    return (
      <Card className={cardClass}>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-sm font-semibold">
            <Network className="h-4 w-4" />
            {errorTitle}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-destructive">{t("admin.agents.meshTable.loadError")}</p>
          <p className="mt-1 text-xs text-muted-foreground">{(peerQuery.error as Error)?.message || ""}</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className={cardClass}>
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-sm font-semibold">
          <Network className="h-4 w-4" />
          {errorTitle}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {!peer ? (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <WifiOff className="h-4 w-4" />
            <span>{t("admin.agents.mesh.notJoined")}</span>
          </div>
        ) : (
          <>
            {/* Joined status */}
            <div className="flex items-center justify-between rounded-md bg-muted/40 p-3">
              <div className="flex items-center gap-2">
                <Wifi className="h-4 w-4 text-emerald-500" />
                <span className="text-sm font-medium">{t("admin.agents.mesh.joined")}</span>
              </div>
              <Badge variant="success">{t("admin.agents.mesh.active")}</Badge>
            </div>

            {/* WG IP */}
            <div className="space-y-1 rounded-md bg-muted/40 p-3">
              <p className="text-xs text-muted-foreground">{t("admin.agents.mesh.wgIp")}</p>
              <p className="font-mono text-sm font-medium">{peer.wg_ip || "-"}</p>
            </div>

            {/* WG Public Key (truncated) */}
            <div className="space-y-1 rounded-md bg-muted/40 p-3">
              <p className="text-xs text-muted-foreground">{t("admin.agents.mesh.publicKey")}</p>
              <p className="truncate font-mono text-xs" title={peer.wg_public_key}>
                {peer.wg_public_key || "-"}
              </p>
            </div>

            {/* Listen Port */}
            <div className="flex items-center justify-between rounded-md bg-muted/40 px-3 py-2">
              <span className="text-xs text-muted-foreground">{t("admin.agents.mesh.listenPort")}</span>
              <span className="font-mono text-sm">{peer.wg_listen_port || 51820}</span>
            </div>

            {/* Network peers */}
            <div
              className="flex cursor-pointer items-center justify-between rounded-md bg-muted/40 px-3 py-2 transition-colors hover:bg-muted/60"
              onClick={() => {
                // Could open a peer list dialog in the future
              }}
              role="button"
              tabIndex={0}
            >
              <span className="text-xs text-muted-foreground">{t("admin.agents.mesh.networkPeers")}</span>
              <div className="flex items-center gap-2">
                <span className="font-medium">{totalPeers > 0 ? totalPeers : 1}</span>
                {totalPeers > 0 && (
                  <span className="text-xs text-muted-foreground">
                    ({totalPeers - 1} {t("admin.agents.mesh.others")})
                  </span>
                )}
                <ExternalLink className="h-3 w-3 text-muted-foreground" />
              </div>
            </div>
            {allPeersQuery.isError && (
              <div className="text-red-500 text-sm mt-1">{t("admin.agents.meshTable.loadPeersFailed") || "加载组网节点失败"}</div>
            )}

            {/* Created / Updated */}
            <div className="flex flex-col gap-1 text-xs text-muted-foreground">
              {peer.created_at > 0 && (
                <span>{t("admin.agents.mesh.joinedAt")}: {formatDateTime(peer.created_at)}</span>
              )}
              {peer.updated_at > 0 && (
                <span>{t("admin.agents.mesh.updatedAt")}: {formatDateTime(peer.updated_at)}</span>
              )}
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}
