---
name: chat-roundtrip-overview
description: Reference walkthrough of the app↔Moses-Manager chat roundtrip exercised by this template
mode: reference
priority: optional
---

# Fullstack Chat Roundtrip — Architecture

A reference template that exercises every chat surface in the app↔Moses-Manager round-trip. Use this when designing apps that:

- Fire button-driven `chat_prompt` platform actions to Moses Manager.
- Have Moses Manager call back into the app via the workspace-tools wedge (auto-discovered OpenAPI).
- Want a server-side completion signal (HMAC-signed webhook).
- Embed in the host shell with sidebar-open / completion postMessage relays.
- Use the per-app data git repo (CHAT-qrd6) for structured state visible to MM.

## Roundtrip flow

```
Frontend                 Moses backend                MM AI runtime              This app's backend
────────                 ─────────────                ─────────────              ──────────────────
[click "Generate"] ───►  POST /apps/fullstack-chat/
                         actions/generate-entry/invoke
                         (user JWT, body: {variables: {topic}})
                                  │
                                  ▼
                         dispatchChatPrompt:
                          - render messageTemplate
                            (variables fenced via
                             variableEscapeMode)
                          - PromptNewConversation()
                          - source="app_action"
                                  │       ◄── 200 {result:{conversationId}}
                                  ▼
                                            MM auto-response triggered
                                            (TriggerAutoResponse, fire-and-forget)
                                                       │
                                                       ▼
                                            workspace-tool MCP call:
                                            POST /api/v1/entries
                                            {text:"…", source:"moses_manager"}
                                                       │       ────────────────►  POST /api/v1/entries
                                                       │                          insert into entries(...)
                                                       │       ◄──────────────────  201 {id, text, ...}
                                                       │
                                            assistant turn ends
                                                       │
                                  ┌────────────────────┘
                                  │
                                  ▼
                         BroadcastAutoResponseReady (WS)
                         + outbound webhook delivery
                          (HMAC-SHA256 in X-Moses-Signature)            POST /api/v1/webhooks/chat-complete
                                                                         (verify HMAC, persist row)

[poll /api/v1/entries every 2s]                                                 GET /api/v1/entries
                                                                         (returns new row)

[receive moses_embed_chat_complete postMessage]
   │
   ▼
(refresh feed immediately, show completion banner)
```

## Files at a glance

| Concern | Where |
|---|---|
| Action declaration | `moses-app.config.json` → `platformActions[].chat_prompt` |
| Variable hardening | `chatPrompt.variableEscapeMode: "fenced"` + `variables[].userSupplied: true` |
| Per-app rate cap | `chatPrompt.rateLimit: { perMinute, perHour }` (subject to platform floor + ceiling) |
| Webhook target | `chatPrompt.completionWebhook.url` (in-cluster service URL only) |
| App-data exposure | `appData.enabled: true`, default `manager.access: read`, `agent.access: read` |
| Workspace-tool registration | `apiEndpoints.specPath: /api/openapi.json` (auto-discovered) |
| MM call-back endpoint | `backend/internal/handler/entries.go` (POST /api/v1/entries) |
| Webhook receiver | `backend/internal/handler/webhook_chat.go` (HMAC + 5-min skew window) |
| Frontend invoke | `frontend/src/App.tsx` `invokeChatPrompt` (uses `VITE_MOSES_API_BASE`) |
| Frontend listener | `frontend/src/App.tsx` `onMessage` (origin-checked postMessage) |

## Key constraints

1. **Relative vs absolute paths.** Calls to THIS app's backend use relative paths (`fetch('api/v1/entries')`); calls to MOSES backend use absolute prefixed by `VITE_MOSES_API_BASE` (`${VITE_MOSES_API_BASE}/apps/.../invoke`). Confusing them produces 404s in production.

