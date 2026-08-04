# Frontend Template - Agent Customization Guide

## What you start with

This template is demo-free: what ships is the Moses plumbing plus one
placeholder Home route. `src/main.tsx` sets the router basename from the
base-path meta, installs the QueryClientProvider and the browser-logger;
`src/utils/baseUrl.ts` (+ test) is `getBasePath()`, the CHAT-pbup base-path
contract; `src/moses-browser-logger.ts` (+ test) is vendored (BLF-B) — do NOT
edit it, it is drift-gated against `shared/`. The data layer is
`src/api/queryClient.ts` (shared QueryClient defaults: staleTime 30s, no focus
refetch) plus `client.ts` (transport only —
`fetchAPI`/`fetchText`/`mutateAPI`/`mutateAPINoContent`) and empty
`hooks.ts` / `queryKeys.ts` factories carrying the canonical shapes. The shell
is `src/App.tsx` (placeholder Home route inside `Layout`),
`src/components/Layout.tsx`, `Layout.css`, `ThemeToggle.tsx`,
`src/styles/theme.css` (design tokens, dark default + `[data-theme="light"]`)
and `src/App.css`. `index.html` keeps the `moses-base-path` meta + the no-flash
theme script. Deploy plumbing — CSP framing, sub-path rewrite, base-path meta
render — lives in `nginx.conf`, `entrypoint.sh`, `Dockerfile`, `helm/`,
`public/`. `npm run lint && npm test && npm run build` is green on a fresh
clone.

## Environment-variable contract

Everything the template reads, and where:

| Variable | Read in | Purpose |
|---|---|---|
| `MOSES_BASE_PATH` | `entrypoint.sh` (runtime) | Sub-path mount (`/apps/<t>/<a>/`); rendered into the `moses-base-path` meta tag and the nginx sub-path rewrite block |
| `MOSES_EMBEDDING_FRAMING` | `entrypoint.sh` (runtime) | `moses-only` (default) \| `public` \| `denied` — CSP `frame-ancestors`; `denied` also emits `X-Frame-Options: DENY` |
| `MOSES_EMBEDDING_ALLOWED_ANCESTORS` | `entrypoint.sh` (runtime) | Explicit CSP source list for `moses-only` (overrides the parity default) |
| `MOSES_EMBEDDING_REPORT_URI` | `entrypoint.sh` (runtime) | Optional CSP report-uri |
| `MOSES_DOMAIN` | `entrypoint.sh` (runtime) | Platform domain used to extend the default `moses-only` ancestors list |
| `VITE_MOSES_CHART_ID` | `moses-browser-logger.ts` (build-time, Dockerfile ARG) | Browser-log reporter target chart; logger is a silent no-op when absent |
| `VITE_MOSES_DEPLOYMENT_ID` | `moses-browser-logger.ts` (build-time, Dockerfile ARG) | Browser-log reporter deployment identity |
| `VITE_MOSES_API_BASE` | `moses-browser-logger.ts` (build-time, Dockerfile ARG) | Log-ingest endpoint base |
| `VITE_BASE_URL` | `src/utils/baseUrl.ts` (build-time, rare) | Legacy build-time base-path fallback; runtime meta tag is preferred |
| `NPM_REGISTRY` | `Dockerfile` (build ARG) | Optional in-cluster npm mirror (CHAT-s6qf3) |

There is no `/api` backend in this template — it is a static SPA served by
nginx. `nginx.conf` serves `/health` directly for K8s probes.

## Non-negotiable plumbing rules

1. **Router basename is required.** `main.tsx` does
   `<BrowserRouter basename={getBasePath()}>`; the ingress does NOT strip the
   `/apps/<t>/<a>` prefix. Never remove it.
2. **Internal links use `<Link to="...">`** (react-router). A raw
   `<a href="/about">` escapes the app mount and hits the platform router.
3. **Fetch paths are relative** (`'api/v1/...'`, never `'/api/...'`) so they
   resolve under the sub-path.
4. **`vite.config.ts` keeps `base: './'`** — relative asset URLs.
5. **React Compiler is enabled** (babel-plugin-react-compiler in
   `vite.config.ts`, React 19). Do NOT add manual
   `useMemo`/`useCallback`/`React.memo` for plain in-component render values —
   the compiler auto-memoizes; write the plain value/function.
