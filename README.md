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
| **ideas-notepad** | frontend | Multi-tab note-taking app with AI refinement support. |

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
- **Quick notes app**: `ideas-notepad`

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

All 5 default templates ship as **Moses-aware** — they declare
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
