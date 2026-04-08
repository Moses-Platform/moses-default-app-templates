import { useState } from 'react';
import { getHealth, getMosesInfo, getCapabilities, getCapability } from '../api/client';
import './APIExamplesPage.css';

export default function APIExamplesPage() {
  const [activeRequest, setActiveRequest] = useState<string | null>(null);
  const [response, setResponse] = useState<any>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const executeRequest = async (name: string, fn: () => Promise<any>) => {
    setActiveRequest(name);
    setLoading(true);
    setError(null);
    try {
      const data = await fn();
      setResponse(data);
    } catch (err: any) {
      setError(err.message || 'Request failed');
    } finally {
      setLoading(false);
    }
  };

  const endpoints = [
    {
      name: 'Health Check',
      method: 'GET',
      path: '/health',
      description: 'Verify service health',
      fn: getHealth,
    },
    {
      name: 'Moses Info',
      method: 'GET',
      path: '/api/v1/moses-info',
      description: 'Get current Moses context from headers',
      fn: getMosesInfo,
    },
    {
      name: 'List Capabilities',
      method: 'GET',
      path: '/api/v1/capabilities',
      description: 'Get all Moses platform capabilities',
      fn: getCapabilities,
    },
    {
      name: 'Get MCP Tools Capability',
      method: 'GET',
      path: '/api/v1/capabilities/mcp-tools',
      description: 'Get detailed info about MCP tools',
      fn: () => getCapability('mcp-tools'),
    },
  ];

  return (
    <div className="api-examples-page">
      <h2>Interactive API Examples</h2>
      <p className="page-intro">
        Explore the Moses API by making live requests. Click any endpoint to see the request
        details and response data.
      </p>

      <section className="section">
        <h3>Available Endpoints</h3>
        <div className="endpoints-list">
          {endpoints.map((endpoint) => (
            <div key={endpoint.name} className="endpoint-card">
              <div className="endpoint-header">
                <span className={`method-badge method-${endpoint.method.toLowerCase()}`}>
                  {endpoint.method}
                </span>
                <code className="endpoint-path">{endpoint.path}</code>
              </div>
              <p className="endpoint-description">{endpoint.description}</p>
              <button
                className="try-button"
                onClick={() => executeRequest(endpoint.name, endpoint.fn)}
                disabled={loading && activeRequest === endpoint.name}
              >
                {loading && activeRequest === endpoint.name ? 'Loading...' : 'Try It'}
              </button>
            </div>
          ))}
        </div>
      </section>

      {(response || error) && (
        <section className="section">
          <h3>Response</h3>
          {error ? (
            <div className="response-error">
              <h4>Error</h4>
              <pre>{error}</pre>
            </div>
          ) : (
            <div className="response-success">
              <div className="response-header">
                <h4>Success - {activeRequest}</h4>
                <span className="status-badge">200 OK</span>
              </div>
              <pre><code>{JSON.stringify(response, null, 2)}</code></pre>
            </div>
          )}
        </section>
      )}

      <section className="section">
        <h3>OpenAPI → MCP Tools</h3>
        <p>
          Moses automatically generates MCP tools from OpenAPI specifications. When a workspace
          tool is deployed, the backend:
        </p>
        <div className="process-steps">
          <div className="process-step">
            <div className="step-number">1</div>
            <div className="step-content">
              <h4>Discover Endpoints</h4>
              <p>Probe 11 standard OpenAPI paths (swagger.json, openapi.yaml, etc.)</p>
            </div>
          </div>
          <div className="process-step">
            <div className="step-number">2</div>
            <div className="step-content">
              <h4>Parse OpenAPI Spec</h4>
              <p>Extract operations, paths, schemas from JSON/YAML spec</p>
            </div>
          </div>
          <div className="process-step">
            <div className="step-number">3</div>
            <div className="step-content">
              <h4>Generate MCP Tools</h4>
              <p>Create workspace_&#123;toolKey&#125;_&#123;operationId&#125; tools</p>
            </div>
          </div>
          <div className="process-step">
            <div className="step-number">4</div>
            <div className="step-content">
              <h4>Dynamic Proxy</h4>
              <p>WorkspaceToolProxy routes MCP calls to service endpoints</p>
            </div>
          </div>
        </div>
      </section>

      <section className="section">
        <h3>Example: This App's OpenAPI</h3>
        <p>
          This showcase app exposes an OpenAPI spec at <code>/api/openapi.json</code>. If deployed
          as a workspace tool, Moses would generate these MCP tools:
        </p>
        <div className="tool-examples">
          <div className="tool-example">
            <code>workspace_showcase_healthCheck</code>
            <p>→ GET /health</p>
          </div>
          <div className="tool-example">
            <code>workspace_showcase_getMosesInfo</code>
            <p>→ GET /api/v1/moses-info</p>
          </div>
          <div className="tool-example">
            <code>workspace_showcase_listCapabilities</code>
            <p>→ GET /api/v1/capabilities</p>
          </div>
          <div className="tool-example">
            <code>workspace_showcase_getCapability</code>
            <p>→ GET /api/v1/capabilities/&#123;id&#125;</p>
          </div>
        </div>
      </section>

      <section className="section">
        <h3>Integration Patterns</h3>
        <div className="info-box">
          <h4>Building Moses-Integrated Apps</h4>
          <ul>
            <li><strong>Headers Middleware:</strong> Extract X-Moses-* headers for context</li>
            <li><strong>OpenAPI Spec:</strong> Document all endpoints with operationIds</li>
            <li><strong>Tenant Scoping:</strong> Use X-Moses-Tenant-ID to filter data</li>
            <li><strong>Health Checks:</strong> Implement /health for deployment verification</li>
            <li><strong>CORS:</strong> Allow Moses backend origin for frontend→backend calls</li>
            <li><strong>Relative URLs:</strong> Use relative api/* paths (no leading /) for nginx proxy compatibility</li>
          </ul>
        </div>
      </section>
    </div>
  );
}
