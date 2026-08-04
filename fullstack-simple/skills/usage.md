# Fullstack Simple - Agent Skill

## What you start with

This template is demo-free: what ships is the Moses plumbing and an empty place
to put your app. Every platform contract is already wired — base-path routing,
the `/health` + `/api/openapi.json` dual-mount, the tenant contract, CSRF,
CORS, the browser-logger, nginx CSP, the Helm chart, and `moses-app.config.json`.

Backend: `backend/cmd/server/main.go` (`buildMux` + the base-path mounts),
`backend/cmd/server/demo_routes.go` (empty `registerDemoRoutes` stub — the
single route hook `buildMux` calls; rename it if you like),
`backend/internal/config/moses.go` (+test), `backend/internal/handler/`
(`health.go`, `openapi.go`, `tenant.go` + `tenant_test.go` — the tenant
contract), `backend/internal/middleware/csrf.go` (+test, vendored — do not
edit) and `cors.go` (+test), `backend/api/openapi.json` starting at
`"paths": {}` with the canonical `servers: [{"url": "/api/v1"}]`.

Frontend: `frontend/src/main.tsx`, `frontend/src/moses-browser-logger.ts`
(vendored), the data layer in `frontend/src/api/` (`queryClient.ts` plus
transport-only `client.ts` and empty `hooks.ts` / `queryKeys.ts` factories),
`frontend/src/App.tsx` / `App.css` / `App.test.tsx`,
`frontend/src/components/ThemeToggle.tsx`, `frontend/src/styles/theme.css`,
`frontend/index.html` (keeps the `moses-base-path` meta).

Deploy: `frontend/entrypoint.sh`, `frontend/nginx.conf`, both `Dockerfile`s,
both `.dockerignore`s, `helm/`.

Start by setting your app identity in `moses-app.config.json`, add routes in
`demo_routes.go`, mirror every route in `backend/api/openapi.json`, and run the
pre-commit checklist below.

## First Steps (identity)

Update `moses-app.config.json` before committing:

```json
{
  "name": "your-app-name",
  "displayName": "Your App Name",
  "description": "What your app actually does"
}
```

The `name` field becomes the Helm release name and MCP tool prefix. The `docker.files`, `services`, and `validation` sections are pre-configured — only change them if you modify the project structure.

## Overview

Minimal Go + React fullstack template. Two containers: React 19 + Vite SPA behind nginx, and a Go stdlib API backend. No database and no persistence layer ship — anything you keep in process memory resets on pod restart. Need persistence? Declare `dependencies.services: ["postgresql"]` in `moses-app.config.json` and Moses provisions PostgreSQL + injects `DATABASE_URL`/`DB_*` env vars.

## Environment-variable contract

Everything the code reads, and where:

| Env var | Read by | Purpose |
|---|---|---|
| `PORT` | `backend/cmd/server/main.go` | Backend listen port (default 8080). |
| `MOSES_BASE_PATH` | `main.go`, `frontend/entrypoint.sh` | Sub-path the app is mounted under (e.g. `/apps/<tenant>/<slug>`). API routes are registered ONCE under it; nginx renders sub-path proxy blocks; `entrypoint.sh` rewrites `<meta name="moses-base-path">` in index.html. `BASE_URL` is the deprecated alias. |
| `MOSES_TENANT_ID` | `backend/internal/config/moses.go` | **Required on a deployed pod.** Authoritative storage/lookup tenant key via `config.SelfTenantID()` (CHAT-pxeo.12). The `X-Moses-Tenant-ID` *header* is caller-context only — never a storage key. |
| `MOSES_DEPLOYED` | `backend/internal/config/moses.go` | `"1"` on platform-deployed pods → `config.Validate()` fail-fasts when `MOSES_TENANT_ID` is unset. |
| `MOSES_STRICT_TENANT_CHECK` | `backend/internal/handler/tenant.go` | Default `true`: a request whose `X-Moses-Tenant-ID` disagrees with `MOSES_TENANT_ID` gets 403 `{"error":"tenant_mismatch","code":"E_TENANT_MISMATCH"}` (no UUIDs in the body). `false`/`0`/`no`/`off` disables. |
| `CORS_ALLOWED_ORIGINS` | `backend/internal/middleware/cors.go` | Default unset: **no CORS headers** (frontend + API share an origin). Comma-separated exact origins to opt in; a single `*` allows any origin; credentials are never combined with the wildcard. |
| `MOSES_EMBEDDING_FRAMING` | `frontend/entrypoint.sh` | `moses-only` (default) / `public` / `denied` → `Content-Security-Policy: frame-ancestors`. |
| `MOSES_EMBEDDING_ALLOWED_ANCESTORS` | `frontend/entrypoint.sh` | Explicit space-separated CSP source list for `moses-only`. |
| `MOSES_EMBEDDING_REPORT_URI` | `frontend/entrypoint.sh` | Optional CSP `report-uri`. |
| `MOSES_DOMAIN` | `frontend/entrypoint.sh` | Drives the chart-parity frame-ancestors fallback when no explicit list is set. |
| `BACKEND_SERVICE_HOST` / `BACKEND_SERVICE_PORT` | `frontend/entrypoint.sh` | Backend service DNS for the nginx `/api/` proxy — injected by the Helm chart; never hardcode the service name. |
| `VITE_MOSES_CHART_ID` / `VITE_MOSES_DEPLOYMENT_ID` / `VITE_MOSES_API_BASE` | `frontend/Dockerfile` build args → `moses-browser-logger.ts` | Browser-log reporter identity (BLF-B). Absent → the logger falls back to location-derived identity. |

