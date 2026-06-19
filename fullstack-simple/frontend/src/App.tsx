/**
 * MOSES ROUTING: All fetch() calls MUST use relative paths (no leading '/').
 * Your app is served at a subpath (/apps/workspace/app-slug/).
 * Relative paths route through the app's nginx proxy to the backend.
 * Absolute paths (fetch('/api/...')) bypass the app and hit the Moses platform (404).
 *
 * CORRECT: fetch('api/v1/status')
 * WRONG:   fetch('/api/v1/status')
 *
 * Data access goes through the typed client + TanStack Query hooks in src/api/
 * (see FRONTEND_DATA_LAYER.md) — no data-loading useEffect, no raw fetch here.
 */
import { useState } from 'react'
import ThemeToggle from './components/ThemeToggle'
import { useStatus, useItems, useCreateItem, useDeleteItem } from './api/hooks'
import { getErrorMessage } from './api/client'

function App() {
  const statusQuery = useStatus()
  const itemsQuery = useItems()
  const createItem = useCreateItem()
  const deleteItem = useDeleteItem()

  const [newTitle, setNewTitle] = useState('')

  const status = statusQuery.data
  const items = itemsQuery.data ?? []

  // Surface a write error alongside any read error in the items card.
  const itemsError =
    (itemsQuery.isError && getErrorMessage(itemsQuery.error)) ||
    (createItem.isError && getErrorMessage(createItem.error)) ||
    (deleteItem.isError && getErrorMessage(deleteItem.error)) ||
    null

  const submitting = createItem.isPending

  const addItem = () => {
    const title = newTitle.trim()
    if (!title || submitting) return
    createItem.mutate({ title }, { onSuccess: () => setNewTitle('') })
  }

  return (
    <div className="app">
      <header className="header">
        <div>
          <h1>Fullstack Simple</h1>
          <p>Minimal Go + React template for Moses</p>
        </div>
        <ThemeToggle />
      </header>

      <main className="main">
        <section className="card">
          <h2>Backend Status</h2>
          {statusQuery.isPending && <p className="loading">Connecting to backend...</p>}
          {statusQuery.isError && <p className="error">Error: {getErrorMessage(statusQuery.error)}</p>}
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

        {status && status.moses.chart_id && (
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
          {itemsQuery.isPending && <p className="loading">Loading items...</p>}
          {itemsError && <p className="error">Error: {itemsError}</p>}
          {!itemsQuery.isPending && items.length === 0 && (
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
                  <button
                    className="btn-delete"
                    onClick={() => deleteItem.mutate(item.id)}
                    title="Delete"
                  >
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
