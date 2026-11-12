"use client";

import { useCallback, useEffect, useRef, useState } from "react";

interface PollState<T> {
  data?: T;
  error?: string;
  loading: boolean;
  updatedAt?: number;
}

// usePoll runs fetcher when deps change and, optionally, on an interval.
// Previous data is kept while a refetch is in flight so the UI never flashes.
// Polling pauses while the tab is hidden.
export function usePoll<T>(
  fetcher: (signal: AbortSignal) => Promise<T>,
  deps: unknown[],
  intervalMs = 0,
  enabled = true,
) {
  const [state, setState] = useState<PollState<T>>({ loading: enabled });
  const [tick, setTick] = useState(0);
  const fetcherRef = useRef(fetcher);
  fetcherRef.current = fetcher;

  const refresh = useCallback(() => setTick((t) => t + 1), []);

  useEffect(() => {
    if (!enabled) return;
    const ctrl = new AbortController();
    setState((s) => ({ ...s, loading: true }));
    fetcherRef
      .current(ctrl.signal)
      .then((data) => setState({ data, loading: false, updatedAt: Date.now() }))
      .catch((err: unknown) => {
        if (ctrl.signal.aborted) return;
        setState((s) => ({ ...s, loading: false, error: err instanceof Error ? err.message : String(err) }));
      });
    return () => ctrl.abort();
  }, [...deps, tick, enabled]);

  useEffect(() => {
    if (!enabled || intervalMs <= 0) return;
    const id = setInterval(() => {
      if (document.visibilityState === "visible") refresh();
    }, intervalMs);
    return () => clearInterval(id);
  }, [intervalMs, enabled, refresh]);

  return { ...state, refresh };
}
