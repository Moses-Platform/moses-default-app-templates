---
name: app-invoked-profiles
description: Tool-surface contract for app-invoked Moses Manager chats and launch_agent agent pods (CHAT-89ig)
mode: reference
priority: required-when-source-app-action
---

# App-invoked profile contract — read this first if you were launched from an app

This skill describes the **narrow** Moses tool surface you get when a Moses
app fires a `chat_prompt` (Path A) or `launch_agent` (Path B) platform
action against you. The two paths share one principle: the calling app
declared its needs in `moses-app.config.json`; the user clicked a button,
not started a conversation; therefore your tool surface is scoped to *this*
app, not to the tenant.

**How do I know which profile I am running under?**

- Call `moses_get_session_info`. The returned `current_profile` field is one of:
  - `app-invoked-mm` (Path A — `ProfileAppInvokedMM`, this conversation came from a `chat_prompt` action),
  - `app-invoked-agent` (Path B — `ProfileAppInvokedAgent`, this agent pod came from a `launch_agent` action),
  - `moses-manager-essentials` / `moses-manager-full` / `moses-manager-autonomous` (you are a USER-typed Manager chat — see `chat-roundtrip-overview.md` Section A),
  - `agent-execution` (you are a Basar / autopilot agent — out of scope here).
- There is no direct chat-context-source signal in `moses_get_session_info`
  today (the response contains `current_profile`, `autopilot_active`,
  tenant/user ids, etc. — it does NOT include `context.source`). Instead,
  use the absence/presence of agent-execution lifecycle tools
  (`moses_notify_push`, `moses_agent_request_build`, `moses_agent_submit_completed`)
  in your `tools/list` to disambiguate Path A (chat-only profile, none of
  these) from Path B (agent profile, all of these).

If you got here from a USER typing into the chat sidebar, this skill does
NOT apply — the full Manager surface is yours. Stop reading this and
proceed normally.

## Why narrow?

The tenant-wide surface (`moses_query` / `moses_create` / `moses_update` /
`moses_delete` / `moses_execute_ticket` / `moses_quick_build` / lane &
ticket CRUD) is **intentionally excluded** from both app-invoked profiles.
The wedge: an app firing a `launch_agent` action should not be able to mutate
charts and lanes outside that app's chart. The profile is the safety rail.

If you find yourself wanting a tool that's not in your default profile,
**call `moses_discover_tools`** with a brief query for the missing
capability. As of CHAT-ymlz the two paths behave **symmetrically** —
discovery extends the runtime surface for both:

- **Path A (chat — `app-invoked-mm`)**: discovery appends the tool to
  this conversation's `chat_conversations.exposed_extra_tools` column
  (CHAT-ci3f Phase 1, schema 885) and it becomes callable on the *next*
  turn. Storage cap: `ProfileMosesManagerFull`.
- **Path B (agent pod — `app-invoked-agent`, CHAT-ymlz)**: discovery
  appends the tool to this execution's
  `agent_pod_executions.exposed_extra_tools` column (schema 888) and it
  becomes callable on the *next* turn — same flow as Path A. Storage
  cap: `ProfileAppInvokedAgent` ∪ `ProfileAgentExecution` ∪
  `ProfileMosesManagerFull` (wider than chat so legitimate agent-only
  tools like `moses_execute_ticket`, `moses_review_execution`,
  `moses_approve_review` are discoverable).

In both paths: don't abandon the task — call discover, reissue
`tools/list`, then retry the tool. Discovery telemetry (`app_discovery_calls`)
still records a signal so operators can spot apps whose declared
profile is too narrow.

## Path A — `ProfileAppInvokedMM` (chat_prompt → Moses Manager)

You are an interactive Moses Manager conversation, scoped to the calling
app's chart. Your **static surface is 4 tools**:

| Tool | Purpose |
|---|---|
| `moses_get_session_info` | Self-awareness: tenant id, app slug, action id, chart id |
| `moses_discover_tools` | **Escape hatch** — appends a tool to this conversation's overlay |
| `moses_global_search` | Read-only fallback for "what is X" questions across the tenant (no CRUD) |
| `moses_get_app_logs` | Loki-scoped: "why did my app fail to do X" |

**Plus dynamic workspace tools** for the calling app's chart, injected at
`tools/list` time by `WorkspaceToolProxy.GetMosesManagerToolsForChart`
(CHAT-cj8m). These are the OpenAPI endpoints declared in the app's
`apiEndpoints.specPath` (e.g. a `POST /api/v1/<resource>` the app declared) —
auto-discovered, chart-scoped, ready to call.

**Intentionally OUT of profile**: `moses_query`, `moses_create`,
`moses_update`, `moses_delete`, `moses_execute_ticket`, `moses_quick_build`,
ticket / chart / lane CRUD. If you need any of these, the right move is to
ask the user — you should rarely need tenant CRUD from inside an app
action; if you do, the app probably should not have used `chat_prompt`.

## Path B — `ProfileAppInvokedAgent` (launch_agent → agent pod)

