# Moses Default App Templates

Self-contained, fully runnable application templates for the [Moses Platform](https://github.com/Moses-Platform). Each subdirectory is a complete project that can be deployed to Moses via agent execution or manual workflow.

## Templates

| Template | Type | Description |
|----------|------|-------------|
| **frontend-template** | frontend | React 19 (+ React Compiler) + Vite + TypeScript served via nginx. Health check, Helm chart, single container. |
| **backend-template** | api | Go stdlib HTTP server with OpenAPI spec, health endpoint, and MCP auto-discovery. |
| **fullstack-simple** | hybrid | Separate React frontend (nginx) + Go backend containers. Nginx proxies API calls. |
| **fullstack-unified** | hybrid | Single Go binary serving static frontend via `go:embed` + API endpoints. Simplest fullstack pattern. |
| **fullstack-showcase** | hybrid | Go + React with a bundled per-app PostgreSQL, multi-service Helm chart, and the full Moses integration surface pre-wired. |
| **fullstack-chat** | hybrid | Go + React scaffold carrying every Moses Manager integration surface (app actions via the iframe SDK + backend proxy, workspace-tool callbacks, HMAC completion webhooks). |
| **fullstack-oidc** | hybrid | Go + React reference app that acts as an OIDC relying party fronted by Moses (BFF pattern), via the vendored `oidcauth` middleware. |

## How Templates Work

Each template includes:

- **`moses-app.config.json`** — App type, services, ports, health checks, Docker and Helm config
- **`Dockerfile`** — Multi-stage build producing a minimal container
- **`helm/`** — Kubernetes Helm chart (deployment, service, probes)
- **`skills/`** — Agent skill files describing the template's architecture (optional)
- **Source code** — Fully working application code

## Usage

Templates are automatically cloned into each Moses tenant via the built-in Moses Git system. Agents use `moses_init_repo(template="<name>")` to create new project repositories from a template.

### Choosing a Template

- **Web page (no backend)**: `frontend-template`
- **API service (no UI)**: `backend-template`
- **Full app, Go serves static files (1 container)**: `fullstack-unified`
- **Full app, separate frontend + backend (2 containers)**: `fullstack-simple`
- **Full app with a database (2 containers + bundled PostgreSQL)**: `fullstack-showcase`
- **App that talks to Moses Manager (platform actions, chat prompts, completion webhooks)**: `fullstack-chat`
- **App that authenticates real users via OIDC**: `fullstack-oidc`

## What a fresh template contains

Every template ships **demo-free by default** — there is no cleanup step to
run. What you get is the Moses platform plumbing and nothing else: the path
contract (see "The deployed-app path contract" below), the `/health` +
`/api/openapi.json` dual mounts, X-Moses middleware, CSRF guard,
browser-logger, nginx/entrypoint CSP handling, the Helm chart, and
`moses-app.config.json`. Each template's `skills/usage.md` documents that
surface plus the template's env-var contract.

Starting a real app is therefore: set your identity fields in
`moses-app.config.json`, add real routes where the route-registration stub
points, mirror them in `api/openapi.json`, and run the template's validation
commands.

## Adding Custom Templates

Fork this repo and add a new subdirectory with a valid `moses-app.config.json`. Moses auto-discovers templates by scanning subdirectories for this config file.

Required fields in `moses-app.config.json`:
- `name` — lowercase alphanumeric with hyphens
- `version` — semver (e.g., "1.0.0")
- `displayName` — human-readable name
- `description` — what the template does
- `appType` — `frontend`, `api`, or `hybrid`
- `entrypoint` — main file (e.g., "index.html" or "main.go")

## Moses-aware vs legacy templates

All 7 default templates ship as **Moses-aware** — they declare
`templateApiVersion: "moses-manager.eu/v1"` in `moses-app.config.json` and serve
under the runtime `MOSES_BASE_PATH` natively, emitting
`Content-Security-Policy: frame-ancestors` themselves per the `embedding`
block. The platform deploys them WITHOUT the URL-rewrite + X-Frame-Options
strip safety net.

When authoring a custom template, you have two options:

- **Easiest path (legacy)**: omit `templateApiVersion`. The Moses platform
  URL-rewrites the `/apps/<tenant>/<slug>/` prefix to `/` before forwarding
  and strips the upstream `X-Frame-Options` header so the iframe renders.
  Your code can stay completely unaware of the public path.
- **Moses-aware**: declare `templateApiVersion: "moses-manager.eu/v1"` and the
  `embedding` block (see schema in
  `moses-platform-prep/backend/internal/types/app_config_types.go`). Your
  runtime must (a) read `MOSES_BASE_PATH` and mount routes there, (b) emit
  `Content-Security-Policy: frame-ancestors` based on
  `MOSES_EMBEDDING_FRAMING` / `MOSES_EMBEDDING_ALLOWED_ANCESTORS`. See
  `frontend-template/entrypoint.sh` and
  `fullstack-unified/main.go` (`withEmbeddingHeaders`) for reference
  implementations.

Both modes work today and will continue to work for the standard N-2 minor
deprecation window. See `moses-platform-prep/DEPRECATIONS.md` and
`moses-platform-prep/arch.md` §App Templates for the full contract.

## The deployed-app path contract

Every Moses-aware template in this repo follows one contract for *where it
mounts what*. This is the authoritative statement — if a template comment, an
older doc, or any agent guidance contradicts it, this section wins. The full
platform-side write-up lives in the deployment skill
(`moses-platform-prep/backend/internal/services/agent_skills/moses-deployment-guide/SKILL.md`,
§ *The deployed-app path contract*); this section is the template-author copy.

A Moses-aware app is deployed at `MOSES_BASE_PATH` (e.g.
`/apps/{tenant}/{slug}/`). The ingress does **NOT** strip that prefix — the
frontend container receives the full prefixed path on browser traffic. In a
multi-container template the frontend nginx then proxies `/api/` (and
`/__moses/`) to the backend container, and it MUST forward the prefix
**unchanged** — `proxy_pass` with no URI part — so the backend sees the same
`MOSES_BASE_PATH`-prefixed path it registered its routes under. A `proxy_pass
http://backend/api/;` URI part silently strips the prefix and every API call
404s (CHAT-yfmwv). Two classes of caller reach a deployed app, and the contract
gives each surface exactly one correct address:

| Surface | Mount it at | Who calls it |
|---------|-------------|--------------|
| Static assets | `MOSES_BASE_PATH` | the browser |
| **All API routes** | **`MOSES_BASE_PATH` — one registration** | the browser **and** the workspace-tool proxy |
| `/health` | the **root** `/health` **and** `{MOSES_BASE_PATH}/health` (cost-free) | K8s probes + platform health checks |
| `/api/openapi.json` (your declared `specPath`) | the **root** `/api/openapi.json` **and** `{MOSES_BASE_PATH}/api/openapi.json` (cost-free) | platform OpenAPI discovery (canonical) + callers that reach the app through the frontend nginx (prefixed) |

### Rules

1. **Register all API routes ONCE, under `MOSES_BASE_PATH`.** This is the API's
   natural browser-facing home, and the platform's workspace-tool proxy now
   calls the API there too — so a single registration serves both callers. Do
   **not** also mount the API canonically; the old copy-pasted
   `registerAPI(prefix)` closure that registered every route twice (canonical +
   prefixed) has been removed from all seven templates.
2. **`/health` at the root `/health`.** K8s liveness/readiness probes call the
   pod IP directly, bypassing the ingress. Templates additionally mount
   `{MOSES_BASE_PATH}/health` (one trivial extra handler) so an app that
   accidentally serves everything under the base path still passes the probe.
3. **`/api/openapi.json` at the root AND under `MOSES_BASE_PATH`.** The platform
   discovery hook fetches it canonically; the base-path alias keeps it reachable
   through the frontend nginx. Like
   `/health`, this is a cost-free second handler for the same static spec.
4. **Frontends use relative fetch paths** (`fetch('api/v1/...')`) so the browser
   resolves them against the prefixed page URL.
5. **Multi-container templates: the frontend nginx forwards `/api/` (and
   `/__moses/`) to the backend with `proxy_pass` carrying NO URI part** — that
   preserves the `MOSES_BASE_PATH` prefix end-to-end. A URI part strips it and
   the backend 404s; `tests/test_nginx_entrypoint.sh` guards the regression
   (CHAT-yfmwv). The **root** `location /api/` block is the one exception: it
   must forward onto `${MOSES_BASE_PATH_PREFIX}/api/` so apex/custom-hostname
   traffic — which the platform serves at the host root with no rewrite, while
   the pod env keeps the canonical `MOSES_BASE_PATH` — still reaches the
   single API registration (CHAT-t5d1u.24; an empty prefix renders the plain
   `/api/` pass-through, so standalone deploys are unchanged).

This is the *current* reality, not a plan: the workspace-tool proxy calls the
app API under `MOSES_BASE_PATH` (platform fix CHAT-uzu24), all seven templates
register the API once under `MOSES_BASE_PATH` (CHAT-8qiu0), and the frontend
nginx forwards the prefix to the backend unchanged (CHAT-yfmwv).

### In-cluster callers

The platform itself calls a deployed app from several places. Template authors
should know the full picture:

| Caller | Path it hits |
|--------|--------------|
| Post-deploy health check | canonical `/health` |
| K8s liveness/readiness probes | canonical `/health` (pod IP, ingress bypassed) |
| `HelmReconcilerService.probeRoute` (recurring liveness) | canonical `/health` |
| OpenAPI discovery | canonical `/api/openapi.json` |
| Workspace-tool proxy | the API **under `MOSES_BASE_PATH`** |
| Browser (frontend, SPA, `fetch`) | static + API under `MOSES_BASE_PATH` |

### OpenAPI `servers[]` must stay base-path-free

In a template's `openapi.json`, the `servers[]` array must contain **only the
canonical API base** (`[{"url": "/api/v1"}]`) or be omitted — **never** a
`/apps/{tenant}/{slug}/...` URL. The platform parses `servers[]` into the
endpoint path and the workspace-tool proxy prepends `MOSES_BASE_PATH`; a
base-path-aware `servers[]` would be double-prefixed and 404.

### Anti-pattern and the two failure modes

**Anti-pattern:** registering the API at only one address, or guessing which
one. The contract removes the guess: one registration, under `MOSES_BASE_PATH`.

Two real failures this contract prevents:

1. **API canonical-only → the browser frontend 404s** — the browser fetches the
   prefixed path and finds nothing there.
2. **API prefixed-only on a pre-fix platform → workspace tools 404** — before
   the proxy fix, the platform still called canonical API paths. Post-fix this
   is correct; the failure mode is named so it is recognizable on an
   un-upgraded platform.

The platform's post-deploy reachability gate checks that the app's Service has
a ready endpoint and GETs the served base path (`{MOSES_BASE_PATH}`) on the
in-cluster Service; a 404 or no ready endpoint fails the deploy — the safety
net behind this contract.

### Standalone vs Moses-managed embedding defaults

When `MOSES_EMBEDDING_FRAMING=moses-only` and `MOSES_EMBEDDING_ALLOWED_ANCESTORS`
is empty, each template's `entrypoint.sh` defaults to the same chart-parity
origin list the platform's resolver emits
(`moses-platform-prep/backend/internal/services/embedding_policy_resolver.go`):
`'self' tauri://localhost http://tauri.localhost https://tauri.localhost`,
plus `https://${MOSES_DOMAIN} https://*.${MOSES_DOMAIN}` when `MOSES_DOMAIN` is a
production-style domain (port-wildcarded `http://`/`https://` forms for
localhost domains). `fullstack-unified` (no `entrypoint.sh`) defaults to
`'self'` only. This keeps
`helm install` of a Moses-aware template **outside** the Moses platform
embeddable from Moses Manager and from the Tauri installer shell — without
the default a standalone deploy would only allow `'self'` and Manager would
get blocked. When the platform deploys the template it supplies a concrete
`MOSES_EMBEDDING_ALLOWED_ANCESTORS` (resolver output), and the explicit
override wins verbatim — no auto-merge with the standalone default.

### Request body / upload size ceiling

On nginx-ingress installs, Moses-managed per-app routes allow request bodies up
to **50 MB** (`proxy-body-size: 50m`); on Envoy Gateway installs the
`ClientTrafficPolicy` buffer is `512Mi` and does not cap streamed bodies. Your
app's own body parser is the authoritative limiter — the platform will not
raise a 413 under 50 MB on either path, so keep upload caps at or below 50 MB.

Every Go template applies `LimitRequestBody` (`http.MaxBytesReader`) with a
**1 MiB default**. Set `MOSES_MAX_BODY_BYTES` to a positive byte count at
startup to override it (for example `52428800` for 50 MiB); unset, invalid,
zero and negative values keep the default. Known oversized Content-Length
requests receive 413 immediately. Streaming/chunked bodies fail with
`*http.MaxBytesError` while being read: handlers must check that error and
return 413 before committing a response or side effects. The middleware
never buffers the whole body. Endpoint-specific smaller limits still apply.

The split simple/showcase/chat/OIDC charts use **one values key**:
`globalEnv.MOSES_MAX_BODY_BYTES: "52428800"` raises both hops to 50 MiB. The
chart injects this into frontend and backend. Rendering fails if any service-local
`env` sets this key, including retained values from an older chart:
"move MOSES_MAX_BODY_BYTES to globalEnv: one value now applies to the frontend proxy and the backend".
Remove the service-local key and move its value to `globalEnv` before deploying.
The frontend entrypoint validates the value and renders nginx's
`client_max_body_size` in bytes,
with the same **1048576-byte** default as Go. Malformed, zero, negative or
overflowing values retain the default and the entrypoint logs
`Invalid MOSES_MAX_BODY_BYTES; using default 1048576`. For standalone
containers, pass the same environment value to both. The frontend-only and
backend-only charts expose `env.MOSES_MAX_BODY_BYTES`; unified exposes
`globalEnv.MOSES_MAX_BODY_BYTES`. An outer ingress or a smaller endpoint cap
can still reject a request; raising the app cap does not raise those limits.

For upload endpoints (images, archives, multipart forms), choose a cap at or
below the platform ceiling and handle overflow explicitly:

```go
const (
    MaxUploadBytes = 50 << 20 // 50 MB — total body cap (the Moses proxy ceiling)
    multipartMem   = 10 << 20 // in-RAM threshold; larger parts spill to tmp files
)

// MaxBytesReader enforces the actual size limit — a body over MaxUploadBytes
// makes the next Read fail, so ParseMultipartForm returns an error.
r.Body = http.MaxBytesReader(w, r.Body, MaxUploadBytes)
// ParseMultipartForm's argument is maxMemory (how much to keep in RAM before
// spilling to disk), NOT the size cap — keep it well below MaxUploadBytes.
if err := r.ParseMultipartForm(multipartMem); err != nil {
    http.Error(w, `{"error":"file too large (max 50MB)"}`, http.StatusRequestEntityTooLarge)
    return
}
```

Do **not** set a parser limit larger than 50 MB expecting larger uploads to
succeed — on nginx-ingress installs the proxy ceiling would 413 first. Raising
it is a platform change (nginx `proxy-body-size` in `moses-platform-prep`
`buildIngressAnnotations`).

## Running on the apps origin

Moses can serve apps from their own **apps origin** instead of the Moses
origin: another port on a local or desktop install (`http://localhost:8081`),
`apps.<moses domain>` in production. The app is then cross-origin to the Moses
window that frames it. Every template here works both before and after that
switch, because each one follows four rules:

- **Relative paths only.** `base: './'` (Vite) or `<base href="./">`, and API
  calls like `fetch('api/v1/things')` — never a leading `/`, never a Moses
  hostname. The page works under `/apps/<tenant>/<slug>/` on either origin.
  There are exactly two intended root paths, both Moses-owned: the
  `<script src="/api/v1/sdk/iframe-sdk.js">` tag and the browser logger's
  `/api/v1/browser-logs/*` requests.
- **No Moses API from the browser.** Platform actions go through the iframe
  SDK, which posts to the app's own `/__moses/invoke`; the app's backend
  proxy talks to Moses. The browser logger asks `/api/v1/browser-logs/*` at
  the root of the page's own origin: the Moses origin and the apps origin
  both forward it to Moses. On an app's own hostname it reaches the app's own
  server until the platform routes those paths there too, and logging stays
  off meanwhile (harmless). Nothing else in the browser calls Moses.
- **Talk to the Moses window by its origin.** Never read `window.parent`
  state, never post to `'*'` or to `window.location.origin`. Post to and
  accept only from `window.MosesSDK.hostOrigin`, which the platform's iframe
  SDK sets from the release that moves apps to their own origin; before that
  release a same-origin parent is used
  (`fullstack-chat/frontend/src/moses/hostMessages.ts` is the reference).
- **Moses pages by the Moses origin.** A link or navigation to a Moses page
  (`href="/…"`, `location.assign('/…')`) lands on the apps origin after the
  switch. Build such URLs from `MosesSDK.hostOrigin`, or open them through
  the Moses window.
- **Framing comes from the platform.** The `frame-ancestors` list is
  `MOSES_EMBEDDING_ALLOWED_ANCESTORS`, which the platform fills with the Moses
  origin. Do not hard-code `frame-ancestors 'self'` or
  `X-Frame-Options: SAMEORIGIN`: on the apps origin `'self'` is the app, not
  Moses, and the app stops rendering in its window.

Data an app kept in the browser (localStorage, IndexedDB, caches, cookies) is
per origin. After the switch the app no longer sees what it stored on the
Moses origin; the data is not deleted, but users start fresh in that app.

Cookies: the edge drops any cookie an app sets with a `Domain=` covering the
Moses or apps host, and any cookie with a reserved Moses or Keycloak name. Use
host-only cookies (no `Domain=`) with names of your own.

### Updating an app made from an older template

Generated repositories are not updated by this repo. Check an existing app
for these, by hand:

1. **Browser logger** (`frontend/src/moses-browser-logger.ts`, or
   `static/moses-browser-logger.js` for `fullstack-unified`). Copy the current
   file from the same template here over yours. Old copies bootstrap without
   credentials and by baked chart/deployment ids: on the current platform
   logging stops for non-public apps and after one hour for public ones. You
   may also delete the `VITE_MOSES_CHART_ID` / `VITE_MOSES_DEPLOYMENT_ID`
   `ARG` and `RUN` lines from `frontend/Dockerfile`; they are no longer read.
   `fullstack-unified` apps: also delete the `<meta name="moses-config">` tag
   from `static/index.html` (it rendered `MOSES_API_BASE`, the Moses origin,
   which is cross-origin on the apps origin) and serve `static/` as plain
   files, as `newStaticHandler` in this template's `main.go` does.
2. **Chat bridge** (apps from `fullstack-chat`). Copy
   `frontend/src/moses/hostMessages.ts` (and its test). The old file posts to
   and accepts from `window.location.origin`, so on the apps origin "Ask Moses"
   no longer opens the chat and completion messages never arrive. Apps forked
   before 2026-11-22 (commit `0219996`) also need the current chat proxy:
   copy `backend/internal/mosesproxy/` and its wiring in
   `backend/cmd/server/main.go`. The old proxy forwards only the
   `access_token` cookie, which never reaches the app, so `/__moses/invoke`
   loses the viewer; the current one forwards the gateway's viewer
   assertion.
3. **Framing headers.** Search your nginx config, entrypoint and server code
   for `frame-ancestors`, `X-Frame-Options` and `frameguard`. Keep only the
   template's rule (`MOSES_EMBEDDING_ALLOWED_ANCESTORS`, with `'self'` as the
   standalone default); remove any hard-coded `'self'`-only or `SAMEORIGIN`
   value.
