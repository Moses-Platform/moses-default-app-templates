package main

import (
	"database/sql"
	"net/http"
)

// registerDemoRoutes is the single place API routes get registered —
// buildMux (main.go) calls it once with the MOSES_BASE_PATH prefix.
//
// WORKED EXAMPLE: example_test.go (this package) — real, CI-compiled code, not
// a comment. Its handler half is internal/handler/example_test.go, its storage
// half the `things` table sketched in internal/database/migrate_demo.go, and
// its spec half the comment above the //go:embed directive in api/api.go.
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
