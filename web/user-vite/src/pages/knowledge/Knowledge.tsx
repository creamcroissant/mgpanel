import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { BookOpen, FileText, RefreshCw, Search } from "lucide-react";
import { fetchKnowledgeArticles } from "@/api/knowledge";
import type { KnowledgeArticle } from "@/types";
import {
  Badge,
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  EmptyState,
  ErrorBanner,
  Input,
  Loading,
  PageShell,
  PageToolbar,
  ResourceCard,
  SectionCard,
} from "@/components/ui";
import { QUERY_KEYS } from "@/lib/constants";
import { formatDate } from "@/lib/format";

const ALL_CATEGORIES = "__all__";

function normalizeText(value: string): string {
  return value.replace(/\u00a0/g, " ").replace(/[ \t]+/g, " ").replace(/\n{3,}/g, "\n\n").trim();
}

function decodeEntities(value: string): string {
  // Use DOMParser instead of innerHTML assignment to avoid XSS from entity
  // decoding via HTML parsing. DOMParser separates HTML parsing from the DOM tree.
  if (typeof document === "undefined" || typeof DOMParser === "undefined") {
    return value;
  }
  const doc = new DOMParser().parseFromString(value, "text/html");
  return doc.body.textContent || value;
}

function articleText(body?: string): string {
  if (!body) return "";

  const text = body
    .replace(/<script[\s\S]*?<\/script>/gi, "")
    .replace(/<style[\s\S]*?<\/style>/gi, "")
    .replace(/<br\s*\/?>/gi, "\n")
    .replace(/<\/(p|div|section|article|h[1-6]|li|ul|ol|blockquote)>/gi, "\n\n")
    .replace(/<[^>]*>/g, " ");

  return normalizeText(decodeEntities(text));
}

function previewText(body?: string): string {
  const text = articleText(body);
  if (!text) return "";
  return text.length > 150 ? `${text.slice(0, 150)}...` : text;
}

function categoryOf(article: KnowledgeArticle, fallback: string): string {
  return article.category?.trim() || fallback;
}

