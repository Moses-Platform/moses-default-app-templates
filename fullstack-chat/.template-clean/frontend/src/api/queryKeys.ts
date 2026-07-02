// Centralized query-key factory — the single source of truth for cache keys.
//
// Reads (useQuery) and writes (useMutation invalidation, or an imperative
// completion handler's invalidateQueries) MUST reference the same key, so
// they can never drift. Add an entry per resource; never inline a string
// array key in a component. (The demo's `entries` key was removed by
// clean_out_template.sh.)
export const queryKeys = {
  // widgets: ['widgets'] as const,
} as const;
