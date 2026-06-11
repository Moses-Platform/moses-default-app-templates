# Moses OIDC Relying-Party Template — Agent Skill

## Overview

`fullstack-oidc` is the **canonical reference app for app-owned OIDC**.
When a deployed app needs real per-user authentication (not just the
trusted `X-Moses-*` header path), start from this template — it
demonstrates the whole pattern end-to-end and the moving parts are meant
to be copied.

- **Frontend**: React 19 + TypeScript + Vite, served by nginx. A
  six-page SPA where every page makes one part of the OIDC integration
  visible.
- **Backend**: Go stdlib HTTP server with the **vendored `oidcauth`
  middleware** in `backend/internal/oidcauth/`.
- **Database**: PostgreSQL — a per-user `entries` table scoped by both
  the deploy-pinned tenant id and the authenticated OIDC subject.

## The app-owned-OIDC chain

App-owned OIDC is a four-link chain. The template demonstrates each
link, and the **How It Works** page in the running app narrates it:

1. **Declare** — the app adds an `access.oidc` block and an
   `access.roles` list to `moses-app.config.json`. That declaration is
   the *only* thing the app author writes to opt in.
2. **Inject** — at deploy time the Moses platform sees `access.oidc`,
   registers a confidential Keycloak client for the app, and injects the
   credentials as `MOSES_OIDC_*` env vars plus a K8s Secret (client
   secret + cookie HMAC key). The app never stores or ships secrets.
3. **Enforce** — the vendored `oidcauth` middleware reads that env
   (`oidcauth.ConfigFromEnv()`), wraps the mux, and runs the
   authorization-code + PKCE BFF flow. Protected routes with no session
   get redirected to Keycloak; the middleware validates the ID token
   against JWKS and sets an HttpOnly session cookie.
4. **Read** — handlers call `oidcauth.IdentityFrom(r.Context())` to get
   the subject/email/name and the roles projected from
   `resource_access.<client>.roles`. Authorization uses
   `Identity.HasRole(...)`.

## The demo route surface

The route split is the whole demo — a viewer can SEE the integration:

| Route                 | Access                | Demonstrates |
|-----------------------|-----------------------|--------------|
| `/health`             | public (implicit)     | K8s probe, never gated |
| `/api/openapi.json`   | public (implicit)     | Platform discovery / workspace-tool surface |
| `/api/v1/public-info` | public (declared)     | Anonymous visitors can read app posture |
| `/api/v1/me`          | protected             | The validated identity projection |
| `/api/v1/entries`     | protected             | Per-user data scoped to the OIDC subject |
| `/api/v1/admin-area`  | protected + role-gated| `oidc-admin` role enforced via `HasRole` |

The contrast between `public-info` and the gated routes — and between a
plain protected route and the role-gated one — is what makes the OIDC
integration legible.

## How authentication works (BFF pattern)

1. A browser request to a protected path with no session is redirected
   to Keycloak (authorization-code flow + PKCE).
2. Keycloak redirects back to `/auth/callback`; the backend exchanges
   the code (confidential client, in-cluster token endpoint), validates
   the ID token against JWKS, and sets an **HttpOnly, Secure,
   SameSite=Lax** session cookie. The browser never sees a token. The
   redirect_uri and session cookie are built for whichever registered
   host the browser used, so login completes on the apex custom hostname
   and the platform sub-path alike.
3. Per request the middleware validates the session cookie and places an
   `Identity` on the request context.

## Authentication vs authorization

The **Roles & Access** page makes the distinction concrete:
authentication answers *who are you* (the session); authorization
answers *what may you do* (the projected roles). `/api/v1/admin-area`
requires the `oidc-admin` role — a signed-in user without it gets a
clean 403, not a 401.

## Dual mode — the header path still works

The `oidcauth` middleware preserves the existing `X-Moses-*` trusted
header path: a pod-to-pod call carrying `X-Moses-User-ID` (set by the
platform proxy on the in-cluster hop) bypasses OIDC entirely. This keeps
the OpenAPI workspace-tool surface working without an OIDC session.
The header path carries **no app roles** — pod-to-pod callers are
authorized by the platform, so `/api/v1/admin-area` correctly 403s them.

## Graceful degradation

When the `MOSES_OIDC_*` env vars are absent (a standalone `helm
install`, or a deploy where the platform did not inject OIDC), the
middleware runs in **pass-through mode**: public routes and the
header-trust path still work, browser requests are NOT redirected. A
misconfigured deploy degrades visibly instead of hard-500ing.

## Silent SSO

When embedded in the Moses Manager iframe, an already-logged-into-Moses
user is authenticated with **no visible login page** via a `prompt=none`
flow. The frontend helper `frontend/src/auth/silentSSO.ts` drives a
hidden iframe against `/auth/silent-check`; the **Silent SSO** page lets
a viewer trigger and watch it.

## Config

The app declares its intent in `moses-app.config.json`:

- `access.oidc` — `mode: "moses-oidc"`, plus `protectedPaths` /
  `publicPaths` (`/health` and the OpenAPI spec are public implicitly).
- `access.roles` — the role vocabulary (`oidc-admin`, `oidc-member`) a
  tenant admin maps real users into. Only mapped roles ever land on a
  token.

The Helm chart turns those into the `MOSES_OIDC_*` env vars the
middleware reads (`helm/values.yaml` `moses.oidc`).

## Routing contract

Standard Moses deployed-app path contract: the API is registered once
under `MOSES_BASE_PATH`; `/health` + `/api/openapi.json` are also at the
canonical root. The `/auth/*` handshake routes are served by the Go
backend (the nginx config proxies `/auth/` to the backend container).
