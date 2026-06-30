// Centralized query-key factory — the single source of truth for cache keys.
//
// Reads (useQuery) and writes (useMutation invalidation) MUST reference the
// same key here, so they can never drift. Add a new entry when you add a new
// resource; never inline a string array key in a component.
export const queryKeys = {
  me: ['me'] as const,
  publicInfo: ['public-info'] as const,
  entries: ['entries'] as const,
  sharedNotes: ['shared-notes'] as const,
  adminArea: ['admin-area'] as const,
};
