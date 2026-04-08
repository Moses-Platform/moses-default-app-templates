/**
 * MOSES ROUTING: All fetch() calls MUST use relative paths (no leading '/').
 * Your app is served at a subpath (/apps/workspace/app-slug/).
 * Relative paths route through the app's nginx proxy to the backend.
 * Absolute paths (fetch('/api/...')) bypass the app and hit the Moses platform (404).
 *
 * CORRECT: fetch('api/v1/status')
 * WRONG:   fetch('/api/v1/status')
 */
import { Routes, Route } from 'react-router-dom';
import Layout from './components/Layout';
import OverviewPage from './pages/OverviewPage';
import MCPToolsPage from './pages/MCPToolsPage';
import DeploymentPage from './pages/DeploymentPage';
import AuthFlowPage from './pages/AuthFlowPage';
import MultiTenancyPage from './pages/MultiTenancyPage';
import APIExamplesPage from './pages/APIExamplesPage';
import UserList from './components/UserList';

function App() {
  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route index element={<OverviewPage />} />
        <Route path="mcp-tools" element={<MCPToolsPage />} />
        <Route path="deployment" element={<DeploymentPage />} />
        <Route path="auth-flow" element={<AuthFlowPage />} />
        <Route path="multi-tenancy" element={<MultiTenancyPage />} />
        <Route path="api-examples" element={<APIExamplesPage />} />
        <Route path="platform-users" element={<UserList />} />
        <Route path="*" element={<div className="error-page"><h1>404 - Page Not Found</h1></div>} />
      </Route>
    </Routes>
  );
}

export default App;
