# Moses Platform Showcase

**Interactive demonstration of Moses platform capabilities**

A fullstack application showcasing Moses platform features including MCP tools, deployment pipelines, authentication, multi-tenancy, and OpenAPI integration. Built with Go backend + React frontend deployed as multi-service Helm chart.

## What This Demonstrates

### Platform Features
- **MCP Tools**: 56+ tools across 5 profiles with unified CRUD and workspace tool proxy
- **Deployment Pipelines**: Agent execution and workspace tools with in-cluster image builds
- **Authentication**: OIDC/JWT, API keys, OAuth tokens, and forward auth
- **Multi-Tenancy**: Complete isolation with 4-tier RBAC
- **OpenAPI Discovery**: Automatic endpoint discovery and MCP tool generation

### Technical Implementation
- **Multi-Service Deployment**: Frontend (nginx+React) + Backend (Go) in single Helm chart
- **Moses Integration**: Header extraction, tenant scoping, context awareness
- **K8s Service Mesh**: Frontend→Backend communication via DNS
- **Production-Ready**: Health checks, CORS, structured logging, graceful shutdown

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Kubernetes Cluster                      │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────────────┐      ┌──────────────────────┐    │
│  │  Frontend Service    │      │  Backend Service     │    │
│  │  (nginx:alpine)      │─────▶│  (Go + pgx v5)       │    │
│  │  Port: 8080           │      │  Port: 8080          │    │
│  │  - React 19 SPA      │      │  - OpenAPI endpoints │    │
│  │  - nginx proxy       │      │  - Moses headers     │    │
│  │  - Health: /health   │      │  - Health: /health   │    │
│  └──────────────────────┘      └──────────────────────┘    │
│           │                              │                   │
│           │ K8s Service DNS              │                   │
│           │ agent-deployed-app-backend   │                   │
│           └──────────────────────────────┘                   │
│                                                               │
│  Environment Variables (Runtime Injection):                  │
│  - BACKEND_SERVICE_HOST: "agent-deployed-app-backend"       │
│  - BACKEND_SERVICE_PORT: "8080"                             │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

### Frontend Stack
- **Framework**: React 19 + TypeScript (React Compiler enabled via `babel-plugin-react-compiler`)
- **Build**: Vite 7
- **Data layer**: TanStack Query (`src/api/` — client + hooks + queryKeys factory)
- **Routing**: react-router-dom (7 pages)
- **Server**: nginx with runtime env var substitution
- **Styling**: Moses design tokens (4px grid, system font stack, indigo primary)

### Backend Stack
- **Language**: Go 1.25 (go.mod `1.25.10`)
- **Dependencies**: net/http mux + pgx v5 (`jackc/pgx/v5/stdlib`) for PostgreSQL
- **Runtime**: alpine:3.21
- **Middleware**: Moses headers, CSRF rejection, CORS allowlist (off by default), logging
- **API**: OpenAPI 3.0.3 spec at `/api/openapi.json`

### Communication Pattern
```
Browser → Frontend nginx:8080 → Proxy /api/* → Backend:8080
                                      ↓
                        Moses Headers (X-Moses-Tenant-ID, etc.)
                        — caller-context only post-CHAT-pxeo.12 —
```

**Tenant identity (CHAT-pxeo.12).** The backend reads its self-tenant
from the `MOSES_TENANT_ID` env var (via `internal/config.SelfTenantID()`)
and uses it as the authoritative storage/lookup key for the `notes`
table. The `X-Moses-Tenant-ID` header is preserved as caller context
(`MosesContext.CallerTenantID`) for audit and the 403 cross-check on
writes — `{"error":"tenant_mismatch","code":"E_TENANT_MISMATCH"}` is
returned when a request supplies a non-empty header that disagrees with
the deploy-pinned env. Toggle the cross-check via
`MOSES_STRICT_TENANT_CHECK=false`. On a deployed pod (`MOSES_DEPLOYED=1`)
the server fail-fast panics if `MOSES_TENANT_ID` is unset; in local dev
it falls back to the sentinel `local-dev`.

**Future schema tightening (CHAT-suvi).** The `notes.tenant_id` column is
currently `TEXT NOT NULL DEFAULT ''` to accommodate the `local-dev`
sentinel string. If a future template revision tightens the column to
`UUID NOT NULL`, any rows written under the sentinel must first be
re-tenanted to a real UUID (or purged) — otherwise the type cast will
reject them. The boot-time fix-up in `internal/database/db.go` that
re-writes legacy `'local-dev' | 'default' | ''` rows is the migration
hook for that future change.

