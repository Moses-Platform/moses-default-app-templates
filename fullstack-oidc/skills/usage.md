# fullstack-oidc — Template Usage

## First step: clean out the demo

Building a real app? Run this ONCE from the template root before writing
any code:

```bash
./clean_out_template.sh
```

It deletes the demo feature code (entries, shared-notes, admin-area,
public-info, the seven demo pages), swaps mixed files for clean twins,
then deletes itself and `.template-clean/`. What remains is a
minimal-but-fully-integrated OIDC relying party that builds and passes
its tests: the vendored `oidcauth` middleware, `/api/v1/me` session
introspection, the auth pill + silent-SSO bootstrap, health/openapi
dual-mounts, X-Moses middleware, helm chart, and config. Afterwards:

1. Edit `moses-app.config.json` identity (name, displayName, description).
2. Add backend routes in `backend/cmd/server/demo_routes.go` (now a
   stub — rename it when you outgrow the name) and handlers in
   `backend/internal/handler/`.
3. Add your schema to `backend/internal/database/migrate_demo.go`.
4. Add frontend routes in `src/App.tsx`, data hooks in `src/api/`.
5. Update `backend/api/openapi.json` paths for every route you add.
6. Verify: `cd backend && go vet ./... && go test ./...`,
   `cd frontend && npm run lint && npm test && npm run build`.

### Manual strip (older clones without the script)

| Action | Paths |
|--------|-------|
| **KEEP** (plumbing) | `backend/internal/oidcauth/` (whole package), `backend/internal/config/`, `backend/internal/middleware/` (moses_headers, cors, logging), `backend/internal/handler/health.go`, `openapi.go`, `me.go`, `me_test.go`, `backend/internal/database/db.go`, `backend/api/api.go`, `backend/cmd/server/main.go`, `frontend/src/auth/` (useAuth, silentSSO + test), `frontend/src/api/queryClient.ts`, `frontend/src/utils/baseUrl.ts`, `frontend/src/moses-browser-logger.ts` (vendored — never edit), `frontend/src/main.tsx`, `frontend/src/styles/theme.css`, `frontend/src/components/ThemeToggle.tsx`, `Layout.css`, `frontend/nginx.conf`, `entrypoint.sh`, both Dockerfiles, `helm/`, `skills/` |
| **REPLACE** (mixed → minimal twin) | `backend/cmd/server/demo_routes.go` (empty stub), `backend/cmd/server/main_test.go` (contract + me tests), `backend/internal/database/migrate_demo.go` (empty schema), `backend/api/openapi.json` (`/me` only), `frontend/src/App.tsx` (placeholder Home), `frontend/src/App.test.tsx`, `frontend/src/components/Layout.tsx` (shell + auth pill, no demo nav), `frontend/src/api/client.ts` / `hooks.ts` / `queryKeys.ts` (me-only), `frontend/index.html` (generic title, no webfonts), `moses-app.config.json` (trimmed access lists) |
| **DELETE** (demo) | `backend/internal/handler/entries.go`, `shared.go`, `shared_test.go`, `demo_handlers.go`, `demo_handlers_test.go`, `frontend/src/pages/` (all 7 pages + `pages.css`), `frontend/src/components/NavIcons.tsx`, `skills/oidc-overview.md` |

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
| `MOSES_OIDC_COOKIE_SECRET` | oidcauth | session-cookie HMAC key (chart `-oidc` Secret, stable across upgrades) |
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
2. **The session dies at the ID-token `exp`.** With Keycloak's default
   ~5-minute Access Token Lifespan the BFF session expires ~5 minutes
   after login and the SPA does NOT re-probe automatically (one-shot
   `silentTried` guard). Raise the Keycloak Access Token Lifespan for
   this client, and/or call `useAuth().refresh()` on 401. Details:
   `skills/oidcauth-middleware.md` § Session lifetime.

## Where things live

- OIDC integration recipe (declare → inject → enforce → read), config
  knobs, security properties: `skills/oidcauth-middleware.md`
- Demo walkthrough + data-scoping rules (user space vs tenant space):
  `skills/oidc-overview.md` (deleted by the clean-out — the data-scoping
  rules also live in the README)
- App-declared runtime secrets: `skills/secrets-tutorial.md`
- Path contract, deployment modes, standalone dev: `README.md`
