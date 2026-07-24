import { useEffect, useState, type HTMLAttributes, type ReactNode } from "react";
import { Copy, Check } from "lucide-react";
import { Button } from "./button";
import { cn } from "@/lib/utils";

interface CopyFieldProps extends HTMLAttributes<HTMLDivElement> {
  label: ReactNode;
  value?: string | null;
  emptyLabel: ReactNode;
  copyLabel: ReactNode;
  copiedLabel: ReactNode;
  helperText?: ReactNode;
  buttonAriaLabel?: string;
}

export function CopyField({
  label,
  value,
  emptyLabel,
  copyLabel,
  copiedLabel,
  helperText,
  buttonAriaLabel,
  className,
  ...props
}: CopyFieldProps) {
  const [copied, setCopied] = useState(false);
  const displayValue = value?.trim() || "";

  useEffect(() => {
    if (!copied) return;
    const timer = window.setTimeout(() => setCopied(false), 1500);
    return () => window.clearTimeout(timer);
  }, [copied]);

  const handleCopy = async () => {
    if (!displayValue) return;
    await navigator.clipboard.writeText(displayValue);
    setCopied(true);
  };

  return (
    <div className={cn("space-y-2", className)} {...props}>
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div className="text-sm font-medium text-foreground">{label}</div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="gap-2 self-start"
          onClick={handleCopy}
          disabled={!displayValue}
          aria-label={buttonAriaLabel ?? String(copyLabel)}
        >
          {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
          {copied ? copiedLabel : copyLabel}
        </Button>
      </div>
      <div className="min-w-0 rounded-md border bg-muted/30 p-3 text-sm text-foreground">
        {displayValue ? (
          <span className="block break-all font-mono leading-6">{displayValue}</span>
        ) : (
          <span className="text-muted-foreground">{emptyLabel}</span>
        )}
      </div>
      {helperText && <p className="text-sm leading-6 text-muted-foreground">{helperText}</p>}
    </div>
  );
}
