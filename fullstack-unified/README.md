# Fullstack Unified

Single-container Moses template: one Go binary serves both API endpoints and the static frontend, with the frontend assets and OpenAPI spec embedded into the binary via `//go:embed`. The simplest viable hybrid app — no nginx, no inter-container networking, no separate frontend build pipeline.

Pick this when you want zero deployment ceremony and your frontend is small enough that build-time embedding makes sense (~tens of MB max, since every build re-bakes the whole asset bundle into the image).

## Layout

```
fullstack-unified/
├── moses-app.config.json     # appType: hybrid, 1 docker image
├── helm/                     # agent-deployed-app chart, single-service
│   ├── Chart.yaml            # name: agent-deployed-app, v1.0.0
│   ├── values.yaml
│   └── templates/
│       ├── deployment.yaml
│       └── service.yaml
├── Dockerfile                # multi-stage: golang:1.24-alpine → alpine:3.19, non-root
├── go.mod
├── main.go                   # //go:embed static/*  +  //go:embed api/openapi.json
├── main_test.go
├── api/
│   └── openapi.json          # OpenAPI 3.x spec → MCP tool autoreg
├── static/                   # embedded into the binary at build time
│   ├── index.html            # contains <meta name="moses-config" data-chart-id="...">
│   ├── app.js
│   ├── style.css
│   ├── favicon.svg
│   └── moses-browser-logger.js
└── skills/
    └── usage.md
```

## When to choose this template vs `fullstack-simple`

| Situation | Pick |
|---|---|
| Plain HTML/CSS/JS frontend, no React/Vite build step | **fullstack-unified** |
| You want one binary you can `docker run` standalone trivially | **fullstack-unified** |
| Frontend has a build pipeline (React + Vite, Next.js, etc.) | `fullstack-simple` |
| You expect frontend changes more often than backend | `fullstack-simple` (separate cache layers) |
| Asset bundle exceeds ~50 MB | `fullstack-simple` (smaller image churn) |

## Embedded asset pipeline (BLF-J)

`main.go` declares two embed directives:

```go
//go:embed static/*
var staticFiles embed.FS

//go:embed api/openapi.json
var openapiSpec []byte
```

At startup, `loadIndexTemplate()` parses `static/index.html` once into an `html/template`. Every `GET /` request renders the template with `MOSES_CHART_ID`, `MOSES_DEPLOYMENT_ID`, and `MOSES_API_BASE` env vars — these populate the `<meta name="moses-config">` tag the browser-logger reads (CHAT-ry35).

To update the spec: edit `api/openapi.json`, then rebuild the binary. There is no live reload — embedded artifacts require a fresh build.

## Moses-aware envs (read at startup)

| Env var | Purpose |
|---|---|
| `MOSES_BASE_PATH` | Path prefix; mounts every route under it. Both raw routes and `/{prefix}/...` aliases work. |
| `MOSES_TENANT_ID` | **Required on a deployed pod (CHAT-pxeo.12).** Authoritative self-identification, exposed to handlers via `internal/config.SelfTenantID()`. The `X-Moses-Tenant-ID` header is caller-context only (audit + 403 cross-check), NOT a fallback. |
| `MOSES_STRICT_TENANT_CHECK` | Optional, default `true`. When a request supplies a non-empty `X-Moses-Tenant-ID` that disagrees with `MOSES_TENANT_ID`, write/diagnostic handlers return 403 with `{"error":"tenant_mismatch","code":"E_TENANT_MISMATCH"}`. Set to `false` to disable. |
| `MOSES_EMBEDDING_FRAMING` | `moses-only` (default) / `public` / `denied`. Drives the `Content-Security-Policy: frame-ancestors` middleware. |
| `MOSES_EMBEDDING_ALLOWED_ANCESTORS` | Explicit override for the moses-only ancestors list. |
| `MOSES_EMBEDDING_REPORT_URI` | Optional CSP report-uri. |
| `MOSES_CHART_ID`, `MOSES_DEPLOYMENT_ID`, `MOSES_API_BASE` | Rendered into `<meta name="moses-config">` for the embedded browser logger. |
| `BASE_URL` | Deprecated alias for `MOSES_BASE_PATH`. Will be removed N-2 minor releases after `MOSES_BASE_PATH` shipped. |

CSP framing is rendered by `withEmbeddingHeaders()` middleware in `main.go` (~line 229) — same matrix as the frontend templates' `entrypoint.sh`.

## Running locally (without Moses)

```bash
go mod download
go run .                 # listens on :8080

# In another terminal:
curl http://localhost:8080/health
curl http://localhost:8080/api/openapi.json
open http://localhost:8080/
```

Standalone Helm install:

```bash
cd helm
helm install fullstack-unified-test . \
  --set images.app.repository=<your-registry>/fullstack-unified-app \
  --set images.app.tag=latest
```

## Extending

- Add API handlers in `main.go` (or split into a `handlers/` package). Register routes on `mux` and update `api/openapi.json` so the new endpoint becomes an MCP tool.
- Frontend lives in `static/`. Edit any of the existing files or add new ones — they get re-embedded on build.
- Need a database? Switch to `fullstack-showcase` — adding Postgres support to a single-binary template doubles the deployment complexity and defeats the purpose.

## Validation + maintenance hooks

`moses-app.config.json` declares `go vet ./...` and `go test ./...` as required validation, plus weekly Renovate sweeps and a 15-minute log-error monitor.

## Conventions enforced by `tests/test_template_parity.sh`

Same as fullstack-simple: chart name `agent-deployed-app` v1.0.0, Go 1.24, non-root + HEALTHCHECK in Dockerfile, no `"name": "my-app"` placeholder, no literal credentials in `helm/values.yaml`.
