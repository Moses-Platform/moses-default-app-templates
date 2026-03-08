import { useEffect, useState } from 'react'

interface StatusResponse {
  app: string
  version: string
  uptime: string
  moses: {
    tenant_id: string
    user_id: string
    chart_id: string
    request_id: string
  }
  env: {
    port: string
    base_url: string
  }
}

function App() {
  const [status, setStatus] = useState<StatusResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetch('/api/v1/status')
      .then(res => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.json()
      })
      .then(data => {
        setStatus(data)
        setLoading(false)
      })
      .catch(err => {
        setError(err.message)
        setLoading(false)
      })
  }, [])

  return (
    <div className="app">
      <header className="header">
        <h1>Fullstack Simple</h1>
        <p>Minimal Go + React template for Moses</p>
      </header>

      <main className="main">
        <section className="card">
          <h2>Backend Status</h2>
          {loading && <p className="loading">Connecting to backend...</p>}
          {error && <p className="error">Error: {error}</p>}
          {status && (
            <div className="status">
              <div className="row">
                <span className="label">App</span>
                <span className="value">{status.app}</span>
              </div>
              <div className="row">
                <span className="label">Version</span>
                <span className="value">{status.version}</span>
              </div>
              <div className="row">
                <span className="label">Uptime</span>
                <span className="value">{status.uptime}</span>
              </div>
            </div>
          )}
        </section>

        {status && status.moses.tenant_id && (
          <section className="card">
            <h2>Moses Context</h2>
            <div className="status">
              {Object.entries(status.moses).map(([key, value]) => (
                <div className="row" key={key}>
                  <span className="label">{key}</span>
                  <span className="value mono">{value || '—'}</span>
                </div>
              ))}
            </div>
          </section>
        )}
      </main>
    </div>
  )
}

export default App
