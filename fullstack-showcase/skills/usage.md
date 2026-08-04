# Fullstack Showcase Template — Usage

Go 1.26 backend (net/http + pgx v5) + React 19 frontend + per-app PostgreSQL,
deployed as a two-service Helm chart.

## What you start with

This template is demo-free: a building-and-testing skeleton with ALL Moses
plumbing intact — the path contract (`MOSES_BASE_PATH` single-mount API +
health/openapi dual-mount), X-Moses header middleware, CSRF rejection, CORS
allowlist, browser-logger, nginx CSP/entrypoint, the Helm chart incl.
PostgreSQL, the tenant-enforcement helpers, and DB connect/retry/28P01
fail-fast.

Backend: `cmd/server/main.go` (`buildMux`, health+openapi dual-mount),
`cmd/server/demo_routes.go` (empty `registerDemoRoutes` stub — your route
hook), `cmd/server/main_test.go` (contract tests),
`internal/config/moses.go`, `internal/middleware/` (moses_headers, csrf —
vendored, logging, cors), `internal/handler/{health,openapi,tenant}.go` +
`tenant_test.go`, `internal/database/db.go` (Connect/retry/28P01) and
`migrate_demo.go` (empty `Migrate` + no-op `MigrateTenant` — put your schema
here), `api/openapi.json` (`"paths": {}`, canonical `servers: /api/v1`) with
`api/api.go` embedding it.

Frontend: `src/main.tsx`, `src/moses-browser-logger.ts`, `src/utils/baseUrl.ts`,
`src/api/{queryClient,client,hooks,queryKeys}.ts` (transport + key factory),
`src/App.tsx` (placeholder route, keep the MOSES ROUTING comment),
`src/App.test.tsx`, `src/components/{Layout.tsx,Layout.css,ThemeToggle.tsx}`,
`src/styles/theme.css`, `src/App.css`, `index.html` (moses-base-path meta +
theme init), plus `nginx.conf`, `entrypoint.sh`, `Dockerfile`,
`vite.config.ts`. `helm/` includes `templates/postgresql.yaml`.

Start by editing the `moses-app.config.json` identity fields, add routes in
`backend/cmd/server/demo_routes.go`, add your schema in
`backend/internal/database/migrate_demo.go`, keep `backend/api/openapi.json`
paths in sync with the mux (the cmd/server tests enforce this), then validate:

```bash
cd backend && go vet ./... && go test ./...
cd frontend && npm install && npm run lint && npm test && npm run build
```

## Environment-variable contract

Every variable the code reads, with its source file:

| Variable | Read in | Meaning |
|----------|---------|---------|
| `PORT` | `backend/cmd/server/main.go` | HTTP listen port (default `8080`) |
| `MOSES_BASE_PATH` | `backend/cmd/server/main.go`, `frontend/entrypoint.sh` | Sub-path the platform deploys the app at (`/apps/<tenant>/<slug>`); API mounts ONCE under it |
| `BASE_URL` | `backend/cmd/server/main.go` | DEPRECATED alias for `MOSES_BASE_PATH` |
| `MOSES_TENANT_ID` | `backend/internal/config/moses.go` | Deploy-pinned self tenant — the ONLY authoritative storage/lookup key (CHAT-pxeo.12) |
| `MOSES_DEPLOYED` | `backend/internal/config/moses.go` | `1` on platform deploys → fail-fast if `MOSES_TENANT_ID` unset |
| `MOSES_STRICT_TENANT_CHECK` | `backend/internal/handler/tenant.go` | Default on; `false` disables the 403 caller/self cross-check |
| `DB_HOST` / `DB_PORT` / `DB_USER` / `DB_PASSWORD` / `DB_NAME` / `DB_SSLMODE` | `backend/internal/database/db.go` | PostgreSQL connection; Helm injects real values (`helm/templates/deployment.yaml`); the `showcase-secret` default is dev-only — 28P01 fail-fast guards misconfig |
| `CORS_ALLOWED_ORIGINS` | `backend/internal/middleware/cors.go` | Comma-separated exact origins; unset (default) = no CORS headers at all; a literal `*` allows any origin but never with credentials |
| `MOSES_PLATFORM_API_KEY` / `MOSES_PLATFORM_API_URL` | nothing (no code in this template reads them) | Platform integration grant credentials — read them yourself if you call the Moses Platform API (see pattern below) |
| `MOSES_EMBEDDING_FRAMING` / `MOSES_EMBEDDING_ALLOWED_ANCESTORS` / `MOSES_EMBEDDING_REPORT_URI` / `MOSES_DOMAIN` | `frontend/entrypoint.sh` | CSP frame-ancestors policy rendered at container start |
| `BACKEND_SERVICE_HOST` / `BACKEND_SERVICE_PORT` | `frontend/entrypoint.sh` | Backend service DNS — **auto-injected by the chart** (`helm/templates/deployment.yaml`), never set manually |
| `VITE_MOSES_CHART_ID` / `VITE_MOSES_DEPLOYMENT_ID` / `VITE_MOSES_API_BASE` | `frontend/src/moses-browser-logger.ts` (build args) | Browser-log reporter wiring (BLF-B); silent no-op when absent |

