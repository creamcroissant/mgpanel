import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import {
  LayoutList,
  Network,
  RefreshCw,
  Waypoints,
  Wifi,
  WifiOff,
} from "lucide-react";
import { fetchMeshStatus, type MeshPeer } from "@/api/admin/mesh";
import { QUERY_KEYS } from "@/lib/constants";
import { formatDateTime } from "@/lib/format";
import { MeshTopologyView } from "./MeshTopologyView";
import {
  Badge,
  Button,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  EmptyState,
  Loading,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui";

interface MeshPeerTableProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onRefetch?: () => void;
  /** agent_host_id → 显示名（探针页已加载的 agent 列表） */
  nameById?: Record<number, string>;
}

export function MeshPeerTable({ open, onOpenChange, onRefetch, nameById }: MeshPeerTableProps) {
  const { t } = useTranslation();

  const meshQuery = useQuery({
    queryKey: [...QUERY_KEYS.ADMIN_SYSTEM, "mesh", "peers"],
    queryFn: () => fetchMeshStatus(),
    enabled: open,
    refetchInterval: open ? 15000 : false,
    placeholderData: keepPreviousData,
  });

  const peers = useMemo(() => meshQuery.data?.data ?? [], [meshQuery.data]);

  const onlineCount = useMemo(
    () => peers.filter((p) => p.online).length,
    [peers]
  );

  const [view, setView] = useState<"table" | "topo">("table");

  const handleRefresh = () => {
    meshQuery.refetch();
    onRefetch?.();
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent aria-describedby={undefined} className="sm:max-w-5xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Network className="h-5 w-5" />
            {t("admin.agents.meshTable.title")}
          </DialogTitle>
        </DialogHeader>

        <div className="flex max-h-[calc(100dvh-12rem)] flex-col gap-4 overflow-y-auto py-2 pr-1">
          {/* Summary bar */}
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-4 text-sm text-muted-foreground">
              <span>
                {t("admin.agents.meshTable.total")}: <strong className="text-foreground">{peers.length}</strong>
              </span>
              <span className="flex items-center gap-1">
                <Wifi className="h-3.5 w-3.5 text-success" />
                {t("admin.agents.meshTable.online")}: <strong className="text-foreground">{onlineCount}</strong>
              </span>
              <span className="flex items-center gap-1">
                <WifiOff className="h-3.5 w-3.5 text-muted-foreground" />
                {t("admin.agents.meshTable.offline")}: <strong className="text-muted-foreground">{peers.length - onlineCount}</strong>
              </span>
            </div>
            <div className="flex items-center gap-1 rounded-md border p-0.5">
              <button
                type="button"
                onClick={() => setView("table")}
                className={`flex items-center gap-1 rounded px-2 py-1 text-xs font-medium transition-colors ${
                  view === "table"
                    ? "bg-card text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground"
                }`}
              >
                <LayoutList className="h-3.5 w-3.5" />
                {t("admin.agents.meshTable.view.table")}
              </button>
              <button
                type="button"
                onClick={() => setView("topo")}
                className={`flex items-center gap-1 rounded px-2 py-1 text-xs font-medium transition-colors ${
                  view === "topo"
                    ? "bg-card text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground"
                }`}
              >
                <Waypoints className="h-3.5 w-3.5" />
                {t("admin.agents.meshTable.view.topology")}
              </button>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={handleRefresh}
              disabled={meshQuery.isFetching}
            >
              <RefreshCw className={`mr-1 h-3.5 w-3.5 ${meshQuery.isFetching ? "animate-spin" : ""}`} />
              {t("common.refresh")}
            </Button>
          </div>

          {/* Table */}
          {meshQuery.isLoading ? (
            <Loading />
          ) : meshQuery.error ? (
            <div className="flex flex-col items-center justify-center gap-3 py-10">
              <p className="text-sm text-destructive">{t("admin.agents.meshTable.loadError")}</p>
              <p className="max-w-md text-center text-xs text-muted-foreground">
                {(meshQuery.error as Error)?.message || ""}
              </p>
              <Button variant="outline" onClick={handleRefresh}>
                {t("common.retry")}
              </Button>
            </div>
          ) : peers.length === 0 ? (
            <EmptyState
              icon={<Network className="h-full w-full" />}
              title={t("admin.agents.meshTable.emptyTitle")}
              description={t("admin.agents.meshTable.emptyDescription")}
              size="sm"
            />
          ) : view === "topo" ? (
            <MeshTopologyView peers={peers} nameById={nameById} />
          ) : (
            <div>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-20">{t("admin.agents.meshTable.status")}</TableHead>
                    <TableHead>{t("admin.agents.meshTable.wgIp")}</TableHead>
                    <TableHead>{t("admin.agents.meshTable.publicKey")}</TableHead>
                    <TableHead>{t("admin.agents.meshTable.listenPort")}</TableHead>
                    <TableHead>{t("admin.agents.meshTable.networkId")}</TableHead>
                    <TableHead>{t("admin.agents.meshTable.latency")}</TableHead>
                    <TableHead>{t("admin.agents.meshTable.packetLoss")}</TableHead>
                    <TableHead>{t("admin.agents.meshTable.joinedAt")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {peers.map((peer: MeshPeer) => (
                    <TableRow key={peer.id}>
                      <TableCell>
                        <Badge
                          variant={peer.online ? "success" : "secondary"}
                          className="whitespace-nowrap"
                        >
                          <div className="flex items-center gap-1">
                            <div
                              className={`h-1.5 w-1.5 rounded-full ${
                                peer.online ? "bg-success" : "bg-muted-foreground"
                              }`}
                            />
                            {peer.online
                              ? t("admin.agents.meshTable.onlineLabel")
                              : t("admin.agents.meshTable.offlineLabel")}
                          </div>
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <span className="font-mono text-xs">{peer.wg_ip}</span>
                      </TableCell>
                      <TableCell>
                        <span
                          className="block max-w-[200px] truncate font-mono text-xs"
                          title={peer.wg_public_key}
                        >
                          {peer.wg_public_key}
                        </span>
                      </TableCell>
                      <TableCell>
                        <span className="font-mono text-xs">{peer.wg_listen_port}</span>
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline">{peer.network_id}</Badge>
                      </TableCell>
                      <TableCell>
                        {peer.latency_ms != null ? (
                          <Badge
                            variant={peer.latency_ms < 50 ? "success" : peer.latency_ms < 150 ? "warning" : "danger"}
                            className="font-mono"
                          >
                            {peer.latency_ms} ms
                          </Badge>
                        ) : (
                          <Badge variant="secondary">-</Badge>
                        )}
                      </TableCell>
                      <TableCell>
                        {peer.packet_loss != null ? (
                          <Badge
                            variant={peer.packet_loss < 10 ? "success" : peer.packet_loss < 30 ? "warning" : "danger"}
                            className="font-mono"
                          >
                            {peer.packet_loss}%
                          </Badge>
                        ) : (
                          <Badge variant="secondary">-</Badge>
                        )}
                      </TableCell>
                      <TableCell className="whitespace-nowrap text-xs text-muted-foreground">
                        {formatDateTime(peer.created_at)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
