import { QueryClient } from '@tanstack/react-query';

// Canonical Moses query client (shared singleton).
//
// Defaults tuned for a deployed Moses app:
//   - staleTime 30s: data stays fresh across remounts/route changes, so
//     navigating back to a page doesn't trigger a refetch storm.
//   - retry 1: one retry on transient failures, then surface the error.
//   - refetchOnWindowFocus off: Moses apps render inside iframes (embed mode,
//     Tauri); focus churn there is noisy and not a useful refetch trigger.
//
// Real-time updates are driven by explicit cache invalidation (see hooks.ts
// mutations, and — in the Moses platform — WebSocket events that call
// queryClient.invalidateQueries), NOT by focus/interval polling.
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});
