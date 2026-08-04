# Moses OIDC Relying-Party Template (`fullstack-oidc`)

**The canonical reference app for app-owned OIDC** — a full-stack app
that authenticates real users via OIDC, fronted by Moses (BFF pattern).

Go backend + React frontend + PostgreSQL, deployed as a multi-service
Helm chart. The distinctive piece is the **vendored `oidcauth`
middleware** (`backend/internal/oidcauth/`) — a self-contained,
stdlib-only OIDC relying-party implementation that any other app can
copy.

## What this demonstrates

App-owned OIDC is a four-link chain. This template ships every link
wired and working, with the app logic left to you:

1. **Declare** — the app adds `access.oidc` + `access.roles` to
   `moses-app.config.json`.
2. **Inject** — the Moses platform registers a confidential Keycloak
   client and injects the `MOSES_OIDC_*` env + a client-secret K8s
   Secret.
3. **Enforce** — the vendored `oidcauth` middleware runs the
   authorization-code + PKCE BFF flow and gates protected routes.
4. **Read** — handlers read the validated identity and the roles Moses
   projected onto the token.

Concretely the template gives you:

- A deployed Moses app acting as an **OIDC relying party** —
  authorization-code flow + PKCE, confidential client, HttpOnly session
  cookie. The browser never holds a token (BFF pattern).
- **Roles**: the user's `resource_access.<client>.roles` projection is
  surfaced on the identity (`GET /api/v1/me`); role-based authorization
  decisions are made server-side by the handlers that need them.
- **Dual mode**: OIDC enforced on protected paths, while the `X-Moses-*`
  trusted-header path keeps working for pod-to-pod MCP / workspace-tool
  calls — but ONLY when the request also carries the
  `X-Moses-Gateway-Auth` marker matching `MOSES_GATEWAY_AUTH_SECRET`.
  Without that env var the header-trust path is disabled entirely
  (fail-safe). See "OIDC env contract" below.
- **Silent SSO**: an already-logged-into-Moses user is authenticated
  with no visible login page (`prompt=none`, hidden iframe) — see
  `frontend/src/auth/silentSSO.ts`.
- **Two data spaces to choose between**: the **user space** (scoped by
  tenant id + OIDC subject = private to one person) and the **tenant space**
  (scoped by tenant id alone = shared by the whole workspace). Most apps need
  both. Default collaborative or agent-fed content to the tenant space —
  content an agent posts via a workspace-tool call arrives under the
  *agent's* `X-Moses-User-ID`, so user-scoped-only data is invisible to the
  human. See **Data scoping** below.

## The route surface

The template ships only the plumbing routes below. Register your own in
`backend/cmd/server/demo_routes.go`, mirror them in
`backend/api/openapi.json`, and list the gated ones in
`moses-app.config.json` → `access.oidc.protectedPaths`.

| Route                 | Access                  | Why |
|-----------------------|-------------------------|-----|
| `/health`             | public (declared)       | K8s probe — never gated |
| `/api/openapi.json`   | public (declared)       | Platform discovery / workspace-tool surface |
| `/api/v1/me`          | protected               | The validated identity |

`protectedPaths` is **deny-by-default**: a route you neither declare public
nor declare protected is gated. Add role gating server-side in the handler,
against the roles on `oidcauth.IdentityFrom(r.Context())`.

## Data scoping — user space vs tenant space

Before you add your first table, make this decision explicitly — getting it
wrong is a silent bug:

| | user space | tenant space |
|---|---|---|
| Storage key | `(tenant_id, owner_sub)` | `tenant_id` |
| Visibility | private to one OIDC subject | every member of the tenant |
| Read filter | `WHERE tenant_id = $1 AND owner_sub = $2` | `WHERE tenant_id = $1` |

`tenant_id` always comes from `config.SelfTenantID()` (the deploy-pinned
`MOSES_TENANT_ID` env), **never** from the `X-Moses-Tenant-ID` request header
(that header is caller context — see `internal/config`).

**Why this matters for agents.** When another agent delivers content into this
app through a Moses **workspace-tool** call, the request arrives on the trusted
`X-Moses-*` header path and `oidcauth` resolves its identity from the
`X-Moses-User-ID` header — i.e. the **agent's** user id, not the human's. A row
scoped by user id alone therefore lands under the agent and the human who owns
the app never sees it. **Default user-facing / collaborative / agent-fed content
to the tenant space; reserve the user space for data that is meant to stay
private.**

## Layout

```
fullstack-oidc/
├── backend/
│   ├── api/openapi.json          OpenAPI spec (embedded via go:embed)
│   ├── cmd/server/main.go        wires oidcauth into the HTTP server
│   ├── cmd/server/demo_routes.go your API route registration hook
│   ├── cmd/server/example_test.go worked `things` routes+handler example (compiled by CI, never shipped)
│   └── internal/
│       ├── oidcauth/             ★ the vendored OIDC middleware
│       ├── config/               deploy-pinned tenant identity
│       ├── database/             db.go (connect/retry) + migrate_demo.go (your schema)
│       ├── handler/              health, openapi, me (session introspection)
│       └── middleware/           moses-headers, cors, logging
├── frontend/
│   └── src/
│       ├── auth/                 useAuth provider + silentSSO helper
│       ├── api/                  fetch client + TanStack Query hooks
│       └── components/Layout.tsx app shell + sidebar nav + auth pill
├── helm/                         multi-service chart (+ oidc cookie Secret)
├── skills/                       agent skill docs (usage, middleware, secrets)
└── moses-app.config.json         declares access.oidc + access.roles
```

## The `oidcauth` package

`backend/internal/oidcauth/` is **vendored** — copy the whole directory
into another template's `backend/internal/oidcauth/` and import it from
that template's own module path. No third-party Go dependencies. See
`skills/oidcauth-middleware.md` for the integration recipe.

