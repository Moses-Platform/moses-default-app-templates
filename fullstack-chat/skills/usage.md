---
name: fullstack-chat-usage
description: How to start building on the fullstack-chat template — clean out the demo, keep the Moses plumbing, env-var contract
mode: reference
priority: required
---

# Fullstack Chat — Template Usage

This template is the CANONICAL Moses-integration reference: iframe-SDK
platform actions (`window.moses.actions.invoke` → `/__moses/invoke` proxy),
HMAC-verified completion webhooks, host-shell postMessage relays, and
workspace-tool auto-discovery. It ships with a small entries-feed DEMO that
exercises all of that end to end.

## First step: clean out the demo

When you start building a real app, run once from the template root:

```bash
./clean_out_template.sh
```

It deletes the demo feature code, swaps mixed files for clean twins with
guidance comments, then deletes itself and `.template-clean/`. The result
builds and passes the remaining tests
(`go vet ./... && go test ./...` in `backend/`,
`npm run lint && npm test && npm run build` in `frontend/`).

If your clone predates the script, strip manually using this map:

| Action | Path | Why |
|---|---|---|
| KEEP | `backend/cmd/server/main.go` | Boot + `buildMux` plumbing: health/openapi dual-mount, webhook route, `/__moses/invoke` proxy, MOSES_BASE_PATH contract |
| KEEP | `backend/cmd/server/validate_env.go` (+test) | Platform env-contract gate (prod fail-fast, chat_prompt-conditional vars) |
| KEEP | `backend/internal/handler/webhook_chat.go` (+test) | Completion-webhook receiver: HMAC dual-slot verify, nonce replay window, appSlug claim, `CompletionSink` seam |
| KEEP | `backend/internal/handler/respond.go`, `tenant.go` (+`tenant_test.go`) | Shared writeJSON/writeError + CHAT-pxeo.12 tenant gate |
| KEEP | `backend/internal/handler/health.go`, `openapi.go` | Probe + spec serving |
| KEEP | `backend/internal/config/moses.go` | Deploy-pinned tenant identity (`SelfTenantID`) |
| KEEP | `backend/internal/database/db.go` | Connect/retry plumbing (28P01 fail-fast, token-free retry lines) |
| KEEP | `backend/internal/middleware/` (all) | CORS allowlist, CSRF (VENDORED — do not edit), logging, X-Moses headers |
| KEEP | `backend/internal/mosesproxy/` | VENDORED from `shared/mosesproxy-go` — do not edit |
| KEEP | `frontend/src/moses/` (+tests) | Integration reference: `invoke.ts` (SDK guard + error shaping), `hostMessages.ts` (origin-checked completion hook + open-chat announce) |
| KEEP | `frontend/src/api/queryClient.ts`, `frontend/src/main.tsx` | Query provider + browser-logger install (logger file is VENDORED) |
| KEEP | `frontend/src/components/ThemeToggle.tsx`, `src/styles/theme.css`, `src/types/moses.d.ts` | Theme plumbing + SDK ambient types |
| KEEP | `frontend/nginx.conf`, `entrypoint.sh`, both `Dockerfile`s, `helm/`, `vite.config.ts`, tsconfigs | Deploy/path-contract plumbing |
| REPLACE (twin) | `backend/cmd/server/demo_routes.go` | Demo route registration → stub with wiring guidance (keeps `registerDemoRoutes` + `newCompletionSink` signatures) |
| REPLACE (twin) | `backend/cmd/server/main_test.go` | Route contract tests retargeted to the webhook-only surface |
| REPLACE (twin) | `backend/internal/database/migrate_demo.go` | Demo schema → empty `Migrate`/`MigrateTenant` stubs |
| REPLACE (twin) | `backend/api/openapi.json` | Drop `/entries`; keep `servers[]` (base-path-free!) + the webhook path |
| REPLACE (twin) | `frontend/index.html` | Generic title; keeps base-path meta + theme init + SDK `<script>` |
| REPLACE (twin) | `frontend/src/App.tsx`, `App.css` | Minimal placeholder still wiring `src/moses/` |
| REPLACE (twin) | `frontend/src/api/client.ts`, `hooks.ts`, `queryKeys.ts` | Keep `fetchAPI`/`getErrorMessage` + data-layer pattern; drop the entries client/hook/key |
| REPLACE (twin) | `moses-app.config.json` | Keep appData/embedding/docker/helm/dependencies/completionWebhook; demo actions → one guided example |
| DELETE | `backend/internal/handler/entries.go`, `entries_test.go` | Demo feature |
| DELETE | `backend/internal/handler/webhook_sink_demo.go` | Demo Postgres sink (twin wires `LogCompletionSink`) |
| DELETE | `frontend/src/App.test.tsx` | Demo-UI tests (contract coverage lives in `src/moses/moses.test.tsx`, which stays) |

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