## Running Locally

### Option 1: Docker Compose (Recommended)

```bash
# Frontend
cd frontend
docker build -t showcase-frontend .
docker run -p 3000:8080 \
  -e BACKEND_SERVICE_HOST=host.docker.internal \
  -e BACKEND_SERVICE_PORT=8080 \
  showcase-frontend

# Backend (separate terminal)
cd backend
docker build -t showcase-backend .
docker run -p 8080:8080 showcase-backend

# Access: http://localhost:3000
```

### Option 2: Development Mode

```bash
# Backend (terminal 1)
cd backend
go run cmd/server/main.go
# Listening on :8080

# Frontend (terminal 2)
cd frontend
npm install
npm run dev
# Access: http://localhost:3000 (Vite proxy configured)
```

## Moses Deployment

### Via Agent Execution Pipeline

When an agent completes work via `moses_agent_submit_completed`:

1. **Config Detection**: `moses-app.config.json` parsed
2. **Image Builds** (in-cluster):
   - `fullstack-showcase-frontend:latest`
   - `fullstack-showcase-backend:latest`
3. **Helm Deploy**: Multi-service chart with:
   - Frontend deployment + service (port 8080)
   - Backend deployment + service (port 8080)
   - Environment variable injection
4. **Health Verification**: Both services must pass probes
5. **OpenAPI Discovery**: Backend endpoints probed
6. **MCP Tools Generated** (one per OpenAPI operationId — `/health` is
   deliberately NOT in the spec, so no phantom health tool is registered):
   - `workspace_showcase_getMosesInfo`
   - `workspace_showcase_listCapabilities`
   - `workspace_showcase_getCapability`
   - `workspace_showcase_getUsers`
   - `workspace_showcase_listNotes` / `createNote` / `getNote` / `deleteNote`

### Via Workspace Tools Pipeline

If registered in marketplace:

1. **Git Clone**: Shallow clone from registry URL
2. **Config Validation**: `moses-app.config.json` required
3. **Multi-Image Build**: The in-cluster image builder builds all images in parallel
4. **Helm Deploy**: Services deployed to tenant namespace
5. **OpenAPI Discovery**: 11 standard paths probed
6. **Dynamic MCP Tools**: WorkspaceToolProxy routes calls

### Accessing Deployed App

```bash
# Get ingress URL
kubectl get ingress -n {tenant-namespace}

# Example URL
https://moses-manager.eu/{tenant-id}/apps/{chart-id}/fullstack-showcase/

# Moses headers automatically injected:
# X-Moses-Tenant-ID: {tenant-uuid}
# X-Moses-User-ID: {user-uuid}
# X-Moses-Chart-ID: {chart-uuid}
```

## File Structure