export default function Knowledge() {
  const { t, i18n } = useTranslation();
  const [selectedCategory, setSelectedCategory] = useState(ALL_CATEGORIES);
  const [search, setSearch] = useState("");
  const [selectedArticle, setSelectedArticle] = useState<KnowledgeArticle | null>(null);

  const {
    data: groupedArticles,
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: [...QUERY_KEYS.KNOWLEDGE, i18n.language],
    queryFn: () => fetchKnowledgeArticles(i18n.language),
    refetchInterval: 30000,
    refetchOnWindowFocus: true,
    staleTime: 10000,
  });

  const fallbackCategory = t("knowledge.uncategorized");
  const articles = useMemo(() => {
    if (!groupedArticles) return [];
    return Object.values(groupedArticles)
      .flat()
      .sort((left, right) => (left.sort ?? 0) - (right.sort ?? 0) || left.id - right.id);
  }, [groupedArticles]);

  const categories = useMemo(() => {
    return Array.from(new Set(articles.map((article) => categoryOf(article, fallbackCategory))));
  }, [articles, fallbackCategory]);

  const filteredArticles = useMemo(() => {
    const keyword = search.trim().toLowerCase();
    return articles.filter((article) => {
      const category = categoryOf(article, fallbackCategory);
      const matchesCategory = selectedCategory === ALL_CATEGORIES || category === selectedCategory;
      const matchesKeyword =
        !keyword ||
        article.title.toLowerCase().includes(keyword) ||
        category.toLowerCase().includes(keyword) ||
        articleText(article.body).toLowerCase().includes(keyword);
      return matchesCategory && matchesKeyword;
    });
  }, [articles, fallbackCategory, search, selectedCategory]);

  if (isLoading) return <Loading />;
  if (error) return <ErrorBanner message={t("error.loadKnowledge")} onRetry={refetch} />;

  const selectedArticleText = articleText(selectedArticle?.body);
  const selectedArticleParagraphs = selectedArticleText ? selectedArticleText.split(/\n{2,}/).filter(Boolean) : [];

  return (
    <PageShell
      data-testid="knowledge-browser"
      title={t("knowledge.title")}
      description={t("knowledge.subtitle")}
      actions={
        <Button variant="outline" className="gap-2" onClick={() => refetch()}>
          <RefreshCw className="h-4 w-4" />
          {t("common.refresh")}
        </Button>
      }
    >
      <PageToolbar
        data-testid="knowledge-category-filter"
        leading={
          <div className="relative min-w-0 flex-1 md:max-w-sm">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              type="search"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder={t("knowledge.searchPlaceholder")}
              className="h-10 pl-9"
            />
          </div>
        }
        filters={
          <div className="flex min-w-0 flex-wrap items-center gap-2" aria-label={t("knowledge.categoryFilter")}>
            <Button
              type="button"
              variant={selectedCategory === ALL_CATEGORIES ? "default" : "outline"}
              size="sm"
              onClick={() => setSelectedCategory(ALL_CATEGORIES)}
            >
              {t("knowledge.allCategories")}
            </Button>
            {categories.map((category) => (
              <Button
                key={category}
                type="button"
                variant={selectedCategory === category ? "default" : "outline"}
                size="sm"
                onClick={() => setSelectedCategory(category)}
              >
                {category}
              </Button>
            ))}
          </div>
        }
      />

      {articles.length === 0 ? (
        <SectionCard title={t("knowledge.title")} description={t("knowledge.subtitle")}>
          <EmptyState icon={<BookOpen className="h-full w-full" />} title={t("knowledge.noArticles")} description={t("knowledge.noArticlesHint")} />
        </SectionCard>
      ) : filteredArticles.length === 0 ? (
        <SectionCard title={t("knowledge.noMatches")} description={t("knowledge.noMatchesHint")}>
          <EmptyState icon={<BookOpen className="h-full w-full" />} title={t("knowledge.noMatches")} description={t("knowledge.noMatchesHint")} />
        </SectionCard>
      ) : (
        <div className="grid min-w-0 gap-3 md:grid-cols-2 xl:grid-cols-3">
          {filteredArticles.map((article) => {
            const category = categoryOf(article, fallbackCategory);
            return (
              <ResourceCard
                key={article.id}
                data-testid="knowledge-article-card"
                icon={<FileText className="h-5 w-5" />}
                title={article.title}
                description={previewText(article.body) || t("knowledge.noBody")}
                status={<Badge variant="secondary">{category}</Badge>}
                meta={
                  article.updated_at ? (
                    <span>{t("knowledge.updatedAt", { date: formatDate(article.updated_at) })}</span>
                  ) : undefined
                }
                actions={
                  <Button type="button" variant="outline" size="sm" onClick={() => setSelectedArticle(article)}>
                    {t("knowledge.readArticle")}
                  </Button>
                }
              />
            );
          })}
        </div>
      )}

      <Dialog open={!!selectedArticle} onOpenChange={(open) => !open && setSelectedArticle(null)}>
        <DialogContent className="max-w-3xl">
          {selectedArticle && (
            <>
              <DialogHeader>
                <DialogTitle>{selectedArticle.title}</DialogTitle>
                <DialogDescription>
                  {categoryOf(selectedArticle, fallbackCategory)}
                  {selectedArticle.updated_at ? ` · ${t("knowledge.updatedAt", { date: formatDate(selectedArticle.updated_at) })}` : ""}
                </DialogDescription>
              </DialogHeader>
              <div className="mt-4 max-h-[60dvh] space-y-4 overflow-y-auto rounded-md border bg-muted/20 p-4 text-sm leading-7 text-foreground">
                {selectedArticleParagraphs.length > 0 ? (
                  selectedArticleParagraphs.map((paragraph, index) => (
                    <p key={index} className="whitespace-pre-wrap break-words">
                      {paragraph}
                    </p>
                  ))
                ) : (
                  <p className="text-muted-foreground">{t("knowledge.noBody")}</p>
                )}
              </div>
            </>
          )}
        </DialogContent>
      </Dialog>
    </PageShell>
  );
}
