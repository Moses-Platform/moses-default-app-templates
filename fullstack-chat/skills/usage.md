---
name: fullstack-chat-usage
description: How to start building on the fullstack-chat template — what ships, the Moses plumbing, env-var contract
mode: reference
priority: required
---

# Fullstack Chat — Template Usage

This template is the CANONICAL Moses-integration reference: iframe-SDK
platform actions (`window.moses.actions.invoke` → `/__moses/invoke` proxy),
HMAC-verified completion webhooks, host-shell postMessage relays, and
workspace-tool auto-discovery.

## What you start with

The template is demo-free: every integration surface above is wired, with an
empty place to put your own feature. A fresh clone builds and passes its tests
(`go vet ./... && go test ./...` in `backend/`,
`npm run lint && npm test && npm run build` in `frontend/`).

Backend:

- `cmd/server/main.go` — boot + `buildMux` plumbing: health/openapi
  dual-mount, webhook route, `/__moses/invoke` proxy, `MOSES_BASE_PATH`
  contract.
- `cmd/server/demo_routes.go` — stub with wiring guidance; keeps the
  `registerDemoRoutes` + `newCompletionSink` signatures (the latter wires
  `LogCompletionSink`). Add your routes here.
- `cmd/server/validate_env.go` (+test) — platform env-contract gate (prod
  fail-fast, `chat_prompt`-conditional vars).
- `cmd/server/main_test.go` — route contract tests over the webhook-only
  surface.
- `internal/handler/webhook_chat.go` (+test) — completion-webhook receiver:
  HMAC dual-slot verify, nonce replay window, appSlug claim, `CompletionSink`
  seam.
- `internal/handler/respond.go`, `tenant.go` (+`tenant_test.go`) — shared
  `writeJSON`/`writeError` + the CHAT-pxeo.12 tenant gate.
- `internal/handler/health.go`, `openapi.go` — probe + spec serving.
- `internal/config/moses.go` — deploy-pinned tenant identity (`SelfTenantID`).
- `internal/database/db.go` — connect/retry plumbing (28P01 fail-fast,
  token-free retry lines); `migrate_demo.go` — empty `Migrate`/`MigrateTenant`
  stubs for your schema.
- `internal/middleware/` — CORS allowlist, CSRF (VENDORED — do not edit),
  logging, X-Moses headers.
- `internal/mosesproxy/` — VENDORED from `shared/mosesproxy-go`; do not edit.
- `api/openapi.json` — keeps `servers[]` (base-path-free!) + the webhook path.

Frontend:

- `src/moses/` (+tests) — the integration reference: `invoke.ts` (SDK guard +
  error shaping) and `hostMessages.ts` (origin-checked completion hook +
  open-chat announce). `src/moses/moses.test.tsx` is the frontend's only test
  file and carries the contract coverage.
- `src/api/queryClient.ts` + `client.ts` / `hooks.ts` / `queryKeys.ts` —
  `fetchAPI`/`getErrorMessage` and the data-layer pattern; `src/main.tsx` —
  query provider + browser-logger install (logger file is VENDORED).
- `src/App.tsx`, `App.css` — minimal placeholder that still wires
  `src/moses/`; `index.html` keeps the base-path meta, theme init and the SDK
  `<script>`.
- `src/components/ThemeToggle.tsx`, `src/styles/theme.css`,
  `src/types/moses.d.ts` — theme plumbing + SDK ambient types.

Deploy: `frontend/nginx.conf`, `entrypoint.sh`, both `Dockerfile`s, `helm/`,
`vite.config.ts`, the tsconfigs, and `moses-app.config.json` (appData /
embedding / docker / helm / dependencies). The shipped config's
`platformActions` is EMPTY — the guided example actions (a `chat_prompt` with
completion webhook and a scoped `launch_agent`) live in
`moses-app.config.with-secrets.example.json`; copy one across when you wire
the feedback loop.

