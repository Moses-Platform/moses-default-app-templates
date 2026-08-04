import { Routes, Route } from 'react-router-dom'
import Layout from './components/Layout'

// Add your pages under src/pages/ and register routes below. Route paths are
// basename-relative — main.tsx sets <BrowserRouter basename={getBasePath()}>
// from the moses-base-path meta tag (CHAT-pbup), so "/" resolves to
// /apps/<tenant>/<slug>/ when deployed through Moses. Use react-router
// <Link to="..."> for internal navigation, never raw absolute <a href="/...">.
//
// This template is frontend-only: point src/api/client.ts at whatever backend
// serves your data. A worked `things` slice ships as REAL, CI-compiled code
// that nothing imports (so it is tree-shaken out of the bundle):
// src/api/example.ts (transport + query key + hooks) and src/example.tsx (the
// consuming component). Move them into client.ts / queryKeys.ts / hooks.ts and
// a page under src/pages/, rename to your real resource, then register the
// route below in place of the placeholder.
//
// Data loading goes through the TanStack Query hooks — never a useEffect
// fetch, and never a raw fetch() in a component.

function HomePlaceholder() {
  return (
    <section className="section">
      <h1>My App</h1>
      <p>
        Replace this placeholder: add pages under <code>src/pages/</code> and
        register routes in <code>src/App.tsx</code>.
      </p>
    </section>
  )
}

function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<HomePlaceholder />} />
      </Routes>
    </Layout>
  )
}

export default App
