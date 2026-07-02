// Typed transport layer for the fullstack-chat roundtrip template.
//   - Reads: fetchAPI (GET, JSON in).
// RELATIVE paths only ('api/v1/...', never '/api/...') so the nginx subpath proxy
// works under /apps/<tenant>/<slug>/. See FRONTEND_DATA_LAYER.md.
//
// AbortSignal is threaded so TanStack Query can cancel in-flight requests on
// unmount / key change / refetch supersession.
//
// SCOPE NOTE: this app has exactly one non-stream server read — the entries
// list below — so the typed client is read-only. The app's *write* path
// (generating an entry) is NOT a REST mutation: it fires a Moses platform
// action through the iframe SDK via src/moses/invoke.ts, an explicit
// on-click action handled imperatively in App.tsx. When a template adds a REST
// write, add a `mutateAPI` helper here (see fullstack-showcase for the
// canonical write shape) and a mutation hook in hooks.ts.

export interface Entry {
  id: string;
  text: string;
  source: string;
  created_at: string;
}

export interface EntriesResponse {
  entries: Entry[];
  count: number;
}

// Normalize an unknown thrown value to a message. Use this in components/hooks
// instead of `error as Error` — a queryFn can reject with anything.
export function getErrorMessage(e: unknown): string {
  if (e instanceof Error) return e.message;
  if (typeof e === 'string') return e;
  return 'Unexpected error';
}

// Build a rich error from a non-2xx response, surfacing the server's body
// (the Go backend uses http.Error with meaningful plaintext) instead of the
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

async function fetchAPI<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    signal,
  });
  if (!response.ok) return failure(response);
  return response.json() as Promise<T>;
}

export const getEntries = (signal?: AbortSignal) => fetchAPI<EntriesResponse>('api/v1/entries', signal);
