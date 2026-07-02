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
// export function useThings(enabled: boolean) {
//   return useQuery({
//     queryKey: queryKeys.things,
//     queryFn: ({ signal }) => listThings(signal),
//     enabled,
//   });
// }
// export function useCreateThing() {
//   const qc = useQueryClient();
//   return useMutation({
//     mutationFn: (body: string) => createThing(body),
//     onSettled: () => qc.invalidateQueries({ queryKey: queryKeys.things }),
//   });
// }
