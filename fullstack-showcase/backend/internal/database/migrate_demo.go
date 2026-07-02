package database

import (
	"database/sql"
	"fmt"
	"log"
)

// migrate_demo.go carries the DEMO schema (the showcase `notes` table).
// clean_out_template.sh replaces this file with an empty-migration stub;
// the Connect/retry/28P01 plumbing in db.go is untouched by the cleanout.

// Migrate runs schema migrations
func Migrate(db *sql.DB) error {
	schema := `
		CREATE TABLE IF NOT EXISTS notes (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL,
			content TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_notes_tenant ON notes(tenant_id);
		CREATE INDEX IF NOT EXISTS idx_notes_created ON notes(tenant_id, created_at DESC);
	`
	_, err := db.Exec(schema)
	return err
}

// MigrateTenant rewrites legacy rows that were stored under the
// "local-dev"/"default"/empty-string tenant (the historic header-fallback
// behaviour) so they are owned by the deploy-pinned tenant id. Idempotent:
// rows already under selfTenantID are untouched, and a second run is a
// no-op.
//
// The rewrite is PER TABLE — every tenant-scoped table you add needs its
// own UPDATE following the `notes` pattern below (also documented in
// skills/usage.md, which survives the demo cleanout).
//
// CHAT-pxeo.12: must run synchronously in main() BEFORE the HTTP listener
// starts, AFTER schema Migrate. Failure → log + non-zero exit.
func MigrateTenant(db *sql.DB, selfTenantID string) error {
	if selfTenantID == "" || selfTenantID == "local-dev" {
		log.Printf("CHAT-pxeo.12: skipping legacy tenant rewrite (selfTenantID=%q is non-authoritative)", selfTenantID)
		return nil
	}
	res, err := db.Exec(
		`UPDATE notes SET tenant_id = $1 WHERE tenant_id IN ('local-dev', 'default', '')`,
		selfTenantID,
	)
	if err != nil {
		return fmt.Errorf("notes tenant rewrite: %w", err)
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		log.Printf("CHAT-pxeo.12: rewrote %d legacy 'notes' rows to tenant %s", n, selfTenantID)
	}
	return nil
}
