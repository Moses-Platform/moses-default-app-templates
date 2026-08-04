---
name: chat-roundtrip-overview
description: Reference walkthrough of the app↔Moses-Manager chat roundtrip exercised by this template
mode: reference
priority: optional
---

# Fullstack Chat Roundtrip — Architecture

> **Starting a real app on this template?** Read `skills/usage.md` first — it
> covers what ships in the template and the full env-var contract
> (MOSES_*, DB_*, PORT).

A reference template that exercises every chat surface in the app↔Moses-Manager round-trip. Use this when designing apps that:

- Fire button-driven `chat_prompt` platform actions to Moses Manager.
- Have Moses Manager call back into the app via the workspace-tools wedge (auto-discovered OpenAPI).
- Want a server-side completion signal (HMAC-signed webhook).
- Embed in the host shell with sidebar-open / completion postMessage relays.
- Use the per-app data git repo (CHAT-qrd6) for structured state visible to MM.

## Two paths from app to AI: which one am I reading about?

Moses offers TWO platform-action paths under epic **CHAT-89ig**. The shipped
`moses-app.config.json` deliberately declares NO actions
(`platformActions: []`) — the guided examples live in
`moses-app.config.with-secrets.example.json`: `ask-moses` (a `chat_prompt`)
and `analyze-app` (a `launch_agent`). Copy one across to activate the path
you need.

- **`chat_prompt`** (Path A) — opens an interactive Moses Manager
  conversation. This is what the `ask-moses` example does, and the focus of
  THIS file.
- **`launch_agent`** (Path B) — launches a one-shot agent pod against a
  synthetic ticket. The agent runs to completion without user
  back-and-forth.

The two paths share one critical contract (CHAT-89ig): when a Moses chat or
agent is created from an app action (`source == "app_action"`), the tool
surface is **narrowed** to the app's own workspace tools + a tiny baseline
(see profile lists below). This is **not** the same as a user-typed
Manager chat or a Basar / autopilot agent.

> **If you are an AI reading this skill from inside an app-invoked chat or
> agent pod, jump to `app-invoked-profiles.md` for your tool-surface
> contract.** This file's Section A still applies for the *user-typed*
> Manager conversation that may end up in this app's context.

The rest of this file is split into:

- **Section A — User-typed Manager → app context (today's full Manager
  behaviour).** What you get when a human types into the chat sidebar
  while the fullstack-chat app is the active context. No profile
  narrowing.
- **Section B — App-invoked (`chat_prompt` Path A and `launch_agent`
  Path B).** The narrow contract; full body lives in
  `app-invoked-profiles.md`.

---

## Section A — User-typed Manager → app context (full surface, today)

When a user types into the Moses Manager sidebar with the fullstack-chat
app as the active context, the chat conversation is created with
`Source != "app_action"` and resolves to one of the **full** Manager
profiles via `getChatDefaultProfile()`:

- `ProfileMosesManagerEssentials` (default) — essentials + per-conversation
  `exposed_extra_tools` overlay.
- `ProfileMosesManagerAutonomous` (auto-upgrade when the user has an
  active autopilot session).
- `ProfileMosesManagerFull` (CRUD-heavy direct project-management chat).

Available tools include the full unified-CRUD surface (`moses_query`,
`moses_create`, `moses_update`, `moses_delete`, `moses_batch`),
ticket/chart/lane workflow tools, skills tools, deployment pipeline tools,
agent-execution tools, plus this app's auto-discovered workspace tools
(one per `operationId` you declare in `backend/api/openapi.json`) — same
Path A WorkspaceToolProxy injection, no profile narrowing.

This is the legacy / full path. **Keep using it for free-form workspace
work, project management, and anything that genuinely needs tenant CRUD.**

---

## Section B — App-invoked profile (`chat_prompt` and `launch_agent`)

When an app fires either platform action, the resulting Moses Manager
conversation (Path A) or agent pod (Path B) gets a **narrow** tool
surface:

