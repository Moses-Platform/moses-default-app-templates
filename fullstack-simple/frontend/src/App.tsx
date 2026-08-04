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

// Build your app here. A worked `things` slice ships as REAL, CI-compiled code
// that nothing imports (so it never reaches a bundle or a binary):
//   backend/cmd/server/example_test.go          routes
//   backend/internal/handler/example_test.go    handlers
//   backend/api/api.go                          the "/things" spec, in a comment
//                                               above //go:embed (openapi.json
//                                               ships, so it stays a comment)
//   src/api/example.ts                          getThings / createThing,
//                                               queryKeys.things, useThings /
//                                               useCreateThing
//   src/example.tsx                             the consuming component
// Move each piece into its real home (client.ts / queryKeys.ts / hooks.ts and a
// component rendered below), rename to your resource, then delete the example
// files.
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
