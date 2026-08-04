// Centralized query-key factory — the single source of truth for cache keys.
//
// Reads (useQuery) and writes (useMutation invalidation) MUST reference the
// same key here, so they can never drift. Add a new entry when you add a new
// resource; never inline a string array key in a component.
//
// WORKED EXAMPLE: exampleQueryKeys in src/api/example.ts (a real, CI-compiled
// file that ships with nothing importing it).
export const queryKeys = {};
