# Fullstack Unified - Agent Skill

## What you start with

This template is demo-free: what ships is the Moses plumbing and an empty place
to put your app. Every platform contract is already wired — base-path routing,
the `/health` + `/api/openapi.json` dual-mount, the tenant contract, CSRF +
CORS, the embedding CSP, the browser-logger, `index.html` templating, the Helm
chart, and `moses-app.config.json`.

`main.go` carries the routing, index templating, embedding CSP and the tenant
helpers (`mosesContext` / `getMosesContext` / `enforceTenantMatch` /
`strictTenantCheckEnabled`). `demo_routes.go` is an empty
`registerDemoRoutes` stub — the single route hook (`registerAPIRoutes` in
`main.go` is its only call site; rename both if you like).
`internal/config/moses.go` (+test) pins the tenant identity; `csrf.go` (+test)
is vendored — do not edit it; `cors.go` (+test) is the opt-in allowlist.
`api/openapi.json` starts at `"paths": {}` with the canonical
`servers: [{"url": "/api/v1"}]`. The embedded frontend is `static/index.html`,
`app.js`, `style.css`, `favicon.svg` and the vendored
`moses-browser-logger.js`. `main_test.go` locks the path + spec↔mux contracts.
`Dockerfile`, `.dockerignore`, `helm/` and `go.mod` complete the build and
deployment surface.

Start by setting your app identity in `moses-app.config.json`, add routes in
`demo_routes.go`, mirror every route in `api/openapi.json`, and run
`go vet ./... && go test ./...`.

## First Steps (identity)

Update `moses-app.config.json` before committing:

```json
{
  "name": "your-app-name",
  "displayName": "Your App Name",
  "description": "What your app actually does"
}
```

The `name` field becomes the Helm release name and MCP tool prefix. The `docker`, `services`, and `validation` sections are pre-configured — only change them if you modify the project structure.

## Overview

Single Go binary serving both static frontend files and API endpoints. One container, one Dockerfile — the simplest fullstack pattern for Moses. Unlike `fullstack-simple` (separate frontend + backend containers with nginx proxy), this template embeds `static/` and `api/openapi.json` into the binary via `//go:embed`.

## Environment-variable contract

Everything the code reads, and where:

| Env var | Read by | Purpose |
|---|---|---|
| `PORT` | `main.go` | Listen port (default 8080). |
| `MOSES_BASE_PATH` | `main.go` | Sub-path the app is mounted under. API + static are registered under it; `/health` and `/api/openapi.json` also answer at the canonical root. `BASE_URL` is the deprecated alias. |
| `MOSES_TENANT_ID` | `internal/config/moses.go` | **Required on a deployed pod.** Authoritative self-identification via `config.SelfTenantID()` (CHAT-pxeo.12). The `X-Moses-Tenant-ID` *header* is caller-context only (audit + 403 cross-check), never a storage key. |
| `MOSES_DEPLOYED` | `internal/config/moses.go` | `"1"` on platform-deployed pods → `config.Validate()` fail-fasts when `MOSES_TENANT_ID` is unset. |
| `MOSES_STRICT_TENANT_CHECK` | `main.go` | Default `true`: a request whose `X-Moses-Tenant-ID` disagrees with `MOSES_TENANT_ID` gets 403 `{"error":"tenant_mismatch","code":"E_TENANT_MISMATCH"}` (no UUIDs in the body). `false`/`0`/`no`/`off` disables. |
| `CORS_ALLOWED_ORIGINS` | `cors.go` | Default unset: **no CORS headers** (one binary serves frontend + API, same origin). Comma-separated exact origins to opt in; a single `*` allows any origin; credentials never combined with the wildcard. |
| `MOSES_EMBEDDING_FRAMING` | `main.go` | `moses-only` (default) / `public` / `denied` → `Content-Security-Policy: frame-ancestors` via `withEmbeddingHeaders`. |
| `MOSES_EMBEDDING_ALLOWED_ANCESTORS` | `main.go` | Explicit ancestors list for `moses-only`. |
| `MOSES_EMBEDDING_REPORT_URI` | `main.go` | Optional CSP `report-uri`. |
| `MOSES_CHART_ID` / `MOSES_DEPLOYMENT_ID` / `MOSES_API_BASE` | `main.go` → `static/index.html` | Rendered into `<meta name="moses-config">` for the browser-logger (BLF-J). When empty the logger falls back to a location-derived `loc` param — it does NOT disable itself. |

No `DB_*` variables are read — this template is DB-free until you declare `dependencies.services: ["postgresql"]` in `moses-app.config.json` (Moses then injects `DATABASE_URL` + `DB_*`).

## The `{{ }}` trap in static/index.html

`main.go` parses `static/index.html` as a Go **html/template** (`loadIndexTemplate`) to substitute the moses-config meta values. Consequence: **any literal `{{ ... }}` in index.html is executed as a template action against the `indexContext` struct** — a stray `{{title}}` (e.g. pasted from a JS templating example) doesn't render literally; it makes template execution error or emit nothing, silently. If you need literal braces in the served HTML, escape them (`{{"{{"}}`) or move that markup into a different static file (only `index.html` is templated). Never remove the `{{.ChartID}}` / `{{.DeploymentID}}` / `{{.APIBase}}` placeholders, the `<base href="./">` tag, the `moses-config` meta, or the `moses-browser-logger.js` script tag — all four are platform plumbing.

## Frontend fetch convention

`static/app.js` shows the pattern: resolve `base` from the hardcoded `<base href="./">` (the browser expands it to the full origin + subpath of the page) and call `fetch(base + '/api/v1/...')`. Root-relative `fetch('/api/v1/...')` bypasses the subpath and 404s on the platform.

## Moses request-context headers

The platform injects `X-Moses-User-ID`, `X-Moses-Chart-ID`, `X-Moses-Request-ID` (and, on workspace-tool calls, `X-Moses-Tenant-ID`) into proxied requests; `getMosesContext` in `main.go` reads them. Rule (CHAT-w6gt, test-locked): **responses never contain tenant UUIDs** — `mosesContext` marks both tenant fields `json:"-"`; echo only user/chart/request IDs.

## Extending

1. Add a handler function (start in `demo_routes.go` — rename the file/function if you like, `registerAPIRoutes` in `main.go` is the single call site)
2. Register the route: `mux.HandleFunc("GET "+baseURL+"/api/v1/things", handleListThings)` — always under `baseURL`
3. Add the endpoint to `api/openapi.json` — the served spec is the single source of truth for MCP tool registration: `servers[0].url` stays exactly `/api/v1`, path keys are RELATIVE to it (never `/api/`-rooted), `/health` never listed. `main_test.go` locks spec↔mux consistency.
4. Add frontend UI in `static/` (vanilla HTML/CSS/JS) — files are re-embedded on build; there is no live reload

## When to Use

- Apps where the frontend is simple enough for vanilla HTML/JS
- When you want minimal container count and complexity
- Prototypes and MVPs

## When NOT to Use

- Apps requiring React, Vue, or other framework builds (use `fullstack-simple`)
- Apps needing separate frontend scaling or release cycles

## Pre-Commit Checklist

Before calling `moses_agent_submit_completed`:
1. `go vet ./...` — must pass
2. `go test ./...` — must pass
3. `go.sum` committed
4. Every `openapi.json` path curls non-404 on your running server
