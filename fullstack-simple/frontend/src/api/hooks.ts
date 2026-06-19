// Canonical Moses data-layer hooks.
//
// Components consume these — NEVER call fetch()/the client directly from a
// component (except an explicit on-click "playground" action) and NEVER load
// data in a useEffect. Reads are useQuery; writes are useMutation with explicit
// cache invalidation. The queryFn signal is threaded so Query can cancel
// in-flight requests on unmount / key change / refetch supersession.
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from './queryKeys';
import {
  getStatus,
  listItems,
  createItem,
  deleteItem,
  type ItemInput,
} from './client';

// ---- Reads -----------------------------------------------------------------

export function useStatus() {
  return useQuery({ queryKey: queryKeys.status, queryFn: ({ signal }) => getStatus(signal) });
}

export function useItems() {
  return useQuery({ queryKey: queryKeys.items, queryFn: ({ signal }) => listItems(signal) });
}

// ---- Writes (mutation + invalidation) --------------------------------------
// No optimistic updates by design (teaching template): invalidate on success so
// the list reconciles with server truth. onSettled also reconciles after error.

export function useCreateItem() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: ItemInput) => createItem(input),
    onSettled: () => qc.invalidateQueries({ queryKey: queryKeys.items }),
  });
}

export function useDeleteItem() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteItem(id),
    onSettled: () => qc.invalidateQueries({ queryKey: queryKeys.items }),
  });
}
