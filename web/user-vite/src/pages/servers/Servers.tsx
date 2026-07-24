import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { Search, Server, SlidersHorizontal, Tag } from "lucide-react";
import { fetchUserServers } from "@/api/server";
import { QUERY_KEYS } from "@/lib/constants";
import {
  Badge,
  Button,
  EmptyState,
  ErrorBanner,
  Input,
  Loading,
  PageShell,
  PageToolbar,
  ResourceCard,
  ResponsiveGrid,
} from "@/components/ui";

const STATUS_FILTERS = ["all", "online", "offline"] as const;
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
      const matchesKeyword =
        !keyword ||
        server.name.toLowerCase().includes(keyword) ||
        server.type.toLowerCase().includes(keyword) ||
        (server.tags ?? []).some((tag) => tag.toLowerCase().includes(keyword));
      const matchesStatus =
        statusFilter === "all" ||
        (statusFilter === "online" ? server.is_online === true : server.is_online !== true);
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
              >
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
        <ResponsiveGrid minColWidth={280} gap={16}>
          {filteredServers.map((server) => (
            <ResourceCard
              key={server.id}
              data-testid="server-resource-card"
              icon={<Server className="h-5 w-5" />}
              title={server.name}
              description={t("servers.cardDescription", { type: server.type, rate: server.rate })}
              status={
                <Badge variant={server.is_online ? "success" : "danger"}>
                  {server.is_online ? t("servers.online") : t("servers.offline")}
                </Badge>
              }
              meta={
                <>
                  <Badge variant="outline">{server.type}</Badge>
                  <Badge variant="secondary">{t("servers.rateValue", { rate: server.rate })}</Badge>
                </>
              }
              footer={
                server.tags && server.tags.length > 0 ? (
                  <div className="flex min-w-0 flex-wrap gap-1.5">
                    {server.tags.map((tag) => (
                      <span
                        key={tag}
                        className="inline-flex min-w-0 items-center gap-1 rounded-md border bg-muted/30 px-2 py-1 text-xs text-muted-foreground"
                      >
                        <Tag className="h-3 w-3 shrink-0" />
                        <span className="break-all">{tag}</span>
                      </span>
                    ))}
                  </div>
                ) : (
                  t("servers.noTags")
                )
              }
            />
          ))}
        </ResponsiveGrid>
      )}
    </PageShell>
  );
}