4. **Moses origin in code.** Search for your Moses hostname, `localhost`
   URLs, `window.parent`, `window.top`, `postMessage(`, root-relative
   `fetch('/` calls and root-relative links or navigation (`href="/`,
   `location.assign('/`, `location.href = '/`) that open Moses pages. Use
   relative paths for the app and `MosesSDK.hostOrigin` for Moses. The two
   intended root paths (iframe SDK, browser logger) stay as they are.
   `MOSES_API_BASE` stays fine for server-side calls from the app's backend;
   do not hand it to the browser.
5. **Cookies.** Replace any cookie set with `Domain=` (covering the Moses or
   apps host) or named like a Moses or Keycloak cookie by a host-only cookie
   with your own name; the edge drops the others after the switch.

## UserID is not authentication

The non-OIDC templates expose `X-Moses-User-ID` as **`UntrustedUserID`**
(`untrusted_user_id` in unified JSON). It is caller-supplied audit context,
never proof of identity and never a basis for access control. Tenant storage
must still use the deploy-pinned `MOSES_TENANT_ID`. Public `/health` and
OpenAPI endpoints remain public. `fullstack-simple` does not read a user ID.

The repo documents gateway authentication for `fullstack-oidc`: its
`oidcauth` package requires a non-empty `MOSES_GATEWAY_AUTH_SECRET` and a
constant-time match against `X-Moses-Gateway-Auth` before trusting user
headers. With no configured secret that trust path is disabled. The repo
does **not** establish that marker injection for non-OIDC apps, nor a
universal guarantee that every edge strips inbound `X-Moses-*` headers.
Do not infer either guarantee or invent a marker for another template.
Apps needing identity should use OIDC or another verified authentication
mechanism. A future coordinated platform contract could supply that same
secret/marker to non-OIDC apps, which could then populate a separate
verified identity while leaving unmatched requests anonymous.

