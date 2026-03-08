package handler

import (
	"net/http"
	"os"
	"path/filepath"
)

// OpenAPI serves the OpenAPI specification
func OpenAPI(w http.ResponseWriter, r *http.Request) {
	// Read OpenAPI spec file
	specPath := filepath.Join("api", "openapi.json")
	content, err := os.ReadFile(specPath)
	if err != nil {
		http.Error(w, "OpenAPI spec not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(content)
}
