// Canonical Moses data-layer hooks.
//
// Components consume these — NEVER call fetch()/the client directly from a
// component (except an explicit on-click "playground"/SDK action) and NEVER
// load data in a useEffect. Reads are useQuery; the queryFn signal is threaded
// so Query can cancel in-flight requests on unmount / key change / refetch
// supersession.
import { useQuery } from '@tanstack/react-query';
import { queryKeys } from './queryKeys';
import { getEntries, type Entry } from './client';

// Steady-state freshness backstop. The roundtrip's authoritative refresh is the
// postMessage completion handler in App.tsx (which invalidates this key the
// instant Moses Manager finishes). This interval covers the case where the host
// shell never forwards a completion event. Refetch pauses automatically when the
// document is hidden — TanStack Query gates interval refetch on focus by default.
const ENTRIES_REFETCH_INTERVAL_MS = 2_000;

// ---- Reads -----------------------------------------------------------------

// The entries list is the only non-stream server read. `select` unwraps the
// { entries, count } envelope so components consume a plain Entry[].
export function useEntries() {
  return useQuery({
    queryKey: queryKeys.entries,
    queryFn: ({ signal }) => getEntries(signal),
    select: (data): Entry[] => data.entries ?? [],
    refetchInterval: ENTRIES_REFETCH_INTERVAL_MS,
    refetchIntervalInBackground: false,
  });
}
