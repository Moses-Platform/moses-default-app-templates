// Canonical Moses data-layer hooks.
//
// Components consume these — NEVER call fetch()/the client directly from a
// component (except an explicit on-click "playground" action) and NEVER load
// data in a useEffect. Reads are useQuery; writes are useMutation with explicit
// cache invalidation. The queryFn signal is threaded so Query can cancel
// in-flight requests on unmount / key change / refetch supersession.
//
// WORKED EXAMPLE: useThings / useCreateThing in src/api/example.ts, consumed by
// src/example.tsx. They are REAL files — tsc type-checks everything under src/,
// so CI compiles them; nothing imports them, so Vite tree-shakes them out of
// the bundle. Move them here (and their key into queryKeys.ts, their transport
// into client.ts) when you adopt them.
import { useQuery } from '@tanstack/react-query';
import { queryKeys } from './queryKeys';
import { getHealth } from './client';

export function useHealth() {
  return useQuery({ queryKey: queryKeys.health, queryFn: ({ signal }) => getHealth(signal) });
}
