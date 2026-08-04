package handler

import (
	"net/http"

	"github.com/moses-platform/fullstack-oidc/api"
)

// OpenAPI serves the OpenAPI specification embedded in the binary.
// Always public — in the implicit public-path set of the OIDCAuth
// middleware so platform discovery and the workspace-tool surface keep
// working without a session.
func OpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(api.Spec)
}
