import { useAuth } from '../auth/useAuth';
import { usePublicInfo } from '../api/hooks';
import {
  IdentityIcon,
  RolesIcon,
  EntriesIcon,
  SharedNotesIcon,
  SilentSSOIcon,
} from '../components/NavIcons';
import './pages.css';

/**
 * Landing page. Explains what app-owned OIDC is and shows the live
 * backend posture (OIDC enforced vs pass-through) read from the always-
 * public /api/v1/public-info route — so even an anonymous visitor SEES
 * the integration before signing in.
 */
export default function OverviewPage() {
  const { phase, me } = useAuth();
  // Demo data, fetched by the demo page itself — useAuth() is pure auth
  // plumbing and deliberately does not carry public-info.
  const { data: info } = usePublicInfo();

  return (
    <div className="page">
      <section className="hero">
        <h2>App-owned OIDC, demonstrated end-to-end</h2>
        <p>
          This reference app is an OIDC relying party fronted by Moses (the
          BFF pattern). It declares <code>access.oidc</code> in its config;
          the platform auto-provisions a Keycloak client and injects the
          credentials; the vendored <code>oidcauth</code> middleware enforces
          auth; and the app reads the user identity and their projected roles.
        </p>
      </section>

      <section className="card">
        <h3>Live backend posture</h3>
        {info ? (
          <p>
            Backend <strong>{info.app}</strong> reports OIDC enforcement is{' '}
            {info.oidc_enabled ? (
              <span className="badge badge-protected">enabled</span>
            ) : (
              <span className="badge badge-public">pass-through</span>
            )}
            . This was read from the always-public{' '}
            <code>/api/v1/public-info</code> route — no session required.
            {!info.oidc_enabled && (
              <>
                {' '}
                Pass-through means the <code>MOSES_OIDC_*</code> env vars are
                absent (a standalone <code>helm install</code>, or a deploy
                where the platform did not inject OIDC). Public routes and the
                pod-to-pod header path still work; browser requests are not
                redirected.
              </>
            )}
          </p>
        ) : (
          <p className="empty-note">Loading backend metadata…</p>
        )}
      </section>

      <section className="card">
        <h3>Your status</h3>
        {phase === 'loading' && <p className="empty-note">Checking your session…</p>}
        {phase === 'anonymous' && (
          <div className="banner banner-warn">
            You are not signed in. Use the “Sign in with Moses” button in the
            header, or visit the <strong>Silent SSO</strong> page to see the
            invisible login path.
          </div>
        )}
        {phase === 'authenticated' && me && (
          <div className="banner banner-ok">
            Signed in as <strong>{me.name || me.username || me.subject}</strong>.
            Open <strong>My Identity</strong> to inspect the validated token
            projection, or <strong>Roles &amp; Access</strong> to see
            authorization in action.
          </div>
        )}
      </section>

      <section className="card">
        <h3>What each page shows</h3>
        <div className="feature-grid">
          <div>
            <h4>
              <span className="feature-icon"><IdentityIcon /></span>
              My Identity
            </h4>
            <p className="empty-note">
              The principal from the validated session — subject, email, name,
              and how it was authenticated.
            </p>
          </div>
          <div>
            <h4>
              <span className="feature-icon"><RolesIcon /></span>
              Roles &amp; Access
            </h4>
            <p className="empty-note">
              Roles projected from <code>resource_access.&lt;client&gt;.roles</code>,
              and a role-gated route that 403s without the right role.
            </p>
          </div>
          <div>
            <h4>
              <span className="feature-icon"><EntriesIcon /></span>
              My Entries
            </h4>
            <p className="empty-note">
              The <strong>user space</strong>: per-user data scoped to the
              authenticated OIDC subject — private to you.
            </p>
          </div>
          <div>
            <h4>
              <span className="feature-icon"><SharedNotesIcon /></span>
              Shared Notes
            </h4>
            <p className="empty-note">
              The <strong>tenant space</strong>: scoped by tenant, shared by the
              whole workspace — and where agent-delivered content lands.
            </p>
          </div>
          <div>
            <h4>
              <span className="feature-icon"><SilentSSOIcon /></span>
              Silent SSO
            </h4>
            <p className="empty-note">
              The <code>prompt=none</code> hidden-iframe path for the embedded
              case — no visible login page.
            </p>
          </div>
        </div>
      </section>
    </div>
  );
}
