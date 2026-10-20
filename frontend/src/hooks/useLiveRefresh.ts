import { useEffect } from "react";

const LIVE_EVENT = "zbm:live";

export function emitLiveRefresh(type?: string) {
  window.dispatchEvent(new CustomEvent(LIVE_EVENT, { detail: { type } }));
}

/** Re-run loader whenever the SSE hub publishes an update. */
export function useLiveRefresh(loader: () => void | Promise<void>, intervalMs = 8000) {
  useEffect(() => {
    let alive = true;
    const run = () => {
      if (!alive) return;
      void loader();
    };
    run();
    const onLive = () => run();
    window.addEventListener(LIVE_EVENT, onLive);
    const id = window.setInterval(run, intervalMs);
    return () => {
      alive = false;
      window.removeEventListener(LIVE_EVENT, onLive);
      window.clearInterval(id);
    };
  }, [loader, intervalMs]);
}