## OIDC env contract

The Moses platform delivers the seven `MOSES_OIDC_*` handshake values
(issuer, internal issuer, client id, client secret, audience,
protected/public paths) in a **platform-managed Kubernetes Secret**
whose name it appends to the chart's `secrets.secretNames[]` list; the
deployment template mounts every entry via `envFrom`, so the values
arrive as plain env vars. They are **NOT chart values** — see the
comment block in `helm/values.yaml` under `moses.oidc`.

The chart's own `-oidc` Secret (`helm/templates/oidc-secret.yaml`)
carries exactly ONE key: the session-cookie encryption key
(`MOSES_OIDC_COOKIE_SECRET`, AES-256-GCM; also HMACs the state cookie),
kept stable across `helm upgrade`. The confidential client secret is no
longer stored there.

| Var | Purpose |
|-----|---------|
| `MOSES_OIDC_ISSUER` | external issuer URL — browser redirects, `iss` |
| `MOSES_OIDC_INTERNAL_ISSUER` | in-cluster issuer — JWKS, token exchange |
| `MOSES_OIDC_CLIENT_ID` | confidential client id |
| `MOSES_OIDC_CLIENT_SECRET` | confidential client secret (platform Secret) |
| `MOSES_OIDC_AUDIENCE` | expected `aud` (defaults to client id) |
| `MOSES_OIDC_PROTECTED_PATHS` | gated path prefixes |
| `MOSES_OIDC_PUBLIC_PATHS` | always-public path prefixes |
| `MOSES_OIDC_COOKIE_SECRET` | session-cookie AES-256-GCM encryption key + state-cookie HMAC key (chart `-oidc` Secret) |
| `MOSES_OIDC_SESSION_MAX_AGE_SECONDS` | BFF session lifetime (default 8h; roles refresh via the refresh-token grant) |
| `MOSES_GATEWAY_AUTH_SECRET` | **arms the X-Moses-\* header-trust path.** Unset ⇒ pod-to-pod header trust is DISABLED entirely (workspace-tool calls fall through to OIDC and get 401). The platform proxy sends the matching `X-Moses-Gateway-Auth` header. |
| `MOSES_INTERAPP_SECRET` | per-tenant HS256 key for inter-app trust tokens (unset ⇒ inter-app path disabled) |
| `MOSES_APP_SLUG` | this app's slug — `iss`/`aud` of inter-app tokens |
| `MOSES_PUBLIC_URL` / `MOSES_PUBLIC_URLS` | external origin(s) used to build `redirect_uri` |

When the OIDC vars are absent the middleware runs in **pass-through
mode** — public routes and the (marker-gated) header-trust path still
work; browser requests are not redirected.

### Session lifetime (decoupled from the token TTL)

The BFF session lives `MOSES_OIDC_SESSION_MAX_AGE_SECONDS` (default
8 h) — deliberately DECOUPLED from the ~minutes-scale Keycloak token
lifetime, so a page refresh does not restart the login. The token
lifetime bounds only the ROLES snapshot: when it lapses the middleware
re-mints the roles via the refresh-token grant, so role revocation
still lands within ~the token TTL. A failed refresh — Keycloak SSO
session idled out, revoked, or unreachable — ends the session; without
a refresh token the session falls back to dying with the token. Overall
login duration is additionally bounded by Keycloak's SSO Session
Idle/Max settings — those are the knobs to tune, not the Access Token
Lifespan.

## Config — `moses-app.config.json`

As shipped:

```json
"access": {
  "mode": "tenant",
  "roles": [],
  "oidc": {
    "mode": "moses-oidc",
    "protectedPaths": ["/api/v1/me"],
    "publicPaths": ["/health", "/api/openapi.json"]
  }
}
```

Extend both lists as you add routes — e.g. append `"/api/v1/things"` to
`protectedPaths`, and declare a role vocabulary in `access.roles`
(`["admin", "member"]`) when you want role gating.

`access.roles` is the role vocabulary a tenant admin maps users into;
Moses projects the mapping into `resource_access.<client>.roles` on the
token. The path lists are **not** chart values — the platform delivers them
to the pod as `MOSES_OIDC_PROTECTED_PATHS` / `MOSES_OIDC_PUBLIC_PATHS` env
vars via the platform-managed Secret (the chart's `moses.oidc` block
deliberately carries no placeholder path fields, so stale chart copies can
never shadow the live config).

Declaring runtime secrets — see [skills/secrets-tutorial.md](skills/secrets-tutorial.md).

## Build & test

```bash
cd backend  && go build ./... && go test ./...
cd frontend && npm install && npm run build && npm test
helm lint helm
```

## Running standalone (without Moses)

```bash
# Backend (terminal 1)
cd backend && go run cmd/server/main.go
# No MOSES_OIDC_* env -> oidcauth runs in pass-through mode.

# Frontend (terminal 2)
cd frontend && npm install && npm run dev
# Access: http://localhost:3000 (Vite proxy -> backend:8080)

# Helm, standalone:
helm install oidc-demo helm \
  --set postgresql.auth.password=$(openssl rand -hex 24)
```

## SameSite

The session cookie is `SameSite=Lax` — see the rationale in the
`sessionSameSite` doc comment in `backend/internal/oidcauth/session.go`.

## Deploying via Moses

An agent runs `moses_init_repo(template="fullstack-oidc")` to scaffold a
new project from this template. On agent completion the platform builds
both images in-cluster, deploys the multi-service Helm chart, registers a
confidential Keycloak client (because `access.oidc` is declared), and
injects the `MOSES_OIDC_*` env. The app then enforces OIDC with zero
further configuration.
