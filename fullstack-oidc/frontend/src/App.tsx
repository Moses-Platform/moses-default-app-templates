/**
 * App shell for a Moses-deployed OIDC relying-party app (BFF pattern):
 * the app declares `access.oidc`, the platform injects the OIDC client,
 * and the vendored Go `oidcauth` middleware enforces auth. useAuth
 * provides the browser-side bootstrap (silent SSO, sign-in/out).
 *
 * MOSES ROUTING: all fetch() calls use RELATIVE paths (no leading '/').
 * The app is served under /apps/<tenant>/<slug>/ — relative paths route
 * through the app's nginx proxy to the Go backend. Absolute paths bypass
 * the proxy and hit the Moses platform (404). See api/client.ts and
 * utils/baseUrl.ts.
 */
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { AuthProvider, useAuth } from './auth/useAuth';
import { getBasePath } from './utils/baseUrl';
import Layout from './components/Layout';

// A worked `things` slice ships as REAL, CI-compiled code that nothing imports
// (so it never reaches a bundle or a binary):
//   backend/cmd/server/example_test.go   routes + ThingsHandler
//   src/api/example.ts                   getThings / createThing,
//                                        queryKeys.things, useThings (with the
//                                        `enabled` auth gate) / useCreateThing
//   src/example.tsx                      the consuming component
// The three halves that must stay declarative/comments, because their artifacts
// ship as-is: the `things` table in backend/internal/database/migrate_demo.go,
// the "/things" path above the //go:embed directive in backend/api/api.go, and
// the access.oidc.protectedPaths entry in moses-app.config.json (without it the
// route stays deny-by-default).
// Move each piece into its real home, rename to your resource, then delete the
// example files.

/** Placeholder landing page — replace with your app's real pages. */
function HomePage() {
  const { phase, me } = useAuth();
  return (
    <div className="container">
      <h2>Your app starts here</h2>
      <p className="text-muted">
        The demo was cleaned out. Auth is wired: this page reads{' '}
        <code>useAuth()</code>
        {phase === 'authenticated' && me
          ? ` — signed in as ${me.name || me.username || me.subject}.`
          : ' — not signed in yet (use the header button).'}
      </p>
      <p className="text-muted">
        Add pages under <code>src/pages/</code>, routes below, data hooks in{' '}
        <code>src/api/</code>, and backend routes in{' '}
        <code>backend/cmd/server/demo_routes.go</code>.
      </p>
    </div>
  );
}

function App() {
  // BrowserRouter basename = the BASE_PATH the app is mounted under, so
  // internal <Link to="/foo"> resolves correctly under /apps/<t>/<s>/.
  const basename = getBasePath();

  return (
    <AuthProvider>
      <BrowserRouter basename={basename}>
        <Routes>
          <Route path="/" element={<Layout />}>
            <Route index element={<HomePage />} />
            <Route
              path="*"
              element={
                <div className="error-page">
                  <h1>404 — Page Not Found</h1>
                </div>
              }
            />
          </Route>
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  );
}

export default App;
