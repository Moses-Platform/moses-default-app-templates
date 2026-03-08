# Fullstack Simple - Agent Skill

## Overview

Minimal Go + React fullstack template. No database, no router — a clean starting point.

## Structure

- **Backend**: Go stdlib HTTP server on port 8080
  - `/health` — health check
  - `/api/v1/status` — app status with Moses context
  - `/api/openapi.json` — OpenAPI spec for MCP tool generation
- **Frontend**: React 18 + Vite, served via nginx on port 8080
  - Single page that calls `/api/v1/status` and displays the result
  - nginx proxies `/api/*` to the backend service

## Extending

1. Add a handler in `backend/internal/handler/`
2. Register the route in `backend/cmd/server/main.go`
3. Add the endpoint to `backend/api/openapi.json`
4. Call the new endpoint from `frontend/src/App.tsx`

## Moses Integration

The backend extracts Moses headers (X-Moses-Tenant-ID, X-Moses-User-ID, etc.) and returns them in the `/api/v1/status` response. This demonstrates Moses context awareness.
