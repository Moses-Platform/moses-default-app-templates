import FeatureCard from '../components/FeatureCard';
import FlowDiagram from '../components/FlowDiagram';
import './MCPToolsPage.css';

export default function MCPToolsPage() {
  const toolProfiles = [
    { profile: 'external-minimal', tools: '~10', description: 'Basic read operations for external clients' },
    { profile: 'moses-manager-full', tools: '~40', description: 'Full Moses Manager UI operations' },
    { profile: 'agent-execution', tools: '~30', description: 'Agent workspace development tools' },
    { profile: 'build-callback', tools: '~8', description: 'Build notification and status updates' },
    { profile: 'autonomous', tools: '~45', description: 'Extends moses-manager-full with write operations' },
  ];

  const crudTools = [
    'moses_query - Flexible read operations with filters',
    'moses_create - Create entities with validation',
    'moses_update - Update entities by ID',
    'moses_delete - Delete entities with cascade',
    'moses_batch - Batch operations for efficiency',
  ];

  const workspaceTools = [
    'OpenAPI discovery from 11 standard paths',
    'Auto-probe service endpoints with port fallback',
    'Parse JSON/YAML OpenAPI 3.x specs',
    'Generate workspace_{toolKey}_{operationId} tools',
    'Dynamic proxy routes calls to service endpoints',
    'Schema validation from OpenAPI definitions',
  ];

  return (
    <div className="mcp-tools-page">
      <h2>Model Context Protocol (MCP) Tools</h2>
      <p className="page-intro">
        Moses implements a comprehensive MCP tool system with 56+ tools across 5 profiles,
        providing everything from basic CRUD operations to dynamic workspace tool generation.
      </p>

      <section className="section">
        <h3>Unified CRUD Tools</h3>
        <p>
          Primary tools for data operations across all Moses entities (tickets, charts, lanes,
          contexts, etc.). These tools replace dozens of entity-specific operations.
        </p>
        <FeatureCard
          icon="📦"
          title="CRUD Hierarchy"
          description="moses_query, moses_create, moses_update, moses_delete, moses_batch"
          details={crudTools}
        />
      </section>

      <section className="section">
        <h3>Workspace Tools</h3>
        <p>
          Dynamic MCP tool generation from OpenAPI specifications. When a workspace tool is
          deployed, Moses automatically discovers its API endpoints and creates corresponding
          MCP tools.
        </p>
        <FeatureCard
          icon="🔌"
          title="OpenAPI → MCP Proxy"
          description="Automatic tool generation from service endpoints"
          details={workspaceTools}
        />
      </section>

      <section className="section">
        <h3>Tool Profiles</h3>
        <p>
          Moses uses profile-based tool access control. Each API key is assigned a profile that
          determines which tools are available.
        </p>
        <div className="profiles-table">
          <table>
            <thead>
              <tr>
                <th>Profile</th>
                <th>Tool Count</th>
                <th>Description</th>
              </tr>
            </thead>
            <tbody>
              {toolProfiles.map((profile) => (
                <tr key={profile.profile}>
                  <td><code>{profile.profile}</code></td>
                  <td>{profile.tools}</td>
                  <td>{profile.description}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="section">
        <h3>MCP Protocol Flow</h3>
        <FlowDiagram
          steps={[
            { label: 'Agent', icon: '🤖', description: 'Claude Code' },
            { label: 'MCP Protocol', icon: '📡', description: 'JSON-RPC' },
            { label: 'Moses Backend', icon: '⚙️', description: 'Tool routing' },
            { label: 'Tools', icon: '🔧', description: 'CRUD/Proxy' },
            { label: 'Database', icon: '💾', description: 'PostgreSQL' },
          ]}
        />
      </section>

      <section className="section">
        <h3>Tool Discovery & Enforcement</h3>
        <div className="info-box">
          <h4>Three-Layer Enforcement</h4>
          <ol>
            <li><strong>Tool Metadata:</strong> All tools registered in registry</li>
            <li><strong>Tool Listing:</strong> Filtered by API key profile (tools/list)</li>
            <li><strong>Tool Execution:</strong> Permission check at runtime</li>
          </ol>
          <p className="info-note">
            Claude discovers available tools via <code>tools/list</code> which now correctly
            filters by the caller's profile, preventing "permission denied" errors on legacy tools.
          </p>
        </div>
      </section>
    </div>
  );
}
