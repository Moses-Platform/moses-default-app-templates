/**
 * WORKED EXAMPLE — the `things` data-layer slice (client + query key + hooks).
 *
 * WHY THIS IS A REAL FILE AND NOT A COMMENT: `npm run lint` / `npm run build`
 * run `tsc -p tsconfig.build.json`, whose `include` is `src`, so every file
 * under src/ is type-checked — a defect here fails CI. Nothing imports this
 * module, so Vite tree-shakes it out of the bundle: it is compiled, never
 * shipped. Keep it that way — do not import it from app code.
 *
 * HOW TO USE IT: move each declaration to its real home — the transport (and
 * mutateAPI) into src/api/client.ts, `exampleQueryKeys.things` into the
 * `queryKeys` object in src/api/queryKeys.ts, the hooks into src/api/hooks.ts —
 * rename Thing/things to your real resource, then delete this file (and
 * src/example.tsx).
 *
 * The backend half of the same slice is REAL, CI-compiled Go:
 * backend/internal/handler/example_test.go (ThingsHandler) and
 * backend/cmd/server/example_test.go (routes). The "/things" spec you add to
 * backend/api/openapi.json is worked out in the comment above the //go:embed
 * directive in backend/api/api.go.
 *
 * Note: firing a Moses platform action is NOT a REST mutation — use
 * src/moses/invoke.ts for that.
 */
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { fetchAPI } from './client';

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

// This template ships READS only, so `mutateAPI` does not exist in client.ts
// yet. A REST write needs it — this is the canonical write shape (identical to
// fullstack-showcase's apart from credentials:'same-origin'). Move it into
// client.ts next to fetchAPI, where it reuses that module's private `failure`
// helper instead of the local copy below.
//
// No CSRF header: the template's guard is Sec-Fetch-Site-based, so a
// same-origin mutation passes with no token.
export async function mutateAPI<T>(
  path: string,
  method: string,
  body?: unknown,
  signal?: AbortSignal,
): Promise<T> {
  const response = await fetch(path, {
    method,
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    body: body === undefined ? undefined : JSON.stringify(body),
    signal,
  });
  if (!response.ok) return exampleFailure(response);
  return response.json() as Promise<T>;
}

// Local stand-in for client.ts's module-private `failure`. Delete this when you
// move mutateAPI into client.ts — the real one is already there.
async function exampleFailure(response: Response): Promise<never> {
  let detail = '';
  try {
    detail = (await response.text()).trim();
  } catch {
    /* body already consumed / unavailable */
  }
  const base = `API error ${response.status}`;
  throw new Error(detail ? `${base}: ${detail.slice(0, 300)}` : `${base}: ${response.statusText || 'request failed'}`);
}

export const createThing = (input: { name: string }, signal?: AbortSignal) =>
  mutateAPI<Thing>('api/v1/things', 'POST', input, signal);

// ---- Query keys (belongs in queryKeys.ts) --------------------------------
//
// Reads (useQuery) and writes (useMutation invalidation, or an imperative
// completion handler's invalidateQueries) MUST reference the same key, so they
// can never drift. Never inline a string array key in a component.
export const exampleQueryKeys = {
  things: ['things'] as const,
  thing: (id: string) => ['things', id] as const,
};

// ---- Hooks (belong in hooks.ts) ------------------------------------------
//
// Components consume these — NEVER call fetch()/the client directly from a
// component (except an explicit on-click SDK action via src/moses/invoke.ts)
// and NEVER load data in a useEffect.

export function useThings() {
  return useQuery({ queryKey: exampleQueryKeys.things, queryFn: ({ signal }) => getThings(signal) });
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
