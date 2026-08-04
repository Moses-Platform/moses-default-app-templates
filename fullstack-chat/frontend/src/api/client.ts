// Typed transport layer.
//   - Reads: fetchAPI (GET, JSON in).
// RELATIVE paths only ('api/v1/...', never '/api/...') so the nginx subpath proxy
// works under /apps/<tenant>/<slug>/. See FRONTEND_DATA_LAYER.md.
//
// AbortSignal is threaded so TanStack Query can cancel in-flight requests on
// unmount / key change / refetch supersession.
//
// TODO: add your typed endpoint functions at the bottom.
//
// WORKED EXAMPLE: src/api/example.ts (transport, incl. the mutateAPI helper
// this template does not ship yet, + query key + hooks) and src/example.tsx
// (the consuming component). They are REAL files — tsc type-checks everything
// under src/, so CI compiles them; nothing imports them, so Vite tree-shakes
// them out of the bundle. The backend half is equally real:
// backend/internal/handler/example_test.go + backend/cmd/server/example_test.go
// (the "/things" spec stays a comment above the //go:embed directive in
// backend/api/api.go, because openapi.json ships).
//
// Note: firing a Moses platform action is NOT a REST mutation — use
// src/moses/invoke.ts for that.

// Normalize an unknown thrown value to a message. Use this in components/hooks
// instead of `error as Error` — a queryFn can reject with anything.
export function getErrorMessage(e: unknown): string {
  if (e instanceof Error) return e.message;
  if (typeof e === 'string') return e;
  return 'Unexpected error';
}

// Build a rich error from a non-2xx response, surfacing the server's body
// (the Go backend uses meaningful JSON/plaintext bodies) instead of the
// generic statusText.
async function failure(response: Response): Promise<never> {
  let detail = '';
  try {
    detail = (await response.text()).trim();
  } catch {
    /* body already consumed / unavailable */
  }
  const base = `API error ${response.status}`;
  throw new Error(detail ? `${base}: ${detail.slice(0, 300)}` : `${base}: ${response.statusText || 'request failed'}`);
}

export async function fetchAPI<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    signal,
  });
  if (!response.ok) return failure(response);
  return response.json() as Promise<T>;
}
