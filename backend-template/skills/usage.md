# Backend Template - Agent Skill

## What you start with

This template is demo-free: what ships is the Moses plumbing and an empty
place to put your API. `cmd/server/main.go` is the server bootstrap (base-path
contract, middleware chain, graceful shutdown) and `cmd/server/demo_routes.go`
holds an empty `registerDemoRoutes` stub — the single route-registration hook
`buildMux` calls (rename the file/function once you outgrow the name).
`internal/handler/health.go` + `openapi.go` serve the dual-mounted `/health`
and `/api/openapi.json` platform contract; `internal/handler/tenant.go`
(+ `tenant_test.go`) carries the `strictTenantCheckEnabled` /
`enforceTenantMatch` 403 cross-check (CHAT-pxeo.12); `internal/config/moses.go`
(+ test) provides `SelfTenantID()` / `Validate()` env-pinned tenant identity;
`internal/middleware/` carries the Moses headers, the CSRF guard (vendored —
do not edit), embedding headers, and logging. `api/openapi.json` starts as
`"paths": {}` with the canonical `servers: [{"url": "/api/v1"}]`, embedded via
`api/api.go`. `helm/`, `Dockerfile`, `go.mod`, `go.sum` complete the build and
deployment surface. `go vet ./... && go test ./...` is green on a fresh clone.

## App identity

Update `moses-app.config.json` before committing:

```json
{
  "name": "your-app-name",
  "displayName": "Your App Name",
  "description": "What your app actually does"
}
```

`name` becomes the Helm release name and the MCP tool prefix. The `docker`,
`service`, and `validation` sections are pre-configured — only change them if
you modify the project structure.

## Environment-variable contract

Everything the code reads from env, and where:

| Env var | Read in | Purpose |
|---|---|---|
| `PORT` | `cmd/server/main.go` | HTTP listen port (default `8080`) |
| `MOSES_BASE_PATH` | `cmd/server/main.go` | Canonical sub-path prefix (`/apps/<tenant>/<slug>`); API routes are registered ONCE under it (CHAT-8qiu0) |
| `BASE_URL` | `cmd/server/main.go` | DEPRECATED alias for `MOSES_BASE_PATH`; honored only when it looks like a path (`/...`). See the platform repo's DEPRECATIONS.md |
| `MOSES_TENANT_ID` | `internal/config/moses.go` | Deploy-pinned self tenant — the ONLY storage/lookup tenant key. Required when deployed |
| `MOSES_DEPLOYED` | `internal/config/moses.go` | `1` on platform-deployed pods; makes `config.Validate()` fail-fast when `MOSES_TENANT_ID` is missing |
| `MOSES_STRICT_TENANT_CHECK` | `internal/handler/tenant.go` | Default `true`: 403 when a request's `X-Moses-Tenant-ID` disagrees with the pinned tenant. `false`/`0`/`no`/`off` disables |
| `MOSES_EMBEDDING_FRAMING` | `internal/middleware/embedding.go` | `moses-only` \| `public` \| `denied` (default `denied` for this api template) |
| `MOSES_EMBEDDING_ALLOWED_ANCESTORS` | `internal/middleware/embedding.go` | CSP frame-ancestors source list for `moses-only` |
| `MOSES_EMBEDDING_REPORT_URI` | `internal/middleware/embedding.go` | Optional CSP report-uri |
| `MOSES_PLATFORM_API_KEY` / `MOSES_PLATFORM_API_URL` | nothing (no code in this template reads them) | Injected by the platform when the `moses-platform` integration grant is approved — read them yourself if you call the Moses Platform API |
| `EXAMPLE_API_KEY` / `EXAMPLE_ENCRYPTION_KEY` | nothing (documented in `skills/secrets-tutorial.md`) | Example external/generated secrets — see the with-secrets example config |

## Tenancy model (CHAT-pxeo.12) — read this before writing handlers

Self-identification is **env-pinned**, never header-driven:

- **Storage/lookup tenant** = `config.SelfTenantID()` (from `MOSES_TENANT_ID`,
  injected at deploy; `local-dev` sentinel in local runs). Exposed on the
  request as `mosesCtx.SelfTenantID`.
- **`X-Moses-Tenant-ID` header** = caller context ONLY (`mosesCtx.CallerTenantID`).
  Use it for audit logs and the 403 cross-check — never as a data scope.
- There is no `mosesCtx.TenantID` field and no "no headers → no filtering"
  mode; browser requests through the platform proxy carry no Moses headers at
  all, and they still get tenant-scoped data via `SelfTenantID`.

