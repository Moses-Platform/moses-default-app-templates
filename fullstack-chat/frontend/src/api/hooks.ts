// Canonical Moses data-layer hooks.
//
// Components consume these — NEVER call fetch()/the client directly from a
// component (except an explicit on-click SDK action via src/moses/invoke.ts)
// and NEVER load data in a useEffect. Reads are useQuery; thread the queryFn
// signal so Query can cancel in-flight requests on unmount / key change /
// refetch supersession.
//
// WORKED EXAMPLE: useThings / useCreateThing in src/api/example.ts, consumed by
// src/example.tsx — which also shows a chat_prompt completion invalidating the
// same key from a useMosesChatComplete handler. They are REAL files — tsc
// type-checks everything under src/, so CI compiles them; nothing imports them,
// so Vite tree-shakes them out of the bundle. Move them here (and their key
// into queryKeys.ts, their transport into client.ts) when you adopt them.

export {};
