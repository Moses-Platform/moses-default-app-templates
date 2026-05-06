# Fullstack Chat Roundtrip

**Reference template that exercises every chat surface in the app↔Moses-Manager roundtrip.**

A minimal Postgres-backed feed app whose sole purpose is to be a smoke-test for the `chat_prompt` platform action, the workspace-tools wedge, the completion webhook, the host-shell postMessage relays, and the per-app data git repo. Use it as the end-to-end verification artifact when validating any change under epic [CHAT-gfcp](../../moses-platform-prep/.beads/) (App→MM round-trip completeness).

## What the demo does

1. User clicks **Generate entry via Moses Manager** in the embedded app.
2. App POSTs to `/apps/fullstack-chat/actions/generate-entry/invoke` on the Moses backend.
3. Moses creates a new visible conversation, fires the AI with the rendered prompt.
4. AI calls the workspace-tool `POST /api/v1/entries` (auto-discovered from the app's OpenAPI).
5. App's backend writes the entry to Postgres and returns 201.
6. AI replies in the chat sidebar; conversation is visible immediately (`conversation_created` WS event).
7. Moses fires the **completion webhook** at `/api/v1/webhooks/chat-complete` with HMAC-SHA256 signature.
8. App receives a `moses_embed_chat_complete` postMessage in the host iframe; refreshes the feed.
9. New entry appears in the feed; status banner shows "Moses Manager completed (stop)".

See [`skills/chat-roundtrip-overview.md`](skills/chat-roundtrip-overview.md) for the full architectural walkthrough and the manual end-to-end test recipe.

## Stack

- **Frontend**: React 18 + Vite, single-page (no router). 6 vitest tests covering postMessage origin checks, status banner transitions, invoke shape.
- **Backend**: Go (stdlib `net/http`), Postgres via `lib/pq`. Handlers: `entries`, `webhook_chat`, `health`, `openapi`. Tests cover HMAC signature verification, timestamp clock-skew rejection, and entries validation.
- **Helm**: multi-service chart (frontend + backend), Postgres declared in `dependencies.services`.
- **OpenAPI** at `/api/openapi.json` declares the entries CRUD + webhook receiver so Moses' `WorkspaceToolService.discoverAndRegisterEndpoints` picks it up after deploy.

## Build verification

```bash
# Frontend
cd frontend
npm install --no-audit --no-fund
npm run lint        # tsc strict mode
npm test            # vitest
npm run build       # tsc + vite

# Backend
cd backend
go vet ./...
go test ./...
go build ./...
```

All four `validation.commands` in `moses-app.config.json` are required and run by Moses' submit-completed gate.

## Configuration knobs

| Field in `moses-app.config.json` | Effect |
|---|---|
| `platformActions[0].chatPrompt.completionWebhook.url` | Where Moses POSTs the AI completion. Internal cluster service URL only. Default: `http://fullstack-chat-backend:8080/api/v1/webhooks/chat-complete`. |
| `platformActions[0].chatPrompt.variableEscapeMode` | `raw` / `fenced` / `stripped`. Default `fenced` here because `topic` is `userSupplied: true`. |
| `platformActions[0].rateLimit.perMinute` | Per-action cap. Subject to platform-floor (1/min, 5/hr) and platform-ceiling (10/min, 100/hr). |
| `appData.enabled` | When true, Moses provisions `{tenantID}/app-data/fullstack-chat/` git repo. MM reads via `moses_read_file`. |
| `appData.manager.access` | `read` (default for this template) / `none`. Whether MM sees the repo in `moses_get_repositories`. |

## Webhook secret rotation (read this before rotating)

The platform supports a 24h overlap window for `app_webhook_secrets` rotation (schema migration `882_app_webhook_secret_rotation.sql`, `secret_previous` + `secret_previous_expires_at`). The platform-side _sender_ is single-slot — once you rotate, every outbound signature uses the new secret immediately. **No-cutover rotation is therefore a contract on the recipient**: during the overlap the recipient must accept signatures from EITHER the active or the previous secret.

> ⚠️ **The reference template only verifies the active secret** (`backend/internal/handler/webhook_chat.go` reads `MOSES_CHAT_WEBHOOK_SECRET` only). If you rotate without first extending this verifier, every webhook delivery will be rejected until you redeploy with the new value. To support no-cutover rotation, extend the verifier to also read `MOSES_CHAT_WEBHOOK_SECRET_PREVIOUS` and fall back to it on primary HMAC mismatch — see the recipient cookbook in `moses-deployment-guide/SKILL.md` for the full flow (Python and Go variants).

## Relationship to platform-prep beads

This template targets the implemented surface of:

- **CHAT-jayp** — sidebar visibility for the auto-fired conversation.
- **CHAT-p1nr** — `conversation_created` WebSocket event for live update.
- **CHAT-9pz5 / CHAT-h8cm** — chat_prompt rate floor + concurrency cap (testable via repeated clicks).
- **CHAT-rg5t** — completion webhook (HMAC-verified in this app's `webhook_chat.go`).
- **CHAT-l7va** — `moses_embed_open_chat` postMessage to host (tested via App.test.tsx).
- **CHAT-avoi** — automatic action registration via `PlatformRegistrationProvisioner.Reconcile`.
- **CHAT-qrd6** — `appData` block to surface the per-app git repo to MM and agents.
- **CHAT-uwlm** — `variableEscapeMode: "fenced"` + `userSupplied: true` flag for prompt-injection hardening.
- **CHAT-xu9i** — exercises the `finishReason: "credential_unset"` path when AI is unconfigured.

## Local development

The template assumes deployment via Moses' Helm pipeline. For pure local dev (no Moses), run the backend and frontend independently with a local Postgres; the chat-action flow won't fire (Moses is the dispatcher), but you can exercise the entries CRUD and the webhook receiver via curl.

```bash
# Backend with local Postgres
DB_HOST=localhost DB_PORT=5432 DB_NAME=fullstack_chat \
  DB_USER=postgres DB_PASSWORD=postgres \
  MOSES_CHAT_WEBHOOK_SECRET=dev-secret \
  go run ./cmd/server

# Frontend with Vite proxy
cd frontend && npm run dev
```

## License

MIT — see `../LICENSE`.
