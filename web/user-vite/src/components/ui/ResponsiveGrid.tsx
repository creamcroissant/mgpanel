import { cn } from "@/lib/utils";

interface ResponsiveGridProps {
  children: React.ReactNode;
  minColWidth?: number;
  gap?: number;
  className?: string;
}

export function ResponsiveGrid({
  children,
  minColWidth = 280,
  gap = 16,
  className,
}: ResponsiveGridProps) {
  return (
    <div
      className={cn("grid", className)}
      style={{
        gridTemplateColumns: `repeat(auto-fill, minmax(${minColWidth}px, 1fr))`,
        gap: `${gap}px`,
      }}
    >
      {children}
    </div>
  );
}
