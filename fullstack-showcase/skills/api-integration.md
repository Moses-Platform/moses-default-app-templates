# Moses API Integration - Agent Skill

How a deployed app integrates with the Moses platform. Everything below
mirrors the actual code in this template — copy from the referenced files,
not from memory.

## Tenant identity: env-pinned, NOT header-derived (CHAT-pxeo.12)

The backend's authoritative tenant comes from the `MOSES_TENANT_ID` env var
the platform injects at deploy time (`internal/config/moses.go` →
`config.SelfTenantID()`). The `X-Moses-Tenant-ID` request header is
caller-context only (audit + the 403 cross-check) — it is ABSENT on
browser-driven requests through the platform's user-facing app proxy, so an
app that stores rows under the header value silently writes them under `""`.

```go
// GOOD — storage keys from the deploy-pinned env (via MosesContext):
mc := middleware.GetMosesContext(r.Context())
if enforceTenantMatch(w, mc) { // 403 cross-check helper, internal/handler/tenant.go
    return
}
tenantID := mc.SelfTenantID // = config.SelfTenantID(), from MOSES_TENANT_ID env

rows, err := db.QueryContext(r.Context(),
    `SELECT id, name FROM things WHERE tenant_id = $1`, tenantID)

// BAD — header-derived storage tenant (forbidden: empty on browser requests,
// caller-spoofable on direct ones):
db.Query(ctx, query, r.Header.Get("X-Moses-Tenant-ID"))
```

Always still filter every query by `tenant_id` — the pin changes WHERE the
value comes from, not the requirement to scope.

## Moses header middleware

`internal/middleware/moses_headers.go` extracts all X-Moses-* headers into a
`MosesContext` carrying BOTH `SelfTenantID` (env) and `CallerTenantID`
(header). Downstream handlers read it via `middleware.GetMosesContext(ctx)`.
Reuse that file as-is; do not re-derive tenant identity per handler.

## OpenAPI spec requirements

The platform probes `/api/openapi.json` (plus 10 other standard paths) after
deploy and generates one MCP tool per operation: `workspace_{toolKey}_{operationId}`.

Canonical shape (enforced by this template's `cmd/server` tests):

```json
{
  "openapi": "3.0.3",
  "servers": [{ "url": "/api/v1" }],
  "paths": {
    "/things": {
      "get": { "operationId": "listThings", "summary": "..." }
    }
  }
}
```

- `servers` MUST be exactly `[{"url": "/api/v1"}]` and stay base-path-free —
  the platform folds `servers[0].url + path` into the endpoint and prepends
  `MOSES_BASE_PATH` itself; a base-path-aware or `/api/`-rooted entry
  double-prefixes and every tool call 404s.
- Path keys are RELATIVE to that base (`/things`, not
  `/api/v1/things`).
- Do NOT list `/health` — it would register a phantom workspace tool.
- Every operation needs a unique `operationId` (it names the MCP tool).

## Multi-service frontend→backend communication

### nginx proxy (see `frontend/nginx.conf` — the real contract)

Root-relative traffic forwards onto the backend's `MOSES_BASE_PATH` mount
(the backend registers its API ONCE under that prefix — root README path
contract):

```nginx
location /api/ {
    # ${MOSES_BASE_PATH_PREFIX} is "" standalone and "/apps/<t>/<slug>" on
    # Moses — rendered by entrypoint.sh at container start.
    proxy_pass http://${BACKEND_SERVICE_HOST}:${BACKEND_SERVICE_PORT}${MOSES_BASE_PATH_PREFIX}/api/;
    proxy_set_header Host $host;
    proxy_pass_header X-Moses-Tenant-ID;  # caller-context only, see above
    proxy_pass_header X-Moses-User-ID;
}
```

For the sub-path mount itself (`/apps/<t>/<slug>/api/...`), entrypoint.sh
renders a `location ^~ ${MOSES_BASE_PATH_PREFIX}/api/` block whose
`proxy_pass` has **NO URI part** (CHAT-yfmwv) — the full prefixed path is
forwarded unchanged to the backend's base-path-mounted routes. A URI part
would strip the matched prefix and the backend would 404. Never hand-write a
URI-part-only `proxy_pass http://backend/api/;` for sub-path traffic.

Frontend JS uses RELATIVE fetch paths only (`fetch('api/v1/...')`).

### Service discovery

`BACKEND_SERVICE_HOST` / `BACKEND_SERVICE_PORT` are **auto-injected by the
Helm chart** for frontend containers (`helm/templates/deployment.yaml`,
"Auto-inject backend service discovery" block) — do NOT set them manually in
values. entrypoint.sh carries shell-default fallbacks for standalone runs.

## CORS: off by default, exact-origin allowlist opt-in

`internal/middleware/cors.go`: no CORS headers are emitted at all unless
`CORS_ALLOWED_ORIGINS` (comma-separated exact origins) is set. A matching
origin is echoed back verbatim with `Vary: Origin`; a wildcard is never
combined with credentials. A Moses-deployed app is same-origin behind the
platform edge, so the default (no CORS) is correct for almost every app.

## Health check

`/health` is dual-mounted (canonical for the kubelet probe, base-path alias
for sub-path callers) and returns a small JSON status —
`internal/handler/health.go`. Keep it out of the OpenAPI spec.

## Stack notes

- Backend: Go 1.26 (`go.mod` 1.26.4), `net/http` mux + **pgx v5**
  (`jackc/pgx/v5/stdlib`) for PostgreSQL — not stdlib-only.
- DB bootstrap: `internal/database/db.go` — retry loop with 28P01
  (invalid_password) fail-fast; schema lives in `migrate_demo.go`.
- Deployment automation: image build (Buildah, in-cluster) → Helm deploy →
  readiness probes → OpenAPI discovery → MCP tool generation.