## Container and chart runtime defaults

All app containers start non-root: Go uses UID/GID 1000; nginx uses the
existing image's UID/GID 101 and keeps port **8080**. nginx writes its
rendered config, copied static tree, PID and temporary files only under
`/tmp`, with logs on stdout/stderr. Helm mounts an `emptyDir` at `/tmp` and
sets read-only root filesystems. For Docker, supply a writable `/tmp`
(e.g. `--read-only --tmpfs /tmp:rw,nosuid,nodev,mode=1777`).

Charts default to `runAsNonRoot`, explicit user/group IDs, dropped `ALL`
capabilities, `allowPrivilegeEscalation: false`, `RuntimeDefault` seccomp,
and `automountServiceAccountToken: false`. Override `podSecurityContext`,
`securityContext` and `automountServiceAccountToken` in values. Multi-service
charts merge each `services[].podSecurityContext` / `securityContext` over
chart defaults; frontend entries select UID/GID 101. Preserve those entries
when replacing the services list. Optional sidecars inherit container
defaults and can override `sidecars[].securityContext` for their image.

Bundled PostgreSQL alpine uses UID/GID 70; Valkey alpine uses UID 999/GID
1000. Their corresponding `postgresql.*` / `redis.*` security values are
independently overridable. Datastore root filesystems are read-only, but
data **cannot** be: PostgreSQL mounts `/var/lib/postgresql/data`,
`/var/run/postgresql`, and `/tmp`; Valkey mounts `/data` and `/tmp`. These
are ephemeral `emptyDir` volumes; use properly owned persistent volumes
for real data. Image users and entrypoint behavior need container validation
when changing image variants/tags.

