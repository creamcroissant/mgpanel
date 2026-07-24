import { useEffect, useState } from "react";

/**
 * useIsMobileViewport returns true when the viewport width is 767px or less.
 * It listens for viewport changes via matchMedia and cleans up on unmount.
 */
export function useIsMobileViewport(): boolean {
  const [isMobile, setIsMobile] = useState(false);

  useEffect(() => {
    const mediaQuery = window.matchMedia("(max-width: 767px)");
    const syncViewport = () => setIsMobile(mediaQuery.matches);

    syncViewport();
    mediaQuery.addEventListener("change", syncViewport);
    return () => mediaQuery.removeEventListener("change", syncViewport);
  }, []);

  return isMobile;
}
