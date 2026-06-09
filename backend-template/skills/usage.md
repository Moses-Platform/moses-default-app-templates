# Backend Template - Agent Skill

## First Steps

Update `moses-app.config.json` with your app's identity before committing:

```json
{
  "name": "your-app-name",
  "displayName": "Your App Name",
  "description": "What your app actually does"
}
```

The `name` field becomes the Helm release name and MCP tool prefix. The `docker`, `service`, and `validation` sections are pre-configured — only change them if you modify the project structure.

## Overview

The Moses Backend Template is a production-ready Go HTTP service template demonstrating best practices for Moses platform integration. It showcases:

- **OpenAPI-first design** for automatic MCP tool generation
- **Moses header middleware** for tenant isolation and request tracking
- **Stdlib-only implementation** (no external dependencies except uuid)
- **Health probes** compatible with Kubernetes liveness/readiness checks
- **Multi-stage Docker builds** for minimal container size

## Project Structure

```
backend-template/
├── cmd/server/main.go           # HTTP server entry point
├── internal/
│   ├── handler/                 # HTTP request handlers
│   │   ├── health.go           # Health check endpoint
│   │   ├── openapi.go          # OpenAPI spec serving
│   │   └── items.go            # CRUD handlers for Item resource
│   ├── middleware/              # HTTP middleware
│   │   ├── moses_headers.go    # Moses platform header extraction
│   │   └── logging.go          # Request logging
│   └── model/                   # Data models
│       └── item.go             # Item model and in-memory store
├── api/
│   └── openapi.json            # OpenAPI 3.0.3 specification
├── helm/                        # Kubernetes deployment
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
│       ├── deployment.yaml
│       ├── service.yaml
│       └── _helpers.tpl
├── Dockerfile                   # Multi-stage Docker build
├── go.mod                       # Go module definition
└── moses-app.config.json       # Moses deployment configuration
```

## Adding New Endpoints

### 1. Define the Handler

Create or update a handler file in `internal/handler/`:

```go
// internal/handler/myresource.go
package handler

import (
    "encoding/json"
    "net/http"
    "github.com/moses-platform/backend-template/internal/middleware"
)

func HandleMyResource(w http.ResponseWriter, r *http.Request) {
    // Extract Moses context
    mosesCtx := middleware.GetMosesContext(r)
    tenantID := mosesCtx.TenantID

    // Your business logic here
    result := map[string]string{
        "message": "Hello from tenant: " + tenantID,
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(result)
}
```

### 2. Register the Route

Add the route in `cmd/server/main.go`:

```go
mux.HandleFunc("GET /api/v1/myresource", handler.HandleMyResource)
```

### 3. Update OpenAPI Spec

Add the endpoint to `api/openapi.json`:

```json
{
  "paths": {
    "/api/v1/myresource": {
      "get": {
        "operationId": "getMyResource",
        "summary": "Get my resource",
        "description": "Retrieves the resource with tenant filtering",
        "tags": ["MyResource"],
        "responses": {
          "200": {
            "description": "Resource retrieved successfully",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "message": {"type": "string"}
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}
```

CRITICAL: The `operationId` becomes the MCP tool name: `workspace_backend_template_getMyResource`

## Moses Header Middleware

The Moses platform automatically injects headers when calling workspace tool APIs:

| Header | Description | Example |
|--------|-------------|---------|
| `X-Moses-Tenant-ID` | Multi-tenant isolation ID | `550e8400-e29b-41d4-a716-446655440000` |
| `X-Moses-User-ID` | User who initiated the call | `123e4567-e89b-12d3-a456-426614174000` |
| `X-Moses-Chart-ID` | Project/workspace ID | `789e0123-e45b-67cd-e890-123456789abc` |
| `X-Moses-Tool-ID` | Workspace tool deployment ID | `def01234-5678-90ab-cdef-0123456789ab` |
| `X-Moses-Request-ID` | Unique request trace ID | `req_abc123def456` |
| `X-Moses-MCP-Source` | MCP call source | `claude-code`, `moses-manager` |
| `X-Moses-API-Key-ID` | API key ID (optional) | `key_xyz789` |

### Using Moses Context

