import { useCallback, useEffect, useState } from 'react';
import { useAuth } from '../auth/useAuth';
import { probeAdminArea, type AdminAreaResult } from '../auth/api';
import './pages.css';

/**
 * Demonstrates AUTHORIZATION (distinct from authentication): the roles
 * Moses projects onto the token, and a role-gated route that 403s a
 * signed-in user who lacks the `oidc-admin` role.
 */
export default function RolesPage() {
  const { phase, me, info, signIn } = useAuth();
  const [probe, setProbe] = useState<AdminAreaResult | null>(null);
  const [probing, setProbing] = useState(false);

  const runProbe = useCallback(async () => {
    setProbing(true);
    try {
      setProbe(await probeAdminArea());
    } catch {
      setProbe(null);
    } finally {
      setProbing(false);
    }
  }, []);

  useEffect(() => {
    if (phase === 'authenticated') void runProbe();
  }, [phase, runProbe]);

  return (
    <div className="page">
      <h2>Roles &amp; Access</h2>
      <p className="page-intro">
        Authentication answers <em>who are you</em>; authorization answers{' '}
        <em>what may you do</em>. Moses projects a tenant admin’s role mapping
        into <code>resource_access.&lt;client&gt;.roles</code> on the ID token,
        and the <code>oidcauth</code> middleware lifts it onto{' '}
        <code>Identity.Roles</code>.
      </p>

      <section className="card">
        <h3>This app’s role vocabulary</h3>
        <p className="empty-note">
          Declared in <code>moses-app.config.json</code> under{' '}
          <code>access.roles</code>. A tenant admin maps real users into these
          names; only mapped roles ever appear on a token.
        </p>
        <div className="chips" style={{ marginTop: 'var(--spacing-3)' }}>
          {(info?.known_roles ?? ['oidc-admin', 'oidc-member']).map((r) => (
            <span key={r} className="chip">
              {r}
            </span>
          ))}
        </div>
      </section>

      {phase === 'anonymous' || !me ? (
        <section className="card">
          <div className="banner banner-warn" style={{ marginBottom: 'var(--spacing-4)' }}>
            Sign in to see the roles on your own token.
          </div>
          <button className="btn btn-primary" onClick={signIn}>
            Sign in with Moses
          </button>
        </section>
      ) : (
        <>
          <section className="card">
            <h3>Your projected roles</h3>
            {me.roles.length > 0 ? (
              <div className="chips">
                {me.roles.map((r) => (
                  <span key={r} className="chip">
                    {r}
                  </span>
                ))}
              </div>
            ) : (
              <div className="chips">
                <span className="chip chip-empty">no app roles on your token</span>
              </div>
            )}
            <p className="empty-note" style={{ marginTop: 'var(--spacing-3)' }}>
              An empty list is normal — a tenant admin has not yet mapped your
              user to any of this app’s roles. The role projection still works;
              you simply hold none.
            </p>
          </section>

          <section className="card">
            <h3>
              Role-gated route{' '}
              <span className="badge badge-role">protected + role-gated</span>
            </h3>
            <p className="empty-note">
              <code>GET /api/v1/admin-area</code> requires the{' '}
              <code>oidc-admin</code> role. The middleware proves you are
              authenticated; the handler additionally enforces the role.
            </p>
            <button
              className="btn btn-primary"
              onClick={runProbe}
              disabled={probing}
              style={{ marginTop: 'var(--spacing-3)' }}
            >
              {probing ? 'Probing…' : 'Probe /api/v1/admin-area'}
            </button>

            {probe?.kind === 'allowed' && (
              <div className="banner banner-ok" style={{ marginTop: 'var(--spacing-4)' }}>
                ✅ 200 OK — {probe.message} (granted via{' '}
                <code>{probe.granted_role}</code>).
              </div>
            )}
            {probe?.kind === 'forbidden' && (
              <div className="banner banner-error" style={{ marginTop: 'var(--spacing-4)' }}>
                ⛔ 403 Forbidden — you are authenticated, but the route needs
                the <code>{probe.required_role}</code> role. {probe.detail}
              </div>
            )}
            {probe?.kind === 'unauthenticated' && (
              <div className="banner banner-warn" style={{ marginTop: 'var(--spacing-4)' }}>
                401 — your session expired. Sign in again.
              </div>
            )}
          </section>
        </>
      )}
    </div>
  );
}
