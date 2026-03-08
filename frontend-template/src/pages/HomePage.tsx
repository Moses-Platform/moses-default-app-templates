import './HomePage.css'

function HomePage() {
  const features = [
    {
      title: 'Subpath Deployment',
      description: 'Works seamlessly under /apps/{tenant}/{slug}/ with relative paths and proper routing',
      icon: '🚀',
    },
    {
      title: 'SPA Routing',
      description: 'React Router with nginx fallback ensures all routes work correctly',
      icon: '🔗',
    },
    {
      title: 'Health Checks',
      description: '/health endpoint for Kubernetes liveness and readiness probes',
      icon: '💚',
    },
    {
      title: 'Moses UI Standards',
      description: 'Design tokens, 4px grid, accessible colors following Moses guidelines',
      icon: '🎨',
    },
    {
      title: 'Iframe Ready',
      description: 'SAMEORIGIN headers allow embedding in Moses Apps page',
      icon: '📦',
    },
    {
      title: 'Production Build',
      description: 'Multi-stage Docker, gzip compression, and optimized caching',
      icon: '⚡',
    },
  ]

  return (
    <div className="home-page">
      <section className="hero">
        <h1>Moses Frontend Template</h1>
        <p className="hero-subtitle">
          A production-ready React + TypeScript + Vite template for building Moses platform applications
        </p>
      </section>

      <section className="features">
        <h2>Features</h2>
        <div className="feature-grid">
          {features.map((feature, index) => (
            <div key={index} className="feature-card">
              <div className="feature-icon">{feature.icon}</div>
              <h3>{feature.title}</h3>
              <p>{feature.description}</p>
            </div>
          ))}
        </div>
      </section>

      <section className="cta">
        <h2>Ready to Build?</h2>
        <p>
          Start customizing this template for your application. Check out the{' '}
          <a href="/about">About page</a> to learn more.
        </p>
      </section>
    </div>
  )
}

export default HomePage
