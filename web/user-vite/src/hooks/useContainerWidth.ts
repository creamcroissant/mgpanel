import { useEffect, useState } from "react";

type ResizeObserverCtor = typeof ResizeObserver;

function getResizeObserver(): ResizeObserverCtor | null {
  if (typeof window === "undefined") {
    return null;
  }
  return window.ResizeObserver ?? null;
}

export function useContainerWidth(ref: React.RefObject<HTMLElement>): number {
  const [width, setWidth] = useState(0);

  useEffect(() => {
    const element = ref.current;
    if (!element) {
      return;
    }

    const updateWidth = () => {
      setWidth(element.getBoundingClientRect().width);
    };

    updateWidth();

    const ResizeObserverImpl = getResizeObserver();
    if (ResizeObserverImpl) {
      const observer = new ResizeObserverImpl(() => updateWidth());
      observer.observe(element);
      return () => observer.disconnect();
    }

    window.addEventListener("resize", updateWidth);
    return () => window.removeEventListener("resize", updateWidth);
  }, [ref]);

  return width;
}
