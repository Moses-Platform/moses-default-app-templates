// Centralized query-key factory — the single source of truth for cache keys.
//
// Reads (useQuery) and writes (useMutation invalidation) MUST reference the
// same key here, so they can never drift. Add a new entry when you add a new
// resource; never inline a string array key in a component.
export const queryKeys = {
  mosesInfo: ['moses-info'] as const,
  health: ['health'] as const,
  capabilities: {
    all: ['capabilities'] as const,
    detail: (id: string) => ['capabilities', id] as const,
  },
  users: ['users'] as const,
  notes: ['notes'] as const,
};
