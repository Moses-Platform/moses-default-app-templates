# Fullstack Simple

Two-container Moses template: a React + Vite SPA served by nginx, and a Go API backend, communicating over Kubernetes service DNS. Pick this when you want a clean separation between frontend assets and an API server. Need a relational database? You don't need a different template — just declare `dependencies.services: ["postgresql"]` (see Extending below).

## Layout

```
fullstack-simple/
├── moses-app.config.json     # appType: hybrid, 2 docker images
├── helm/                     # agent-deployed-app chart, services[] array
│   ├── Chart.yaml            # name: agent-deployed-app, v1.0.0
│   ├── values.yaml           # services + optional redis
│   └── templates/
│       ├── deployment.yaml   # iterates Values.services
│       ├── service.yaml
│       └── redis.yaml        # only renders when redis.enabled: true
├── frontend/
│   ├── Dockerfile            # multi-stage: deps → build → nginx:alpine
│   ├── nginx.conf            # template, rendered at startup by entrypoint.sh
│   ├── entrypoint.sh         # CSP + base-path + /api/ proxy renderer
│   ├── package.json
│   ├── src/                  # React 19 + TypeScript + Vite
│   └── index.html            # contains <meta name="moses-base-path" content="__MOSES_BASE_PATH__">
└── backend/
    ├── Dockerfile            # multi-stage: golang:1.27.4-alpine3.24 → alpine:3.24.4, non-root
    ├── api/
    │   ├── api.go            # //go:embed openapi.json
    │   └── openapi.json      # OpenAPI 3.x spec → MCP tool autoreg
    ├── cmd/server/
    │   ├── main.go           # entry point + buildMux (MOSES_BASE_PATH contract)
    │   ├── demo_routes.go    # your API route registration hook
    │   ├── example_test.go   # worked `things` route example (compiled by CI, never shipped)
    │   └── main_test.go      # path-contract + spec↔mux consistency tests
    └── internal/
        ├── config/           # SelfTenantID() from MOSES_TENANT_ID
        ├── handler/          # health, ServeOpenAPI, tenant 403 cross-check
        └── middleware/       # CORS allowlist, cross-site CSRF rejection
```

## Two-container wiring

The frontend nginx proxies `/api/*` → `http://${BACKEND_SERVICE_HOST}:${BACKEND_SERVICE_PORT}/api/*`. The Helm chart auto-injects:

```yaml
- name: BACKEND_SERVICE_HOST
  value: "{fullname}-backend"   # e.g. app-c35f1d9b-stable-backend
- name: BACKEND_SERVICE_PORT
  value: "8080"
```

`{fullname}` resolves to `nameOverride` (which Moses sets to `app-{chartID[:8]}-stable` for stable releases or `agent-{execID[:8]}` for dev). Frontend JavaScript MUST use **relative** paths (`fetch('api/...')`) — absolute `/api/...` would bypass this nginx proxy and hit the Moses platform backend.

## Moses-aware envs (read by frontend entrypoint and backend middleware)

| Env var | Purpose |
|---|---|
| `MOSES_BASE_PATH` | Path prefix the app is mounted under (e.g. `/apps/<tenant>/<slug>/`). Frontend renders into `<meta name="moses-base-path">`; backend strips it from incoming routes. |
| `MOSES_TENANT_ID` | **Required on a deployed pod (CHAT-pxeo.12).** Authoritative storage/lookup key for every tenant-scoped store you add. Read via `internal/config.SelfTenantID()`. The `X-Moses-Tenant-ID` header is caller-context only (audit + 403 cross-check), NOT a fallback for storage. |
| `MOSES_STRICT_TENANT_CHECK` | Optional, default `true`. When a write request supplies a non-empty `X-Moses-Tenant-ID` that disagrees with `MOSES_TENANT_ID`, the handler returns 403 with `{"error":"tenant_mismatch","code":"E_TENANT_MISMATCH"}`. Set to `false` to disable the cross-check. |
| `MOSES_EMBEDDING_FRAMING` | `moses-only` (default) / `public` / `denied`. Drives `Content-Security-Policy: frame-ancestors`. |
| `MOSES_EMBEDDING_ALLOWED_ANCESTORS` | Space-separated CSP source list when `framing=moses-only`. |
| `MOSES_EMBEDDING_REPORT_URI` | Optional `report-uri` for CSP violations. |
| `MOSES_DOMAIN` | Used by entrypoint to compute the chart-parity ancestors fallback when no explicit list is supplied. |

## Optional Redis sidecar

Set `redis.enabled: true` and supply `redis.auth.existingSecret`, the name
of a precreated Kubernetes Secret in the release namespace.
`redis.auth.passwordKey` defaults to `password`. Moses creates the Secret
and supplies the reference; standalone installs must precreate it. Helm
rejects an empty reference when Redis is enabled. Credentials never go in
chart values and the chart does not create a credential Secret.

The chart provisions a Valkey 8-alpine Deployment (`helm/templates/redis.yaml`)
with AUTH required. It auto-injects `REDIS_HOST`, `REDIS_PORT`, `REDIS_ADDR`
and `REDIS_PASSWORD` (from a Kubernetes Secret via `secretKeyRef`) into
services. Redis clients must use `REDIS_PASSWORD` to authenticate; an address
alone produces `NOAUTH`. Never hardcode the service name — it derives from
`{fullname}-redis`.

