import { useMosesInfo } from '../api/hooks';
import FlowDiagram from '../components/FlowDiagram';
import './AuthFlowPage.css';

export default function AuthFlowPage() {
  const { data: mosesInfo, isPending: loading } = useMosesInfo();

  const authMethods = [
    { name: 'OIDC/JWT', description: 'Keycloak integration for web users' },
    { name: 'API Keys', description: 'SHA256-hashed keys for MCP clients' },
    { name: 'OAuth Tokens', description: 'Runtime credential injection for agents' },
    { name: 'Forward Auth', description: 'Ingress-level auth via moses-backend' },
  ];

  const mosesHeaders = [
    { header: 'X-Moses-Tenant-ID', description: 'Caller tenant UUID (audit-only post-CHAT-pxeo.12; storage uses MOSES_TENANT_ID env)' },
    { header: 'X-Moses-User-ID', description: 'Authenticated user UUID' },
    { header: 'X-Moses-Chart-ID', description: 'Current chart/project UUID' },
    { header: 'X-Moses-Tool-ID', description: 'Workspace tool UUID (if applicable)' },
    { header: 'X-Moses-Request-ID', description: 'Correlation ID for request tracing' },
    { header: 'X-Moses-MCP-Source', description: 'MCP client identifier' },
    { header: 'X-Moses-API-Key-ID', description: 'API key UUID (for MCP auth)' },
  ];

  return (
    <div className="auth-flow-page">
      <h2>Authentication & Authorization</h2>
      <p className="page-intro">
        Moses implements multi-layer authentication with OIDC, JWT, API keys, and forward auth,
        ensuring secure access across web UI, MCP clients, and agent workspaces.
      </p>

      <section className="section">
        <h3>Authentication Flow</h3>
        <FlowDiagram
          steps={[
            { label: 'Browser', icon: '🌐', description: 'User request' },
            { label: 'Ingress', icon: '🚪', description: 'NGINX controller' },
            { label: 'ForwardAuth', icon: '🔐', description: 'Auth check' },
            { label: 'Moses Backend', icon: '⚙️', description: 'Validate JWT' },
            { label: 'Keycloak', icon: '🔑', description: 'Identity provider' },
          ]}
        />
      </section>

      <section className="section">
        <h3>Authentication Methods</h3>
        <div className="methods-grid">
          {authMethods.map((method) => (
            <div key={method.name} className="method-card">
              <h4>{method.name}</h4>
              <p>{method.description}</p>
            </div>
          ))}
        </div>
      </section>

      <section className="section">
        <h3>Live Context</h3>
        {loading ? (
          <div className="context-loading">Loading...</div>
        ) : mosesInfo ? (
          <div className="headers-display">
            <div className="header-item mode">
              <span className="header-label">Deployment Mode:</span>
              <span className={`header-value mode-${mosesInfo.deployment_mode}`}>
                {mosesInfo.deployment_mode}
              </span>
            </div>
            {Object.entries(mosesInfo).map(([key, value]) => {
              if (key === 'deployment_mode' || key === 'headers_present') return null;
              return (
                <div key={key} className="header-item">
                  <span className="header-label">X-Moses-{key.split('_').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join('-')}:</span>
                  <code className="header-value">{value as string || 'N/A'}</code>
                </div>
              );
            })}
          </div>
        ) : (
          <div className="context-error">Failed to load context</div>
        )}
      </section>

      <section className="section">
        <h3>Moses Headers</h3>
        <div className="headers-table">
          <table>
            <thead>
              <tr>
                <th>Header</th>
                <th>Description</th>
              </tr>
            </thead>
            <tbody>
              {mosesHeaders.map((item) => (
                <tr key={item.header}>
                  <td><code>{item.header}</code></td>
                  <td>{item.description}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="section">
        <h3>Security Features</h3>
        <div className="info-box">
          <h4>Multi-Layer Protection</h4>
          <ul>
            <li><strong>Forward Auth:</strong> Ingress-level authentication before requests reach apps</li>
            <li><strong>JWT Validation:</strong> Cryptographically signed tokens with expiry</li>
            <li><strong>API Key Hashing:</strong> SHA256 hashing, never storing plaintext keys</li>
            <li><strong>RBAC Enforcement:</strong> Permission checks at every API endpoint and MCP tool</li>
            <li><strong>Tenant Isolation:</strong> Automatic filtering by tenant_id at database layer</li>
            <li><strong>Audit Logging:</strong> All auth events tracked with context</li>
          </ul>
        </div>
      </section>
    </div>
  );
}
