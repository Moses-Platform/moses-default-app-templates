// Canonical Moses data-layer hooks.
//
// Components consume these — NEVER call fetch()/the client directly from a
// component (except an explicit on-click "playground" action) and NEVER load
// data in a useEffect. Reads are useQuery; writes are useMutation with explicit
// cache invalidation. The queryFn signal is threaded so Query can cancel
// in-flight requests on unmount / key change / refetch supersession.
//
// Pattern for writes (mutation + invalidation, no optimistic updates):
//   export function useCreateItem() {
//     const qc = useQueryClient();
//     return useMutation({
//       mutationFn: (input: ItemInput) => createItem(input),
//       onSettled: () => qc.invalidateQueries({ queryKey: queryKeys.items }),
//     });
//   }
import { useQuery } from '@tanstack/react-query';
import { queryKeys } from './queryKeys';
import { getHealth } from './client';

export function useHealth() {
  return useQuery({ queryKey: queryKeys.health, queryFn: ({ signal }) => getHealth(signal) });
}
