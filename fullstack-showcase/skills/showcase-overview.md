# Moses Platform Showcase - Agent Skill

## Overview

This showcase application demonstrates Moses platform capabilities:
- **MCP Tools**: 56+ tools across 5 profiles (external-minimal, moses-manager-full, agent-execution, build-callback, autonomous)
- **Deployment Pipelines**: Agent execution and workspace tools with in-cluster image builds + Helm
- **Authentication**: OIDC/JWT, API keys, OAuth tokens, forward auth
- **Multi-Tenancy**: Complete isolation with 4-tier RBAC (GlobalAdmin, TenantAdmin, Editor, Viewer)
- **OpenAPI Discovery**: Auto-probe endpoints and generate dynamic MCP tools

## Architecture

### Frontend
- **Tech**: React 18 + TypeScript + Vite
- **Deployment**: nginx:alpine with entrypoint.sh for env var substitution
- **Routing**: SPA with react-router-dom, 6 pages
- **API Proxy**: nginx proxies `/api/*` to backend service

### Backend
- **Tech**: Go 1.22 with stdlib (no external dependencies for HTTP)
- **Deployment**: alpine:3.19 runtime
- **Endpoints**: /health, /api/v1/moses-info, /api/v1/capabilities, /api/v1/capabilities/{id}
- **Middleware**: Moses headers extraction, CORS, logging

### Multi-Service Helm
- **Frontend Service**: agent-deployed-app-frontend:8080
- **Backend Service**: agent-deployed-app-backend:8080
- **Communication**: Frontend → Backend via K8s DNS
- **Environment**: BACKEND_SERVICE_HOST/PORT injected at runtime

## Key Features for Agents

### Moses Context
The app demonstrates Moses header integration:
- Extracts X-Moses-Tenant-ID, X-Moses-User-ID, X-Moses-Chart-ID, etc.
- Shows "standalone" vs "mcp-proxy" deployment modes
- Displays current tenant/user context on Overview page

### Educational Content
Each page explains a Moses platform component:
1. **Overview**: Hero, context display, feature cards
2. **MCP Tools**: CRUD tools, workspace tools, profiles, protocol flow
3. **Deployment**: Agent execution pipeline, workspace tools, in-cluster image builds
4. **Auth Flow**: Authentication methods, headers table, live context
5. **Multi-Tenancy**: Isolation layers, role hierarchy, namespace security
6. **API Examples**: Interactive endpoint testing, OpenAPI → MCP explanation

## When to Reference This Showcase

- **Learning Moses Architecture**: Direct users to specific pages for deep dives
- **Integration Examples**: Reference moses-info endpoint for header extraction patterns
- **Multi-Service Deployments**: Show helm/ structure for frontend+backend apps
- **OpenAPI → MCP**: Explain how workspace tools generate dynamic MCP tools

## API Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| /health | GET | Service health check |
| /api/v1/moses-info | GET | Current Moses context from headers |
| /api/v1/capabilities | GET | List all Moses capabilities |
| /api/v1/capabilities/{id} | GET | Get single capability details |

All endpoints return JSON. The moses-info endpoint shows deployment mode and extracts all X-Moses-* headers.
