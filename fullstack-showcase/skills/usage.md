# Fullstack Showcase Template — Usage

Go 1.25 backend (net/http + pgx v5) + React 19 frontend + per-app PostgreSQL,
deployed as a two-service Helm chart. The demo content is an educational tour
of Moses platform capabilities; the plumbing underneath is the real template.

## First step: clean out the demo

Building a real app on this template? Run this ONCE from the template root:

```bash
./clean_out_template.sh
```

It deletes all showcase/demo code, swaps mixed files for clean minimal twins,
then deletes itself. What remains is a building-and-testing skeleton with ALL
Moses plumbing intact: path contract (MOSES_BASE_PATH single-mount API +
health/openapi dual-mount), X-Moses header middleware, CSRF rejection, CORS
allowlist, browser-logger, nginx CSP/entrypoint, Helm chart incl. PostgreSQL,
tenant-enforcement helpers, DB connect/retry/28P01 fail-fast.

After running it: edit `moses-app.config.json` identity fields, add routes in
`backend/cmd/server/demo_routes.go`'s successor, add your schema in
`backend/internal/database/migrate_demo.go`, keep `backend/api/openapi.json`
paths in sync with the mux (the cmd/server tests enforce this), then validate:

```bash
cd backend && go vet ./... && go test ./...
cd frontend && npm install && npm run lint && npm test && npm run build
```

### Manual strip (older clones without the script)

| Action | Paths |
|--------|-------|
| **KEEP** (plumbing) | `backend/cmd/server/main.go` (buildMux health+openapi dual-mount), `backend/internal/config/moses.go`, `backend/internal/middleware/` (moses_headers, csrf — vendored, logging, cors), `backend/internal/handler/{health,openapi,tenant}.go` + `tenant_test.go`, `backend/internal/database/db.go` (Connect/retry/28P01), `backend/api/api.go`, `frontend/src/{main.tsx,moses-browser-logger.ts,utils/baseUrl.ts,api/queryClient.ts,components/ThemeToggle.tsx,styles/theme.css,App.css,components/Layout.css}`, `frontend/{nginx.conf,entrypoint.sh,Dockerfile,vite.config.ts}`, `helm/` (incl. `templates/postgresql.yaml`), `skills/{usage,api-integration,secrets-tutorial}.md` |
| **REPLACE** (with a minimal clean version) | `backend/cmd/server/demo_routes.go` (empty registerDemoRoutes stub), `backend/cmd/server/main_test.go` (contract-only tests), `backend/internal/database/migrate_demo.go` (empty Migrate + no-op MigrateTenant), `backend/api/openapi.json` (`"paths": {}`, keep servers `/api/v1`), `frontend/src/App.tsx` (placeholder route, keep MOSES ROUTING comment), `frontend/src/components/Layout.tsx` (minimal shell), `frontend/src/api/{client,hooks,queryKeys}.ts` (keep transport + factory), `frontend/src/App.test.tsx`, `frontend/index.html` (keep moses-base-path meta + theme init), `moses-app.config.json` + `moses-app.config.with-secrets.example.json` (drop demo quickActions/platformActions/integrations + showcase-overview skill) |
| **DELETE** (demo only) | `backend/internal/handler/{moses_info,capabilities,capabilities_test,notes,notes_test,notes_tenant_test,users}.go`, `backend/internal/model/`, `frontend/src/pages/`, `frontend/src/components/{FeatureCard,FlowDiagram,NotesPanel}.{tsx,css}`, `frontend/src/components/UserList.tsx`, `skills/showcase-overview.md` |

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
| `MOSES_PLATFORM_API_KEY` / `MOSES_PLATFORM_API_URL` | `backend/internal/handler/users.go` (demo) | Platform integration grant credentials (see pattern below) |
| `MOSES_EMBEDDING_FRAMING` / `MOSES_EMBEDDING_ALLOWED_ANCESTORS` / `MOSES_EMBEDDING_REPORT_URI` / `MOSES_DOMAIN` | `frontend/entrypoint.sh` | CSP frame-ancestors policy rendered at container start |
| `BACKEND_SERVICE_HOST` / `BACKEND_SERVICE_PORT` | `frontend/entrypoint.sh` | Backend service DNS — **auto-injected by the chart** (`helm/templates/deployment.yaml`), never set manually |
| `VITE_MOSES_CHART_ID` / `VITE_MOSES_DEPLOYMENT_ID` / `VITE_MOSES_API_BASE` | `frontend/src/moses-browser-logger.ts` (build args) | Browser-log reporter wiring (BLF-B); silent no-op when absent |

## Preserved pattern: platform API consumption with graceful degradation

The demo `users.go` is deleted on cleanout — this is the pattern to copy when
your app calls the Moses Platform API. Declare the grant in
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

The demo `migrate_demo.go` is replaced by a stub on cleanout. When you add
tenant-scoped tables, keep this boot-time rewrite (idempotent, one UPDATE per
table) so rows written under the historic `local-dev`/`default`/`''` fallback
get re-owned by the deploy-pinned tenant:

```go
// CHAT-pxeo.12: runs synchronously in main() BEFORE the HTTP listener
// starts, AFTER schema Migrate. One UPDATE per tenant-scoped table.
func MigrateTenant(db *sql.DB, selfTenantID string) error {
    if selfTenantID == "" || selfTenantID == "local-dev" {
        return nil // non-authoritative sentinel — skip
    }
    res, err := db.Exec(
        `UPDATE notes SET tenant_id = $1 WHERE tenant_id IN ('local-dev', 'default', '')`,
        selfTenantID,
    )
    if err != nil {
        return fmt.Errorf("notes tenant rewrite: %w", err)
    }
    if n, _ := res.RowsAffected(); n > 0 {
        log.Printf("rewrote %d legacy 'notes' rows to tenant %s", n, selfTenantID)
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
