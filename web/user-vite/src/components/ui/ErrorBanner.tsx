import { Card, CardContent } from "@/components/ui/card";
import { AlertCircle } from "lucide-react";

interface ErrorBannerProps {
  message: string;
  onRetry?: () => void;
}

export default function ErrorBanner({ message, onRetry }: ErrorBannerProps) {
  return (
    <Card className="bg-destructive/10 border-destructive/20">
      <CardContent className="flex flex-row items-center gap-3 p-4">
        <AlertCircle className="h-5 w-5 text-destructive flex-shrink-0" />
        <p className="text-destructive text-sm flex-1">{message}</p>
        {onRetry && (
          <button
            onClick={onRetry}
            className="text-sm text-destructive hover:underline"
          >
            Retry
          </button>
        )}
      </CardContent>
    </Card>
  );
}
