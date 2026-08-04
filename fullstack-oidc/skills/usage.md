# fullstack-oidc — Template Usage

## What you start with

The template is demo-free: a minimal-but-fully-integrated OIDC relying
party that builds and passes its tests. The vendored `oidcauth` middleware
(`backend/internal/oidcauth/`, whole package) runs the authorization-code +
PKCE BFF flow; `backend/internal/handler/me.go` (+`me_test.go`) is the
`/api/v1/me` session introspection endpoint; `health.go` + `openapi.go` are
the dual-mounted platform contract; `backend/internal/config/` pins the
tenant identity; `backend/internal/middleware/` carries moses_headers, cors
and logging; `backend/internal/database/db.go` is the connect/retry
plumbing and `migrate_demo.go` an empty schema hook;
`backend/cmd/server/main.go` wires it all together, with
`demo_routes.go` as an empty route-registration stub and `main_test.go`
holding the contract + `/me` tests. `backend/api/openapi.json` declares
`/me` only (embedded via `api/api.go`).

On the frontend, `src/auth/` (useAuth, silentSSO + test) is the auth pill +
silent-SSO bootstrap; `src/api/` holds `queryClient.ts` plus the
transport/hooks/query-key trio; `src/utils/baseUrl.ts` the base-path helper;
`src/moses-browser-logger.ts` is vendored — never edit it. `src/App.tsx` is a
placeholder Home route, `src/components/Layout.tsx` the shell + auth pill
(with `Layout.css`, `ThemeToggle.tsx`, `styles/theme.css`), and `index.html`
carries a generic title and no webfonts. `frontend/nginx.conf`,
`entrypoint.sh`, both Dockerfiles, `helm/`, `skills/` and
`moses-app.config.json` (declaring `access.oidc` + `access.roles`) complete
the deployment surface.

To start building:

1. Edit `moses-app.config.json` identity (name, displayName, description).
2. Add backend routes in `backend/cmd/server/demo_routes.go` (rename it when
   you outgrow the name) and handlers in `backend/internal/handler/`.
3. Add your schema to `backend/internal/database/migrate_demo.go`.
4. Add frontend routes in `src/App.tsx`, data hooks in `src/api/`.
5. Update `backend/api/openapi.json` paths for every route you add.
6. Verify: `cd backend && go vet ./... && go test ./...`,
   `cd frontend && npm run lint && npm test && npm run build`.

## Env-var contract (everything the code reads)