- **Path A** — `ProfileAppInvokedMM`: 4 static tools
  (`moses_get_session_info`, `moses_discover_tools`,
  `moses_global_search`, `moses_get_app_logs`) plus chart-scoped
  workspace tools auto-injected at `tools/list` time by
  `WorkspaceToolProxy.GetMosesManagerToolsForChart` (CHAT-cj8m).
- **Path B** — `ProfileAppInvokedAgent`: 9 static tools
  (`moses_get_session_info`, `moses_discover_tools`, `moses_read_file`,
  `moses_list_files`, `moses_notify_push`, `moses_agent_request_build`,
  `moses_agent_submit_completed`, `moses_agent_report_failure`,
  `moses_await_deployment`) plus the same chart-scoped workspace tool
  union.
- **Discovery escape hatch — symmetric between the two paths (CHAT-ymlz)**:
  - Path A (chat) — calling `moses_discover_tools` writes the result
    into the conversation's `chat_conversations.exposed_extra_tools`
    overlay (CHAT-ci3f Phase 1, schema 885); the tool is callable on
    the next turn. Storage cap: `ProfileMosesManagerFull`.
  - Path B (agent pod, CHAT-ymlz) — `moses_discover_tools` writes the
    result into the execution's
    `agent_pod_executions.exposed_extra_tools` overlay (schema 888);
    the tool is callable on the next turn — same model as Path A.
    Storage cap: `ProfileAppInvokedAgent` ∪ `ProfileAgentExecution`
    ∪ `ProfileMosesManagerFull` (wider than chat so legitimate
    agent-only tools like `moses_execute_ticket`,
    `moses_review_execution`, `moses_approve_review` are
    discoverable).
  - In both paths: don't abandon the task — call discover, reissue
    `tools/list`, then retry. If the tool stays uncallable after a
    successful discovery (it lies outside the storage cap), escalate
    via `moses_agent_report_failure` (Path B) or surface a clear "I
    cannot do X from this action" reply (Path A) so the app owner can
    widen the declared profile in `moses-app.config.json`.

INTENTIONALLY EXCLUDED from both profiles: `moses_query`, `moses_create`,
`moses_update`, `moses_delete`, `moses_execute_ticket`,
`moses_quick_build`, ticket/chart/lane CRUD. The wedge: an app fired
this action, not the user — tenant-wide blast radius is wrong.

The full contract, including how to detect which profile you are running
under, lives in **`skills/app-invoked-profiles.md`** (this template ships
both skills). Sources of truth in the platform repo:

- `backend/internal/mcp/tools/config.go` — `ProfileAppInvokedMM` /
  `ProfileAppInvokedAgent` constants and their static tool lists.
- `backend/pkg/prompts/moses_manager.go` — `ModeAppInvokedMM` /
  `ModeAppInvokedAgent` + `buildAppInvokedManagerPrompt`.
- `backend/internal/mcp/tools/workspace_tool_proxy.go` —
  `GetMosesManagerToolsForChart` chart-scoped union.
- Parent epic: **CHAT-89ig**. Template wave: **CHAT-9oqo (AIPF-9)**.
- Documentation epic: **CHAT-ieru** — see
  `arch/backend/MCP_SERVER.md` § "App-invoked profiles" in the
  platform repo for the canonical contract.

---

## Section A details (Path A `chat_prompt` roundtrip — current behaviour)

The original chat-roundtrip walkthrough below applies to BOTH user-typed
Manager chats AND app-invoked `chat_prompt` actions — the *transport*
(POST /apps/.../actions/.../invoke → dispatchChatPrompt → MM auto-response
→ workspace-tool callback → completion webhook → postMessage) is identical.
What differs is the **tool surface visible inside the conversation**, and
that is described in `app-invoked-profiles.md`.

## Roundtrip flow

`<resource>` below stands for whichever path you declare in
`backend/api/openapi.json` and register in
`backend/cmd/server/demo_routes.go`; `<slug>` is the deployed app slug and
`<action-id>` the `platformActions[].id` (`ask-moses` in the example config).
The workspace-tool leg of the diagram only exists once you have declared such
a `<resource>` — the `ask-moses` example as shipped tells MM to reply
directly, without tool calls.