```go
func MyHandler(w http.ResponseWriter, r *http.Request) {
    mosesCtx := middleware.GetMosesContext(r)
    if enforceTenantMatch(w, mosesCtx) { // 403 on caller mismatch (writes)
        return
    }
    rows := store.GetByTenant(mosesCtx.SelfTenantID) // storage scope: env-pinned
    log.Printf("[%s] user=%s caller_tenant=%s",
        mosesCtx.RequestID, mosesCtx.UserID, mosesCtx.CallerTenantID) // audit
    ...
}
```

## Adding a new endpoint

### 1. Handler in `internal/handler/`

```go
// internal/handler/things.go
package handler

import (
    "encoding/json"
    "net/http"

    "github.com/moses-platform/backend-template/internal/middleware"
)

func ListThings(w http.ResponseWriter, r *http.Request) {
    mosesCtx := middleware.GetMosesContext(r)
    // Scope storage by the deploy-pinned tenant, never by the header.
    things := store.GetByTenant(mosesCtx.SelfTenantID)
    // Never put the tenant UUID in the response body (CHAT-w6gt).
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]any{"things": things})
}
```

### 2. Register the route — ALWAYS under basePath

Routes are registered in `cmd/server/demo_routes.go` (`registerDemoRoutes`,
called once from `buildMux`). The basePath prefix is mandatory — without it
the route is unreachable on a sub-path deploy:

```go
mux.HandleFunc("GET "+basePath+"/api/v1/things", handler.ListThings)
```

Do NOT re-register under the bare path; `/health` and `/api/openapi.json`
dual-mounting is already handled in `buildMux`.

### 3. Update `api/openapi.json` — paths RELATIVE to servers[0].url

`servers` is exactly `[{"url": "/api/v1"}]` and paths keys are relative to
it. The platform folds `servers[0].url + path` into the tool endpoint, so an
`/api/`-rooted key double-prefixes (the template's own
`TestOpenAPISpec_CanonicalServersAndRelativePaths` fails it), and `/health`
must never be listed (phantom tool):

```json
{
  "paths": {
    "/things": {
      "get": {
        "operationId": "listThings",
        "summary": "List things",
        "responses": {
          "200": { "description": "OK" }
        }
      }
    }
  }
}
```

The `operationId` becomes the MCP tool name:
`workspace_<your-app-name>_listThings`.

### 4. Test it

Add a route test in `cmd/server/main_test.go`; `TestOpenAPISpec_MuxConsistency`
already fails if the spec and mux drift apart.

## Moses header reference

Headers injected by the platform's WorkspaceToolProxy on MCP-driven calls
(absent on browser-driven requests — never rely on them for scoping):

| Header | MosesContext field | Purpose |
|--------|--------------------|---------|
| `X-Moses-Tenant-ID` | `CallerTenantID` | Caller's tenant — audit + 403 cross-check only |
| `X-Moses-User-ID` | `UserID` | User who initiated the call |
| `X-Moses-Chart-ID` | `ChartID` | Project/workspace ID |
| `X-Moses-Tool-ID` | `ToolID` | Workspace tool deployment ID |
| `X-Moses-Request-ID` | `RequestID` | Request trace ID — use in logs |
| `X-Moses-MCP-Source` | `MCPSource` | `claude-code`, `moses-manager`, ... |
| `X-Moses-API-Key-ID` | `APIKeyID` | API key ID (optional, audit) |

`SelfTenantID` is NOT header-derived — it is populated from env by the
middleware on every request.

## Building and testing

```bash
go run cmd/server/main.go          # local run (local-dev tenant sentinel)
curl http://localhost:8080/health
curl http://localhost:8080/api/openapi.json

go vet ./... && go test ./...      # the validation gate Moses runs too

docker build -t my-app:latest .
docker run -p 8080:8080 my-app:latest
```

## Deploy to Moses

1. Commit to a Git repository and register it via the Moses Apps page.
2. Moses clones, builds in-cluster, deploys via Helm, discovers the spec at
   `/api/openapi.json` (alias `/api/spec`), and generates MCP tools from the
   operationIds.

## Best practices

1. Scope every data access by `mosesCtx.SelfTenantID`; call
   `enforceTenantMatch` first in write handlers.
2. Keep `api/openapi.json` in sync with the mux — the tests enforce it.
3. Use meaningful operationIds; they become MCP tool names.
4. `/health` must answer fast (< 5s) — it backs the K8s probes.
5. Log with `mosesCtx.RequestID` for cross-service tracing.
6. External secrets go in `moses-app.config.json → secrets.external[]` —
   see `skills/secrets-tutorial.md`; never hardcode them.