## Preserved pattern: platform API consumption with graceful degradation

This is the pattern to copy when your app calls the Moses Platform API — the
template ships no such handler itself. Declare the grant in
`moses-app.config.json` (`integrations.required`, type `moses-platform`); once
a tenant admin approves it, the platform injects `MOSES_PLATFORM_API_KEY` +
`MOSES_PLATFORM_API_URL` into the pod:

```go
type UsersHandler struct {
    apiKey string
    apiURL string
    client *http.Client
}

func NewUsersHandler() *UsersHandler {
    return &UsersHandler{
        apiKey: os.Getenv("MOSES_PLATFORM_API_KEY"),
        apiURL: os.Getenv("MOSES_PLATFORM_API_URL"),
        client: &http.Client{Timeout: 10 * time.Second},
    }
}

func (h *UsersHandler) Users(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    // Graceful degradation: grant not approved yet → empty result +
    // actionable message, NOT an error. The app stays usable.
    if h.apiKey == "" || h.apiURL == "" {
        json.NewEncoder(w).Encode(map[string]interface{}{
            "users":   []PlatformUser{},
            "message": "Platform API not configured — approve the moses-platform integration grant to enable",
        })
        return
    }

    req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet,
        h.apiURL+"/api/v1/platform/users", nil)
    req.Header.Set("X-API-Key", h.apiKey)

    resp, err := h.client.Do(req)
    if err != nil {
        log.Printf("platform users: platform API call failed: %v", err)
        writeUsersError(w, http.StatusBadGateway) // generic envelope
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        // NEVER forward upstream bodies/status verbatim to the browser —
        // log the truncated detail server-side, return a generic envelope.
        body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
        log.Printf("platform users: platform API returned %d: %s",
            resp.StatusCode, strings.TrimSpace(string(body)))
        writeUsersError(w, http.StatusBadGateway)
        return
    }

    w.WriteHeader(http.StatusOK)
    io.Copy(w, resp.Body)
}
```

## Preserved pattern: per-table legacy-tenant rewrite (MigrateTenant)

`migrate_demo.go` ships as an empty `Migrate` + no-op `MigrateTenant`. When you
add tenant-scoped tables, keep this boot-time rewrite (idempotent, one UPDATE
per table) so rows written under the historic `local-dev`/`default`/`''`
fallback get re-owned by the deploy-pinned tenant:

```go
// CHAT-pxeo.12: runs synchronously in main() BEFORE the HTTP listener
// starts, AFTER schema Migrate. One UPDATE per tenant-scoped table.
func MigrateTenant(db *sql.DB, selfTenantID string) error {
    if selfTenantID == "" || selfTenantID == "local-dev" {
        return nil // non-authoritative sentinel — skip
    }
    res, err := db.Exec(
        `UPDATE things SET tenant_id = $1 WHERE tenant_id IN ('local-dev', 'default', '')`,
        selfTenantID,
    )
    if err != nil {
        return fmt.Errorf("things tenant rewrite: %w", err)
    }
    if n, _ := res.RowsAffected(); n > 0 {
        log.Printf("rewrote %d legacy 'things' rows to tenant %s", n, selfTenantID)
    }
    // Repeat the UPDATE for every additional tenant-scoped table.
    return nil
}
```

## Rules that keep the platform integration working

- **Relative fetch paths only** in the frontend (`fetch('api/v1/...')`, never
  `/api/...`) — absolute paths bypass the nginx proxy and 404 on the platform.
- **API registered ONCE under `MOSES_BASE_PATH`**; only `/health` and
  `/api/openapi.json` are dual-mounted (kubelet probe / platform discovery hit
  the canonical path). See `buildMux` in `main.go`.
- **OpenAPI spec**: `servers` exactly `[{"url": "/api/v1"}]`, paths RELATIVE
  to it (never `/api/`-rooted — double-prefix), `/health` never listed (it
  would register a phantom workspace tool). `cmd/server` tests enforce all of
  this plus spec↔mux consistency.
- **Storage tenant = `MOSES_TENANT_ID` env**, never the `X-Moses-Tenant-ID`
  header (caller-context/audit only). Use the `enforceTenantMatch` helper in
  `internal/handler/tenant.go` for the 403 cross-check.
- **Secrets**: declare runtime secrets in `moses-app.config.json` →
  `secrets.external[]` (see `skills/secrets-tutorial.md` and the with-secrets
  example config), never hardcode.
- **PostgreSQL data is ephemeral** (emptyDir) until you switch
  `helm/templates/postgresql.yaml` to a PVC — see the comment in that file.
