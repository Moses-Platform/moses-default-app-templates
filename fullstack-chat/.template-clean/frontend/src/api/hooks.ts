// Canonical Moses data-layer hooks.
//
// Components consume these — NEVER call fetch()/the client directly from a
// component (except an explicit on-click SDK action via src/moses/invoke.ts)
// and NEVER load data in a useEffect. Reads are useQuery; thread the queryFn
// signal so Query can cancel in-flight requests on unmount / key change /
// refetch supersession.
//
// TODO: add your hooks (the demo's useEntries was removed by
// clean_out_template.sh), e.g.
//
//   import { useQuery } from '@tanstack/react-query';
//   import { queryKeys } from './queryKeys';
//   import { fetchAPI } from './client';
//
//   export function useWidgets() {
//     return useQuery({
//       queryKey: queryKeys.widgets,
//       queryFn: ({ signal }) => fetchAPI<WidgetsResponse>('api/v1/widgets', signal),
//     });
//   }

export {};
