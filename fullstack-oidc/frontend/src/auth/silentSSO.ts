/**
 * Silent SSO helper (CHAT-t5d1u.28.8).
 *
 * When the fullstack-oidc app is embedded in the Moses Manager iframe, a
 * user who is already logged into Moses should NOT see a login page.
 * The backend's vendored `oidcauth` middleware exposes `/auth/silent-check`,
 * which runs an OIDC authorization request with `prompt=none`:
 *
 *   - already-logged-into-Keycloak  -> a session cookie is set, no UI
 *   - not logged in                 -> Keycloak returns login_required
 *                                      and the callback bounces back
 *                                      with `?silent_sso=failed`
 *
 * The standard pattern is to run that probe inside a hidden <iframe> so
 * the visible page never navigates. This module drives that iframe and
 * resolves a boolean: did silent SSO establish a session?
 *
 * All URLs are RELATIVE (no leading slash) so they resolve against the
 * app's MOSES_BASE_PATH-prefixed page URL — see ../utils/baseUrl.ts.
 */

import { getBasePath } from '../utils/baseUrl';

/** Result of a silent-SSO attempt. */
export type SilentSSOResult =
  | { authenticated: true }
  | { authenticated: false; reason: 'login_required' | 'timeout' | 'error' };

/** Options for {@link attemptSilentSSO}. */
export interface SilentSSOOptions {
  /** Abort the probe after this many ms (default 8000). */
  timeoutMs?: number;
}

/**
 * Build the `/auth/silent-check` URL. The backend appends
 * `?silent_sso=ok|failed` to the `return_to` target after the probe.
 * We point `return_to` at a tiny static result page so the hidden
 * iframe lands somewhere same-origin and cheap.
 */
function silentCheckURL(): string {
  const base = getBasePath();
  const prefix = base === '/' ? '/' : base + '/';
  // return_to is the post-probe landing inside the iframe; it must be a
  // same-origin relative path. We reuse the SPA index — the marker
  // query param is what we read, not the page content.
  const returnTo = encodeURIComponent(`${base === '/' ? '/' : base + '/'}silent-result`);
  return `${prefix}auth/silent-check?return_to=${returnTo}`;
}

/**
 * Run a silent (prompt=none) SSO probe in a hidden iframe.
 *
 * Resolves `{ authenticated: true }` when the probe established a
 * session, or `{ authenticated: false, reason }` otherwise. Never
 * rejects — a failed silent SSO is a normal outcome (the user simply
 * isn't logged into Moses yet) and the caller should fall back to the
 * interactive `/auth/login` flow.
 */
export function attemptSilentSSO(
  opts: SilentSSOOptions = {},
): Promise<SilentSSOResult> {
  const timeoutMs = opts.timeoutMs ?? 8000;

  return new Promise<SilentSSOResult>((resolve) => {
    if (typeof document === 'undefined') {
      resolve({ authenticated: false, reason: 'error' });
      return;
    }

    const iframe = document.createElement('iframe');
    iframe.style.display = 'none';
    iframe.setAttribute('aria-hidden', 'true');
    iframe.title = 'oidc-silent-sso';

    let settled = false;
    const cleanup = () => {
      window.clearTimeout(timer);
      iframe.removeEventListener('load', onLoad);
      if (iframe.parentNode) iframe.parentNode.removeChild(iframe);
    };
    const finish = (r: SilentSSOResult) => {
      if (settled) return;
      settled = true;
      cleanup();
      resolve(r);
    };

    const timer = window.setTimeout(
      () => finish({ authenticated: false, reason: 'timeout' }),
      timeoutMs,
    );

    const onLoad = () => {
      // The probe ends at `${returnTo}?silent_sso=ok|failed`. Reading
      // the iframe's location is only possible because it stays
      // same-origin throughout (the cross-origin hop to Keycloak and
      // back is invisible to us — we only observe the final landing).
      try {
        const url = iframe.contentWindow?.location.href ?? '';
        if (url.includes('silent_sso=ok')) {
          finish({ authenticated: true });
        } else if (url.includes('silent_sso=failed')) {
          finish({ authenticated: false, reason: 'login_required' });
        }
        // Any other load is an intermediate hop — keep waiting until
        // the marker appears or the timeout fires.
      } catch {
        // A cross-origin frame throws on location access; that means
        // we are mid-handshake on Keycloak's origin. Keep waiting.
      }
    };

    iframe.addEventListener('load', onLoad);
    iframe.src = silentCheckURL();
    document.body.appendChild(iframe);
  });
}

/**
 * Convenience: kick off the interactive login by navigating the
 * top-level window to `/auth/login`. Use this as the fallback after
 * {@link attemptSilentSSO} reports `authenticated: false`.
 */
export function startInteractiveLogin(returnTo?: string): void {
  const base = getBasePath();
  const prefix = base === '/' ? '/' : base + '/';
  const rt = returnTo
    ? `?return_to=${encodeURIComponent(returnTo)}`
    : '';
  window.location.assign(`${prefix}auth/login${rt}`);
}

/**
 * Convenience: trigger RP-initiated logout by navigating to
 * `/auth/logout`. The backend clears the session cookie and redirects
 * through Keycloak's end-session endpoint.
 */
export function logout(): void {
  const base = getBasePath();
  const prefix = base === '/' ? '/' : base + '/';
  window.location.assign(`${prefix}auth/logout`);
}