| Env var | Read by | Purpose |
|---------|---------|---------|
| `PORT` | `cmd/server/main.go` | HTTP listen port (default 8080) |
| `MOSES_BASE_PATH` | main.go, oidcauth | deploy sub-path (`/apps/<tenant>/<slug>`); `BASE_URL` is a deprecated alias |
| `MOSES_TENANT_ID` | `internal/config/moses.go` | deploy-pinned tenant id — ALL data scoping; never use the `X-Moses-Tenant-ID` header for storage keys |
| `MOSES_DEPLOYED` | `internal/config/moses.go` | `1` on platform pods ⇒ missing `MOSES_TENANT_ID` fails startup |
| `DB_HOST` / `DB_PORT` / `DB_USER` / `DB_PASSWORD` / `DB_NAME` / `DB_SSLMODE` | `internal/database/db.go` | PostgreSQL connection (defaults: localhost/5432/app/…/appdb/disable) |
| `MOSES_OIDC_ISSUER` | `oidcauth/config.go` | external issuer — browser redirects + `iss` validation |
| `MOSES_OIDC_INTERNAL_ISSUER` | oidcauth | in-cluster issuer — JWKS + token exchange |
| `MOSES_OIDC_CLIENT_ID` / `MOSES_OIDC_CLIENT_SECRET` | oidcauth | confidential client (platform-managed Secret via `secrets.secretNames[]` envFrom) |
| `MOSES_OIDC_AUDIENCE` | oidcauth | expected `aud` (empty ⇒ client id) |
| `MOSES_OIDC_PROTECTED_PATHS` / `MOSES_OIDC_PUBLIC_PATHS` | oidcauth | path policy (mirrors `access.oidc` in the app config) |
| `MOSES_OIDC_COOKIE_SECRET` | oidcauth | session-cookie AES-256-GCM encryption key + state-cookie HMAC key (chart `-oidc` Secret, stable across upgrades) |
| `MOSES_OIDC_SESSION_MAX_AGE_SECONDS` | oidcauth | BFF session lifetime (default 8h; decoupled from the token TTL — roles refresh via the refresh-token grant) |
| `MOSES_OIDC_INSECURE_COOKIE` | oidcauth | `1` drops cookie `Secure` — plain-HTTP local dev only |
| `MOSES_GATEWAY_AUTH_SECRET` | `oidcauth/paths.go` | **arms X-Moses-\* header trust**; unset ⇒ pod-to-pod calls are NOT trusted and fall through to OIDC (401) |
| `MOSES_PUBLIC_URL` / `MOSES_PUBLIC_URLS` | oidcauth | external origin(s) for building `redirect_uri` (multi-home aware) |
| `MOSES_INTERAPP_SECRET` | `oidcauth/interapp.go` | per-tenant HS256 key for inter-app trust tokens (unset ⇒ path disabled) |
| `MOSES_APP_SLUG` | `oidcauth/interapp.go` | own slug — `iss` when minting / `aud` when verifying inter-app tokens |
| `CORS_ALLOWED_ORIGINS` | `internal/middleware/cors.go` | comma-separated EXACT origins allowed cross-origin; unset ⇒ no CORS headers (same-origin only). Exact matches are echoed with credentials; a literal `*` allows any origin but is never combined with credentials. |
| `BACKEND_SERVICE_HOST` / `BACKEND_SERVICE_PORT` | `frontend/entrypoint.sh` | nginx → backend proxy target (Helm-injected) |
| `MOSES_EMBEDDING_FRAMING` / `MOSES_EMBEDDING_ALLOWED_ANCESTORS` / `MOSES_EMBEDDING_REPORT_URI` | `frontend/entrypoint.sh` | CSP frame-ancestors policy rendered into nginx.conf |

## The two things most likely to bite you

1. **Header-trust needs the gateway marker.** Workspace-tool / MCP calls
   into this app are only trusted when `MOSES_GATEWAY_AUTH_SECRET` is set
   AND the request carries the matching `X-Moses-Gateway-Auth` header.
   Without the env var the entire header-trust path is off (fail-safe)
   and pod-to-pod calls 401.
2. **The session outlives the token — but not a failed refresh.** The
   BFF session lives `MOSES_OIDC_SESSION_MAX_AGE_SECONDS` (default 8h),
   decoupled from the minutes-scale token TTL; roles re-mint via the
   refresh-token grant when the token lapses. If that refresh fails
   (Keycloak SSO session idled out / revoked / unreachable) the session
   ends and protected XHRs 401 — recover with `useAuth().refresh()`.
   Details: `skills/oidcauth-middleware.md` § Session lifetime.

## Where things live

- OIDC integration recipe (declare → inject → enforce → read), config
  knobs, security properties: `skills/oidcauth-middleware.md`
- Data-scoping rules (user space vs tenant space): `README.md` §
  *Data scoping*. In short: scope by `(tenant_id, owner_sub)` only for data
  that must stay private to one person, and by `tenant_id` alone for anything
  collaborative or agent-fed — a workspace-tool call arrives under the
  *agent's* `X-Moses-User-ID`, so user-scoped rows an agent writes are
  invisible to the human. `tenant_id` always comes from
  `config.SelfTenantID()`, never from the `X-Moses-Tenant-ID` header.
- App-declared runtime secrets: `skills/secrets-tutorial.md`
- Path contract, deployment modes, standalone dev: `README.md`
