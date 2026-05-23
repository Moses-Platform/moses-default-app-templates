# Moses OIDC Relying-Party Template (`fullstack-oidc`)

**The canonical reference app for app-owned OIDC** — a full-stack app
that authenticates real users via OIDC, fronted by Moses (BFF pattern).

Go backend + React frontend + PostgreSQL, deployed as a multi-service
Helm chart. The distinctive piece is the **vendored `oidcauth`
middleware** (`backend/internal/oidcauth/`) — a self-contained,
stdlib-only OIDC relying-party implementation that any other app can
copy.

## What this demonstrates

App-owned OIDC is a four-link chain. This template makes every link
visible, and the running app's **How It Works** page narrates it:

1. **Declare** — the app adds `access.oidc` + `access.roles` to
   `moses-app.config.json`.
2. **Inject** — the Moses platform registers a confidential Keycloak
   client and injects the `MOSES_OIDC_*` env + a client-secret K8s
   Secret.
3. **Enforce** — the vendored `oidcauth` middleware runs the
   authorization-code + PKCE BFF flow and gates protected routes.
4. **Read** — handlers read the validated identity and the roles Moses
   projected onto the token.

Concretely the app shows:

- A deployed Moses app acting as an **OIDC relying party** —
  authorization-code flow + PKCE, confidential client, HttpOnly session
  cookie. The browser never holds a token (BFF pattern).
- A clear **route split**: a PUBLIC route, two PROTECTED routes, and one
  PROTECTED + ROLE-GATED route — so a viewer can SEE the integration.
- **Roles**: the user's `resource_access.<client>.roles` projection,
  surfaced in the UI and enforced server-side on `/api/v1/admin-area`.
- **Dual mode**: OIDC enforced on protected paths, while the existing
  `X-Moses-*` trusted-header path keeps working for pod-to-pod MCP /
  workspace-tool calls.
- **Silent SSO**: an already-logged-into-Moses user is authenticated
  with no visible login page (`prompt=none`, hidden iframe).
- **Per-user data**: the `entries` table is scoped by both the
  deploy-pinned tenant id and the authenticated OIDC subject.

## The six demo pages

| Page            | What it shows |
|-----------------|---------------|
| Overview        | The pattern + the live backend posture (OIDC enforced vs pass-through) |
| My Identity     | The authenticated principal from the validated session (`GET /api/v1/me`) |
| Roles & Access  | Projected roles + a role-gated route that 403s without `oidc-admin` |
| My Entries      | Per-user data scoped to the OIDC subject (protected read/write) |
| Silent SSO      | The `prompt=none` hidden-iframe path for the embedded case |
| How It Works    | The full declare → inject → enforce → read walkthrough + route map |

## The route surface

| Route                 | Access                  | Why |
|-----------------------|-------------------------|-----|
| `/health`             | public (implicit)       | K8s probe — never gated |
| `/api/openapi.json`   | public (implicit)       | Platform discovery / workspace-tool surface |
| `/api/v1/public-info` | public (declared)       | Anonymous visitors can read the app posture |
| `/api/v1/me`          | protected               | The validated identity |
| `/api/v1/entries`     | protected               | Per-user data |
| `/api/v1/admin-area`  | protected + role-gated  | Requires the `oidc-admin` app role |

## Layout

```
fullstack-oidc/
├── backend/
│   ├── api/openapi.json          OpenAPI spec (embedded via go:embed)
│   ├── cmd/server/main.go        wires oidcauth into the HTTP server
│   └── internal/
│       ├── oidcauth/             ★ the vendored OIDC middleware
│       ├── config/               deploy-pinned tenant identity
│       ├── database/             PostgreSQL connection + schema
│       ├── handler/              health, openapi, me, admin-area, entries
│       └── middleware/           moses-headers, cors, logging
├── frontend/
│   └── src/
│       ├── auth/                 useAuth hook + silentSSO helper + api client
│       ├── components/Layout.tsx app shell + sidebar nav + auth pill
│       └── pages/                the six demo pages
├── helm/                         multi-service chart (+ oidc Secret)
├── skills/                       agent skill docs
└── moses-app.config.json         declares access.oidc + access.roles
```

## The `oidcauth` package

`backend/internal/oidcauth/` is **vendored** — copy the whole directory
into another template's `backend/internal/oidcauth/` and import it from
that template's own module path. No third-party Go dependencies. See
`skills/oidcauth-middleware.md` for the integration recipe.

## OIDC env contract

The platform injects these into the backend pod (see `helm/values.yaml`
`moses.oidc` and `helm/templates/oidc-secret.yaml`):

| Var | Purpose |
|-----|---------|
| `MOSES_OIDC_ISSUER` | external issuer URL — browser redirects, `iss` |
| `MOSES_OIDC_INTERNAL_ISSUER` | in-cluster issuer — JWKS, token exchange |
| `MOSES_OIDC_CLIENT_ID` | confidential client id |
| `MOSES_OIDC_CLIENT_SECRET` | confidential client secret (K8s Secret) |
| `MOSES_OIDC_AUDIENCE` | expected `aud` (defaults to client id) |
| `MOSES_OIDC_PROTECTED_PATHS` | gated path prefixes |
| `MOSES_OIDC_PUBLIC_PATHS` | always-public path prefixes |
| `MOSES_OIDC_COOKIE_SECRET` | session-cookie HMAC key (K8s Secret) |

The two sensitive values (client secret, cookie HMAC key) come from the
`-oidc` Secret rendered by `helm/templates/oidc-secret.yaml` and are
consumed via `secretKeyRef` — they are never plaintext in the
Deployment manifest.

When the OIDC vars are absent the middleware runs in **pass-through
mode** — public routes and the header-trust path still work; browser
requests are not redirected.

## Config — `moses-app.config.json`

```json
"access": {
  "mode": "tenant",
  "roles": ["oidc-admin", "oidc-member"],
  "oidc": {
    "mode": "moses-oidc",
    "protectedPaths": ["/api/v1/me", "/api/v1/entries", "/api/v1/admin-area"],
    "publicPaths": ["/api/v1/public-info", "/health", "/api/openapi.json"]
  }
}
```

`access.roles` is the role vocabulary a tenant admin maps users into;
Moses projects the mapping into `resource_access.<client>.roles` on the
token. The Helm chart's `moses.oidc.protectedPaths` /
`moses.oidc.publicPaths` mirror the `access.oidc` lists.

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
both images (Kaniko), deploys the multi-service Helm chart, registers a
confidential Keycloak client (because `access.oidc` is declared), and
injects the `MOSES_OIDC_*` env. The app then enforces OIDC with zero
further configuration.