```
fullstack-showcase/
├── moses-app.config.json          # V2 config: hybrid app, 2 docker files, postgres dep
├── moses-app.config.with-secrets.example.json  # secrets.external[] example
├── clean_out_template.sh          # One-shot demo strip (see skills/usage.md)
├── .template-clean/               # Clean twins the script copies over mixed files
├── backend/
│   ├── Dockerfile                 # Multi-stage Go build (golang:1.25-alpine → alpine:3.21)
│   ├── go.mod                     # Go 1.25.10, pgx v5
│   ├── cmd/server/
│   │   ├── main.go                # Server entry + buildMux (health/openapi dual-mount)
│   │   ├── demo_routes.go         # ALL demo route registration (cleanout swaps this)
│   │   └── main_test.go           # Path-contract + spec↔mux consistency tests
│   ├── internal/
│   │   ├── config/moses.go        # MOSES_TENANT_ID deploy-pin (CHAT-pxeo.12)
│   │   ├── database/
│   │   │   ├── db.go              # Connect/retry/28P01 fail-fast plumbing
│   │   │   └── migrate_demo.go    # Demo schema + tenant rewrite (cleanout swaps this)
│   │   ├── handler/               # HTTP handlers
│   │   │   ├── health.go          # /health (dual-mounted)
│   │   │   ├── openapi.go         # Serves embedded spec
│   │   │   ├── tenant.go          # enforceTenantMatch 403 cross-check (survives cleanout)
│   │   │   ├── moses_info.go      # Demo: Moses context echo
│   │   │   ├── capabilities.go    # Demo: capability catalog
│   │   │   ├── notes.go           # Demo: tenant-scoped CRUD on Postgres
│   │   │   └── users.go           # Demo: MOSES_PLATFORM_API_KEY consumption
│   │   ├── middleware/            # Moses headers, CSRF (vendored), CORS, logging
│   │   └── model/                 # Demo capability data model
│   └── api/openapi.json           # OpenAPI 3.0.3 spec (embedded via api/api.go)
├── frontend/
│   ├── Dockerfile                 # Multi-stage React build (deps layer cached)
│   ├── nginx.conf                 # Template with ${ENV_VARS} + CSP
│   ├── entrypoint.sh              # Renders CSP + sub-path blocks at start
│   ├── package.json
│   ├── vite.config.ts             # React Compiler enabled
│   ├── index.html                 # moses-base-path meta + theme init
│   ├── src/
│   │   ├── main.tsx               # Router basename + browser logger install
│   │   ├── moses-browser-logger.ts  # Vendored BLF-B reporter
│   │   ├── App.tsx                # Router + routes (MOSES ROUTING contract)
│   │   ├── App.css                # Moses design tokens
│   │   ├── styles/theme.css
│   │   ├── api/                   # TanStack Query data layer
│   │   │   ├── client.ts          # Typed fetch transport (+ getErrorMessage)
│   │   │   ├── hooks.ts           # useQuery/useMutation hooks
│   │   │   ├── queryKeys.ts       # Central query-key factory
│   │   │   └── queryClient.ts     # Shared client (staleTime/retry defaults)
│   │   ├── utils/baseUrl.ts
│   │   ├── components/
│   │   │   ├── Layout.tsx         # App shell + sidebar nav
│   │   │   ├── ThemeToggle.tsx
│   │   │   ├── FeatureCard.tsx    # Demo
│   │   │   ├── FlowDiagram.tsx    # Demo
│   │   │   ├── NotesPanel.tsx     # Demo: live query + mutation
│   │   │   └── UserList.tsx       # Demo: platform users table
│   │   └── pages/                 # Demo: 6 showcase pages (+ CSS)
│   └── public/favicon.svg
├── helm/
│   ├── Chart.yaml
│   ├── values.yaml               # Multi-service + postgresql block
│   └── templates/
│       ├── _helpers.tpl
│       ├── deployment.yaml       # Loop over services + env auto-injection
│       ├── service.yaml          # ClusterIP per service
│       └── postgresql.yaml       # Per-app Postgres (emptyDir — ephemeral!)
├── skills/
│   ├── usage.md                 # START HERE: cleanout + env contract + patterns
│   ├── api-integration.md       # Agent skill: Moses integration patterns
│   ├── secrets-tutorial.md      # Runtime secrets declaration
│   └── showcase-overview.md     # Demo: what the showcase shows
└── README.md                     # This file
```

## Customization Guide

### Adding a New Page

1. **Create Page Component**: `frontend/src/pages/NewPage.tsx`
2. **Add Route**: Update `App.tsx` routes
3. **Add Navigation**: Update `Layout.tsx` navItems
4. **Create Styles**: `frontend/src/pages/NewPage.css`

### Adding a Backend Endpoint

1. **Create Handler**: `backend/internal/handler/new_feature.go`
2. **Register Route**: `cmd/server/main.go` mux.HandleFunc
3. **Update OpenAPI**: Add to `api/openapi.json` with operationId
4. **Add Client Function**: `frontend/src/api/client.ts`

### Changing Resource Limits

Update `helm/values.yaml`:
```yaml
services:
  - name: backend
    resources:
      limits:
        cpu: "500m"      # Increase CPU
        memory: "512Mi"  # Increase memory
```

### Adding Environment Variables

1. **Backend**: Add to `helm/values.yaml` under service env
2. **Frontend**: Add to env and update `entrypoint.sh` envsubst
3. **Access**: `os.Getenv("VAR_NAME")` (Go) or `import.meta.env` (Vite)

Declaring runtime secrets — see [skills/secrets-tutorial.md](skills/secrets-tutorial.md).

## Moses Integration Patterns

### Moses Headers Middleware

