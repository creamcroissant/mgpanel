import { ChevronLeft, ChevronRight } from "lucide-react";

import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";

interface PaginationProps {
  page: number;
  totalPages: number;
  onPageChange: (page: number) => void;
  className?: string;
}

const Pagination = ({ page, totalPages, onPageChange, className }: PaginationProps) => {
  if (totalPages <= 1) return null;

  const canGoPrev = page > 1;
  const canGoNext = page < totalPages;

  const createRange = () => {
    if (totalPages <= 7) {
      return Array.from({ length: totalPages }, (_, index) => index + 1);
    }

    if (page <= 4) {
      return [1, 2, 3, 4, 5, "ellipsis", totalPages] as const;
    }

    if (page >= totalPages - 3) {
      return [1, "ellipsis", totalPages - 4, totalPages - 3, totalPages - 2, totalPages - 1, totalPages] as const;
    }

    return [1, "ellipsis", page - 1, page, page + 1, "ellipsis", totalPages] as const;
  };

  const range = createRange();

  return (
    <nav className={cn("flex items-center justify-center gap-2", className)} aria-label="pagination">
      <Button
        variant="outline"
        size="sm"
        onClick={() => onPageChange(page - 1)}
        disabled={!canGoPrev}
      >
        <ChevronLeft className="h-4 w-4" />
      </Button>
      {range.map((item, index) => {
        if (item === "ellipsis") {
          return (
            <span key={`ellipsis-${index}`} className="px-2 text-sm text-muted-foreground">
              ...
            </span>
          );
        }

        const pageNumber = item as number;
        return (
          <Button
            key={pageNumber}
            variant={pageNumber === page ? "default" : "outline"}
            size="sm"
            onClick={() => onPageChange(pageNumber)}
            className="min-w-[2rem]"
          >
            {pageNumber}
          </Button>
        );
      })}
      <Button
        variant="outline"
        size="sm"
        onClick={() => onPageChange(page + 1)}
        disabled={!canGoNext}
      >
        <ChevronRight className="h-4 w-4" />
      </Button>
    </nav>
  );
};

export { Pagination };
