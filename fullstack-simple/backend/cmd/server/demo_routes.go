package main

import (
	"net/http"

	"github.com/moses-platform/fullstack-simple/internal/handler"
)

// registerDemoRoutes wires the template's demo endpoints. clean_out_template.sh
// replaces this file with an empty stub — keep ALL demo route registration
// here so the plumbing routes in buildMux (health, openapi) stay untouched.
//
// API endpoints are registered ONCE under MOSES_BASE_PATH (see the MOSES
// ROUTING comment on buildMux). Every route added here must also appear in
// api/openapi.json (relative to servers[0].url = /api/v1) so it becomes an
// MCP tool via the platform's WorkspaceToolProxy.
func registerDemoRoutes(mux *http.ServeMux, basePath string) {
	// Status — echoes Moses request-context headers (X-Moses-User-ID etc.).
	mux.HandleFunc(basePath+"/api/v1/status", handler.Status)

	// CRUD example — in-memory items scoped by the deploy-pinned tenant.
	// For database-backed CRUD patterns, see the fullstack-showcase template.
	items := handler.NewItemsHandler()
	mux.HandleFunc(basePath+"/api/v1/items", items.Handle)
	mux.HandleFunc(basePath+"/api/v1/items/", items.HandleWithID)
}
