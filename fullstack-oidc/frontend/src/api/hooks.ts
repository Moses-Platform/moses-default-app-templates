// Canonical Moses data-layer hooks for the fullstack-oidc app.
//
// Components consume these — NEVER call fetch()/the client directly from a
// component (except an explicit on-click "playground" action) and NEVER load
// data in a useEffect. Reads are useQuery; writes are useMutation with explicit
// cache invalidation. The queryFn signal is threaded so Query can cancel
// in-flight requests on unmount / key change / refetch supersession.
//
// AUTH + QUERY INTERPLAY (the distinctive part of this template):
//   - useMe()         drives the auth lifecycle; null === anonymous.
//   - useEntries()    is gated by `enabled` on the authenticated phase, so the
//                     protected route is never hit while anonymous.
//   - a 401 thrown mid-session surfaces as UnauthenticatedError (queryClient
//     never retries it) — the caller falls back to the sign-in action.
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from './queryKeys';
import {
  getMe,
  getPublicInfo,
  listEntries,
  createEntry,
  listSharedNotes,
  createSharedNote,
  probeAdminArea,
} from './client';

// ---- Reads -----------------------------------------------------------------

/** The authenticated principal (null when anonymous). Source of auth truth. */
export function useMe() {
  return useQuery({ queryKey: queryKeys.me, queryFn: ({ signal }) => getMe(signal) });
}

/** Always-public backend posture — no session required. */
export function usePublicInfo() {
  return useQuery({ queryKey: queryKeys.publicInfo, queryFn: ({ signal }) => getPublicInfo(signal) });
}

/**
 * The signed-in user's entries (protected). Gated by auth state: the query
 * only fires once `authenticated` is true, so the protected route is never
 * called while anonymous.
 */
export function useEntries(enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.entries,
    queryFn: ({ signal }) => listEntries(signal),
    enabled,
  });
}

/**
 * The workspace's shared notes (protected, TENANT space). Like useEntries it is
 * gated by auth state, but the list is the same for every member of the tenant —
 * including notes an agent posted via a workspace-tool call. Contrast with
 * useEntries, whose list is private to the signed-in OIDC subject.
 */
export function useSharedNotes(enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.sharedNotes,
    queryFn: ({ signal }) => listSharedNotes(signal),
    enabled,
  });
}

/**
 * The role-gated /api/v1/admin-area probe. Also gated by auth state — the
 * point of the demo is that a signed-in user without the role gets `forbidden`
 * (authorization), distinct from the anonymous 401 (authentication).
 */
export function useAdminArea(enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.adminArea,
    queryFn: ({ signal }) => probeAdminArea(signal),
    enabled,
  });
}

// ---- Writes (mutation + invalidation) --------------------------------------
// No optimistic updates by design (teaching template): invalidate on success so
// the list reconciles with server truth. onSettled also reconciles after error.

export function useCreateEntry() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: string) => createEntry(body),
    onSettled: () => qc.invalidateQueries({ queryKey: queryKeys.entries }),
  });
}

export function useCreateSharedNote() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: string) => createSharedNote(body),
    onSettled: () => qc.invalidateQueries({ queryKey: queryKeys.sharedNotes }),
  });
}
