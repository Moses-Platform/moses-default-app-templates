// Canonical Moses data-layer hooks.
//
// Components consume these — NEVER call fetch()/the client directly from a
// component (except an explicit on-click "playground" action) and NEVER load
// data in a useEffect. Reads are useQuery; writes are useMutation with explicit
// cache invalidation. Thread the queryFn signal so Query can cancel in-flight
// requests on unmount / key change / refetch supersession.
//
// Pattern (uncomment + adapt once you add client functions):
//
//   import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
//   import { queryKeys } from './queryKeys';
//   import { listWidgets, createWidget } from './client';
//
//   export function useWidgets() {
//     return useQuery({ queryKey: queryKeys.widgets, queryFn: ({ signal }) => listWidgets(signal) });
//   }
//
//   export function useCreateWidget() {
//     const qc = useQueryClient();
//     return useMutation({
//       mutationFn: createWidget,
//       // No optimistic updates by default: invalidate on settle so the list
//       // reconciles with server truth.
//       onSettled: () => qc.invalidateQueries({ queryKey: queryKeys.widgets }),
//     });
//   }

export {};