Clients must authenticate with the injected password whenever Redis is enabled.
This scaffold does not ship a Redis client; enabling the chart does not add
client authentication code.

## Running locally (without Moses)

```bash
# Backend
cd backend
go mod download
go run ./cmd/server          # listens on :8080

# Frontend (separate terminal)
cd frontend
npm install
npm run dev                   # Vite dev server on :3000, proxies /api/ to :8080
```

For a `helm install` standalone test:

```bash
cd helm
helm install fullstack-simple-test . \
  --set images.frontend.repository=<your-registry>/fullstack-simple-frontend \
  --set images.frontend.tag=latest \
  --set images.backend.repository=<your-registry>/fullstack-simple-backend \
  --set images.backend.tag=latest
```

## Extending

- Backend handlers: add to `backend/internal/handler/`, register in `cmd/server/demo_routes.go` (`registerDemoRoutes`, the single hook `buildMux` calls) — always under the `basePath` prefix and exactly once. Update `backend/api/openapi.json` so the new endpoint becomes an MCP tool via WorkspaceToolProxy.
  - **The served `openapi.json` is the single source of truth for MCP tool registration** — Moses registers each tool at `servers[0].url` + the path key and calls *that* exact path. If the spec declares a path your router does not serve, every agent tool call 404s while the relative-path UI keeps working. `apiEndpoints.basePath` in `moses-app.config.json` does **not** control this (it's a discovery/health hint). If you change the API base (this template uses `/api/v1`), change it consistently in **all four** places: the router (`cmd/server/demo_routes.go`), `openapi.json` (paths + any `servers`), the frontend `fetch()` calls, and `apiEndpoints.basePath`. Curl every declared path on your own running server (expect non-404) before submit. `cmd/server/main_test.go` locks spec↔mux consistency.
- Frontend pages: standard Vite/React. Data goes through the TanStack Query hooks in `frontend/src/api/hooks.ts` (built on `client.ts` + `queryKeys.ts`), which keep the relative-path convention — never `useEffect`+`fetch`.
- Database: declare `dependencies.services: ["postgresql"]` in `moses-app.config.json` — Moses provisions a PostgreSQL database for your app and injects `DATABASE_URL` + `DB_*` env vars automatically (no chart changes needed). If instead you want an in-pod database you fully control, ship your own `helm/templates/postgresql.yaml` and declare `dependencies.bundledServices: ["postgresql"]` (see `fullstack-showcase`).

Declaring runtime secrets — see [skills/secrets-tutorial.md](skills/secrets-tutorial.md).

## Validation + maintenance hooks

The `moses-app.config.json` declares:

- `validation.commands[]`: lint and test commands Moses runs in the agent pod (pre-submit validation, from the base-branch config) before `moses_agent_submit_completed` is accepted (frontend lint/test + backend vet/test).
- `maintenance.renovate`: weekly Monday 4am sweep (Moses runs a Renovate detection Job; updates arrive as tickets/approvals).
- `maintenance.logMonitor`: 15-minute window log error monitor that fires Moses triggers when errors exceed 5 per window.

Self-maintenance is enabled via the Moses UI toggle (the app's ⋯ menu → Self-maintenance, or the Self-Maintenance app); the `maintenance` block only provides its configuration — it does not turn the feature on.

## Conventions enforced by `tests/test_template_parity.sh`

- Helm chart name is **`agent-deployed-app`** v1.0.0 (must match the platform's expected chart).
- All Go modules use `go 1.27.4`; all Go Dockerfiles use `golang:1.27.4-alpine3.24` (runtime `alpine:3.24.4`).
- All backend Dockerfiles run as a non-root `appuser` (uid 1000) with HEALTHCHECK.
- No `"name": "my-app"` placeholder in `moses-app.config.json` (must be a per-template instance name).
- No literal credentials in `helm/values.yaml` — use `existingSecret` / `passwordKey` references for bundled credentials.

## Request body limits

The default body cap is 1 MiB (`http.MaxBytesReader`). Set
`MOSES_MAX_BODY_BYTES` to a positive byte count to override it. Handlers
must handle `*http.MaxBytesError` as 413 before committing responses or
side effects; chunked bodies are limited while read, without buffering.

For this split frontend/backend chart, change only
`globalEnv.MOSES_MAX_BODY_BYTES` in `helm/values.yaml` (for example `"52428800"`
for 50 MiB). The chart injects that value into both containers. Rendering fails
if any service-local `env` sets this key, including retained values from an older
chart: "move MOSES_MAX_BODY_BYTES to globalEnv: one value now applies to the frontend proxy and the backend".
Remove the service-local key and move its value to `globalEnv` before deploying.
At startup, the frontend entrypoint validates it and renders nginx's
`client_max_body_size` in bytes; Go reads the same
`MOSES_MAX_BODY_BYTES`. Both default to **1048576 bytes (1 MiB)**. Invalid,
zero, negative or overflowing values fall back to that default; nginx startup
logs `Invalid MOSES_MAX_BODY_BYTES; using default 1048576`. With standalone
containers, pass the same environment value to both containers. Keep the cap
within the outer ingress limit; smaller endpoint-specific limits still apply.
