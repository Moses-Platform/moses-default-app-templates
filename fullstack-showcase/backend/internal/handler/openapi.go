package handler

import (
	"net/http"

	"github.com/moses-platform/fullstack-showcase/api"
)

// OpenAPI serves the OpenAPI specification embedded in the binary.
func OpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(api.Spec)
}
