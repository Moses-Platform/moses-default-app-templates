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

## Two paths: chat_prompt vs launch_agent

Under epic [CHAT-89ig](../../moses-platform-prep/.beads/) this template now showcases BOTH Moses platform-action paths so you can compare the contracts side by side:

| Concern | Path A — `chat_prompt` | Path B — `launch_agent` |
|---|---|---|
| Action id (this template) | `generate-entry` | `summarize-feed` |
| What it creates | Visible Moses Manager conversation | One-shot agent pod against a synthetic ticket |
| User experience | Streaming AI reply in the chat sidebar | Background work; user observes via the feed and execution UI |
| Profile (CHAT-btd4) | `ProfileAppInvokedMM` (4 static tools) | `ProfileAppInvokedAgent` (9 static tools) |
| Mode (CHAT-ohlv) | `ModeAppInvokedMM` | `ModeAppInvokedAgent` |
| Workspace-tool surface | Chart-scoped union via `GetMosesManagerToolsForChart` (CHAT-cj8m) | Same chart-scoped union |
| Escape hatch | `moses_discover_tools` | `moses_discover_tools` (NEW — `ProfileAgentExecution` lacked this) |
| OUT of profile | `moses_query`/`moses_create`/`moses_update`/`moses_delete`, `moses_execute_ticket`, `moses_quick_build`, ticket/chart/lane CRUD | Same exclusions as Path A |
| Rate limit (this template) | 5/min, 50/hr | 2/min, 10/hr (tighter; agent pods are heavier) |
| Completion signal | `chatPrompt.completionWebhook` (HMAC-signed POST) + postMessage | Standard ticket completion via `moses_agent_submit_completed` + deployment pipeline |

**When to use which:**

- Use **`chat_prompt`** when you want the user to watch the AI think and
  optionally steer mid-stream. Best for short, conversational tasks
  (generate text, answer a question, draft a one-liner).
- Use **`launch_agent`** when the work is well-defined, multi-step, and
  benefits from being autonomous. Best for "go fetch X, transform it,
  POST it back" tasks where the user does not need to read a transcript.

### The narrow profile contract (CHAT-89ig)

Both paths surface a deliberately small static tool set. The key
discipline: the calling app declared its needs in
`moses-app.config.json`; the user clicked a button, not started a
conversation; therefore the tool surface is scoped to *this app* only.
Tenant-wide CRUD is out of profile by design. The discovery escape
hatch (`moses_discover_tools`) is **symmetric across both paths
(CHAT-ymlz)**:

- For `chat_prompt` (Path A), discovery appends the tool to the
  conversation's `chat_conversations.exposed_extra_tools` overlay
  (CHAT-ci3f Phase 1, schema 885); it becomes callable on the next
  turn. Storage cap: `ProfileMosesManagerFull`.
- For `launch_agent` (Path B), discovery appends the tool to the
  execution's `agent_pod_executions.exposed_extra_tools` overlay
  (CHAT-ymlz, schema 888); it becomes callable on the next turn.
  Storage cap: `ProfileAppInvokedAgent` ∪ `ProfileAgentExecution`
  ∪ `ProfileMosesManagerFull` (wider than chat so legitimate
  agent-only tools are discoverable).

In either path, if a tool stays uncallable AFTER discovery (it lies
outside the storage cap), widen the declared profile in
`moses-app.config.json` and re-deploy. See
`skills/app-invoked-profiles.md` for the per-path instructions.

Sources of truth (read these for the canonical surface):

- `moses-platform-prep/backend/internal/mcp/tools/config.go` —
  `ProfileAppInvokedMM`, `ProfileAppInvokedAgent`, the static tool lists.
- `moses-platform-prep/backend/pkg/prompts/moses_manager.go` —
  `ModeAppInvokedMM`, `ModeAppInvokedAgent`,
  `buildAppInvokedManagerPrompt` (the thin app-invoked system prompt
  body).
- `moses-platform-prep/backend/internal/mcp/tools/workspace_tool_proxy.go`
  — `GetMosesManagerToolsForChart` chart-scoped union (CHAT-cj8m).
- The platform repo's `arch/backend/MCP_SERVER.md` § "App-invoked
  profiles" subsection (CHAT-ieru) — canonical contract.

The agent skill `skills/app-invoked-profiles.md` ships **inside this
template** so any agent launched under either path reads the contract
without inferring it. The companion `skills/chat-roundtrip-overview.md`
is now split into a Section A (user-typed Manager) and a Section B
(app-invoked) that points to the new skill.

### Example: how the Path B agent uses workspace tools

When a user clicks **Summarize feed via narrow agent** with optional
`focus="sentiment"`, the new agent pod's prompt instructs it to:

```
1. GET  /api/v1/entries          (workspace tool — chart-scoped)
2. compose summary line (≤120 chars)
3. POST /api/v1/entries          (workspace tool — chart-scoped)
   body: {"text": "<summary>", "source": "agent"}
4. moses_agent_submit_completed   (lifecycle tool — in profile)
```

The agent never touches `moses_query` or any other tenant-wide tool. If
it wanted to (e.g. to look up tickets), it would have to call
`moses_discover_tools` first. That is the wedge.

## Stack

