// Typed transport layer for this Moses app.
//   - Reads: fetchAPI (GET, JSON in).
//   - Writes: mutateAPI (body in, JSON entity back).
// RELATIVE paths only ('api/v1/...', never '/api/...') so the nginx subpath
// proxy works under /apps/<tenant>/<slug>/. See utils/baseUrl.ts.
//
// credentials:'same-origin' carries the HttpOnly session cookie set by the
// vendored oidcauth BFF middleware. The browser never holds a token.
//
// AbortSignal is threaded so TanStack Query can cancel in-flight requests on
// unmount / key change / refetch supersession.
//
// ADD YOUR ENDPOINTS at the bottom: one typed wrapper per route, built on
// fetchAPI/mutateAPI, plus a matching hook in ./hooks.ts and a cache key in
// ./queryKeys.ts.

/** The authenticated principal, as returned by GET /api/v1/me. */
export interface MeResponse {
  authenticated: boolean;
  source: 'session' | 'moses-headers' | 'interapp' | '';
  subject: string;
  email: string;
  name: string;
  username: string;
  /** resource_access.<client>.roles projected by Moses onto the token. */
  roles: string[];
}

/**
 * Thrown by fetchAPI/mutateAPI when the session is missing/expired (401).
 * Hooks catch this to drop the query result and let the auth layer
 * re-establish a session, rather than surfacing a raw error banner.
 */
export class UnauthenticatedError extends Error {
  constructor() {
    super('unauthenticated');
    this.name = 'UnauthenticatedError';
  }
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

/** GET a JSON resource (relative path, session cookie attached). */
export async function fetchAPI<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, {
    credentials: 'same-origin',
    headers: { Accept: 'application/json' },
    signal,
  });
  if (response.status === 401) throw new UnauthenticatedError();
  if (!response.ok) return failure(response);
  return response.json() as Promise<T>;
}

/** Write that expects a JSON entity back (200/201). */
export async function mutateAPI<T>(path: string, method: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, {
    method,
    credentials: 'same-origin',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal,
  });
  if (response.status === 401) throw new UnauthenticatedError();
  if (!response.ok) return failure(response);
  return response.json() as Promise<T>;
}

/**
 * Fetch the current identity. Resolves `null` when the caller is not
 * authenticated (the backend returns 401) — the auth bootstrap then runs
 * silent SSO or the interactive login. Distinct from fetchAPI: an
 * anonymous /api/v1/me is a normal state, not an error.
 */
export async function getMe(signal?: AbortSignal): Promise<MeResponse | null> {
  const resp = await fetch('api/v1/me', {
    credentials: 'same-origin',
    headers: { Accept: 'application/json' },
    signal,
  });
  if (resp.status === 401) return null;
  if (!resp.ok) return failure(resp);
  return resp.json() as Promise<MeResponse>;
}

// ---- your endpoints below ---------------------------------------------
// export const listThings = (signal?: AbortSignal) => fetchAPI<Thing[]>('api/v1/things', signal);
// export const createThing = (body: string) => mutateAPI<Thing>('api/v1/things', 'POST', { body });
