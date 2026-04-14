# Fullstack Unified - Agent Skill

## First Steps

Update `moses-app.config.json` with your app's identity before committing:

```json
{
  "name": "your-app-name",
  "displayName": "Your App Name",
  "description": "What your app actually does"
}
```

The `name` field becomes the Helm release name and MCP tool prefix. The `docker`, `services`, and `validation` sections are pre-configured — only change them if you modify the project structure.

## Overview

Single Go binary serving both static frontend files and API endpoints. One container, one Dockerfile — the simplest fullstack pattern for Moses.

## Architecture

Unlike `fullstack-simple` (separate frontend + backend containers with nginx proxy), this template embeds static files directly into the Go binary using `//go:embed`. One container handles everything.

## Structure

- **main.go**: Single Go entry point using stdlib `net/http`
  - `/health` — health check (always at root for Kubernetes probes)
  - `/api/v1/status` — app status with Moses context headers
  - `/api/openapi.json` — OpenAPI spec for MCP tool generation
  - `/*` — static file serving via `embed.FS`
- **static/**: Frontend files embedded at compile time
  - `index.html`, `style.css`, `app.js`, `favicon.svg`
- **api/openapi.json**: OpenAPI 3.0 spec (also embedded)
- **Dockerfile**: Multi-stage build (golang → alpine)
- **helm/**: Standard Moses Helm chart (single service)

## Extending

1. Add a handler function in `main.go`
2. Register the route on the mux: `mux.HandleFunc(baseURL+"/api/v1/yourEndpoint", handler)`
3. Add the endpoint to `api/openapi.json`
4. Add frontend UI in `static/` (vanilla HTML/CSS/JS, or add a build step)

## When to Use

- Apps where frontend is simple enough for vanilla HTML/JS
- When you want minimal container count and complexity
- When the Go backend should serve its own UI
- Prototypes and MVPs

## When NOT to Use

- Apps requiring React, Vue, or other framework builds (use `fullstack-simple` instead)
- Apps needing separate frontend scaling
- Apps where frontend and backend have different release cycles

## Moses Integration

The backend extracts Moses headers (X-Moses-Tenant-ID, X-Moses-User-ID, X-Moses-Chart-ID, X-Moses-Request-ID) and returns them in the `/api/v1/status` response. BASE_URL environment variable is respected for ingress path prefixing.