```
Frontend                 Moses backend                MM AI runtime              This app's backend
────────                 ─────────────                ─────────────              ──────────────────
[click your action button]
  │ iframe SDK: window.moses.actions.invoke
  │ POST <base-path>/__moses/invoke  (same-origin; access_token cookie
  ▼                                   + X-Requested-With: moses-iframe)
This app's backend mosesproxy
  │ pod-to-pod, user JWT lifted into Bearer
  └───────────────────►  POST /api/v1/apps/<slug>/
                         actions/<action-id>/invoke
                         (body: {variables: {question}, chartId})
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
                                            POST /api/v1/<resource>
                                                       │       ────────────────►  POST /api/v1/<resource>
                                                       │                          your handler persists it
                                                       │       ◄──────────────────  201 {...}
                                                       │
                                            assistant turn ends
                                                       │
                                  ┌────────────────────┘
                                  │
                                  ▼
                         BroadcastAutoResponseReady (WS)
                         + outbound webhook delivery
                          (HMAC-SHA256 in X-Moses-Signature)            POST /api/v1/webhooks/chat-complete
                                                                         (verify HMAC, hand to CompletionSink)

[receive moses_embed_chat_complete postMessage]
   │
   ▼
(invalidate the relevant TanStack Query key, show completion state)
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
| MM call-back endpoint | yours to add: register the route in `backend/cmd/server/demo_routes.go` and declare it in `backend/api/openapi.json` so it is auto-discovered as a workspace tool |
| Webhook receiver | `backend/internal/handler/webhook_chat.go` (HMAC dual-slot + 5-min skew + nonce replay window + appSlug claim; persistence via the injected `CompletionSink`) |
| Frontend invoke | `frontend/src/moses/invoke.ts` `invokeAction` → iframe SDK `window.moses.actions.invoke` → POST `/__moses/invoke` on this app's own backend (mosesproxy forwards to `MOSES_INTERNAL_API_BASE`) |
| Frontend listener | `frontend/src/moses/hostMessages.ts` `useMosesChatComplete` (origin-checked postMessage) + `announceOpenChat` (open-chat announce to the host shell) |

## Key constraints

1. **All browser calls are relative / same-origin.** Calls to THIS app's backend use relative paths (`fetch('api/v1/<resource>')` — never a leading `/`) through the app's nginx. Calls to MOSES platform actions go through the iframe SDK (`src/moses/invoke.ts`), which POSTs same-origin to `/__moses/invoke`; the app's backend proxy — not the browser — talks to moses-backend (CHAT-pswm.8/.9). The iframe never fetches moses-backend directly, and there is no `VITE_MOSES_API_BASE` involved in the roundtrip.

2. **postMessage origin check.** Always compare `e.origin === window.location.origin`. The host iframe shell deploys apps under a Moses subpath, so iframe and host are same-origin. Cross-origin custom-domain apps need an origin allowlist (future work).

3. **Variable trust.** The example's `question` variable is `userSupplied: true`. The template renders it inside fences via `variableEscapeMode: "fenced"` so MM treats it as content, not instructions. If you adapt this template for higher-trust contexts, you can drop the fencing — but never feed unaudited user input into a `raw`-mode template.

4. **Webhook secret.** The `MOSES_CHAT_WEBHOOK_SECRET` env var is the shared HMAC key. Moses provisions it at install (rotated via the admin endpoint). Recipients ignore unsigned or stale-timestamp payloads.

5. **Webhook secret rotation is recipient-driven.** Schema-882 supports a 24h overlap window (`secret_previous` + `secret_previous_expires_at`), but the platform sender is single-slot — every outbound signature uses the active secret. To get true no-cutover rotation the recipient must verify against BOTH secrets during the overlap: try `MOSES_CHAT_WEBHOOK_SECRET` first, fall back to `MOSES_CHAT_WEBHOOK_SECRET_PREVIOUS` (optional, empty outside an overlap) on mismatch, reject if both fail. **This template's `webhook_chat.go` already implements dual-slot verification** — set `MOSES_CHAT_WEBHOOK_SECRET_PREVIOUS` during the overlap window and unset it once rotation is complete. See `moses-deployment-guide/SKILL.md` for the full recipient cookbook.

## Manual end-to-end test recipe

Prerequisite: copy the `ask-moses` action from
`moses-app.config.with-secrets.example.json` into `moses-app.config.json`
(shipped `platformActions` is empty), wire a button in the frontend that calls
`invokeAction('ask-moses', { question })` (`frontend/src/moses/invoke.ts`),
and — if you want to see the workspace-tool callback leg — declare at least
one write path in `backend/api/openapi.json`, register it in
`backend/cmd/server/demo_routes.go`, and extend the prompt to call it. Then:

1. Deploy this app (e.g. via marketplace install or `moses_init_repo --template fullstack-chat`).

   **Activation precondition (CHAT-1j43 / DEPS-A3, A4)**: the chat_prompt action is INACTIVE on first deploy. How activation happens depends on who triggered the deploy:

   - **User-initiated deploy + deployer has `MANAGE/DEPLOYMENTS` permission** → DEPS-A3 auto-grants on the deployer's behalf during the post-Helm reconcile. Action becomes invocable within seconds. Continue to step 2.
   - **Autonomous deploy (agent-driven), or deployer without `MANAGE/DEPLOYMENTS`** → DEPS-A4 banner fires in the embedded SelectedAppPanel: "Approve permissions for <app> — 1 action inactive". Click **Approve** in the banner before continuing. The banner fires within ~2s of the post-Helm reconcile completing.

2. Open it in the Moses UI.
3. Supply the action's `question` variable ("what does this app do?").
4. Click your action button.
5. Observe:
   - The invoke resolves with a `conversationId`; `useMosesChatComplete` fires when the turn ends.
   - Sidebar: a new conversation titled per `chatPrompt.conversationTitle` appears. Its body shows the rendered prompt with the question fenced.
   - If you declared a write path, MM's workspace-tool call lands on your handler.
6. Verify webhook receipt in the backend logs — the shipped `LogCompletionSink` logs each verified payload (swap it for your own sink in `newCompletionSink`, `backend/cmd/server/demo_routes.go`).
7. Repeat 6 times within a minute → 6th invoke returns 429 (`rate_limited`) due to the per-action cap.
8. Open two browser tabs, click in both within 1 second → second returns 429 (`chat_prompt_concurrency_limit`) with the in-flight conversationId.

### Troubleshooting

- **Clicking the action button returns 409 `action_not_activated`**: the chat_prompt action wasn't activated. Check the post-deploy banner / App Permissions modal — see step 1's activation precondition. Manual recovery: Tenant Admin → Apps → <your app> → Approve grant.
- **Returns 403 `action_pending_reapproval`**: the action was previously approved but the app added new actions / scopes. The DEPS-A4 banner shows the diff; approve to re-cover.

## Negative scenarios this template helps verify

- **Sidebar visibility (CHAT-jayp)**: the auto-fired conversation MUST appear in the sidebar.
- **Real-time WS push (CHAT-p1nr)**: the conversation appears within milliseconds, not the 15s poll wait.
- **Rate floor (CHAT-9pz5)**: rapid clicks return 429 before the per-action limit if app omits its own.
- **Concurrency cap (CHAT-h8cm)**: parallel clicks return 429 with `inflightConversationId` field.
- **Completion signal (CHAT-rg5t)**: webhook recipient receives signed payload; postMessage reaches iframe.
- **Open-chat postMessage (CHAT-l7va)**: host shell opens sidebar to the new conversation.
- **Provisioner reconcile (CHAT-avoi)**: action row appears in `tenant_app_platform_actions` post-deploy without imperative MCP call.
- **Prompt-injection hardening (CHAT-uwlm)**: the user-supplied variable is fenced; MM does not interpret it as instructions.
- **MM credentials unset (CHAT-xu9i)**: with no AI subscription pinned, the conversation surfaces a clear remediation status message.
- **AppData repo (CHAT-qrd6)**: `moses_get_repositories` lists `app-data/<app-slug>` (read-only by default to MM and agents).

Use this template as a smoke-test artifact during integration; deploy it, run the recipe above, and any divergence indicates a regression in the corresponding bead.
