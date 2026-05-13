# `mosesproxy-go`

Tiny, stdlib-only Go HTTP handler that ships in every Moses default
app template's backend. It receives platform-action invokes from the
in-iframe Moses SDK (`window.moses.actions.invoke`) on a well-known
path (`/__moses/invoke`) and forwards them pod-to-pod to moses-backend
with the user's JWT preserved. This is the receiving half of the
iframe SDK contract; see CHAT-pswm (epic) for the full round-trip
architecture.

## Install (vendored — canonical)

Consuming templates **vendor** this package by copying `proxy.go` +
`proxy_test.go` into their own `backend/internal/mosesproxy/`
subdirectory and importing from their own module path. No `replace`
directive, no Go workspace, no path dependency on the templates
monorepo — the template ships as a self-contained repo that compiles
without `shared/` on disk (CHAT-45nq8).

The canonical reference is `fullstack-chat/backend/internal/mosesproxy/`.
Each vendored file carries a `VENDORED COPY` header comment at the top
that documents the re-sync recipe:

```go
// VENDORED COPY of shared/mosesproxy-go/proxy.go.
//
// Canonical source: ../../shared/mosesproxy-go/proxy.go
// (relative to the templates monorepo root)
//
// To re-sync after upstream changes, run (from the templates monorepo root):
//   cp shared/mosesproxy-go/proxy.go fullstack-chat/backend/internal/mosesproxy/proxy.go
//   cp shared/mosesproxy-go/proxy_test.go fullstack-chat/backend/internal/mosesproxy/proxy_test.go
//   cd fullstack-chat/backend && go build ./... && go test ./...
```

Preserve that header on re-sync; drop only the canonical file's
package-level comment so the "VENDORED COPY" notice stays at the top.
CI runs `tools/check-vendored-mosesproxy.sh` on every PR that touches
either `shared/mosesproxy-go/**` or any vendored copy and fails if the
bodies diverge (modulo the header).

Then mount the handler in `main.go`, importing from your template's
own module path:

```go
import (
    "net/http"

    "github.com/moses-platform/moses-templates/fullstack-chat/backend/internal/mosesproxy"
)

func main() {
    mux := http.NewServeMux()

    // Receive in-iframe SDK calls. The well-known path is
    // mosesproxy.InvokePath ("/__moses/invoke"); the SDK expects
    // that exact path.
    cfg := mosesproxy.ConfigFromEnv()
    cfg.RequireRequestedWith = true // recommended; see CSRF section
    mux.HandleFunc(mosesproxy.InvokePath, mosesproxy.NewHandler(cfg))

    // ...rest of your app's routes
    http.ListenAndServe(":8080", mux)
}
```

The handler is a plain `http.HandlerFunc` and composes cleanly with
`net/http`, `chi`, `gorilla/mux`, or `gin` (via `gin.WrapF`).

## Install (workspace, dev only)

Inside the templates monorepo it's occasionally convenient to consume
the canonical `shared/mosesproxy-go/` module directly — e.g. when
iterating on the proxy and a downstream template in the same change.
For that flow only, wire a Go workspace (`../../go.work` at the repo
root) or a `replace` directive in your template's `go.mod`:

```go
// templates-monorepo development only — user repos always vendor.
require github.com/moses-platform/moses-templates-shared/mosesproxy-go v0.0.0

replace github.com/moses-platform/moses-templates-shared/mosesproxy-go => ../../shared/mosesproxy-go
```

This shape is **not** suitable for distribution: `moses_init_repo`
clones a template subtree as a standalone repo, and a `replace`
pointing at `../../shared/` is broken outside the monorepo. Always
re-vendor before merging template changes.

## Env contract

The platform injects four env vars at deploy time. Build a `Config`
from them with `ConfigFromEnv()`, or assemble one yourself for tests.

| Var                       | Source                                                                   | Required |
| ------------------------- | ------------------------------------------------------------------------ | -------- |
| `MOSES_INTERNAL_API_BASE` | Cluster-DNS base URL for moses-backend (e.g. `http://moses-backend.moses.svc.cluster.local:8080`). Injected by CHAT-pswm.1. | yes |
| `MOSES_APP_SLUG`          | This template's platform-action slug (the segment in `/api/v1/apps/{slug}/...`). | yes |
| `MOSES_CHART_ID`          | Chart UUID this deployment belongs to. Forwarded in the invoke body so moses-backend resolves the right conversation scope. | no  |
| `MOSES_TENANT_ID`         | Tenant UUID. Forwarded as the `X-Tenant-ID` header when set; aids moses-backend's multi-tenant resolution. | no  |

When either required var is missing the handler returns
`503 moses_unconfigured` on every call — it never silently degrades.
The SDK surfaces the error envelope to the caller.

