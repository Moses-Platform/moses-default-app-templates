// Canonical Moses data-layer hooks.
//
// Components consume these — NEVER call fetch()/the client directly from a
// component and NEVER load data in a useEffect. Reads are useQuery; writes are
// useMutation with explicit cache invalidation. The queryFn signal is threaded
// so Query can cancel in-flight requests on unmount / key change / refetch
// supersession.
//
// This frontend-only starter ships a single live read (useHealth) so the
// pattern is exercised end-to-end. Add your query + mutation hooks here as you
// grow a backend; keep the client and key factory as the single sources of
// truth. See FRONTEND_DATA_LAYER.md.
import { useQuery } from '@tanstack/react-query';
import { queryKeys } from './queryKeys';
import { getHealth } from './client';

// ---- Reads -----------------------------------------------------------------

export function useHealth() {
  return useQuery({ queryKey: queryKeys.health, queryFn: ({ signal }) => getHealth(signal) });
}

// ---- Writes (mutation + invalidation) --------------------------------------
// No backend writes in this starter. The canonical mutation shape is:
//
//   export function useCreateThing() {
//     const qc = useQueryClient();
//     return useMutation({
//       mutationFn: (input: ThingInput) => createThing(input),
//       onSettled: () => qc.invalidateQueries({ queryKey: queryKeys.things }),
//     });
//   }
//
// The list re-fetches automatically via invalidation — no manual refetch.
