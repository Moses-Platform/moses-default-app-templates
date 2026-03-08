import './AboutPage.css'

function AboutPage() {
  return (
    <div className="about-page">
      <h1>About This Template</h1>

      <section className="about-section">
        <h2>What is this?</h2>
        <p>
          This is a production-ready frontend template for building applications on the Moses platform.
          It demonstrates best practices for subpath deployment, SPA routing, health checks, and integration
          with Moses infrastructure.
        </p>
      </section>

      <section className="about-section">
        <h2>Technology Stack</h2>
        <ul>
          <li><strong>React 18</strong> - Modern UI framework with hooks and concurrent features</li>
          <li><strong>TypeScript</strong> - Type safety and better developer experience</li>
          <li><strong>Vite</strong> - Fast development server and optimized production builds</li>
          <li><strong>React Router</strong> - Client-side routing for single-page applications</li>
          <li><strong>nginx</strong> - Production web server with health checks and caching</li>
        </ul>
      </section>

      <section className="about-section">
        <h2>How to Customize</h2>
        <ol>
          <li>Update <code>moses-app.config.json</code> with your app details</li>
          <li>Modify the design tokens in <code>src/App.css</code></li>
          <li>Add your pages to <code>src/pages/</code></li>
          <li>Create components in <code>src/components/</code></li>
          <li>Update routes in <code>src/App.tsx</code></li>
          <li>Build with <code>npm run build</code></li>
          <li>Deploy via Moses platform</li>
        </ol>
      </section>

      <section className="about-section">
        <h2>Moses Integration Points</h2>
        <ul>
          <li><strong>Subpath Deployment</strong> - Vite configured with <code>base: './'</code></li>
          <li><strong>Health Endpoint</strong> - nginx serves <code>/health</code> for K8s probes</li>
          <li><strong>Security Headers</strong> - SAMEORIGIN, nosniff, XSS protection</li>
          <li><strong>Design Tokens</strong> - CSS variables matching Moses UI standards</li>
          <li><strong>4px Grid</strong> - Spacing system aligned with platform</li>
        </ul>
      </section>

      <section className="about-section">
        <h2>Links</h2>
        <ul>
          <li><a href="/">Home</a></li>
          <li><a href="https://github.com/your-org/moses" target="_blank" rel="noopener noreferrer">Moses Documentation</a></li>
        </ul>
      </section>
    </div>
  )
}

export default AboutPage
