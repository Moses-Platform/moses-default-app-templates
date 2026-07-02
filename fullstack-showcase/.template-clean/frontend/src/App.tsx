/**
 * MOSES ROUTING: All fetch() calls MUST use relative paths (no leading '/').
 * Your app is served at a subpath (/apps/workspace/app-slug/).
 * Relative paths route through the app's nginx proxy to the backend.
 * Absolute paths (fetch('/api/...')) bypass the app and hit the Moses platform (404).
 *
 * CORRECT: fetch('api/v1/status')
 * WRONG:   fetch('/api/...')
 */
import { Routes, Route } from 'react-router-dom';
import Layout from './components/Layout';

// Placeholder home page — replace with your app's pages. Data loading goes
// through the TanStack Query hooks in ./api/hooks.ts (never useEffect+fetch).
function HomePage() {
  return (
    <div>
      <h2>Hello from your new Moses app</h2>
      <p>
        The showcase demo was cleaned out. Add pages under <code>src/pages/</code>, routes below,
        API calls in <code>src/api/client.ts</code>, and backend routes in{' '}
        <code>backend/cmd/server/demo_routes.go</code>.
      </p>
    </div>
  );
}

function App() {
  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route index element={<HomePage />} />
        <Route path="*" element={<div className="error-page"><h1>404 - Page Not Found</h1></div>} />
      </Route>
    </Routes>
  );
}

export default App;
