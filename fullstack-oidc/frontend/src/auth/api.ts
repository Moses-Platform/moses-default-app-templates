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
  roles: string[];
}

/** Always-public app metadata from GET /api/v1/public-info. */
export interface PublicInfo {
  app: string;
  oidc_enabled: boolean;
}

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