## Wire diagram

```
iframe (window.moses.actions.invoke)
   │  POST /__moses/invoke    { actionId, variables }
   │  credentials: same-origin → access_token cookie OR Authorization Bearer
   ▼
app's own nginx → app backend (this handler)
   │  Extract user JWT (Bearer header, else access_token cookie)
   │  Read MOSES_INTERNAL_API_BASE / APP_SLUG / CHART_ID / TENANT_ID
   │  POST ${MOSES_INTERNAL_API_BASE}/api/v1/apps/${APP_SLUG}/actions/${actionId}/invoke
   │    Authorization: Bearer <user_jwt>
   │    X-Tenant-ID: ${MOSES_TENANT_ID}        (when set)
   │    Content-Type: application/json
   │    body: { variables, chartId: ${CHART_ID} }
   ▼
moses-backend (cluster service DNS, no gateway hop)
   AuthMW (Bearer-exempt from CSRF) → user-scoped handler → dispatcher
   chat_conversations.user_id populated → sidebar visibility preserved
```

## Trust boundary

This proxy widens the JWT trust window from "browser → moses" to
"browser → app-backend → moses". That's acceptable for first-party
templates deployed inside the same tenant cluster because:

1. The app backend runs in the same Kubernetes namespace as
   moses-backend (or an adjacent namespace under the same operator).
2. The second hop never leaves the cluster — `MOSES_INTERNAL_API_BASE`
   resolves to a service DNS name, not an internet route.
3. The proxy is `~200 LOC` of audited stdlib code with no third-party
   deps; the attack surface is bounded.
4. The handler does NOT validate the JWT (it has no access to the
   signing key) and does NOT log it; it pass-through forwards.

For third-party marketplace apps the trust boundary should be
tightened by removing this hop entirely (direct browser → gateway
posts with explicit CSRF handling). Tracked separately; not the
contract for default templates.

### CSRF on `/__moses/invoke`

The endpoint is same-origin POST with cookie credentials. A drive-by
CSRF gadget on another origin can target it the same way it could
target any other same-origin POST your template exposes. Two defences
are available:

- **Browser defaults**: modern browsers send `SameSite=Lax` cookies on
  cross-origin POSTs only when the request is the top-level
  navigation. A drive-by form-POST from another origin is blocked by
  default in Chrome/Firefox/Safari.
- **`X-Requested-With: moses-iframe` gate** (`Config.RequireRequestedWith`):
  when true, the proxy rejects requests that don't carry the custom
  header. A drive-by `<form>`/`<img>` cannot set custom headers, so
  this closes the window even when SameSite default is overridden.
  The SDK emits the header as of CHAT-pswm.9. The field still defaults
  to `false` so existing consumers that haven't picked up the SDK
  update don't break, but the canonical `fullstack-chat` reference
  impl turns it on (`main.go`: `proxyCfg.RequireRequestedWith = true`)
  and **new templates SHOULD set it to `true` in their `main.go`**.

## Behaviour summary

| Scenario                                   | Response                                                |
| ------------------------------------------ | ------------------------------------------------------- |
| Method ≠ POST                              | `405 method_not_allowed`                                |
| Missing required env                        | `503 moses_unconfigured`                                |
| `RequireRequestedWith=true` and header missing | `403 missing_requested_with`                            |
| No Authorization header & no `access_token` cookie | `401 no_user_jwt`                                       |
| Body > 64 KiB                              | `400 bad_request` (LimitReader cuts the parse)          |
| Missing `actionId` in body                  | `400 missing_action_id`                                 |
| Upstream connection refused / timeout       | `502 moses_unreachable`                                 |
| Upstream returns any status                 | Status + Content-Type + body forwarded verbatim         |
| Upstream `Set-Cookie` headers              | **Stripped** — never reflected to the iframe            |

## What is NOT sent / done

- The handler does NOT validate JWT signatures (it has no key).
- The handler does NOT log the JWT.
- The handler does NOT reflect upstream `Set-Cookie` headers — those
  belong to moses-backend's origin, not the app's.
- The handler does NOT rate-limit on the app side. The upstream
  enforces per-action floors (e.g. `chat_prompt` 1/min, 5/hr); their
  429 envelopes pass through untouched.

## Tests

```
go test -v -cover ./...
```

The test suite stands up an `httptest.Server` as the moses-backend
stand-in and asserts on the captured forwarded request. No external
network calls. Current coverage: 92%+.

## See also

- `CHAT-pswm` (epic) — the round-trip architecture
- `CHAT-pswm.1` — env injection (provisioner side)
- `CHAT-pswm.2` — the in-iframe SDK that calls this proxy
- `CHAT-pswm.9` — `fullstack-chat` adoption (canonical reference impl)
