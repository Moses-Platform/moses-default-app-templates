package main

import (
	"database/sql"
	"net/http"
)

// registerDemoRoutes is the single place API routes get registered —
// buildMux (main.go) calls it once with the MOSES_BASE_PATH prefix. The
// showcase demo routes were removed by clean_out_template.sh; add YOUR
// routes here, e.g.:
//
//	itemsHandler := handler.NewItemsHandler(db)
//	mux.HandleFunc(basePath+"/api/v1/items", itemsHandler.Items)
//	mux.HandleFunc(basePath+"/api/v1/items/", itemsHandler.Items)
//
// Rules (enforced by the cmd/server tests):
//   - Register API routes ONCE under basePath — never a second root mount.
//   - Mirror every route in api/openapi.json with paths RELATIVE to the
//     servers[0].url "/api/v1" base (never /api/-rooted, never /health).
//   - Scope storage by the deploy-pinned tenant (MosesContext.SelfTenantID
//     via middleware.GetMosesContext) and reuse the enforceTenantMatch 403
//     cross-check from internal/handler/tenant.go.
func registerDemoRoutes(mux *http.ServeMux, basePath string, db *sql.DB) {
	_ = mux
	_ = basePath
	_ = db
}
