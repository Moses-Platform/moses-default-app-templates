// App schema. The demo tables that shipped with the template were
// removed by clean_out_template.sh — put YOUR schema here. Connect/retry
// plumbing lives in db.go; main.go calls Migrate once at startup.
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
		-- your tables here, e.g.:
		-- CREATE TABLE IF NOT EXISTS things (
		-- 	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		-- 	tenant_id TEXT NOT NULL DEFAULT '',
		-- 	body TEXT NOT NULL DEFAULT '',
		-- 	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		-- );
		SELECT 1;
	`
	_, err := db.Exec(schema)
	return err
}
