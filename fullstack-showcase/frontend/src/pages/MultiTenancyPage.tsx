import { useMosesInfo } from '../api/hooks';
import NotesPanel from '../components/NotesPanel';
import './MultiTenancyPage.css';

export default function MultiTenancyPage() {
  const { data: mosesInfo, isPending, isError } = useMosesInfo();

  const isolationLayers = [
    { layer: 'Database', description: 'tenant_id column on all tables, enforced by foreign keys' },
    { layer: 'Store Layer', description: 'WHERE tenant_id = $1 in all queries' },
    { layer: 'API Layer', description: 'JWT extraction and tenant context injection' },
    { layer: 'RBAC Layer', description: 'Permission checks for every operation' },
  ];

  const roles = [
    { role: 'GlobalAdmin', permissions: 'All operations across all tenants', level: 4 },
    { role: 'TenantAdmin', permissions: 'Full control within tenant', level: 3 },
    { role: 'Editor', permissions: 'Create, read, update resources', level: 2 },
    { role: 'Viewer', permissions: 'Read-only access', level: 1 },
  ];

  return (
    <div className="multi-tenancy-page">
      <h2>Multi-Tenancy & RBAC</h2>
      <p className="page-intro">
        Moses implements complete tenant isolation across database, application, and infrastructure
        layers with a 4-tier role hierarchy.
      </p>

      <section className="section">
        <h3>Tenant Isolation Layers</h3>
        <div className="layers-list">
          {isolationLayers.map((item, index) => (
            <div key={item.layer} className="layer-item">
              <div className="layer-number">{index + 1}</div>
              <div className="layer-content">
                <h4>{item.layer}</h4>
                <p>{item.description}</p>
              </div>
            </div>
          ))}
        </div>
      </section>

      <section className="section">
        <h3>Role Hierarchy</h3>
        <div className="roles-table">
          <table>
            <thead>
              <tr>
                <th>Role</th>
                <th>Permissions</th>
                <th>Level</th>
              </tr>
            </thead>
            <tbody>
              {roles.map((role) => (
                <tr key={role.role}>
                  <td><strong>{role.role}</strong></td>
                  <td>{role.permissions}</td>
                  <td><span className="level-badge">{role.level}</span></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="section">
        <h3>Live Tenant Context</h3>
        {isPending ? (
          <div className="tenant-loading">Loading tenant context...</div>
        ) : isError ? (
          <div className="tenant-loading">Tenant context unavailable.</div>
        ) : mosesInfo ? (
          <div className="tenant-info">
            <div className="tenant-item">
              <span className="tenant-label">Current Tenant:</span>
              <code className="tenant-value">{mosesInfo.tenant_id || 'N/A (standalone mode)'}</code>
            </div>
            <div className="tenant-item">
              <span className="tenant-label">User ID:</span>
              <code className="tenant-value">{mosesInfo.user_id || 'N/A'}</code>
            </div>
            {!mosesInfo.headers_present && (
              <div className="tenant-note">
                When deployed via Moses, all requests are automatically scoped to your tenant.
                The backend enforces tenant isolation at every layer.
              </div>
            )}
          </div>
        ) : (
          <div className="tenant-loading">Tenant context unavailable.</div>
        )}
      </section>

      <section className="section">
        <h3>Tenant-Scoped Data (live query + mutation)</h3>
        <NotesPanel />
      </section>

      <section className="section">
        <h3>Namespace Isolation</h3>
        <div className="info-box">
          <h4>Kubernetes-Level Security</h4>
          <p>
            Each tenant gets isolated Kubernetes resources with NetworkPolicy and ResourceQuota enforcement:
          </p>
          <ul>
            <li><strong>NetworkPolicy:</strong> Restrict pod-to-pod communication within tenant namespace</li>
            <li><strong>ResourceQuota:</strong> Limit CPU, memory, and storage per tenant</li>
            <li><strong>RBAC:</strong> ServiceAccounts with minimal permissions</li>
            <li><strong>Secrets Isolation:</strong> Tenant secrets never shared across namespaces</li>
            <li><strong>Audit Logs:</strong> K8s API audit logs track all operations with tenant context</li>
          </ul>
        </div>
      </section>

      <section className="section">
        <h3>Permission System</h3>
        <pre><code>{`// Example permission checks in code:

// HTTP endpoint
r.Use(middleware.RBACMiddleware("tickets:write"))

// MCP tool metadata
RequiredPermission: "charts:read"

// Manual check
if !userCtx.HasPermission("deployments:delete") {
    return errors.New("permission denied")
}

// Permission format: <resource>:<action>
// Resources: tickets, charts, lanes, contexts, agents, deployments
// Actions: read, write, delete, admin`}</code></pre>
      </section>
    </div>
  );
}