6. **Server state via TanStack Query** (`src/api/`): reads are `useQuery`,
   writes are `useMutation` + key invalidation. No `useEffect`-fetch, no
   manual refetch after mutations.
7. **Vendored files** (`src/moses-browser-logger.ts`) stay byte-identical to
   the repo's `shared/` copy — the drift gate fails otherwise.

## How to Customize

### 1. App identity

Edit `moses-app.config.json`:

```json
{
  "name": "my-app",
  "displayName": "My Application",
  "description": "Brief description of what your app does"
}
```

### 2. Add pages + routes

```typescript
// src/pages/MyPage.tsx
import './MyPage.css'

function MyPage() {
  return (
    <div className="my-page">
      <h1>My New Page</h1>
    </div>
  )
}

export default MyPage
```

Register in `src/App.tsx`:

```typescript
import MyPage from './pages/MyPage'

// Add to <Routes> (paths are basename-relative):
<Route path="/my-page" element={<MyPage />} />
```

### 3. Design tokens

Tokens live in `src/styles/theme.css` (NOT App.css): emerald brand, three-tier
backgrounds, dark default + `[data-theme="light"]` override. Keep spacing on
the 4px grid (`var(--spacing-4)` etc.) and WCAG 2.1 AA contrast. Global
element styles/utilities (`.btn`, forms, tables) are in `src/App.css`.

No external font CDNs — the font stacks fall back to system fonts. Self-host
webfonts under `public/` if you need them.

### 4. Data layer (reads + writes)

```typescript
// src/api/client.ts — typed read + write, RELATIVE paths
export interface Thing { id: string; name: string }
export const getThings = (signal?: AbortSignal) =>
  fetchAPI<{ things: Thing[] }>('api/v1/things', signal)
export const createThing = (input: { name: string }, signal?: AbortSignal) =>
  mutateAPI<Thing>('api/v1/things', 'POST', input, signal)

// src/api/queryKeys.ts — central key factory
export const queryKeys = {
  things: ['things'] as const,
}

// src/api/hooks.ts — components consume hooks, never fetch()
export function useThings() {
  return useQuery({ queryKey: queryKeys.things, queryFn: ({ signal }) => getThings(signal) })
}
export function useCreateThing() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: { name: string }) => createThing(input),
    onSettled: () => qc.invalidateQueries({ queryKey: queryKeys.things }),
  })
}
```

Mutations invalidate their read keys (`qc.invalidateQueries({ queryKey: queryKeys.things })`).

### 5. App-absolute URLs (sharing etc.)

`getBasePath()` returns `/` (root) or `/apps/<t>/<a>` (no trailing slash) —
join accordingly:

```typescript
import { getBasePath } from './utils/baseUrl'

const base = getBasePath()
const shareUrl = `${base === '/' ? '' : base}/my-page`
```

### 6. Build, test, deploy

```bash
npm install
npm run dev        # http://localhost:5173
npm run lint       # tsc --noEmit (the Moses validation gate)
npm test           # vitest
npm run build      # production dist/

docker build -t my-app:latest .
docker run -p 8080:8080 my-app:latest   # nginx listens on 8080
```

Deploy: commit to Git, register via the Moses Apps page; Moses builds
in-cluster and serves the app at `/apps/{tenant}/{slug}/`.

## Security headers (what nginx actually sends)

- `Content-Security-Policy: frame-ancestors ...` — rendered at container
  start from `MOSES_EMBEDDING_FRAMING` / `MOSES_EMBEDDING_ALLOWED_ANCESTORS`
  (default `moses-only`); `denied` additionally sends `X-Frame-Options: DENY`.
- `X-Content-Type-Options: nosniff`.
- `X-XSS-Protection` is deliberately NOT sent (deprecated/no-op; CSP is the
  mechanism). The repo's parity test enforces this.

## Troubleshooting

**Routes 404 in production** — nginx falls back to `index.html`
(`try_files`); check the sub-path rewrite block rendered by `entrypoint.sh`
(`MOSES_BASE_PATH`).

**Assets 404** — keep `base: './'` in `vite.config.ts`.

**Links jump out of the app** — you used `<a href="/...">`; use
`<Link to="/...">`.

**App not healthy after deploy** — `/health` is served by nginx at root;
probes bypass the ingress.

**Styles missing** — import the CSS in the component: `import './X.css'`.
