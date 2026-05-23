# Moses Backend Template

A production-ready Go HTTP service template for the Moses platform, demonstrating best practices for OpenAPI-first API design, multi-tenant architecture, and automatic MCP tool generation.

## Features

- **OpenAPI 3.0.3 Specification**: Automatic MCP tool generation from operationIds
- **Moses Header Middleware**: Built-in support for Moses platform headers (tenant ID, user ID, request tracking)
- **Multi-Tenant Architecture**: Automatic tenant isolation when deployed on Moses
- **Health Probes**: Kubernetes-compatible liveness and readiness checks
- **Zero Dependencies**: Stdlib-only implementation (except UUID generation)
- **Multi-Stage Docker Build**: Optimized container size with Alpine Linux
- **Helm Chart**: Production-ready Kubernetes deployment
- **In-Memory Store**: Simple example demonstrating tenant-filtered data access

## Quick Start

### Local Development

```bash
# Run the server
go run cmd/server/main.go

# In another terminal, test endpoints
curl http://localhost:8080/health
curl http://localhost:8080/api/v1/items
curl http://localhost:8080/api/openapi.json

# Create an item
curl -X POST http://localhost:8080/api/v1/items \
  -H "Content-Type: application/json" \
  -d '{"name":"Test Item","description":"Created via curl"}'
```

### Docker Build

```bash
# Build the image
docker build -t backend-template:latest .

# Run the container
docker run -p 8080:8080 backend-template:latest

# Test the container
curl http://localhost:8080/health
```

### Deploy to Moses Platform

1. **Commit to Git**: Push this repository to a Git hosting service (GitHub, GitLab, etc.)

2. **Register in Moses**:
   - Navigate to Moses workspace tools marketplace
   - Click "Register New Tool"
   - Enter your repository URL
   - Moses will automatically detect `moses-app.config.json`

3. **Automatic Deployment**:
   - Moses clones the repository
   - Kaniko builds the Docker image
   - Helm deploys to Kubernetes
   - OpenAPI spec is discovered at `/api/openapi.json`
   - MCP tools are generated from operationIds

4. **Use MCP Tools**:
   ```javascript
   // AI agents can now call:
   workspace_backend_template_listItems({})
   workspace_backend_template_createItem({
     name: "New Item",
     description: "Created by AI agent"
   })
   workspace_backend_template_getItem({ id: "item-uuid" })
   workspace_backend_template_healthCheck({})
   ```

## Project Structure

```
backend-template/
├── cmd/server/main.go              # HTTP server entry point
├── internal/
│   ├── handler/                    # HTTP request handlers
│   │   ├── health.go              # Health check endpoint
│   │   ├── openapi.go             # OpenAPI spec serving
│   │   └── items.go               # Item CRUD handlers
│   ├── middleware/                 # HTTP middleware
│   │   ├── moses_headers.go       # Moses platform integration
│   │   └── logging.go             # Request logging
│   └── model/
│       └── item.go                # Item model and store
├── api/
│   └── openapi.json               # OpenAPI 3.0.3 specification
├── helm/                           # Kubernetes deployment
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
│       ├── deployment.yaml
│       ├── service.yaml
│       └── _helpers.tpl
├── skills/
│   └── usage.md                   # Agent skill documentation
├── Dockerfile                      # Multi-stage Docker build
├── go.mod                          # Go module definition
├── moses-app.config.json          # Moses deployment config
└── README.md                       # This file
```

## API Endpoints

| Method | Path | Description | MCP Tool Name |
|--------|------|-------------|---------------|
| GET | `/health` | Health check | `workspace_backend_template_healthCheck` |
| GET | `/api/openapi.json` | OpenAPI spec | N/A |
| GET | `/api/spec` | OpenAPI spec (alias) | N/A |
| GET | `/api/v1/items` | List all items | `workspace_backend_template_listItems` |
| GET | `/api/v1/items/{id}` | Get item by ID | `workspace_backend_template_getItem` |
| POST | `/api/v1/items` | Create new item | `workspace_backend_template_createItem` |

## Moses Platform Integration

### Header Injection

When deployed on Moses, all API calls automatically receive these headers:

```
X-Moses-Tenant-ID: 550e8400-e29b-41d4-a716-446655440000
X-Moses-User-ID: 123e4567-e89b-12d3-a456-426614174000
X-Moses-Chart-ID: 789e0123-e45b-67cd-e890-123456789abc
X-Moses-Tool-ID: def01234-5678-90ab-cdef-0123456789ab
X-Moses-Request-ID: req_abc123def456
X-Moses-MCP-Source: claude-code
X-Moses-API-Key-ID: key_xyz789
```

### Tenant Isolation (CHAT-pxeo.12)

Self-tenant identification is read from the `MOSES_TENANT_ID` env var
via `internal/config.SelfTenantID()`, NOT from the request header. The
`X-Moses-Tenant-ID` header is preserved as caller context for audit and
the 403 cross-check on writes.

```go
import "github.com/moses-platform/backend-template/internal/middleware"

func MyHandler(w http.ResponseWriter, r *http.Request) {
    mosesCtx := middleware.GetMosesContext(r)

    // Storage scope: deploy-pinned self tenant (env-driven)
    items := store.GetByTenant(mosesCtx.SelfTenantID)

    // Audit only: caller context from the X-Moses-Tenant-ID header
    log.Printf("[%s] Request from user %s, caller_tenant=%s",
        mosesCtx.RequestID, mosesCtx.UserID, mosesCtx.CallerTenantID)
}
```

