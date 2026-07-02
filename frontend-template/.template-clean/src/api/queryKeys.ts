// Centralized query-key factory — the single source of truth for cache keys.
//
// Reads (useQuery) and writes (useMutation invalidation) MUST reference the
// same key here, so they can never drift. Add a new entry when you add a new
// resource; never inline a string array key in a component. Example:
//
//   things: ['things'] as const,
//   thing: (id: string) => ['things', id] as const,
export const queryKeys = {};
