# Moses OIDC Relying-Party Template — Agent Skill

## Overview

`fullstack-oidc` is a reference full-stack app that acts as an **OIDC
relying party** fronted by the Moses platform, using the BFF (Backend
For Frontend) pattern. It is the template to start from when a deployed
app needs real per-user authentication rather than the trusted
`X-Moses-*` header path alone.

- **Frontend**: React 19 + TypeScript + Vite, served by nginx.
- **Backend**: Go stdlib HTTP server with the **vendored `oidcauth`
  middleware** in `backend/internal/oidcauth/`.
- **Database**: PostgreSQL — a per-user `entries` table scoped by both
  the deploy-pinned tenant id and the authenticated OIDC subject.

This is the CHAT-t5d1u.28.8 deliverable: a working skeleton plus a
complete, unit-tested `oidcauth` package. The polished demo pages land
in ticket 28.14.

## How authentication works (BFF pattern)

1. The platform registers a confidential Keycloak client for the app
   and injects five env vars into the backend pod:
   `MOSES_OIDC_ISSUER`, `MOSES_OIDC_INTERNAL_ISSUER`,
   `MOSES_OIDC_CLIENT_ID`, `MOSES_OIDC_CLIENT_SECRET`,
   `MOSES_OIDC_AUDIENCE`.
2. A browser request to a protected path with no session is redirected
   to Keycloak (authorization-code flow + PKCE).
3. Keycloak redirects back to `/auth/callback`; the backend exchanges
   the code (confidential client, in-cluster token endpoint), validates
   the ID token against JWKS, and sets an **HttpOnly, Secure,
   SameSite=Lax** session cookie. The browser never sees a token.
4. Per-request the middleware validates the session cookie.

## Dual mode — the header path still works

The `oidcauth` middleware preserves the existing `X-Moses-*` trusted
header path: a pod-to-pod call carrying `X-Moses-User-ID` (set by the
platform proxy on the in-cluster hop) bypasses OIDC entirely. This keeps
the OpenAPI workspace-tool surface working without an OIDC session.
`/health` and the OpenAPI spec path are always public.

## Silent SSO

When embedded in the Moses Manager iframe, an already-logged-into-Moses
user is authenticated with **no visible login page** via a `prompt=none`
flow. The frontend helper `frontend/src/auth/silentSSO.ts` drives a
hidden iframe against `/auth/silent-check`.

## Config

The app declares its intent in `moses-app.config.json` under
`access.oidc` (`mode: "moses-oidc"`, `protectedPaths`, `publicPaths`).
The Helm chart turns those into the `MOSES_OIDC_*` env vars the
middleware reads.

## Routing contract

Standard Moses deployed-app path contract: API registered once under
`MOSES_BASE_PATH`; `/health` + `/api/openapi.json` also at the canonical
root. The `/auth/*` handshake routes are served by the Go backend (the
nginx config proxies `/auth/` to the backend container).
