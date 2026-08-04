// Canonical Moses data-layer hooks for this app.
//
// Components consume these — NEVER call fetch()/the client directly from a
// component and NEVER load data in a useEffect. Reads are useQuery; writes
// are useMutation with explicit cache invalidation. The queryFn signal is
// threaded so Query can cancel in-flight requests.
//
// AUTH + QUERY INTERPLAY:
//   - useMe() drives the auth lifecycle; null === anonymous.
//   - Gate protected-route queries with `enabled` on the authenticated
//     phase so they never fire while anonymous.
//   - A 401 mid-session surfaces as UnauthenticatedError (queryClient
//     never retries it) — the caller falls back to the sign-in action.
import { useQuery } from '@tanstack/react-query';
import { queryKeys } from './queryKeys';
import { getMe } from './client';

/** The authenticated principal (null when anonymous). Source of auth truth. */
export function useMe() {
  return useQuery({ queryKey: queryKeys.me, queryFn: ({ signal }) => getMe(signal) });
}

// ---- your hooks below ---------------------------------------------------
//
// WORKED EXAMPLE: useThings / useCreateThing in src/api/example.ts, consumed by
// src/example.tsx — note the `enabled` gate, so the protected-route query never
// fires while anonymous. They are REAL files — tsc type-checks everything under
// src/, so CI compiles them; nothing imports them, so Vite tree-shakes them out
// of the bundle. Move them here (and their key into queryKeys.ts, their
// transport into client.ts) when you adopt them.