2. **postMessage origin check.** Always compare `e.origin === window.location.origin`. The host iframe shell deploys apps under a Moses subpath, so iframe and host are same-origin. Cross-origin custom-domain apps need an origin allowlist (future work).

3. **Variable trust.** Topic is `userSupplied: true`. The template renders it inside `<topic>...</topic>` fences via `variableEscapeMode: "fenced"` so MM treats it as content, not instructions. If you adapt this template for higher-trust contexts, you can drop the fencing — but never feed unaudited user input into a `raw`-mode template.

4. **Webhook secret.** The `MOSES_CHAT_WEBHOOK_SECRET` env var is the shared HMAC key. Moses provisions it at install (rotated via the admin endpoint). Recipients ignore unsigned or stale-timestamp payloads.

5. **Webhook secret rotation is recipient-driven.** Schema-882 supports a 24h overlap window (`secret_previous` + `secret_previous_expires_at`), but the platform sender is single-slot — every outbound signature uses the active secret. To get true no-cutover rotation the recipient must verify against BOTH secrets during the overlap: try `MOSES_CHAT_WEBHOOK_SECRET` first, fall back to `MOSES_CHAT_WEBHOOK_SECRET_PREVIOUS` (optional, empty outside an overlap) on mismatch, reject if both fail. **This template's `webhook_chat.go` already implements dual-slot verification** — set `MOSES_CHAT_WEBHOOK_SECRET_PREVIOUS` during the overlap window and unset it once rotation is complete. See `moses-deployment-guide/SKILL.md` for the full recipient cookbook.

## Manual end-to-end test recipe

After the platform implementation lands and this template deploys:

1. Deploy this app (e.g. via marketplace install or `moses_init_repo --template fullstack-chat`).
2. Open it in the Moses UI.
3. Type a topic in the input ("octopus").
4. Click "Generate entry via Moses Manager".
5. Observe:
   - Status banner: "Invoking…" → "Awaiting response (conv-…)" → "Moses Manager completed (stop)".
   - Sidebar: a new conversation titled "New feed entry" appears. Its body shows the rendered prompt with the topic fenced.
   - Feed: a new entry with `source: moses_manager` appears within ~2s of completion.
6. Verify in DB (admin only): `SELECT * FROM chat_completions ORDER BY received_at DESC LIMIT 1;` shows `signature_valid = true`.
7. Repeat 6 times within a minute → 6th invoke returns 429 (`rate_limited`) due to the per-action cap.
8. Open two browser tabs, click in both within 1 second → second returns 429 (`chat_prompt_concurrency_limit`) with the in-flight conversationId.

## Negative scenarios this template helps verify

- **Sidebar visibility (CHAT-jayp)**: the auto-fired conversation MUST appear in the sidebar.
- **Real-time WS push (CHAT-p1nr)**: the conversation appears within milliseconds, not the 15s poll wait.
- **Rate floor (CHAT-9pz5)**: rapid clicks return 429 before the per-action limit if app omits its own.
- **Concurrency cap (CHAT-h8cm)**: parallel clicks return 429 with `inflightConversationId` field.
- **Completion signal (CHAT-rg5t)**: webhook recipient receives signed payload; postMessage reaches iframe.
- **Open-chat postMessage (CHAT-l7va)**: host shell opens sidebar to the new conversation.
- **Provisioner reconcile (CHAT-avoi)**: action row appears in `tenant_app_platform_actions` post-deploy without imperative MCP call.
- **Prompt-injection hardening (CHAT-uwlm)**: topic value is fenced; MM does not interpret it as instructions.
- **MM credentials unset (CHAT-xu9i)**: with no AI subscription pinned, the conversation surfaces a clear remediation status message.
- **AppData repo (CHAT-qrd6)**: `moses_get_repositories` lists `app-data/fullstack-chat` (read-only by default to MM and agents).

Use this template as a smoke-test artifact during integration; deploy it, run the recipe above, and any divergence indicates a regression in the corresponding bead.
