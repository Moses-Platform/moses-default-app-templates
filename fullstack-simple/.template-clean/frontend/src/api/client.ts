// Typed transport layer.
//   - Reads: fetchAPI (GET, JSON in).
//   - Writes: mutateAPI (body in, JSON entity back) / mutateAPINoContent (204/empty).
// RELATIVE paths only ('api/v1/...', never '/api/...') so the nginx subpath proxy
// works under /apps/<tenant>/<slug>/.
//
// AbortSignal is threaded so TanStack Query can cancel in-flight requests on
// unmount / key change / refetch supersession.
//
// Add your endpoint functions at the bottom, e.g.:
//   export interface Widget { id: string; name: string }
//   export const listWidgets = (signal?: AbortSignal) => fetchAPI<Widget[]>('api/v1/widgets', signal);
//   export const createWidget = (input: { name: string }) => mutateAPI<Widget>('api/v1/widgets', 'POST', input);

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

export async function fetchAPI<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    signal,
  });
  if (!response.ok) return failure(response);
  return response.json() as Promise<T>;
}

// Write that expects a JSON entity back (200/201).
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