No `DB_*` variables are read — this template is DB-free until you declare the dependency.

## Tenant scoping (get this right)

Key every stored record by `config.SelfTenantID()` — the **`MOSES_TENANT_ID` env var pinned at deploy time** — NOT by the `X-Moses-Tenant-ID` request header (CHAT-pxeo.12). The header is caller audit context; it is absent on browser requests through the platform's app proxy and only drives the optional 403 cross-check in `tenant.go`. When you add storage, keep this contract: env for keys, header for audit.

## Moses request-context headers

The platform injects `X-Moses-User-ID`, `X-Moses-Chart-ID`, `X-Moses-Request-ID` (and, on workspace-tool calls, `X-Moses-Tenant-ID`) into proxied requests — read them with `r.Header.Get(...)` in your handlers. Rule (CHAT-w6gt): **never put tenant UUIDs in a response body**; echo only user/chart/request IDs.

nginx forwards these request headers to the backend by default — no `proxy_*` directives are needed for them (and `proxy_pass_header` would be a no-op: it controls upstream *response* headers).

## API Path Convention

**All frontend fetch calls must use relative paths** (no leading `/`):

```typescript
// Correct — works behind subpath ingress
fetch('api/v1/things')

// Wrong — resolves to platform backend, not app backend
fetch('/api/v1/things')
```

Vite `base: './'` makes relative URLs resolve against the page URL, so they route through the app's own nginx proxy under `/apps/<tenant>/<slug>/`. Don't call `fetch` from components directly — go through the typed client (`src/api/client.ts`) + TanStack Query hooks (`src/api/hooks.ts`) with keys from `src/api/queryKeys.ts`.

## Extending

1. Add a handler in `backend/internal/handler/`
2. Register the route in `backend/cmd/server/demo_routes.go` (rename the file/function if you like — `buildMux` is the single call site) — always `basePath+"/api/v1/..."`
3. Add the endpoint to `backend/api/openapi.json` — the served spec is the single source of truth for MCP tool registration: `servers[0].url` stays exactly `/api/v1`, path keys are RELATIVE to it (never `/api/`-rooted), `/health` never listed. `main_test.go` locks spec↔mux consistency.
4. Add the client function + Query hook + query key in `frontend/src/api/`, consume from a component

## Build System Requirements

### Frontend (React 19 + Vite + TypeScript)
- **TypeScript strict mode** is enabled; `npm run lint` runs `tsc -p tsconfig.build.json --noEmit` (there is no ESLint in this template)
- **React Compiler is enabled** (`babel-plugin-react-compiler` in `vite.config.ts`) — do NOT add manual `useMemo`/`useCallback`/`React.memo`
- **Tests**: vitest + Testing Library (`npm test`); wrap rendered components in a `QueryClientProvider` with retries off (see `App.test.tsx`)
- **Lock file**: `package-lock.json` MUST stay committed for reproducible builds
- **Build command**: `npm run build` (`tsc -p tsconfig.build.json && vite build`)

### Backend (Go)
- Run `go vet ./...` and `go test ./...` before committing
- `go.sum` must be committed
- Never hardcode service addresses — use the env contract above

### Pre-Commit Checklist
Before calling `moses_agent_submit_completed`:
1. `cd frontend && npm run lint && npm test && npm run build` — must succeed
2. `cd backend && go vet ./... && go test ./...` — must pass
3. All lock files committed (`package-lock.json`, `go.sum`)
4. Every `openapi.json` path curls non-404 on your running server
