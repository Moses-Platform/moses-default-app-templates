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
// Real-time updates are driven primarily by explicit cache invalidation: the
// app's postMessage completion handler calls queryClient.invalidateQueries the
// instant Moses Manager finishes (see App.tsx). The entries query additionally
// keeps a gentle background refetchInterval (see hooks.ts) as a backstop for
// the case where the host never forwards a completion event.
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});