```go
// Extract Moses context from headers
type MosesContext struct {
    TenantID  string
    UserID    string
    ChartID   string
    // ... more fields
}

func MosesHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        mosesCtx := MosesContext{
            TenantID: r.Header.Get("X-Moses-Tenant-ID"),
            // ... extract all headers
        }
        ctx := context.WithValue(r.Context(), mosesContextKey, mosesCtx)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### Tenant-Scoped Queries

```go
// Always filter by tenant_id
query := `SELECT * FROM resources WHERE id = $1 AND tenant_id = $2`
db.Query(ctx, query, resourceID, mosesCtx.TenantID)
```

### OpenAPI → MCP Tools

```json
// OpenAPI spec with operationId
{
  "paths": {
    "/api/v1/capabilities": {
      "get": {
        "operationId": "listCapabilities",  // ← Tool name generated
        "summary": "List capabilities"
      }
    }
  }
}

// Moses generates: workspace_showcase_listCapabilities
```

## Testing the Deployment

```bash
# 1. Check pods are running
kubectl get pods -n {namespace} | grep fullstack-showcase

# 2. Test backend health
kubectl port-forward -n {namespace} svc/agent-deployed-app-backend 8080:8080
curl http://localhost:8080/health

# 3. Test frontend health
kubectl port-forward -n {namespace} svc/agent-deployed-app-frontend 8080:8080
curl http://localhost:8080/health

# 4. Check frontend→backend communication
kubectl port-forward -n {namespace} svc/agent-deployed-app-frontend 8080:8080
curl http://localhost:8080/api/v1/moses-info

# 5. Verify OpenAPI discovery
curl http://localhost:8080/api/openapi.json
```

## Common Issues

### Frontend Can't Reach Backend

**Symptom**: API calls fail with CORS or connection errors

**Fix**: Check environment variables in frontend pod
```bash
kubectl exec -n {namespace} {frontend-pod} -- env | grep BACKEND
# Should show:
# BACKEND_SERVICE_HOST=agent-deployed-app-backend
# BACKEND_SERVICE_PORT=8080
```

### Missing Moses Headers

**Symptom**: `deployment_mode: "standalone"` always shown

**Fix**: Ensure app is accessed via Moses ingress, not direct K8s service
```bash
# WRONG: Direct service access
kubectl port-forward svc/agent-deployed-app-frontend 8080:8080

# RIGHT: Via Moses ingress
https://moses-manager.eu/{tenant-id}/apps/{chart-id}/fullstack-showcase/
```

### Health Probes Failing

**Symptom**: Pods restarting, not becoming Ready

**Fix**: Check health endpoint and probe configuration
```bash
kubectl logs -n {namespace} {pod-name}
# Should show: GET /health 200 OK

# Check probe timing
kubectl describe pod -n {namespace} {pod-name}
# Adjust initialDelaySeconds if needed
```

## Production Considerations

### Security
- All user data filtered by `tenant_id`
- RBAC checks on sensitive operations
- CORS restricted to known origins
- No secrets in code or config files
- Health endpoints public, data endpoints protected

### Performance
- Static asset caching (1 year)
- Gzip compression in nginx
- Health probes exclude access logs
- Resource limits prevent runaway pods

### Observability
- Structured logging with request IDs
- Health endpoints for monitoring
- Graceful shutdown (30s timeout)
- Error responses with context

## Learning Resources

### Moses Documentation
- **Architecture**: See `arch.md` for platform overview
- **Deployment**: See `docs/deployment/` for pipeline details
- **MCP Tools**: See `backend/internal/mcp/` for tool implementation
- **Multi-Tenancy**: See `arch/security/` for RBAC model

### This Showcase
- **Overview Page**: Platform introduction + Moses context
- **MCP Tools Page**: Tool system architecture
- **Deployment Page**: Build + deploy pipelines
- **Auth Flow Page**: Authentication methods
- **Multi-Tenancy Page**: RBAC + isolation layers
- **API Examples Page**: Interactive endpoint testing

## Contributing

When enhancing this showcase:

1. **Follow Moses Standards**:
   - Backend: See `coding-standards/MOSES_BACKEND_STANDARDS.md`
   - Frontend: See `coding-standards/MOSES_UI_UX_STANDARDS.md`

2. **Update Documentation**:
   - Keep README in sync with changes
   - Update skills/ files for agents
   - Maintain OpenAPI spec accuracy

3. **Test Multi-Service**:
   - Verify frontend→backend communication
   - Check Moses headers extraction
   - Test OpenAPI tool generation

## License

Part of the Moses ecosystem. See main repository for license information.

---

**Built with Moses Platform** | React + Go + Kubernetes
