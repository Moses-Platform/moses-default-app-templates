# Fullstack Simple - Agent Skill

## Overview

Minimal Go + React fullstack template. No database, no router — a clean starting point with a working CRUD example.

## Structure

- **Backend**: Go stdlib HTTP server on port 8080
  - `/health` — health check
  - `/api/v1/status` — app status with Moses context
  - `/api/v1/items` — CRUD example (GET list, POST create)
  - `/api/v1/items/{id}` — CRUD example (DELETE)
  - `/api/openapi.json` — OpenAPI spec for MCP tool generation
- **Frontend**: React 18 + Vite, served via nginx on port 8080
  - Single page that displays backend status and an interactive items list
  - nginx proxies `/api/*` to the backend service

## CRUD Endpoints

The template includes a working in-memory CRUD example with items scoped per tenant:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/items` | List all items for the current tenant |
| POST | `/api/v1/items` | Create an item (`{"title": "..."}`) — returns the created item with UUID and timestamp |
| DELETE | `/api/v1/items/{id}` | Delete an item by UUID |

Items are stored in memory using `sync.RWMutex` and are scoped by the `X-Moses-Tenant-ID` header that Moses injects automatically. Data resets on pod restart — for persistent storage, see the fullstack-showcase template.

## API Path Convention

**All frontend fetch calls must use relative paths** (no leading `/`):

```typescript
// Correct — works behind subpath ingress
fetch('api/v1/items')

// Wrong — resolves to platform backend, not app backend
fetch('/api/v1/items')
```

The Vite config uses `base: './'` which makes relative URLs resolve against the current page URL. When deployed behind a subpath (e.g. `/apps/tenant/app-slug-frontend/`), relative paths correctly route through the app's own nginx proxy to the backend.

## Extending

1. Add a handler in `backend/internal/handler/`
2. Register the route in `backend/cmd/server/main.go`
3. Add the endpoint to `backend/api/openapi.json`
4. Call the new endpoint from `frontend/src/App.tsx` using **relative paths**

## Moses Integration

The backend extracts Moses headers (X-Moses-Tenant-ID, X-Moses-User-ID, etc.) and returns them in the `/api/v1/status` response. The items CRUD endpoints use X-Moses-Tenant-ID to scope data per tenant, demonstrating Moses multi-tenancy.
