# Moses API Integration - Agent Skill

## Moses Header Middleware Pattern

All Moses-integrated apps should extract Moses headers to get tenant/user context:

```go
// Backend example (internal/middleware/moses_headers.go)
type MosesContext struct {
    TenantID    string
    UserID      string
    ChartID     string
    ToolID      string
    RequestID   string
    MCPSource   string
    APIKeyID    string
}

func MosesHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        mosesCtx := MosesContext{
            TenantID:   r.Header.Get("X-Moses-Tenant-ID"),
            UserID:     r.Header.Get("X-Moses-User-ID"),
            ChartID:    r.Header.Get("X-Moses-Chart-ID"),
            // ... extract other headers
        }
        ctx := context.WithValue(r.Context(), mosesContextKey, mosesCtx)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

## OpenAPI Spec Requirements

For automatic MCP tool generation, provide a complete OpenAPI spec:

### File Location
Place at one of 11 standard paths:
- `/api/openapi.json` or `/api/openapi.yaml` (recommended)
- `/swagger.json` or `/swagger.yaml`
- `/openapi.json` or `/openapi.yaml`
- `/api/spec`, `/api-docs`, `/docs/openapi.json`, `/api-doc`

### Required Fields
```json
{
  "openapi": "3.0.3",
  "info": { "title": "...", "version": "..." },
  "paths": {
    "/api/v1/endpoint": {
      "get": {
        "operationId": "uniqueOperationId",  // REQUIRED for tool naming
        "summary": "Short description",
        "description": "Detailed explanation",
        "tags": ["Category"],
        "responses": { "200": { ... } }
      }
    }
  }
}
```

### Tool Naming Convention
Moses generates tools as: `workspace_{toolKey}_{operationId}`

Example:
- OpenAPI operationId: `getMosesInfo`
- Tool registration key: `showcase`
- Generated tool: `workspace_showcase_getMosesInfo`

## Multi-Service Frontend→Backend Communication

When deploying frontend + backend as separate services:

### Frontend nginx.conf
```nginx
location /api/ {
    proxy_pass http://${BACKEND_SERVICE_HOST}:${BACKEND_SERVICE_PORT}/api/;
    proxy_set_header Host $host;
    proxy_pass_header X-Moses-Tenant-ID;  # Forward Moses headers
    proxy_pass_header X-Moses-User-ID;
}
```

### Frontend Dockerfile entrypoint
```bash
#!/bin/sh
envsubst '${BACKEND_SERVICE_HOST} ${BACKEND_SERVICE_PORT}' < /etc/nginx/nginx.conf.template > /etc/nginx/nginx.conf
exec nginx -g 'daemon off;'
```

### Helm values.yaml
```yaml
services:
  - name: frontend
    env:
      BACKEND_SERVICE_HOST: "agent-deployed-app-backend"
      BACKEND_SERVICE_PORT: "8080"
  - name: backend
    port: 8080
```

Service DNS: `agent-deployed-app-backend.{namespace}.svc.cluster.local`

## Tenant Isolation

All data operations MUST filter by tenant:

```go
// GOOD - Tenant-scoped query
query := `SELECT * FROM resources WHERE id = $1 AND tenant_id = $2`
db.Query(ctx, query, resourceID, mosesCtx.TenantID)

// BAD - Cross-tenant data leak
query := `SELECT * FROM resources WHERE id = $1`
db.Query(ctx, query, resourceID)
```

## CORS Configuration

Allow Moses backend origin:

```go
func CORS(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        origin := r.Header.Get("Origin")
        if env == "production" {
            // Restrict to K8s service mesh
            if origin != "" {
                w.Header().Set("Access-Control-Allow-Origin", origin)
            }
        } else {
            w.Header().Set("Access-Control-Allow-Origin", "*")
        }
        w.Header().Set("Access-Control-Allow-Headers", 
            "Content-Type, Authorization, X-Moses-Tenant-ID, X-Moses-User-ID")
        // ...
    })
}
```

## Health Check Best Practices

Implement /health for deployment verification:

```go
func Health(w http.ResponseWriter, r *http.Request) {
    response := map[string]string{
        "status":  "healthy",
        "service": "your-service-name",
        "version": "1.0.0",
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

Moses deployment automation:
1. The in-cluster image builder builds and pushes images
2. Helm deploys with health probes
3. Waits for readinessProbe to pass
4. OpenAPI discovery (if API service)
5. Generates MCP tools from operationIds
