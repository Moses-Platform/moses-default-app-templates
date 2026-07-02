import { useAuth } from '../auth/useAuth';
import './pages.css';

/**
 * Shows the authenticated principal exactly as the backend resolved it
 * from the validated session — the visible proof that the OIDC round-
 * trip worked. Anonymous visitors see a sign-in prompt instead.
 */
export default function IdentityPage() {
  const { phase, me, via, note, signIn } = useAuth();

  if (phase === 'loading') {
    return (
      <div className="page">
        <h2>My Identity</h2>
        <p className="empty-note">Checking your session…</p>
      </div>
    );
  }

  if (phase === 'anonymous' || !me) {
    return (
      <div className="page">
        <h2>My Identity</h2>
        <p className="page-intro">
          This page reads <code>GET /api/v1/me</code> — a protected route. The
          vendored <code>oidcauth</code> middleware returns 401 because there
          is no session cookie yet.
        </p>
        <div className="card">
          <div className="banner banner-warn" style={{ marginBottom: 'var(--spacing-4)' }}>
            Not signed in — there is no identity to show.
          </div>
          <button className="btn btn-primary" onClick={signIn}>
            Sign in with Moses
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="page">
      <h2>My Identity</h2>
      <p className="page-intro">
        Everything below comes from the backend’s validated session — the
        browser never holds the token (BFF pattern). The backend projected it
        from the OIDC ID token Moses minted.
      </p>

      <section className="card">
        <h3>Authenticated principal</h3>
        <dl className="kv">
          <dt>Subject (sub)</dt>
          <dd>
            <code>{me.subject}</code>
          </dd>
          <dt>Name</dt>
          <dd>{me.name || '—'}</dd>
          <dt>Email</dt>
          <dd>{me.email || '—'}</dd>
          <dt>Username</dt>
          <dd>{me.username || '—'}</dd>
          <dt>Auth source</dt>
          <dd>
            <span className="badge badge-protected">{me.source}</span>{' '}
            {me.source === 'session'
              ? '— a validated OIDC session cookie'
              : '— trusted X-Moses-* pod-to-pod headers'}
          </dd>
          <dt>How signed in</dt>
          <dd>{via === 'silent-sso' ? 'Silently (prompt=none)' : 'Existing session cookie'}</dd>
          <dt>App admin?</dt>
          <dd>
            {me.roles.includes('oidc-admin') ? (
              <span className="badge badge-role">yes</span>
            ) : (
              <span className="badge badge-public">no</span>
            )}{' '}
            <span className="empty-note">
              (display only — the SERVER-side role check lives on{' '}
              <code>/api/v1/admin-area</code>, see Roles &amp; Access)
            </span>
          </dd>
        </dl>
        {note && (
          <p className="banner banner-info" style={{ marginTop: 'var(--spacing-4)' }}>
            {note}
          </p>
        )}
      </section>

      <section className="card">
        <h3>How this was resolved</h3>
        <p className="empty-note">
          The <code>oidcauth</code> middleware verified the session cookie’s
          HMAC tag, confirmed it had not expired, and placed an{' '}
          <code>Identity</code> on the request context. The{' '}
          <code>/api/v1/me</code> handler simply read it back with{' '}
          <code>oidcauth.IdentityFrom(r.Context())</code> — no token parsing in
          the handler itself.
        </p>
      </section>
    </div>
  );
}
