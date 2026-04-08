/**
 * MOSES ROUTING: All fetch() calls MUST use relative paths (no leading '/').
 * Your app is served at a subpath (/apps/workspace/app-slug/).
 * Relative paths route through the app's nginx proxy to the backend.
 * Absolute paths (fetch('/api/...')) bypass the app and hit the Moses platform (404).
 *
 * CORRECT: fetch('api/v1/status')
 * WRONG:   fetch('/api/v1/status')
 */
import { useEffect, useState, useCallback } from 'react'

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

interface Item {
  id: string
  title: string
  created_at: string
}

function App() {
  const [status, setStatus] = useState<StatusResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const [items, setItems] = useState<Item[]>([])
  const [itemsLoading, setItemsLoading] = useState(true)
  const [itemsError, setItemsError] = useState<string | null>(null)
  const [newTitle, setNewTitle] = useState('')
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    // Relative path — routed through nginx proxy to backend (NEVER use '/api/...')
    fetch('api/v1/status')
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

  const loadItems = useCallback(() => {
    setItemsLoading(true)
    fetch('api/v1/items')
      .then(res => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.json()
      })
      .then(data => {
        setItems(data)
        setItemsError(null)
        setItemsLoading(false)
      })
      .catch(err => {
        setItemsError(err.message)
        setItemsLoading(false)
      })
  }, [])

  useEffect(() => {
    loadItems()
  }, [loadItems])

  const addItem = () => {
    const title = newTitle.trim()
    if (!title || submitting) return

    setSubmitting(true)
    fetch('api/v1/items', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title }),
    })
      .then(res => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.json()
      })
      .then((item: Item) => {
        setItems(prev => [...prev, item])
        setNewTitle('')
        setSubmitting(false)
      })
      .catch(err => {
        setItemsError(err.message)
        setSubmitting(false)
      })
  }

  const deleteItem = (id: string) => {
    fetch(`api/v1/items/${id}`, { method: 'DELETE' })
      .then(res => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        setItems(prev => prev.filter(item => item.id !== id))
      })
      .catch(err => {
        setItemsError(err.message)
      })
  }

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

        <section className="card">
          <h2>Items</h2>
          <div className="items-input">
            <input
              type="text"
              className="input"
              placeholder="New item title..."
              value={newTitle}
              onChange={e => setNewTitle(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && addItem()}
              disabled={submitting}
            />
            <button className="btn" onClick={addItem} disabled={submitting || !newTitle.trim()}>
              Add
            </button>
          </div>
          {itemsLoading && <p className="loading">Loading items...</p>}
          {itemsError && <p className="error">Error: {itemsError}</p>}
          {!itemsLoading && items.length === 0 && (
            <p className="empty">No items yet. Add one above.</p>
          )}
          {items.length > 0 && (
            <ul className="items-list">
              {items.map(item => (
                <li className="items-row" key={item.id}>
                  <div className="items-info">
                    <span className="value">{item.title}</span>
                    <span className="label">{new Date(item.created_at).toLocaleString()}</span>
                  </div>
                  <button className="btn-delete" onClick={() => deleteItem(item.id)} title="Delete">
                    &times;
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>
      </main>
    </div>
  )
}

export default App
