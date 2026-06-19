// Centralized query-key factory — the single source of truth for cache keys.
//
// Reads (useQuery) and writes (useMutation invalidation, or — here — the
// imperative completion handler's invalidateQueries) MUST reference the same
// key, so they can never drift. Add a new entry when you add a new resource;
// never inline a string array key in a component.
export const queryKeys = {
  entries: ['entries'] as const,
};
