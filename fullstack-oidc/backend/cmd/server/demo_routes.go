// DEMO ROUTE WIRING — clean_out_template.sh replaces this file with a
// stub. All demo route registration is concentrated here so buildMux
// (main.go) survives the clean-out untouched.
package main

import (
	"database/sql"
	"net/http"

	"github.com/moses-platform/fullstack-oidc/internal/handler"
)

// registerDemoRoutes wires the demo API surface — registered ONCE under
// MOSES_BASE_PATH, exactly like the kept /api/v1/me route in buildMux.
//
//	/api/v1/public-info   — PUBLIC  (declared in access.oidc.publicPaths)
//	/api/v1/entries       — PROTECTED (USER space: per-user, private)
//	/api/v1/shared-notes  — PROTECTED (TENANT space: shared by the workspace)
//	/api/v1/admin-area    — PROTECTED + ROLE-GATED (oidc-admin role)
//
// The contrast between public-info and the rest is the whole demo:
// one route any visitor can read, the others gated by the vendored
// oidcauth middleware. entries vs shared-notes is the SECOND contrast:
// entries is owned per OIDC subject (private), shared-notes is owned per
// tenant (visible to the whole workspace — and to content agents post via
// workspace tools, which arrive under the agent's X-Moses-User-ID).
// admin-area goes one step further and enforces an app role projected from
// the token (see handler.AdminArea).
func registerDemoRoutes(mux *http.ServeMux, basePath string, oidcEnabled bool, db *sql.DB) {
	entriesHandler := handler.NewEntriesHandler(db)
	sharedNotesHandler := handler.NewSharedNotesHandler(db)

	mux.HandleFunc(basePath+"/api/v1/public-info", handler.PublicInfo(oidcEnabled))
	mux.HandleFunc(basePath+"/api/v1/entries", entriesHandler.Entries)
	mux.HandleFunc(basePath+"/api/v1/shared-notes", sharedNotesHandler.SharedNotes)
	mux.HandleFunc(basePath+"/api/v1/admin-area", handler.AdminArea)
}
