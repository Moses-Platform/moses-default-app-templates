import { QueryClient } from '@tanstack/react-query';
import { UnauthenticatedError } from './client';

// Canonical Moses query client (shared singleton).
//
// Defaults tuned for a deployed Moses app:
//   - staleTime 30s: data stays fresh across remounts/route changes, so
//     navigating back to a page doesn't trigger a refetch storm.
//   - retry: one retry on transient failures, but NEVER retry a 401 —
//     an expired/missing session is resolved by the auth layer (silent
//     SSO / interactive login), not by hammering the protected route.
//   - refetchOnWindowFocus off: Moses apps render inside iframes (embed
//     mode, Tauri); focus churn there is noisy and not a useful trigger.
//
// Real-time updates are driven by explicit cache invalidation (see hooks.ts
// mutations), NOT by focus/interval polling.
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: (failureCount, error) => {
        if (error instanceof UnauthenticatedError) return false;
        return failureCount < 1;
      },
      refetchOnWindowFocus: false,
    },
  },
});
