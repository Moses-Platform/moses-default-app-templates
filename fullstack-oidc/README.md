# Moses OIDC Relying-Party Template (`fullstack-oidc`)

**A full-stack app that authenticates real users via OIDC, fronted by
Moses (BFF pattern).**

Go backend + React frontend + PostgreSQL, deployed as a multi-service
Helm chart. The distinctive piece is the **vendored `oidcauth`
middleware** (`backend/internal/oidcauth/`) — a self-contained,
stdlib-only OIDC relying-party implementation.

> **Status (CHAT-t5d1u.28.8)**: this is the template skeleton + a
> complete, unit-tested `oidcauth` package. The polished demo pages and
> walkthrough land in ticket 28.14.

## What this demonstrates

- A deployed Moses app acting as an **OIDC relying party** —
  authorization-code flow + PKCE, confidential client, HttpOnly session
  cookie. The browser never holds a token (BFF pattern).
- **Dual mode**: OIDC enforced on protected paths, while the existing
  `X-Moses-*` trusted-header path keeps working for pod-to-pod MCP /
  workspace-tool calls.
- **Silent SSO**: an already-logged-into-Moses user is authenticated
  with no visible login page (`prompt=none`, hidden iframe).
- Per-user data: the `entries` table is scoped by both the deploy-pinned
  tenant id and the authenticated OIDC subject.

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
│       ├── handler/              health, openapi, me, entries
│       └── middleware/           moses-headers, cors, logging
├── frontend/
│   └── src/auth/                 silentSSO helper + API client
├── helm/                         multi-service chart (+ oidc Secret)
├── skills/                       agent skill docs
└── moses-app.config.json         declares access.oidc
```

## The `oidcauth` package

`backend/internal/oidcauth/` is **vendored** — copy the whole directory
into another template's `backend/internal/oidcauth/` and import it from
that template's own module path. No third-party Go dependencies. See
`skills/oidcauth-middleware.md` for the integration recipe.

## OIDC env contract

The platform injects these into the backend pod (see
`helm/values.yaml` `moses.oidc`):

| Var | Purpose |
|-----|---------|
| `MOSES_OIDC_ISSUER` | external issuer URL — browser redirects, `iss` |
| `MOSES_OIDC_INTERNAL_ISSUER` | in-cluster issuer — JWKS, token exchange |
| `MOSES_OIDC_CLIENT_ID` | confidential client id |
| `MOSES_OIDC_CLIENT_SECRET` | confidential client secret (Secret) |
| `MOSES_OIDC_AUDIENCE` | expected `aud` (defaults to client id) |
| `MOSES_OIDC_PROTECTED_PATHS` | gated path prefixes |
| `MOSES_OIDC_PUBLIC_PATHS` | always-public path prefixes |
| `MOSES_OIDC_COOKIE_SECRET` | session-cookie HMAC key (Secret) |

When the OIDC vars are absent the middleware runs in **pass-through
mode** — public routes and the header-trust path still work; browser
requests are not redirected.

## Build & test

```bash
cd backend  && go build ./... && go test ./...
cd frontend && npm install && npm run build && npm test
```

## SameSite

The session cookie is `SameSite=Lax` — see the rationale in the
`sessionSameSite` doc comment in `backend/internal/oidcauth/session.go`.
