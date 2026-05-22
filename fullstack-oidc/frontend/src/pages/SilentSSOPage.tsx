import { useState } from 'react';
import { useAuth } from '../auth/useAuth';
import { attemptSilentSSO, type SilentSSOResult } from '../auth/silentSSO';
import './pages.css';

/**
 * Demonstrates the silent-SSO (`prompt=none`) path used for the embedded-
 * iframe case. When the app is embedded in the Moses Manager iframe and
 * the user is already logged into Moses, they are authenticated with NO
 * visible login page. This page lets a viewer trigger that probe and
 * watch the outcome.
 */
export default function SilentSSOPage() {
  const { phase, via, refresh } = useAuth();
  const [result, setResult] = useState<SilentSSOResult | null>(null);
  const [running, setRunning] = useState(false);

  async function run() {
    setRunning(true);
    setResult(null);
    try {
      const r = await attemptSilentSSO();
      setResult(r);
      // If the probe established a session, refresh the shared auth state
      // so the rest of the app reflects it without a page reload.
      if (r.authenticated) await refresh();
    } finally {
      setRunning(false);
    }
  }

  return (
    <div className="page">
      <h2>Silent SSO</h2>
      <p className="page-intro">
        When this app is embedded in the Moses Manager iframe, a user who is
        already logged into Moses should never see a login page. The{' '}
        <code>oidcauth</code> middleware exposes <code>/auth/silent-check</code>,
        which runs an OIDC authorization request with{' '}
        <code>prompt=none</code> inside a hidden iframe.
      </p>

      <section className="card">
        <h3>What happens</h3>
        <ul className="steps">
          <li>
            <strong>Hidden iframe loads /auth/silent-check</strong>
            <p>
              The visible page never navigates. The middleware kicks off an
              authorization-code + PKCE flow with <code>prompt=none</code>.
            </p>
          </li>
          <li>
            <strong>Keycloak decides — silently</strong>
            <p>
              Already logged into Moses → Keycloak returns a code, the backend
              sets the session cookie, no UI. Not logged in → Keycloak returns{' '}
              <code>login_required</code>.
            </p>
          </li>
          <li>
            <strong>The iframe lands on a marker URL</strong>
            <p>
              The callback bounces back with <code>?silent_sso=ok</code> or{' '}
              <code>?silent_sso=failed</code>; the helper reads the marker and
              resolves a boolean. A failed probe is a normal outcome, never an
              error.
            </p>
          </li>
        </ul>
      </section>

      <section className="card">
        <h3>Try it</h3>
        <p className="empty-note">
          The app already ran this probe once at startup. Run it again here to
          watch it directly.
        </p>
        <button
          className="btn btn-primary"
          onClick={run}
          disabled={running}
          style={{ marginTop: 'var(--spacing-3)' }}
        >
          {running ? 'Running silent probe…' : 'Run silent SSO probe'}
        </button>

        {result?.authenticated === true && (
          <div className="banner banner-ok" style={{ marginTop: 'var(--spacing-4)' }}>
            ✅ Silent SSO established a session — no login page was shown.
          </div>
        )}
        {result?.authenticated === false && (
          <div className="banner banner-warn" style={{ marginTop: 'var(--spacing-4)' }}>
            Silent SSO did not establish a session ({result.reason}). This is
            expected when you are not logged into Moses — fall back to the
            interactive “Sign in with Moses” button.
          </div>
        )}
      </section>

      <section className="card">
        <h3>Current state</h3>
        <p className="empty-note">
          {phase === 'authenticated'
            ? via === 'silent-sso'
              ? 'You are signed in — and it happened silently at startup.'
              : 'You are signed in via an existing session cookie.'
            : 'You are not signed in. A silent probe above will tell you whether Moses can sign you in without a login page.'}
        </p>
      </section>
    </div>
  );
}
