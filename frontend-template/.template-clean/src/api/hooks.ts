// Canonical Moses data-layer hooks.
//
// Components consume these — NEVER call fetch()/the client directly from a
// component and NEVER load data in a useEffect. Reads are useQuery; writes are
// useMutation with explicit cache invalidation. Thread the queryFn signal so
// Query can cancel in-flight requests on unmount / key change / refetch
// supersession. See FRONTEND_DATA_LAYER.md.
//
// The demo hook (useHealth) was removed by ./clean_out_template.sh. Add your
// hooks here; keep client.ts and queryKeys.ts as the single sources of truth.
//
// ---- Read shape --------------------------------------------------------
//
//   import { useQuery } from '@tanstack/react-query';
//   import { queryKeys } from './queryKeys';
//   import { getThings } from './client';
//
//   export function useThings() {
//     return useQuery({ queryKey: queryKeys.things, queryFn: ({ signal }) => getThings(signal) });
//   }
//
// ---- Write shape (mutation + invalidation) ------------------------------
//
//   import { useMutation, useQueryClient } from '@tanstack/react-query';
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

export {};
