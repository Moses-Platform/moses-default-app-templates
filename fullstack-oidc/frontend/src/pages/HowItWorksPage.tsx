import './pages.css';

/**
 * The teaching page: the whole app-owned-OIDC chain in one place, plus
 * the route map so a reader can see which routes are public, protected,
 * or role-gated.
 */
export default function HowItWorksPage() {
  return (
    <div className="page">
      <h2>How It Works</h2>
      <p className="page-intro">
        App-owned OIDC is a four-link chain: the app <em>declares</em> intent,
        the platform <em>injects</em> credentials, the middleware{' '}
        <em>enforces</em> auth, and the app <em>reads</em> identity and roles.
        Each link is shown below.
      </p>

      <section className="card">
        <h3>The chain</h3>
        <ul className="steps">
          <li>
            <strong>Declare — moses-app.config.json</strong>
            <p>
              The app adds an <code>access.oidc</code> block (mode{' '}
              <code>moses-oidc</code>, plus <code>protectedPaths</code> /{' '}
              <code>publicPaths</code>) and an <code>access.roles</code> list
              naming the roles it understands. That declaration is the only
              thing the app author writes to opt in.
            </p>
          </li>
          <li>
            <strong>Inject — the Moses platform</strong>
            <p>
              At deploy time the platform sees <code>access.oidc</code>,
              registers a confidential Keycloak client for the app, and injects
              the credentials as <code>MOSES_OIDC_*</code> env vars plus a K8s
              Secret (client secret + cookie HMAC key). The app never stores or
              ships these.
            </p>
          </li>
          <li>
            <strong>Enforce — the vendored oidcauth middleware</strong>
            <p>
              <code>oidcauth.ConfigFromEnv()</code> reads the injected env;{' '}
              <code>oidcauth.New(...).Handler</code> wraps the mux. Protected
              paths with no session get redirected into the authorization-code
              + PKCE flow; the middleware validates the ID token against JWKS
              and sets an HttpOnly session cookie (BFF — the browser never
              holds a token).
            </p>
          </li>
          <li>
            <strong>Read — your handlers</strong>
            <p>
              A handler calls <code>oidcauth.IdentityFrom(r.Context())</code> to
              get the subject, email, name, and the roles projected from{' '}
              <code>resource_access.&lt;client&gt;.roles</code>. Authorization
              decisions use <code>Identity.HasRole(...)</code>.
            </p>
          </li>
        </ul>
      </section>

      <section className="card">
        <h3>Route map</h3>
        <table className="route-table">
          <thead>
            <tr>
              <th>Route</th>
              <th>Access</th>
              <th>Why</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>
                <code>/health</code>
              </td>
              <td>
                <span className="badge badge-public">public</span>
              </td>
              <td>K8s probe — implicitly public, never gated.</td>
            </tr>
            <tr>
              <td>
                <code>/api/openapi.json</code>
              </td>
              <td>
                <span className="badge badge-public">public</span>
              </td>
              <td>Platform discovery + workspace-tool surface — implicitly public.</td>
            </tr>
            <tr>
              <td>
                <code>/api/v1/public-info</code>
              </td>
              <td>
                <span className="badge badge-public">public</span>
              </td>
              <td>
                Declared in <code>access.oidc.publicPaths</code>. Anonymous
                visitors can read the app’s posture.
              </td>
            </tr>
            <tr>
              <td>
                <code>/api/v1/me</code>
              </td>
              <td>
                <span className="badge badge-protected">protected</span>
              </td>
              <td>
                Declared in <code>access.oidc.protectedPaths</code> — requires a
                validated session.
              </td>
            </tr>
            <tr>
              <td>
                <code>/api/v1/entries</code>
              </td>
              <td>
                <span className="badge badge-protected">protected</span>
              </td>
              <td>Per-user data scoped to the authenticated subject.</td>
            </tr>
            <tr>
              <td>
                <code>/api/v1/admin-area</code>
              </td>
              <td>
                <span className="badge badge-role">role-gated</span>
              </td>
              <td>
                Protected AND requires the <code>oidc-admin</code> app role —
                authentication plus authorization.
              </td>
            </tr>
          </tbody>
        </table>
      </section>

      <section className="card">
        <h3>Dual mode &amp; graceful degradation</h3>
        <p className="empty-note">
          The middleware preserves the existing <code>X-Moses-*</code> trusted-
          header path: a pod-to-pod MCP / workspace-tool call carrying{' '}
          <code>X-Moses-User-ID</code> bypasses the OIDC redirect, so the
          OpenAPI tool surface keeps working. And when the{' '}
          <code>MOSES_OIDC_*</code> env vars are absent (a standalone{' '}
          <code>helm install</code>), the middleware runs in pass-through mode —
          public routes and the header path still work, browser requests are
          not redirected. A misconfigured deploy degrades visibly, it does not
          hard-fail.
        </p>
      </section>

      <section className="card">
        <h3>Reuse this in your own app</h3>
        <p className="empty-note">
          The <code>oidcauth</code> package is vendored, not imported — copy{' '}
          <code>backend/internal/oidcauth/</code> into your template, import it
          from your own module path, declare <code>access.oidc</code>, and wire
          the chart’s OIDC env. It has no third-party Go dependencies. See{' '}
          <code>skills/oidcauth-middleware.md</code> for the full recipe.
        </p>
      </section>
    </div>
  );
}
