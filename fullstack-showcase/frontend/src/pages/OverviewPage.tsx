import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { getMosesInfo } from '../api/client';
import FeatureCard from '../components/FeatureCard';
import './OverviewPage.css';

export default function OverviewPage() {
  const navigate = useNavigate();
  const [mosesInfo, setMosesInfo] = useState<any>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getMosesInfo()
      .then(setMosesInfo)
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  const features = [
    {
      icon: '🔧',
      title: 'MCP Tools',
      description: '56+ tools across 5 profiles with unified CRUD and dynamic proxy generation',
      path: '/mcp-tools',
    },
    {
      icon: '🚀',
      title: 'Deployment Pipeline',
      description: 'Dual-pipeline system for apps and workspace tools with Kaniko + Helm',
      path: '/deployment',
    },
    {
      icon: '🔐',
      title: 'Authentication',
      description: 'Multi-layer auth with OIDC, JWT, API keys, and forward auth',
      path: '/auth-flow',
    },
    {
      icon: '👥',
      title: 'Multi-Tenancy',
      description: 'Complete tenant isolation with 4-tier role hierarchy and RBAC',
      path: '/multi-tenancy',
    },
    {
      icon: '🔍',
      title: 'OpenAPI Discovery',
      description: 'Auto-probe endpoints and generate dynamic MCP tools from specs',
      path: '/deployment',
    },
    {
      icon: '💻',
      title: 'API Examples',
      description: 'Interactive playground to explore Moses API endpoints',
      path: '/api-examples',
    },
  ];

  return (
    <div className="overview-page">
      <section className="hero-section">
        <h2>Welcome to Moses Platform</h2>
        <p className="hero-subtitle">
          A comprehensive MCP-based development platform with agent workspace provisioning,
          deployment automation, and enterprise-grade security.
        </p>
      </section>

      <section className="moses-context-section">
        <h3>Current Moses Context</h3>
        {loading ? (
          <div className="context-loading">Loading context...</div>
        ) : mosesInfo ? (
          <div className="context-info">
            <div className="context-item">
              <span className="context-label">Deployment Mode:</span>
              <span className={`context-value mode-${mosesInfo.deployment_mode}`}>
                {mosesInfo.deployment_mode === 'mcp-proxy' ? '🟢 MCP Proxy' : '🟡 Standalone'}
              </span>
            </div>
            {mosesInfo.headers_present ? (
              <>
                <div className="context-item">
                  <span className="context-label">Tenant ID:</span>
                  <code className="context-value">{mosesInfo.tenant_id || 'N/A'}</code>
                </div>
                <div className="context-item">
                  <span className="context-label">User ID:</span>
                  <code className="context-value">{mosesInfo.user_id || 'N/A'}</code>
                </div>
                <div className="context-item">
                  <span className="context-label">Chart ID:</span>
                  <code className="context-value">{mosesInfo.chart_id || 'N/A'}</code>
                </div>
                <div className="context-item">
                  <span className="context-label">Request ID:</span>
                  <code className="context-value">{mosesInfo.request_id || 'N/A'}</code>
                </div>
              </>
            ) : (
              <div className="context-standalone">
                <p>
                  Running in standalone mode. When deployed via Moses, this section will display
                  your tenant, user, and chart context from X-Moses-* headers.
                </p>
              </div>
            )}
          </div>
        ) : (
          <div className="context-error">Failed to load Moses context</div>
        )}
      </section>

      <section className="features-section">
        <h3>Explore Platform Capabilities</h3>
        <div className="features-grid">
          {features.map((feature) => (
            // key=title because two features intentionally deep-link to the
            // same path (Deployment Pipeline and OpenAPI Discovery both land
            // on /deployment); titles are unique even when paths overlap.
            <FeatureCard
              key={feature.title}
              icon={feature.icon}
              title={feature.title}
              description={feature.description}
              onClick={() => navigate(feature.path)}
            />
          ))}
        </div>
      </section>
    </div>
  );
}
