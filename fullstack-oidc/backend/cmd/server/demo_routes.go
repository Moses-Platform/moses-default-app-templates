// App route wiring — register YOUR API routes here (or rename this file
// once you outgrow the name; buildMux in main.go calls registerDemoRoutes
// exactly once).
package main

import (
	"database/sql"
	"net/http"
)

// registerDemoRoutes is where your API surface goes. Register every
// route ONCE under MOSES_BASE_PATH, exactly like the kept /api/v1/me
// route in buildMux.
//
// Remember to:
//   - add each route to backend/api/openapi.json (paths are relative to
//     the servers[] "/api/v1" entry) so the platform's workspace-tool
//     discovery picks it up;
//   - list protected routes in moses-app.config.json access.oidc
//     .protectedPaths (or leave protectedPaths empty for deny-by-default);
//   - scope data by config.SelfTenantID() — never by the
//     X-Moses-Tenant-ID request header;
//   - use oidcauth.IdentityFrom(r.Context()) for the caller's identity.
//
// WORKED EXAMPLE: example_test.go (this package) — real, CI-compiled code, not
// a comment. It carries both halves (routes + ThingsHandler), plus the reminder
// to add "/api/v1/things" to access.oidc.protectedPaths in
// moses-app.config.json or the route stays deny-by-default. Its storage half is
// the `things` table sketched in internal/database/migrate_demo.go; its spec
// half is the comment above the //go:embed directive in api/api.go.
func registerDemoRoutes(mux *http.ServeMux, basePath string, oidcEnabled bool, db *sql.DB) {
	_, _, _, _ = mux, basePath, oidcEnabled, db // silence unused params until you add routes
}