Note: there is deliberately NO `frontend/src/utils/baseUrl.ts` here (other
templates have one). This app has no client-side router — it is a single
page, and all backend calls are relative (`api/v1/...`), so no basename /
URL-join helper is needed. Copy it from `fullstack-showcase` only if you
add react-router.

## Env-var contract (what the backend reads)

| Var | Required | Read in | Purpose |
|---|---|---|---|
| `MOSES_TENANT_ID` | prod-fatal | `internal/config/moses.go` | Deploy-pinned tenant; ALL storage keys (never trust the header) |
| `MOSES_CHART_ID` | prod-fatal | `validate_env.go`, `mosesproxy` | Chart scoping; folded into invoke body |
| `MOSES_APP_SLUG` | prod-fatal | `validate_env.go`, `mosesproxy`, `webhook_chat.go` | Proxy forward path + webhook appSlug claim check |
| `MOSES_CHAT_WEBHOOK_SECRET` | prod-fatal *when a `chat_prompt` action is declared* (CHAT-ct5q) | `webhook_chat.go` | Active HMAC key for the completion webhook |
| `MOSES_CHAT_WEBHOOK_SECRET_PREVIOUS` | optional | `webhook_chat.go` | Rotation-overlap fallback key (CHAT-v5al) |
| `MOSES_INTERNAL_API_BASE` | prod-fatal *when a `chat_prompt` action is declared* | `mosesproxy/proxy.go` | Cluster-DNS moses-backend base the `/__moses/invoke` proxy forwards to |
| `MOSES_API_BASE` | NOT required | — | Injected by the provisioner when a public URL exists; nothing here consumes it |
| `MOSES_DEPLOYED` | set by platform | `validate_env.go`, `config` | `=1` flips missing-env warnings to fail-fast |
| `MOSES_BASE_PATH` (alias `BASE_URL`, deprecated) | optional | `main.go`, `entrypoint.sh` | Sub-path mount (`/apps/<tenant>/<slug>`); API registers ONCE under it |
| `MOSES_STRICT_TENANT_CHECK` | optional (default on) | `handler/tenant.go` | Disable the 403 header-vs-env cross-check (hot-fix only) |
| `MOSES_APP_CONFIG_PATH` | optional | `validate_env.go` | Override config discovery for the chat_prompt gate |
| `MOSES_EMBEDDING_FRAMING` / `_ALLOWED_ANCESTORS` / `_REPORT_URI`, `MOSES_DOMAIN` | optional | `frontend/entrypoint.sh` | CSP frame-ancestors rendering |
| `CORS_ALLOWED_ORIGINS` | optional (default: no CORS) | `middleware/cors.go` | Comma-separated exact origins; `*` allowed but never with credentials |
| `DB_HOST`/`DB_PORT`/`DB_USER`/`DB_PASSWORD`/`DB_NAME`/`DB_SSLMODE` | injected with the `postgresql` dependency | `database/db.go` | Postgres connection (defaults: `app`/`appdb` per platform convention) |
| `PORT` | optional (default 8080) | `main.go` | Listen port |
| `BACKEND_SERVICE_HOST`/`_PORT` | injected (frontend pod) | `frontend/entrypoint.sh` | nginx → backend proxy target |
| `NODE_ENV` | chart-set | — | Conventional; no longer drives CORS |

## Validation commands

```bash
cd backend  && go vet ./... && go test ./...
cd frontend && npm install && npm run lint && npm test && npm run build
```

These are the same four commands Moses' submit-completed gate runs
(`moses-app.config.json` → `validation.commands`).

## Where the integration reference lives

- Roundtrip architecture + flow diagram: `skills/chat-roundtrip-overview.md`
- Narrow app-invoked tool profiles (chat_prompt / launch_agent):
  `skills/app-invoked-profiles.md`
- Declaring runtime secrets: `skills/secrets-tutorial.md`
