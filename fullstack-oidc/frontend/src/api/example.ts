/**
 * WORKED EXAMPLE — the `things` data-layer slice (client + query key + hooks).
 *
 * WHY THIS IS A REAL FILE AND NOT A COMMENT: `npm run lint` / `npm run build`
 * run `tsc -p tsconfig.build.json`, whose `include` is `src`, so every file
 * under src/ is type-checked — a defect here fails CI. Nothing imports this
 * module, so Vite tree-shakes it out of the bundle: it is compiled, never
 * shipped. Keep it that way — do not import it from app code.
 *
 * HOW TO USE IT: move each declaration to its real home — the transport into
 * src/api/client.ts, `exampleQueryKeys.things` into the `queryKeys` object in
 * src/api/queryKeys.ts, the hooks into src/api/hooks.ts — rename Thing/things
 * to your real resource, then delete this file (and src/example.tsx).
 *
 * The backend half of the same slice is REAL, CI-compiled Go:
 * backend/cmd/server/example_test.go (routes + ThingsHandler). The "/things"
 * spec you add to backend/api/openapi.json is worked out in the comment above
 * the //go:embed directive in backend/api/api.go — and the route needs an
 * access.oidc.protectedPaths entry in moses-app.config.json or it stays
 * deny-by-default.
 */
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { fetchAPI, mutateAPI } from './client';

// ---- Transport (belongs in client.ts) ------------------------------------
//
// RELATIVE paths only — 'api/v1/things', never '/api/v1/things'. A leading
// slash bypasses the nginx subpath proxy and 404s under
// /apps/<tenant>/<slug>/.
//
// AbortSignal is threaded through so TanStack Query can cancel in-flight
// requests on unmount / key change / refetch supersession.
//
// The read is typed as an OBJECT ({ things: [...] }), never a bare array —
// the Go handler encodes map[string]any{"things": things} and the OpenAPI
// response schema says the same. All three layers must agree.

export interface Thing {
  id: string;
  name: string;
}

export const getThings = (signal?: AbortSignal) => fetchAPI<{ things: Thing[] }>('api/v1/things', signal);

export const createThing = (input: { name: string }, signal?: AbortSignal) =>
  mutateAPI<Thing>('api/v1/things', 'POST', input, signal);

// ---- Query keys (belongs in queryKeys.ts) --------------------------------
//
// Reads (useQuery) and writes (useMutation invalidation) MUST reference the
// same key, so they can never drift. Never inline a string array key in a
// component.
export const exampleQueryKeys = {
  things: ['things'] as const,
  thing: (id: string) => ['things', id] as const,
};

// ---- Hooks (belong in hooks.ts) ------------------------------------------
//
// Components consume these — NEVER call fetch()/the client directly from a
// component and NEVER load data in a useEffect.
//
// AUTH + QUERY INTERPLAY: gate protected-route queries with `enabled` on the
// authenticated phase so they never fire while anonymous. A 401 mid-session
// surfaces as UnauthenticatedError (queryClient never retries it) and the
// caller falls back to the sign-in action.

export function useThings(enabled: boolean) {
  return useQuery({
    queryKey: exampleQueryKeys.things,
    queryFn: ({ signal }) => getThings(signal),
    enabled,
  });
}

export function useCreateThing() {
  const qc = useQueryClient();
  return useMutation({
    // Wrap the client call — do NOT pass `createThing` directly. Query v5
    // hands mutationFn (variables, context) and `context` is not an
    // AbortSignal, so the bare reference does not typecheck.
    mutationFn: (input: { name: string }) => createThing(input),
    // Invalidate on settle — never call refetch() by hand.
    onSettled: () => qc.invalidateQueries({ queryKey: exampleQueryKeys.things }),
  });
}
