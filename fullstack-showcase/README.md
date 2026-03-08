# Moses Platform Showcase

**Interactive demonstration of Moses platform capabilities**

A fullstack application showcasing Moses platform features including MCP tools, deployment pipelines, authentication, multi-tenancy, and OpenAPI integration. Built with Go backend + React frontend deployed as multi-service Helm chart.

## What This Demonstrates

### Platform Features
- **MCP Tools**: 56+ tools across 5 profiles with unified CRUD and workspace tool proxy
- **Deployment Pipelines**: Agent execution and workspace tools with Kaniko builds
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
│  │  (nginx:alpine)      │─────▶│  (Go + stdlib)       │    │
│  │  Port: 8080           │      │  Port: 8080          │    │
│  │  - React 18 SPA      │      │  - OpenAPI endpoints │    │
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
- **Framework**: React 18 + TypeScript
- **Build**: Vite 5
- **Routing**: react-router-dom (6 pages)
- **Server**: nginx with runtime env var substitution
- **Styling**: Moses design tokens (4px grid, Inter font, indigo primary)

### Backend Stack
- **Language**: Go 1.22
- **Dependencies**: stdlib only (net/http)
- **Runtime**: alpine:3.19
- **Middleware**: Moses headers, CORS, logging
- **API**: OpenAPI 3.0.3 spec at `/api/openapi.json`

### Communication Pattern
```
Browser → Frontend nginx:8080 → Proxy /api/* → Backend:8080
                                      ↓
                        Moses Headers (X-Moses-Tenant-ID, etc.)
```

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
2. **Kaniko Builds**:
   - `fullstack-showcase-frontend:latest`
   - `fullstack-showcase-backend:latest`
3. **Helm Deploy**: Multi-service chart with:
   - Frontend deployment + service (port 8080)
   - Backend deployment + service (port 8080)
   - Environment variable injection
4. **Health Verification**: Both services must pass probes
5. **OpenAPI Discovery**: Backend endpoints probed
6. **MCP Tools Generated**:
   - `workspace_showcase_healthCheck`
   - `workspace_showcase_getMosesInfo`
   - `workspace_showcase_listCapabilities`
   - `workspace_showcase_getCapability`

### Via Workspace Tools Pipeline

If registered in marketplace:

1. **Git Clone**: Shallow clone from registry URL
2. **Config Validation**: `moses-app.config.json` required
3. **Multi-Image Build**: Kaniko builds all images in parallel
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
├── moses-app.config.json          # V2 config: hybrid app, 2 docker files
├── backend/
│   ├── Dockerfile                 # Multi-stage Go build
│   ├── go.mod                     # Go 1.22, stdlib only
│   ├── cmd/server/main.go         # HTTP server entry point
│   ├── internal/
│   │   ├── handler/               # HTTP handlers
│   │   │   ├── health.go
│   │   │   ├── openapi.go
│   │   │   ├── moses_info.go
│   │   │   └── capabilities.go
│   │   ├── middleware/            # Moses headers, CORS, logging
│   │   └── model/                 # Capability data model
│   └── api/openapi.json           # OpenAPI 3.0.3 spec
├── frontend/
│   ├── Dockerfile                 # Multi-stage React build
│   ├── nginx.conf                 # Template with ${ENV_VARS}
│   ├── entrypoint.sh             # envsubst for runtime config
│   ├── package.json
│   ├── vite.config.ts
│   ├── index.html
│   ├── src/
│   │   ├── main.tsx
│   │   ├── App.tsx               # Router + routes
│   │   ├── App.css               # Moses design tokens
│   │   ├── api/client.ts         # Fetch wrapper
│   │   ├── utils/baseUrl.ts
│   │   ├── components/
│   │   │   ├── Layout.tsx        # App shell + sidebar nav
│   │   │   ├── FeatureCard.tsx   # Reusable card component
│   │   │   └── FlowDiagram.tsx   # CSS-based flow diagrams
│   │   └── pages/
│   │       ├── OverviewPage.tsx       # Hero + Moses context + features
│   │       ├── MCPToolsPage.tsx       # MCP system explained
│   │       ├── DeploymentPage.tsx     # Pipeline flows
│   │       ├── AuthFlowPage.tsx       # Authentication methods
│   │       ├── MultiTenancyPage.tsx   # RBAC + isolation
│   │       └── APIExamplesPage.tsx    # Interactive API testing
│   └── public/favicon.svg
├── helm/
│   ├── Chart.yaml
│   ├── values.yaml               # Multi-service: frontend + backend
│   └── templates/
│       ├── _helpers.tpl
│       ├── deployment.yaml       # Loop over services
│       └── service.yaml          # ClusterIP per service
├── skills/
│   ├── showcase-overview.md     # Agent skill: what this app shows
│   └── api-integration.md       # Agent skill: Moses integration patterns
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