```go
import "github.com/moses-platform/backend-template/internal/middleware"

func MyHandler(w http.ResponseWriter, r *http.Request) {
    mosesCtx := middleware.GetMosesContext(r)

    // Enforce tenant isolation
    if mosesCtx.TenantID == "" {
        // Local development - no filtering
    } else {
        // Production - filter by tenant
        items := store.GetByTenant(mosesCtx.TenantID)
    }

    // Use request ID for logging
    log.Printf("[%s] Processing request for user %s",
        mosesCtx.RequestID, mosesCtx.UserID)
}
```

## Building and Testing

### Local Development

```bash
# Run locally
go run cmd/server/main.go

# Test health endpoint
curl http://localhost:8080/health

# Test items endpoint
curl http://localhost:8080/api/v1/items

# Test with Moses headers (simulate platform call)
curl -H "X-Moses-Tenant-ID: test-tenant-123" \
     -H "X-Moses-User-ID: test-user-456" \
     http://localhost:8080/api/v1/items
```

### Docker Build

```bash
# Build image
docker build -t backend-template:latest .

# Run container
docker run -p 8080:8080 backend-template:latest

# Test in container
curl http://localhost:8080/health
```

### Deploy to Moses

1. Commit your changes to a Git repository
2. Register as a workspace tool in Moses UI
3. Moses will:
   - Clone the repository
   - Build Docker image in-cluster
   - Deploy using Helm chart
   - Discover OpenAPI spec at `/api/openapi.json`
   - Generate MCP tools from operationIds

## OpenAPI Discovery

Moses probes 11 standard paths for OpenAPI specs:
- `/api/openapi.json` (primary)
- `/api/spec` (alias)
- `/openapi.json`
- `/swagger.json`
- `/api/v1/openapi.json`
- `/docs/openapi.json`
- And 5 more alternates

This template serves specs at both `/api/openapi.json` and `/api/spec` for maximum compatibility.

## MCP Tool Generation

Each `operationId` in the OpenAPI spec becomes an MCP tool:

**OpenAPI operationId**: `listItems`
**Generated MCP tool**: `workspace_backend_template_listItems`

AI agents can call these tools directly:
```javascript
// Agent calls this via MCP
workspace_backend_template_listItems({})

// Moses injects headers automatically
// GET /api/v1/items
// X-Moses-Tenant-ID: <user's tenant>
// X-Moses-User-ID: <user's ID>
```

## Best Practices

1. **Always use Moses context**: Filter by `tenantID` for multi-tenant isolation
2. **Keep OpenAPI in sync**: Update `api/openapi.json` when adding endpoints
3. **Use meaningful operationIds**: They become MCP tool names
4. **Health checks must be fast**: Respond in < 5 seconds for K8s probes
5. **Log with request IDs**: Use `mosesCtx.RequestID` for request tracing
6. **Graceful shutdown**: Handle SIGTERM for zero-downtime deployments

## Extending the Template

### Add Database Support

```go
// Replace in-memory store with PostgreSQL
import (
    "github.com/jackc/pgx/v5/pgxpool"
)

pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
```

### Add Authentication

```go
// Add JWT validation middleware
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        // Validate JWT...
        next.ServeHTTP(w, r)
    })
}
```

### Add More Resources

1. Create model in `internal/model/`
2. Create handler in `internal/handler/`
3. Register routes in `cmd/server/main.go`
4. Update `api/openapi.json` with new paths
5. Rebuild and redeploy

## Troubleshooting

### Health Check Failing
- Verify `/health` responds within 5 seconds
- Check container logs: `kubectl logs <pod-name>`
- Ensure PORT env variable is set correctly (8080)

### OpenAPI Not Discovered
- Verify file exists at `/app/api/openapi.json` in container
- Check OpenAPI is valid JSON: `jq . api/openapi.json`
- View Moses logs for discovery errors

### Tenant Filtering Not Working
- Ensure `X-Moses-Tenant-ID` header is being injected
- Check middleware is registered: `h = middleware.MosesHeaders(h)`
- Verify handler calls `GetMosesContext(r)`

## Resources

- **Moses Documentation**: [moses-manager.eu/docs](https://moses-manager.eu/docs)
- **Go HTTP Server**: [pkg.go.dev/net/http](https://pkg.go.dev/net/http)
- **OpenAPI Spec**: [spec.openapis.org/oas/v3.0.3](https://spec.openapis.org/oas/v3.0.3.html)
- **Kubernetes Probes**: [kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