You are a one-shot agent pod created by an app's `launch_agent` action.
Your **static surface is 9 tools**:

| Tool | Purpose |
|---|---|
| `moses_get_session_info` | Self-awareness |
| `moses_discover_tools` | **Escape hatch** (NEW vs legacy `agent-execution` — that profile lacked discovery) |
| `moses_read_file` | Read repo files; backend-mediated, no git creds in pod |
| `moses_list_files` | Enumerate repo paths |
| `moses_notify_push` | Trigger the deployment pipeline after committing to `agent/{ticket-id}` |
| `moses_agent_request_build` | Request an in-cluster image build of the current branch |
| `moses_agent_submit_completed` | Mark the ticket done — REQUIRED before pod exit |
| `moses_agent_report_failure` | Mark the ticket failed — alternative terminal call |
| `moses_await_deployment` | Block on Helm health when the action's outcome depends on deploy |

**Plus dynamic workspace tools** for the calling app's chart (same
mechanism as Path A — chart-scoped, OpenAPI-discovered).

**Intentionally OUT of profile** (vs the standard ~50-tool
`ProfileAgentExecution` you may have seen on Basar / autopilot agents):
all CRUD (`moses_query` / `moses_create` / `moses_update` / `moses_delete`),
`moses_execute_ticket`, `moses_quick_build`, plus `moses_get_app_logs`
(agents emit via stdout; Loki is the post-hoc lens for users, not for
in-pod self-inspection).

## How the dynamic chart-scoped surface gets there

CHAT-cj8m. At `tools/list` time, the platform's MCP tool resolver:

1. Looks up `chat_conversations.context.app_slug` (Path A) or the agent
   pod's owning execution → ticket → chart (Path B).
2. Resolves the chart's deployed workspace tools via
   `WorkspaceToolProxy.GetMosesManagerToolsForChart(chartID)`.
3. Unions those tools into the static profile list.
4. Returns the union as the visible surface.

If your app declared `apiEndpoints.specPath: "/api/openapi.json"` and the
deployment exposed it, every documented operation (with a stable
`operationId`) is callable as a workspace tool — no extra registration.

## What if a tool I called returns "tool X is not available in your current profile"?

That is the gate at `ai_chat_service.go ExecuteMCPTool`. The fix
depends on which path you are on:

**Path A (chat — `app-invoked-mm`)**:

1. Call `moses_discover_tools` with a query like `"X"` or
   `"<capability I need>"`.
2. The discovery result writes the tool name to the conversation's
   `exposed_extra_tools` column.
3. On the next turn, the tool is callable.
4. If discovery returns nothing relevant, the capability is genuinely
   not available to you — reply to the user with a clear "I cannot do X
   from this action".

**Path B (agent pod — `app-invoked-agent`, CHAT-ymlz — symmetric with Path A)**:

1. Call `moses_discover_tools` with a query like `"X"` or
   `"<capability I need>"`. This both records the
   `app_discovery_calls` telemetry signal AND appends the tool name to
   this execution's `agent_pod_executions.exposed_extra_tools` column
   (schema 888).
2. Reissue `tools/list` (the platform unions the static profile with
   the overlay) — the discovered tool is now visible.
3. Retry the call — both the `tools/list` filter and the
   `ExecuteTool` allowed-tools gate read from the same overlay, so the
   call passes if the tool falls within the agent-context storage cap
   (`ProfileAppInvokedAgent` ∪ `ProfileAgentExecution` ∪
   `ProfileMosesManagerFull`).
4. If discovery returns nothing relevant OR the gate still rejects on
   retry (the tool lies outside the cap), escalate via
   `moses_agent_report_failure` with a clear message naming the missing
   tool/capability so the app owner can widen the declared profile in
   `moses-app.config.json` and re-deploy.

## Sources of truth (platform repo)

- `backend/internal/mcp/tools/config.go` — profile constants
  (`ProfileAppInvokedMM`, `ProfileAppInvokedAgent`) and their static
  tool lists (`getAppInvokedMMTools`, `getAppInvokedAgentTools`).
- `backend/pkg/prompts/moses_manager.go` — `ModeAppInvokedMM` /
  `ModeAppInvokedAgent` and `buildAppInvokedManagerPrompt` (the thin
  app-invoked system prompt).
- `backend/internal/mcp/tools/workspace_tool_proxy.go` —
  `GetMosesManagerToolsForChart` chart-scoped union.
- `backend/internal/api/ai_chat_service.go` — `resolveChatProfile` and
  the `ExecuteMCPTool` gate that emits the
  `tool %s is not available in your current profile` error.
- Parent epic: **CHAT-89ig**. Template wave: **CHAT-9oqo (AIPF-9)**.
- See also `arch/backend/MCP_SERVER.md` § "App-invoked profiles" in the
  platform repo (CHAT-ieru) for the canonical contract.

## Cross-reference

The companion skill `chat-roundtrip-overview.md` covers the **user-typed**
Manager → app-context behaviour (full surface + autopilot upgrades). Read
that one when `profile != app-invoked-*`.
