// Typed transport layer for the Moses fullstack-oidc app.
//   - Reads: fetchAPI (GET, JSON in).
//   - Writes: mutateAPI (body in, JSON entity back).
// RELATIVE paths only ('api/v1/...', never '/api/...') so the nginx subpath
// proxy works under /apps/<tenant>/<slug>/. See FRONTEND_DATA_LAYER.md.
//
// credentials:'same-origin' carries the HttpOnly session cookie set by the
// vendored oidcauth BFF middleware. The browser never holds a token.
//
// AbortSignal is threaded so TanStack Query can cancel in-flight requests on
// unmount / key change / refetch supersession.

/** The authenticated principal, as returned by GET /api/v1/me. */
export interface MeResponse {
  authenticated: boolean;
  source: 'session' | 'moses-headers' | '';
  subject: string;
  email: string;
  name: string;
  username: string;
  /** resource_access.<client>.roles projected by Moses onto the token. */
  roles: string[];
  /** Server-computed: true when `roles` includes `oidc-admin`. */
  is_app_admin: boolean;
}

/** Always-public app metadata from GET /api/v1/public-info. */
export interface PublicInfo {
  app: string;
  oidc_enabled: boolean;
  /** The role vocabulary the app declares in moses-app.config.json. */
  known_roles: string[];
}

/** A per-user entry from GET/POST /api/v1/entries (USER space — private). */
export interface Entry {
  id: string;
  owner_sub: string;
  body: string;
  created_at: string;
}

/**
 * A tenant-shared note from GET/POST /api/v1/shared-notes (TENANT space).
 * Visible to every member of the workspace; `author_sub` is attribution only.
 */
export interface SharedNote {
  id: string;
  author_sub: string;
  body: string;
  created_at: string;
}

/** Outcome of probing the role-gated /api/v1/admin-area route. */
export type AdminAreaResult =
  | { kind: 'allowed'; message: string; granted_role: string }
  | { kind: 'forbidden'; required_role: string; detail: string }
  | { kind: 'unauthenticated' };

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

async function fetchAPI<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, {
    credentials: 'same-origin',
    headers: { Accept: 'application/json' },
    signal,
  });
  if (response.status === 401) throw new UnauthenticatedError();
  if (!response.ok) return failure(response);
  return response.json() as Promise<T>;
}

// Write that expects a JSON entity back (200/201).
async function mutateAPI<T>(path: string, method: string, body?: unknown, signal?: AbortSignal): Promise<T> {
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

/** Fetch public app metadata (never requires a session). */
export const getPublicInfo = (signal?: AbortSignal) => fetchAPI<PublicInfo>('api/v1/public-info', signal);

/** List the signed-in user's entries (protected route). */
export const listEntries = (signal?: AbortSignal) => fetchAPI<Entry[]>('api/v1/entries', signal);

/** Create an entry owned by the signed-in user (protected route). */
export const createEntry = (body: string) => mutateAPI<Entry>('api/v1/entries', 'POST', { body });

/** List the workspace's shared notes — tenant-scoped, every member sees them. */
export const listSharedNotes = (signal?: AbortSignal) => fetchAPI<SharedNote[]>('api/v1/shared-notes', signal);

/** Post a note to the tenant-shared list (protected route). */
export const createSharedNote = (body: string) => mutateAPI<SharedNote>('api/v1/shared-notes', 'POST', { body });

/**
 * Probe the role-gated /api/v1/admin-area route. This is the clearest
 * demonstration that authentication and authorization are separate: a
 * signed-in user without the `oidc-admin` role gets `forbidden`.
 */
export async function probeAdminArea(signal?: AbortSignal): Promise<AdminAreaResult> {
  const resp = await fetch('api/v1/admin-area', {
    credentials: 'same-origin',
    headers: { Accept: 'application/json' },
    signal,
  });
  if (resp.status === 401) return { kind: 'unauthenticated' };
  if (resp.status === 403) {
    const j = await resp.json();
    return { kind: 'forbidden', required_role: j.required_role, detail: j.detail };
  }
  if (!resp.ok) return failure(resp);
  const j = await resp.json();
  return { kind: 'allowed', message: j.message, granted_role: j.granted_role };
}
