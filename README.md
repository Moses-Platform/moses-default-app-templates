# Moses Default App Templates

Self-contained, fully runnable application templates for the [Moses Platform](https://github.com/Moses-Platform). Each subdirectory is a complete project that can be deployed to Moses via agent execution or manual workflow.

## Templates

| Template | Type | Description |
|----------|------|-------------|
| **frontend-template** | frontend | React 18 + Vite + TypeScript served via nginx. Health check, Helm chart, single container. |
| **backend-template** | api | Go stdlib HTTP server with OpenAPI spec, health endpoint, and MCP auto-discovery. |
| **fullstack-simple** | hybrid | Separate React frontend (nginx) + Go backend containers. Nginx proxies API calls. |
| **fullstack-unified** | hybrid | Single Go binary serving static frontend via `go:embed` + API endpoints. Simplest fullstack pattern. |
| **fullstack-showcase** | hybrid | Feature-rich Go + React demo with PostgreSQL, multiple API endpoints, and 6 UI pages. |
| **fullstack-chat** | hybrid | Go + React reference app exercising every Moses Manager integration surface (app actions, workspace-tool callbacks, completion webhooks). |
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
- **Educational demo with database**: `fullstack-showcase`
- **App that authenticates real users via OIDC**: `fullstack-oidc`

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

All 6 default templates ship as **Moses-aware** — they declare
`templateApiVersion: "moses.ai/v1"` in `moses-app.config.json` and serve
under the runtime `MOSES_BASE_PATH` natively, emitting
`Content-Security-Policy: frame-ancestors` themselves per the `embedding`
block. The platform deploys them WITHOUT the URL-rewrite + X-Frame-Options
strip safety net.

When authoring a custom template, you have two options:

- **Easiest path (legacy)**: omit `templateApiVersion`. The Moses platform
  URL-rewrites the `/apps/<tenant>/<slug>/` prefix to `/` before forwarding
  and strips the upstream `X-Frame-Options` header so the iframe renders.
  Your code can stay completely unaware of the public path.
- **Moses-aware**: declare `templateApiVersion: "moses.ai/v1"` and the
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
| `/api/openapi.json` (your declared `specPath`) | the **root** `/api/openapi.json` **and** `{MOSES_BASE_PATH}/api/openapi.json` (cost-free) | platform OpenAPI discovery (canonical) + the Phase-2 reachability probe (prefixed) |

### Rules

1. **Register all API routes ONCE, under `MOSES_BASE_PATH`.** This is the API's
   natural browser-facing home, and the platform's workspace-tool proxy now
   calls the API there too — so a single registration serves both callers. Do
   **not** also mount the API canonically; the old copy-pasted
   `registerAPI(prefix)` closure that registered every route twice (canonical +
   prefixed) has been removed from all six templates.
2. **`/health` at the root `/health`.** K8s liveness/readiness probes call the
   pod IP directly, bypassing the ingress. Templates additionally mount
   `{MOSES_BASE_PATH}/health` (one trivial extra handler) so an app that
   accidentally serves everything under the base path still passes the probe.
3. **`/api/openapi.json` at the root AND under `MOSES_BASE_PATH`.** The platform
   discovery hook fetches it canonically; the base-path alias keeps it reachable
   through the frontend nginx (e.g. for the Phase-2 reachability probe). Like
   `/health`, this is a cost-free second handler for the same static spec.
4. **Frontends use relative fetch paths** (`fetch('api/v1/...')`) so the browser
   resolves them against the prefixed page URL.
5. **Multi-container templates: the frontend nginx forwards `/api/` (and
   `/__moses/`) to the backend with `proxy_pass` carrying NO URI part** — that
   preserves the `MOSES_BASE_PATH` prefix end-to-end. A URI part strips it and
   the backend 404s; `tests/test_nginx_entrypoint.sh` guards the regression
   (CHAT-yfmwv).

This is the *current* reality, not a plan: the workspace-tool proxy calls the
app API under `MOSES_BASE_PATH` (platform fix CHAT-uzu24), all six templates
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

The platform's post-deploy reachability gate (CHAT-iu0qa) probes the app
through its real ingress URL and reports a mis-mounted app as a failed deploy —
the safety net behind this contract.

### Standalone vs Moses-managed embedding defaults

When `MOSES_EMBEDDING_FRAMING=moses-only` and `MOSES_EMBEDDING_ALLOWED_ANCESTORS`
is empty, each template's `entrypoint.sh` defaults to the same chart-parity
origin list the platform's resolver emits
(`moses-platform-prep/backend/internal/services/embedding_policy_resolver.go`):
`'self' tauri://localhost http://tauri.localhost https://tauri.localhost`,
plus `https://*.${MOSES_DOMAIN}` when `MOSES_DOMAIN` is set. This keeps
`helm install` of a Moses-aware template **outside** the Moses platform
embeddable from Moses Manager and from the Tauri installer shell — without
the default a standalone deploy would only allow `'self'` and Manager would
get blocked. When the platform deploys the template it supplies a concrete
`MOSES_EMBEDDING_ALLOWED_ANCESTORS` (resolver output), and the explicit
override wins verbatim — no auto-merge with the standalone default.

## Test surface

The repo ships a smoke test for the nginx entrypoint logic:

```bash
./tests/test_nginx_entrypoint.sh
```

It renders each Moses-aware template's `nginx.conf` template under several
`MOSES_EMBEDDING_FRAMING` combinations and asserts the resulting CSP +
`X-Frame-Options` headers match the framing matrix. Requires `envsubst`
(part of `gettext` on most distros).

## License

Apache License 2.0 — see [LICENSE](LICENSE)
