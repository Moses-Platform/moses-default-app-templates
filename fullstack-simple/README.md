# Fullstack Simple

Two-container Moses template: a React + Vite SPA served by nginx, and a Go API backend, communicating over Kubernetes service DNS. Pick this when you want a clean separation between frontend assets and an API server, but you don't need a database (use `fullstack-showcase` if you do).

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
    ├── Dockerfile            # multi-stage: golang:1.24-alpine → alpine:3.19, non-root
    ├── api/
    │   ├── api.go            # //go:embed openapi.json
    │   └── openapi.json      # OpenAPI 3.x spec → MCP tool autoreg
    ├── cmd/server/main.go
    └── internal/
        ├── handler/          # ServeOpenAPI, status, health
        └── middleware/       # MOSES_BASE_PATH stripping, request-id, logging
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
| `MOSES_TENANT_ID` | **Required on a deployed pod (CHAT-pxeo.12).** Authoritative storage/lookup key for the in-memory items map. Read via `internal/config.SelfTenantID()`. The `X-Moses-Tenant-ID` header is caller-context only (audit + 403 cross-check), NOT a fallback for storage. |
| `MOSES_STRICT_TENANT_CHECK` | Optional, default `true`. When a write request supplies a non-empty `X-Moses-Tenant-ID` that disagrees with `MOSES_TENANT_ID`, the handler returns 403 with `{"error":"tenant_mismatch","code":"E_TENANT_MISMATCH"}`. Set to `false` to disable the cross-check. |
| `MOSES_EMBEDDING_FRAMING` | `moses-only` (default) / `public` / `denied`. Drives `Content-Security-Policy: frame-ancestors`. |
| `MOSES_EMBEDDING_ALLOWED_ANCESTORS` | Space-separated CSP source list when `framing=moses-only`. |
| `MOSES_EMBEDDING_REPORT_URI` | Optional `report-uri` for CSP violations. |
| `MOSES_DOMAIN` | Used by entrypoint to compute the chart-parity ancestors fallback when no explicit list is supplied. |

## Optional Redis sidecar

Set `redis.enabled: true` in `values.yaml`. The chart provisions a Valkey 8-alpine StatefulSet and the deployment template auto-injects `REDIS_HOST`, `REDIS_PORT`, `REDIS_ADDR` env vars into every container. Agents should NEVER hardcode the service name — it derives from `{fullname}-redis`.

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

- Backend handlers: add to `backend/internal/handler/`, register in `cmd/server/main.go`. Update `backend/api/openapi.json` so the new endpoint becomes an MCP tool via WorkspaceToolProxy.
- Frontend pages: standard Vite/React. Use the `useApi()` hook for the relative-path convention.
- Database: copy the `helm/templates/postgresql.yaml` pattern from `fullstack-showcase` and add `dependencies.services: ["postgresql"]` in `moses-app.config.json`.

Declaring runtime secrets — see [skills/secrets-tutorial.md](skills/secrets-tutorial.md).

## Validation + maintenance hooks

The `moses-app.config.json` declares:

- `validation.commands[]`: lint and test commands run by Moses on every agent commit (frontend lint/test + backend vet/test).
- `maintenance.renovate`: weekly Monday 4am sweep (currently agent-driven; no Renovate bot wired).
- `maintenance.logMonitor`: 15-minute window log error monitor that fires Moses triggers when errors exceed 5 per window.

## Conventions enforced by `tests/test_template_parity.sh`

- Helm chart name is **`agent-deployed-app`** v1.0.0 (must match the platform's expected chart).
- All Go modules use `go 1.24`; all Go Dockerfiles use `golang:1.24-alpine`.
- All backend Dockerfiles run as a non-root `appuser` (uid 1000) with HEALTHCHECK.
- No `"name": "my-app"` placeholder in `moses-app.config.json` (must be a per-template instance name).
- No literal credentials in `helm/values.yaml` — use `__MOSES_GENERATE_PASSWORD__` for any password Moses should generate.
