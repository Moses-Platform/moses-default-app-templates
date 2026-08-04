// Typed transport layer for the Moses frontend template.
//   - Reads: fetchAPI (GET, JSON in) / fetchText (GET, plaintext in).
//   - Writes: mutateAPI (body in, JSON entity back) / mutateAPINoContent (204/empty).
// RELATIVE paths only ('api/v1/...', never '/api/...') so the nginx subpath proxy
// works under /apps/<tenant>/<slug>/. See FRONTEND_DATA_LAYER.md.
//
// AbortSignal is threaded so TanStack Query can cancel in-flight requests on
// unmount / key change / refetch supersession.
//
// Add typed reads/writes below and matching hooks in hooks.ts — never call
// fetch() from a component.
//
// WORKED EXAMPLE: src/api/example.ts (transport + query key + hooks) and
// src/example.tsx (the consuming component). They are REAL files — tsc
// type-checks everything under src/, so CI compiles them; nothing imports them,
// so Vite tree-shakes them out of the bundle.

// Normalize an unknown thrown value to a message. Use this in components/hooks
// instead of `error as Error` — a queryFn can reject with anything.
export function getErrorMessage(e: unknown): string {
  if (e instanceof Error) return e.message;
  if (typeof e === 'string') return e;
  return 'Unexpected error';
}

// Build a rich error from a non-2xx response, surfacing the server's body
// (a Go backend using http.Error returns meaningful plaintext, e.g. "Title is
// required") instead of the generic statusText.
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

// JSON read. Add typed reads via this once you have a JSON backend.
export async function fetchAPI<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    signal,
  });
  if (!response.ok) return failure(response);
  return response.json() as Promise<T>;
}

// Plaintext read (e.g. a probe endpoint returning "healthy\n", not JSON).
export async function fetchText(path: string, signal?: AbortSignal): Promise<string> {
  const response = await fetch(path, { signal });
  if (!response.ok) return failure(response);
  return (await response.text()).trim();
}

// Write that expects a JSON entity back (200/201). The template's CSRF guard
// is Sec-Fetch-Site-based, so a same-origin mutation passes with no token; do
// not add a CSRF header here.
export async function mutateAPI<T>(path: string, method: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal,
  });
  if (!response.ok) return failure(response);
  return response.json() as Promise<T>;
}

// Write that expects no entity back (204 / empty body).
export async function mutateAPINoContent(path: string, method: string, signal?: AbortSignal): Promise<void> {
  const response = await fetch(path, {
    method,
    headers: { 'Content-Type': 'application/json' },
    signal,
  });
  if (!response.ok) await failure(response);
}
