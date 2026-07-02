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
 * — no data-loading useEffect, no raw fetch in components. React Compiler is
 * enabled (vite.config.ts): do NOT add manual useMemo/useCallback/React.memo.
 */
import ThemeToggle from './components/ThemeToggle'

// The template demo was removed by clean_out_template.sh. Build your app here:
//   1. Backend route in backend/cmd/server/demo_routes.go (+ api/openapi.json)
//   2. Typed client function in src/api/client.ts
//   3. Query hook in src/api/hooks.ts with a key from src/api/queryKeys.ts
//   4. Consume the hook from a component rendered below
function App() {
  return (
    <div className="app">
      <header className="header">
        <div>
          <h1>Your App</h1>
          <p>Fullstack Simple template — demo removed, ready for real work</p>
        </div>
        <ThemeToggle />
      </header>

      <main className="main">
        <section className="card">
          <h2>Getting started</h2>
          <p className="empty">
            Add your first endpoint in the backend, then wire it through
            src/api/client.ts and src/api/hooks.ts.
          </p>
        </section>
      </main>
    </div>
  )
}

export default App
