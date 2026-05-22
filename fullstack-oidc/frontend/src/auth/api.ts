/**
 * Thin API client for the fullstack-oidc backend.
 *
 * MOSES ROUTING: all paths are RELATIVE (no leading slash) so the
 * browser resolves them against the app's MOSES_BASE_PATH-prefixed page
 * URL. `credentials: 'same-origin'` ensures the HttpOnly session cookie
 * set by the oidcauth middleware travels with each request.
 */

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

/** A per-user entry from GET/POST /api/v1/entries. */
export interface Entry {
  id: string;
  owner_sub: string;
  body: string;
  created_at: string;
}

/** Outcome of probing the role-gated /api/v1/admin-area route. */
export type AdminAreaResult =
  | { kind: 'allowed'; message: string; granted_role: string }
  | { kind: 'forbidden'; required_role: string; detail: string }
  | { kind: 'unauthenticated' };

/**
 * Fetch the current identity. Resolves `null` when the caller is not
 * authenticated (the backend returns 401) — the caller should then run
 * silent SSO or the interactive login.
 */
export async function fetchMe(): Promise<MeResponse | null> {
  const resp = await fetch('api/v1/me', {
    credentials: 'same-origin',
    headers: { Accept: 'application/json' },
  });
  if (resp.status === 401) return null;
  if (!resp.ok) throw new Error(`me: HTTP ${resp.status}`);
  return resp.json();
}

/** Fetch public app metadata (never requires a session). */
export async function fetchPublicInfo(): Promise<PublicInfo> {
  const resp = await fetch('api/v1/public-info', {
    credentials: 'same-origin',
    headers: { Accept: 'application/json' },
  });
  if (!resp.ok) throw new Error(`public-info: HTTP ${resp.status}`);
  return resp.json();
}

/** List the signed-in user's entries (protected route). */
export async function listEntries(): Promise<Entry[]> {
  const resp = await fetch('api/v1/entries', {
    credentials: 'same-origin',
    headers: { Accept: 'application/json' },
  });
  if (resp.status === 401) throw new Error('entries: not authenticated');
  if (!resp.ok) throw new Error(`entries: HTTP ${resp.status}`);
  return resp.json();
}

/** Create an entry owned by the signed-in user (protected route). */
export async function createEntry(body: string): Promise<Entry> {
  const resp = await fetch('api/v1/entries', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    body: JSON.stringify({ body }),
  });
  if (!resp.ok) throw new Error(`create entry: HTTP ${resp.status}`);
  return resp.json();
}

/**
 * Probe the role-gated /api/v1/admin-area route. This is the clearest
 * demonstration that authentication and authorization are separate: a
 * signed-in user without the `oidc-admin` role gets `forbidden`.
 */
export async function probeAdminArea(): Promise<AdminAreaResult> {
  const resp = await fetch('api/v1/admin-area', {
    credentials: 'same-origin',
    headers: { Accept: 'application/json' },
  });
  if (resp.status === 401) return { kind: 'unauthenticated' };
  if (resp.status === 403) {
    const j = await resp.json();
    return { kind: 'forbidden', required_role: j.required_role, detail: j.detail };
  }
  if (!resp.ok) throw new Error(`admin-area: HTTP ${resp.status}`);
  const j = await resp.json();
  return { kind: 'allowed', message: j.message, granted_role: j.granted_role };
}
