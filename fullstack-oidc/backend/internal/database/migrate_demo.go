// App schema — put YOUR schema here. Connect/retry plumbing lives in
// db.go; main.go calls Migrate once at startup.
package database

import "database/sql"

// Migrate runs schema migrations at startup. Add CREATE TABLE IF NOT
// EXISTS statements for your resources.
//
// Data-scoping rules (the part that is easy to get wrong):
//
//   - EVERY table gets a `tenant_id TEXT NOT NULL` column, filled from
//     config.SelfTenantID() — never from the X-Moses-Tenant-ID header.
//   - TENANT space (shared by the whole workspace): filter reads by
//     tenant_id ALONE. Default collaborative / agent-fed content here —
//     a workspace-tool call arrives under the AGENT's X-Moses-User-ID,
//     so rows scoped by user id alone would be invisible to the human.
//   - USER space (private to one person): filter reads by
//     (tenant_id, owner_sub) where owner_sub is the authenticated OIDC
//     subject (oidcauth.IdentityFrom(r.Context()).Subject). Reserve this
//     shape for genuinely personal data.
func Migrate(db *sql.DB) error {
	schema := `
		-- your tables here. This is the storage layer of the worked
		-- "things" vertical slice (handler + routes as compiled-but-never-
		-- shipped declarations in cmd/server/example_test.go; wiring notes in
		-- cmd/server/demo_routes.go; spec sketch commented in api/api.go):
		-- CREATE TABLE IF NOT EXISTS things (
		-- 	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		-- 	tenant_id TEXT NOT NULL DEFAULT '',   -- config.SelfTenantID(), always
		-- 	owner_sub TEXT NOT NULL DEFAULT '',   -- only needed for USER space
		-- 	name TEXT NOT NULL DEFAULT '',
		-- 	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		-- );
		-- CREATE INDEX IF NOT EXISTS idx_things_tenant ON things(tenant_id);
		SELECT 1;
	`
	_, err := db.Exec(schema)
	return err
}
