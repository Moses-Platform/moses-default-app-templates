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