Bundled datastores consume a precreated Kubernetes Secret in the release
namespace. Moses creates the credential Secret and supplies only
`postgresql.auth.existingSecret` / `redis.auth.existingSecret` and
`auth.passwordKey` (default `password`). Standalone installs must precreate
that Secret and pass its name. Enabled services fail Helm rendering if the
reference is empty. Charts never render bundled credential Secrets or carry
password values. PostgreSQL servers and applications use the same reference;
applications receive both `DATABASE_PASSWORD` and `DB_PASSWORD`, with no
runtime password default. Valkey requires `REDIS_PASSWORD` via `secretKeyRef`
and `--requirepass`; Redis clients must authenticate with that variable.

The inter-app signing key arrives as `MOSES_INTERAPP_SECRET` through
`secrets.secretNames`. All seven charts render the optional singular Secret
and ConfigMap before that ordered Secret list. The platform appends the
canonical signing-key Secret last and rejects explicit env entries that
would override it. Do not put the signing key in `env`, `globalEnv`, or
service-local `env`. Chat mounts secrets only in backend services; its nginx
frontend is not a credential consumer. This is a clean break: deploy
installations from scratch.

nginx sends `nosniff` and `Referrer-Policy: strict-origin-when-cross-origin`,
hides its version, and applies immutable one-year caching only to
`/assets/<name>-<hash>.<extension>` (Vite's content-hash naming convention).
Unhashed public files and HTML revalidate. Reserve that naming convention
for content-hashed build outputs. CSP remains `frame-ancestors`: a generic
`script-src` / `connect-src` policy would break the inline theme bootstrap,
Moses SDK connections, or app-specific resources. Set an app-specific CSP
with nonces/hashes and explicit connection origins when those are known.

## Test surface

The repo ships repo-level test scripts (both run in CI via
`.github/workflows/build-and-test.yml`, alongside per-module `go vet`/`go
test` and per-frontend `npm run lint`/`npm run typecheck`/`npm test`/`npm run
build`):

`lint` and `typecheck` are two different programs and both are required.
`lint` is `tsc -p tsconfig.build.json --noEmit`, whose `exclude` drops
`*.test.*` / `*.spec.*` — correct for the production build, which is why the
exclude stays. `typecheck` is `tsc -p tsconfig.json --noEmit`, which keeps
`"include": ["src"]` and therefore *does* type-check the test files. Without
it they are checked by nothing: `npm test` runs vitest, and vitest transpiles
TypeScript, which strips types without ever verifying them.

```bash
./tests/test_nginx_entrypoint.sh
```

Renders each Moses-aware template's `nginx.conf` template under several
`MOSES_EMBEDDING_FRAMING` combinations and asserts the resulting CSP +
`X-Frame-Options` headers match the framing matrix. It runs the real
entrypoints against immutable fixture inputs, verifies writable output and
static-file preservation, and stubs only nginx itself. Requires Python 3
and `envsubst`
(part of `gettext` on most distros).

```bash
./tests/test_template_parity.sh
```

Codifies the cross-template uniformity rules: Helm chart naming, app-config
placeholders and reserved slugs, README presence, Go/Docker base-image
version parity, Dockerfile hardening and reproducibility, no literal
credentials in values files, and `.dockerignore` coverage per declared build
context.

`tests/test_runtime_hardening.py` extends parity with runtime wiring,
identity naming, error disclosure, nginx caching, chart defaults and
maintenance assertions. `tests/test_helm_hardening.sh` lints and renders all
seven charts, verifies security/volumes/service ports, tests required Secret
references and signing-key source ordering, and exercises value overrides
(Helm + Ruby's standard YAML library required).
Parity runs these checks; Helm rendering is skipped explicitly if Helm is absent.

The `tools/check-vendored-*.sh` gates (csrf, browser-logger, mosesproxy,
base-path, runtime) additionally keep each template's vendored copies in sync.
Runtime middleware uses `backend-template` (body limits) and `fullstack-chat`
(logging) as canonical copies; the other gates use `shared/` sources.
Runtime parity runs from the template-parity job; the existing shared-source
gates retain their path-filtered workflows.

## Dependency updates (Renovate)

`.github/renovate.json5` keeps the templates from rotting — a stale template
does not just sit there, it keeps minting stale repos through
`moses_init_repo`. It covers npm across the five frontends and Go modules
across the eight modules in `go.work`, plus Dockerfile base images and
GitHub Actions. Updates are grouped so the templates move in **lockstep**
(the five frontends use matching versions for shared dependencies; splitting them into
five PRs would leave the set permanently out of sync): one PR for all npm
minor/patch, one PR per npm major spanning every frontend, and the same
shape for Go. Weekly, Monday morning, with a 7-day minimum release age;
security fixes bypass both.

**The Go toolchain and parity-pinned base images are deliberately NOT updated by
Renovate.** `go 1.27.4` / `golang:1.27.4-alpine3.24` / `alpine:3.24.4` are pinned
in all eight `go.mod` files, `go.work`, the Dockerfiles, *and* in `EXPECTED_GO_VERSION` /
`EXPECTED_GOLANG_TAG` / `EXPECTED_ALPINE_TAG` inside
`tests/test_template_parity.sh` — no dependency manager can edit that last
one, so any bot-authored bump lands a PR that cannot pass its own parity
gate. The pin is also a cross-repo contract with `moses-platform-prep`
(same patch in its `backend/go.mod` and agent-base image). Bumping it is a
deliberate, coordinated change; the procedure is written out in
`renovate.json5`'s `PINNED` block. Renovate still surfaces new releases on
the Dependency Dashboard.

The `go 1.27.4` directive sets the minimum compiler version; it is not an
exact toolchain lock. Older developer toolchains must upgrade (or allow Go
toolchain selection). The Docker builder selects the versioned image above.

Frontend builders use `node:26.13.1-alpine3.24`, with CI reading the same
Node patch from `.node-version`; nginx uses the stable `nginx:1.30.7-alpine3.24`.
Parity rules 5 and 15 check these image tags, `npm ci` in each frontend
Dockerfile, shared direct-dependency versions, and package/lockfile agreement.
The Dockerfile check guards against regressions to `npm install`; it does not
prove shell execution or that dependencies were installed.
Update image expectations and `.node-version` together when changing Node.
CI also runs standalone `govulncheck` for all eight Go modules and
`npm audit --audit-level=high` (including development dependencies) for each
frontend. All third-party Actions are pinned to reviewed commit SHAs.

The config is schema-validated on every PR by the `contract (renovate
config)` CI job (`renovate-config-validator --strict`).

## License

Apache License 2.0 — see [LICENSE](LICENSE)

**Exceptions** (files that carry their own license headers — do not
re-license or strip them): `shared/browser-logger/index.ts` and its
vendored copies (`*/src/moses-browser-logger.ts` plus the vanilla-JS port
`fullstack-unified/static/moses-browser-logger.js`), and four
`fullstack-chat` Go files (`backend/cmd/server/validate_env.go`,
`validate_env_test.go`, `main_test.go`,
`moses_proxy_integration_test.go`) are licensed under the Business Source
License 1.1 (BSL-1.1), as marked in their headers.
