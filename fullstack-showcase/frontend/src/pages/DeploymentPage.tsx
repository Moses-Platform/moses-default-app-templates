import FlowDiagram from '../components/FlowDiagram';
import './DeploymentPage.css';

export default function DeploymentPage() {
  const resourceTiers = [
    { tier: 'Small', cpu: '100m-200m', memory: '128Mi-256Mi', use: 'Lightweight apps, tools' },
    { tier: 'Medium', cpu: '200m-500m', memory: '256Mi-512Mi', use: 'Standard apps, APIs' },
    { tier: 'Large', cpu: '500m-1000m', memory: '512Mi-1Gi', use: 'Heavy workloads, databases' },
  ];

  return (
    <div className="deployment-page">
      <h2>Deployment Pipeline</h2>
      <p className="page-intro">
        Moses features two deployment pipelines: Agent Execution for workspace-generated apps
        and Workspace Tools for marketplace integrations.
      </p>

      <section className="section">
        <h3>Agent Execution Pipeline</h3>
        <p>
          Triggered when an agent completes work via <code>moses_agent_submit_completed</code>.
          Builds Docker images with Kaniko and deploys using Helm charts from the workspace repo.
        </p>
        <FlowDiagram
          title="Execution Flow"
          steps={[
            { label: 'Agent Work', icon: '🤖', description: 'Complete task' },
            { label: 'Submit Completed', icon: '✅', description: 'moses_agent_submit_completed' },
            { label: 'Kaniko Build', icon: '📦', description: 'In-cluster build' },
            { label: 'Helm Deploy', icon: '🚢', description: 'K8s deployment' },
            { label: 'Ingress Route', icon: '🌐', description: 'Path-based routing' },
            { label: 'Live App', icon: '🎉', description: 'Accessible via URL' },
          ]}
        />
      </section>

      <section className="section">
        <h3>Workspace Tools Pipeline</h3>
        <p>
          Marketplace tools are registered, cloned from Git, and deployed automatically.
          OpenAPI endpoints are discovered and converted to dynamic MCP tools.
        </p>
        <FlowDiagram
          title="Tool Deployment Flow"
          steps={[
            { label: 'Marketplace', icon: '🏪', description: 'Tool registry' },
            { label: 'Clone Repo', icon: '📥', description: 'Git shallow clone' },
            { label: 'Config Parse', icon: '📄', description: 'moses-app.config.json' },
            { label: 'Kaniko Build', icon: '📦', description: 'Multi-image build' },
            { label: 'Helm Deploy', icon: '🚢', description: 'Multi-service chart' },
            { label: 'OpenAPI Discovery', icon: '🔍', description: '11 probe paths' },
            { label: 'MCP Tools', icon: '🔧', description: 'Dynamic proxy' },
          ]}
        />
      </section>

      <section className="section">
        <h3>Resource Tiers</h3>
        <div className="resource-table">
          <table>
            <thead>
              <tr>
                <th>Tier</th>
                <th>CPU</th>
                <th>Memory</th>
                <th>Use Cases</th>
              </tr>
            </thead>
            <tbody>
              {resourceTiers.map((tier) => (
                <tr key={tier.tier}>
                  <td><strong>{tier.tier}</strong></td>
                  <td><code>{tier.cpu}</code></td>
                  <td><code>{tier.memory}</code></td>
                  <td>{tier.use}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="section">
        <h3>Helm Chart Structure</h3>
        <pre><code>{`helm/
├── Chart.yaml              # Chart metadata
├── values.yaml             # Default configuration
└── templates/
    ├── deployment.yaml     # Pod definitions
    ├── service.yaml        # Service definitions
    ├── ingress.yaml        # Routing rules (optional)
    └── _helpers.tpl        # Template helpers

# Multi-service values.yaml example:
services:
  - name: frontend
    image: {repository: "app-frontend", tag: "latest"}
    port: 80
    replicas: 1
  - name: backend
    image: {repository: "app-backend", tag: "latest"}
    port: 8080
    replicas: 1`}</code></pre>
      </section>

      <section className="section">
        <h3>Kaniko Builds</h3>
        <div className="info-box">
          <h4>In-Cluster Docker Builds</h4>
          <p>
            Kaniko builds Docker images inside Kubernetes without requiring a Docker daemon.
            This enables secure, unprivileged container builds in the cluster.
          </p>
          <ul>
            <li>No privileged containers required</li>
            <li>Multi-stage builds supported</li>
            <li>Direct push to GCR/registry</li>
            <li>Build logs available via K8s API</li>
            <li>Automatic cleanup after completion</li>
          </ul>
        </div>
      </section>
    </div>
  );
}
