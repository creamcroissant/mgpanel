import { Link } from "react-router-dom";
import { Home } from "lucide-react";
import { ROUTES } from "@/lib/constants";
import { Button } from "@/components/ui/button";

export default function NotFound() {
  return (
    <div className="min-h-screen bg-background">
      <div className="mx-auto flex min-h-screen max-w-3xl flex-col items-center justify-center px-4 py-12 text-center">
        <div className="flex items-center gap-3">
          <span className="h-3 w-3 rounded-full bg-primary" />
          <span className="text-xs font-semibold uppercase tracking-[0.3em] text-muted-foreground">404</span>
        </div>
        <h1 className="mt-4 text-4xl font-semibold tracking-tight text-foreground sm:text-5xl">Page not found</h1>
        <p className="mt-3 max-w-xl text-sm text-muted-foreground sm:text-base">
          The page you are looking for does not exist or has been moved.
        </p>
        <div className="mt-8">
          <Button asChild>
            <Link to={ROUTES.DASHBOARD}>
              <Home className="mr-2 h-4 w-4" />
              Go Home
            </Link>
          </Button>
        </div>
      </div>
    </div>
  );
}
