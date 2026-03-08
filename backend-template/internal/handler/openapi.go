package handler

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// ServeOpenAPI serves the static OpenAPI specification file.
// This endpoint is registered at both /api/openapi.json and /api/spec
// to match Moses platform's 11 standard OpenAPI discovery paths.
//
// The OpenAPI spec enables automatic MCP tool generation via WorkspaceToolProxy.
// Each operationId in the spec becomes an MCP tool: workspace_{toolKey}_{operationId}
func ServeOpenAPI(w http.ResponseWriter, r *http.Request) {
	// Determine OpenAPI spec file path
	// In Docker: /app/api/openapi.json
	// In local dev: ./api/openapi.json
	specPath := filepath.Join("api", "openapi.json")
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		specPath = filepath.Join("/app", "api", "openapi.json")
	}

	// Open the spec file
	file, err := os.Open(specPath)
	if err != nil {
		http.Error(w, "OpenAPI spec not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	// Set appropriate headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour

	// Stream file to response
	if _, err := io.Copy(w, file); err != nil {
		http.Error(w, "Failed to serve OpenAPI spec", http.StatusInternalServerError)
		return
	}
}
