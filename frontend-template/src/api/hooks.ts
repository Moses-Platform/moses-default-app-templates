// Canonical Moses data-layer hooks.
//
// Components consume these — NEVER call fetch()/the client directly from a
// component and NEVER load data in a useEffect. Reads are useQuery; writes are
// useMutation with explicit cache invalidation. Thread the queryFn signal so
// Query can cancel in-flight requests on unmount / key change / refetch
// supersession. See FRONTEND_DATA_LAYER.md.
//
// Add your hooks here; keep client.ts and queryKeys.ts as the single sources
// of truth.
//
// WORKED EXAMPLE: useThings / useCreateThing in src/api/example.ts, consumed by
// src/example.tsx. They are REAL files — tsc type-checks everything under src/,
// so CI compiles them; nothing imports them, so Vite tree-shakes them out of
// the bundle. Move them here (and their key into queryKeys.ts, their transport
// into client.ts) when you adopt them.

export {};