- **Frontend**: React 18 + Vite, single-page (no router). 7 vitest tests covering postMessage origin checks, status banner transitions, invoke shape, and the `moses_embed_open_chat` post-up envelope.
- **Backend**: Go (stdlib `net/http`), Postgres via `lib/pq`. Handlers: `entries`, `webhook_chat`, `health`, `openapi`. Tests cover HMAC signature verification (including dual-slot rotation acceptance), timestamp clock-skew rejection, fail-closed behavior when no secret is configured, and entries validation.
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

## Webhook secret rotation

The platform supports a 24h overlap window for `app_webhook_secrets` rotation (schema migration `882_app_webhook_secret_rotation.sql`, `secret_previous` + `secret_previous_expires_at`). The platform-side _sender_ is single-slot — once you rotate, every outbound signature uses the new secret immediately. **No-cutover rotation is therefore a contract on the recipient**: during the overlap the recipient must accept signatures from EITHER the active or the previous secret.

This template's `backend/internal/handler/webhook_chat.go` ships with dual-slot verification:

- `MOSES_CHAT_WEBHOOK_SECRET` (required) — the currently active signing secret. Always tried first.
- `MOSES_CHAT_WEBHOOK_SECRET_PREVIOUS` (optional) — fallback that only fires on primary HMAC mismatch. Unset outside an overlap window.

### Platform-deployed apps (CHAT-1j43 / DEPS-A2 — recommended)

The platform's webhook-secret publisher creates a Kubernetes Secret named `moses-webhook-secret-{appSlug}` in the app's namespace at deploy time and mounts both keys (`MOSES_CHAT_WEBHOOK_SECRET` and, during a 24h rotation overlap, `MOSES_CHAT_WEBHOOK_SECRET_PREVIOUS`) into the backend container via the chart's existing `.Values.secrets.secretName` envFrom slot.

**No app redeploy or env-var change is needed during rotation.** The platform's `WebhookSecretRotationService.Rotate` (CHAT-v5al) patches the live K8s Secret with both new and previous values; the kubelet propagates the new env to the running pod within seconds. The recipient picks up the new value automatically.

For the canonical platform-injected runtime env contract, see `moses-deployment-guide/SKILL.md` § *"Platform-injected runtime env contract"*.

### Local development (no Moses)

For local-dev / docker-run setups outside the platform, you set both env vars manually:

```bash
# Initial install
MOSES_CHAT_WEBHOOK_SECRET=$(openssl rand -hex 32) go run ./cmd/server

# Rotation: stage both old + new for 24h
MOSES_CHAT_WEBHOOK_SECRET=$NEW_SECRET \
  MOSES_CHAT_WEBHOOK_SECRET_PREVIOUS=$OLD_SECRET \
  go run ./cmd/server

# After 24h, drop _PREVIOUS
MOSES_CHAT_WEBHOOK_SECRET=$NEW_SECRET go run ./cmd/server
```

### Off-cluster recipients

If your `completionWebhook.url` points at an HTTPS URL outside the cluster (rare), the K8s Secret bridge does NOT reach you. Operators recover the secret from the install-time audit log (see `moses-deployment-guide/SKILL.md` § *"Off-cluster webhook recipients"*).

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
- **CHAT-89ig** (parent epic) — app-invoked prompt-profile architecture. The `summarize-feed` action exercises `ProfileAppInvokedAgent`; the `generate-entry` action now flows through `ProfileAppInvokedMM`. Children: CHAT-btd4 (profiles), CHAT-ohlv (modes + thin prompt), CHAT-iq3i (chat wiring), CHAT-6q99 (agent wiring), CHAT-cj8m (chart-scoped workspace tools), CHAT-3h8z (symmetry test), CHAT-ieru (docs), **CHAT-9oqo (this template wave)**.

## Local development

The template assumes deployment via Moses' Helm pipeline. For pure local dev (no Moses), run the backend and frontend independently with a local Postgres; the chat-action flow won't fire (Moses is the dispatcher), but you can exercise the entries CRUD and the webhook receiver via curl.

**CHAT-pxeo.12 — tenant identity from env, not header.** The backend now reads its self-tenant identifier from `MOSES_TENANT_ID` (via `internal/config.SelfTenantID()`), NOT from the `X-Moses-Tenant-ID` request header. The header is preserved as caller context (audit / cross-check), but storage and lookup keys come from the deploy-pinned env. On a deployed pod (`MOSES_DEPLOYED=1`) the server fail-fast panics if `MOSES_TENANT_ID` is unset; in local dev it falls back to the sentinel `local-dev`. A 403 with `{"error":"tenant_mismatch","code":"E_TENANT_MISMATCH"}` is returned when a request supplies a non-empty `X-Moses-Tenant-ID` that disagrees with the deploy-pinned value. Toggle the cross-check via `MOSES_STRICT_TENANT_CHECK=false` if you need to debug.

```bash
# Backend with local Postgres
# MOSES_TENANT_ID is optional in local dev — omit and the sentinel
# 'local-dev' is used. Set it to mirror a real deploy.
DB_HOST=localhost DB_PORT=5432 DB_NAME=fullstack_chat \
  DB_USER=postgres DB_PASSWORD=postgres \
  MOSES_CHAT_WEBHOOK_SECRET=dev-secret \
  MOSES_TENANT_ID=local-dev \
  go run ./cmd/server

# Frontend with Vite proxy
cd frontend && npm run dev
```

## License

MIT — see `../LICENSE`.
