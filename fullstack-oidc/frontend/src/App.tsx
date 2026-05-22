/**
 * fullstack-oidc reference app (CHAT-t5d1u.28.14).
 *
 * The canonical reference for app-owned OIDC: a Moses-deployed app that
 * declares `access.oidc`, lets the platform inject the OIDC client, and
 * uses the vendored `oidcauth` BFF middleware to enforce auth + read the
 * user's projected roles.
 *
 * MOSES ROUTING: all fetch() calls use RELATIVE paths (no leading '/').
 * The app is served under /apps/<tenant>/<slug>/ — relative paths route
 * through the app's nginx proxy to the Go backend. Absolute paths bypass
 * the proxy and hit the Moses platform (404). See auth/api.ts and
 * utils/baseUrl.ts.
 */
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { AuthProvider } from './auth/useAuth';
import { getBasePath } from './utils/baseUrl';
import Layout from './components/Layout';
import OverviewPage from './pages/OverviewPage';
import IdentityPage from './pages/IdentityPage';
import RolesPage from './pages/RolesPage';
import EntriesPage from './pages/EntriesPage';
import SilentSSOPage from './pages/SilentSSOPage';
import HowItWorksPage from './pages/HowItWorksPage';

function App() {
  // BrowserRouter basename = the BASE_PATH the app is mounted under, so
  // internal <Link to="/foo"> resolves correctly under /apps/<t>/<s>/.
  const basename = getBasePath();

  return (
    <AuthProvider>
      <BrowserRouter basename={basename}>
        <Routes>
          <Route path="/" element={<Layout />}>
            <Route index element={<OverviewPage />} />
            <Route path="identity" element={<IdentityPage />} />
            <Route path="roles" element={<RolesPage />} />
            <Route path="entries" element={<EntriesPage />} />
            <Route path="silent-sso" element={<SilentSSOPage />} />
            <Route path="how-it-works" element={<HowItWorksPage />} />
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