| Env var | Purpose |
|---|---|
| `MOSES_TENANT_ID` | **Required on a deployed pod.** Authoritative storage/lookup key, surfaced via `internal/config.SelfTenantID()`. |
| `MOSES_DEPLOYED` | Set to `1` by the platform's deployment template; flips `config.Validate()` from warn-only to fail-fast on missing `MOSES_TENANT_ID`. |
| `MOSES_STRICT_TENANT_CHECK` | Optional, default `true`. When a request supplies a non-empty `X-Moses-Tenant-ID` that disagrees with `MOSES_TENANT_ID`, write/diagnostic handlers return 403 with `{"error":"tenant_mismatch","code":"E_TENANT_MISMATCH"}`. Set to `false` to disable. |

### OpenAPI Discovery

Moses automatically discovers OpenAPI specs at these paths:
1. `/api/openapi.json` ✅
2. `/api/spec` ✅
3. `/openapi.json`
4. `/swagger.json`
5. `/api/v1/openapi.json`
6. And 6 more standard locations

This template serves the spec at both primary paths for maximum compatibility.

## MCP Tool Generation

Each `operationId` in the OpenAPI spec becomes an MCP tool:

**OpenAPI Definition**:
```json
{
  "paths": {
    "/api/v1/items": {
      "get": {
        "operationId": "listItems",
        "summary": "List all items"
      }
    }
  }
}
```

**Generated MCP Tool**:
```
Tool Name: workspace_backend_template_listItems
Description: List all items
Input Schema: (from OpenAPI parameters)
Output Schema: (from OpenAPI responses)
```

AI agents call the tool, Moses injects headers automatically, and your API receives tenant-isolated requests.

## Extending the Template

### Add a New Endpoint

1. **Create handler** in `internal/handler/myresource.go`:
   ```go
   func HandleMyResource(w http.ResponseWriter, r *http.Request) {
       mosesCtx := middleware.GetMosesContext(r)
       // Your logic here
   }
   ```

2. **Register route** in `cmd/server/main.go`:
   ```go
   mux.HandleFunc("GET /api/v1/myresource", handler.HandleMyResource)
   ```

3. **Update OpenAPI** in `api/openapi.json`:
   ```json
   {
     "paths": {
       "/api/v1/myresource": {
         "get": {
           "operationId": "getMyResource",
           "summary": "Get my resource"
         }
       }
     }
   }
   ```

4. **Rebuild and redeploy** - Moses will auto-generate the new MCP tool

### Add Database Support

Replace the in-memory store with PostgreSQL:

```go
import "github.com/jackc/pgx/v5/pgxpool"

pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
defer pool.Close()
```

Update `helm/values.yaml` to add database configuration:

```yaml
env:
  DATABASE_URL: "postgres://user:pass@postgres:5432/dbname"
```

### Add Authentication

Implement JWT validation middleware:

```go
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if !validateToken(token) {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP server port | `8080` |
| `BASE_URL` | Public base URL | `http://localhost:8080` |
| `NODE_ENV` | Environment (for compatibility) | `production` |

Declaring runtime secrets — see [skills/secrets-tutorial.md](skills/secrets-tutorial.md).

### Helm Values

See `helm/values.yaml` for all configuration options:
- Resource limits (CPU, memory)
- Replica count
- Health check intervals
- Service port mapping
- Environment variables

## Health Checks

The template includes Kubernetes-compatible health probes:

**Liveness Probe**: Checks if the service is alive
- Path: `/health`
- Initial delay: 10s
- Period: 30s
- Timeout: 5s

**Readiness Probe**: Checks if the service is ready to accept traffic
- Path: `/health`
- Initial delay: 5s
- Period: 10s
- Timeout: 5s

## Building for Production

### Optimization Tips

1. **Use multi-stage builds** (already implemented in Dockerfile)
2. **Minimize dependencies** (template uses stdlib only)
3. **Set resource limits** in `helm/values.yaml`
4. **Enable health probes** (already configured)
5. **Use Alpine base image** for minimal container size

### Security Considerations

1. **Tenant isolation**: Always filter by `mosesCtx.TenantID`
2. **Input validation**: Validate all request parameters
3. **Error handling**: Don't leak sensitive information in errors
4. **HTTPS only**: Moses handles TLS termination
5. **Rate limiting**: Consider adding rate limiting middleware

## Troubleshooting

### Health Check Failing

```bash
# Check if service is responding
kubectl exec -it <pod-name> -- wget -O- http://localhost:8080/health

# View logs
kubectl logs <pod-name>

# Check environment variables
kubectl exec -it <pod-name> -- env | grep PORT
```

### OpenAPI Not Discovered

```bash
# Verify spec is accessible
kubectl exec -it <pod-name> -- wget -O- http://localhost:8080/api/openapi.json

# Validate JSON syntax
jq . api/openapi.json

# Check Moses discovery logs
kubectl logs -n moses deployment/moses-backend | grep openapi
```

### Tenant Filtering Not Working

```bash
# Verify middleware is registered
# Check cmd/server/main.go:
# h = middleware.MosesHeaders(h)

# Test with Moses headers
curl -H "X-Moses-Tenant-ID: test-123" http://localhost:8080/api/v1/items

# Check handler extracts context
# middleware.GetMosesContext(r)
```

## License

This template is provided as-is for use with the Moses platform.

## Resources

- [Moses Platform Documentation](https://moses-manager.eu/docs)
- [OpenAPI Specification](https://spec.openapis.org/oas/v3.0.3.html)
- [Go HTTP Server](https://pkg.go.dev/net/http)
- [Kubernetes Health Probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
- [Helm Charts](https://helm.sh/docs/topics/charts/)

## Support

For issues or questions about this template:
1. Check the [skills/usage.md](skills/usage.md) documentation
2. Review Moses platform documentation
3. Open an issue in the repository
