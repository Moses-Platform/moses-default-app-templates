package handler

import (
	"net/http"

	"github.com/moses-platform/backend-template/api"
)

// ServeOpenAPI serves the OpenAPI specification embedded in the binary.
// This endpoint is registered at both /api/openapi.json and /api/spec
// to match Moses platform's standard OpenAPI discovery paths.
//
// The OpenAPI spec enables automatic MCP tool generation via WorkspaceToolProxy:
// each operationId becomes an MCP tool: workspace_{toolKey}_{operationId}.
func ServeOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(api.Spec)
}
