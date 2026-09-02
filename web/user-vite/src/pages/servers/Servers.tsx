import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { Search, Server, SlidersHorizontal } from "lucide-react";
import { fetchUserServers } from "@/api/server";
import { QUERY_KEYS } from "@/lib/constants";
import {
  Button,
  EmptyState,
  ErrorBanner,
  Input,
  Loading,
  PageShell,
  PageToolbar,
} from "@/components/ui";
import { ServerCard } from "@/components/server";
import { getServerStatus } from "@/lib/country";

const STATUS_FILTERS = ["all", "online", "degraded", "offline"] as const;
type StatusFilter = (typeof STATUS_FILTERS)[number];

export default function Servers() {
  const { t } = useTranslation();
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [typeFilter, setTypeFilter] = useState("all");
  const {
    data: servers = [],
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: QUERY_KEYS.SERVERS,
    queryFn: fetchUserServers,
  });

  const serverTypes = useMemo(() => {
    return Array.from(new Set(servers.map((server) => server.type).filter(Boolean))).sort();
  }, [servers]);

  const filteredServers = useMemo(() => {
    const keyword = search.trim().toLowerCase();
    return servers.filter((server) => {
      const serverStatus = getServerStatus(server);
      const matchesKeyword =
        !keyword ||
        server.name.toLowerCase().includes(keyword) ||
        server.type.toLowerCase().includes(keyword) ||
        (server.country && server.country.toLowerCase().includes(keyword)) ||
        (server.region && server.region.toLowerCase().includes(keyword)) ||
        (server.tags ?? []).some((tag) => tag.toLowerCase().includes(keyword));

      let matchesStatus = true;
      if (statusFilter === "online") {
        matchesStatus = serverStatus === 1;
      } else if (statusFilter === "degraded") {
        matchesStatus = serverStatus === 2;
      } else if (statusFilter === "offline") {
        matchesStatus = serverStatus === 0;
      }

      const matchesType = typeFilter === "all" || server.type === typeFilter;
      return matchesKeyword && matchesStatus && matchesType;
    });
  }, [search, servers, statusFilter, typeFilter]);

  if (isLoading) return <Loading />;
  if (error) return <ErrorBanner message={t("error.loadServers")} onRetry={refetch} />;

  const showEmptyInventory = servers.length === 0;
  const showEmptyFilter = servers.length > 0 && filteredServers.length === 0;

  return (
    <PageShell
      data-testid="server-resource-browser"
      title={t("servers.title")}
      description={t("servers.subtitle")}
      actions={
        <Button variant="outline" onClick={() => refetch()}>
          {t("common.refresh")}
        </Button>
      }
    >
      <PageToolbar
        data-testid="server-filter-toolbar"
        leading={
          <div className="relative min-w-0 flex-1 md:max-w-sm">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              type="search"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder={t("servers.searchPlaceholder")}
              aria-label={t("servers.searchPlaceholder")}
              className="pl-9"
            />
          </div>
        }
        filters={
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            {STATUS_FILTERS.map((filter) => (
              <Button
                key={filter}
                type="button"
                variant={statusFilter === filter ? "default" : "outline"}
                size="sm"
                onClick={() => setStatusFilter(filter)}
                className="gap-1.5"
              >
                {filter === "online" && (
                  <span className="h-2 w-2 rounded-full bg-emerald-500" />
                )}
                {filter === "degraded" && (
                  <span className="h-2 w-2 rounded-full bg-amber-500" />
                )}
                {filter === "offline" && (
                  <span className="h-2 w-2 rounded-full bg-rose-500" />
                )}
                {t(`servers.filters.${filter}`)}
              </Button>
            ))}
          </div>
        }
        actions={
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <span className="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground">
              <SlidersHorizontal className="h-3.5 w-3.5" />
              {t("servers.typeFilter")}
            </span>
            <Button
              type="button"
              variant={typeFilter === "all" ? "default" : "outline"}
              size="sm"
              onClick={() => setTypeFilter("all")}
            >
              {t("common.all")}
            </Button>
            {serverTypes.map((type) => (
              <Button
                key={type}
                type="button"
                variant={typeFilter === type ? "default" : "outline"}
                size="sm"
                onClick={() => setTypeFilter(type)}
              >
                {type}
              </Button>
            ))}
          </div>
        }
      />

      {showEmptyInventory ? (
        <EmptyState
          icon={<Server className="h-full w-full" />}
          title={t("servers.noServers")}
          description={t("servers.noServersHint")}
          size="lg"
        />
      ) : showEmptyFilter ? (
        <EmptyState
          icon={<Search className="h-full w-full" />}
          title={t("servers.noMatchedServers")}
          description={t("servers.noMatchedServersHint")}
          size="lg"
        />
      ) : (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {filteredServers.map((server) => (
            <ServerCard key={server.id} server={server} />
          ))}
        </div>
      )}
    </PageShell>
  );
}
