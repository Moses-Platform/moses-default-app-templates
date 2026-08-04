// Centralized query-key factory — the single source of truth for cache keys.
//
// Reads (useQuery) and writes (useMutation invalidation, or an imperative
// completion handler's invalidateQueries) MUST reference the same key, so
// they can never drift. Add an entry per resource; never inline a string
// array key in a component.
//
// WORKED EXAMPLE: exampleQueryKeys in src/api/example.ts (a real, CI-compiled
// file that ships with nothing importing it).
export const queryKeys = {} as const;
