package database

import (
	"database/sql"
)

// migrate_demo.go carries the app schema. The showcase demo schema was
// removed by clean_out_template.sh; the Connect/retry/28P01 plumbing in
// db.go is untouched. Add your own tables below.

// Migrate runs schema migrations. Add CREATE TABLE IF NOT EXISTS
// statements (idempotent — it runs on every boot), e.g.:
//
//	schema := `
//		CREATE TABLE IF NOT EXISTS items (
//			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
//			tenant_id TEXT NOT NULL DEFAULT '',
//			name TEXT NOT NULL,
//			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
//		);
//		CREATE INDEX IF NOT EXISTS idx_items_tenant ON items(tenant_id);
//	`
//	_, err := db.Exec(schema)
//	return err
func Migrate(db *sql.DB) error {
	_ = db
	return nil
}

// MigrateTenant rewrites legacy rows stored under the historic
// "local-dev"/"default"/empty-string header-fallback tenant so they are
// owned by the deploy-pinned tenant id (CHAT-pxeo.12). It must stay wired
// in main() (synchronously BEFORE the HTTP listener, AFTER Migrate).
//
// The rewrite is PER TABLE: for every tenant-scoped table you add, add one
// idempotent UPDATE following this pattern (full worked example in
// skills/usage.md):
//
//	res, err := db.Exec(
//		`UPDATE items SET tenant_id = $1 WHERE tenant_id IN ('local-dev', 'default', '')`,
//		selfTenantID,
//	)
func MigrateTenant(db *sql.DB, selfTenantID string) error {
	if selfTenantID == "" || selfTenantID == "local-dev" {
		return nil // non-authoritative sentinel — skip
	}
	_ = db
	return nil
}
