// DEMO SCHEMA — the entries + chat_completions tables backing the
// chat-roundtrip demo. clean_out_template.sh replaces this file with an
// empty-schema twin (same function signatures, so main.go stays untouched);
// put your own schema in the replacement's Migrate.
// Connection/retry plumbing stays in db.go, which survives the cleanout.
package database

import (
	"database/sql"
	"fmt"
	"log"
)

// MigrateTenant rewrites legacy rows that were stored under the
// "local-dev"/"default"/empty-string tenant (the historic header-fallback
// behaviour) so they are owned by the deploy-pinned tenant id. Idempotent:
// rows already under selfTenantID are untouched, and a second run is a
// no-op.
//
// CHAT-pxeo.12: must run synchronously in main() BEFORE the HTTP listener
// starts, AFTER schema Migrate. Failure → log + non-zero exit.
func MigrateTenant(db *sql.DB, selfTenantID string) error {
	if selfTenantID == "" || selfTenantID == "local-dev" {
		// Don't rewrite anything when we don't have an authoritative
		// real tenant. Local-dev runs leave legacy rows as-is.
		log.Printf("CHAT-pxeo.12: skipping legacy tenant rewrite (selfTenantID=%q is non-authoritative)", selfTenantID)
		return nil
	}
	res, err := db.Exec(
		`UPDATE entries SET tenant_id = $1 WHERE tenant_id IN ('local-dev', 'default', '')`,
		selfTenantID,
	)
	if err != nil {
		return fmt.Errorf("entries tenant rewrite: %w", err)
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		log.Printf("CHAT-pxeo.12: rewrote %d legacy 'entries' rows to tenant %s", n, selfTenantID)
	}

	res, err = db.Exec(
		`UPDATE chat_completions SET tenant_id = $1 WHERE tenant_id IN ('local-dev', 'default', '')`,
		selfTenantID,
	)
	if err != nil {
		return fmt.Errorf("chat_completions tenant rewrite: %w", err)
	}
	n, _ = res.RowsAffected()
	if n > 0 {
		log.Printf("CHAT-pxeo.12: rewrote %d legacy 'chat_completions' rows to tenant %s", n, selfTenantID)
	}
	return nil
}

// Migrate runs schema migrations for the chat-roundtrip entries store.
func Migrate(db *sql.DB) error {
	schema := `
		CREATE TABLE IF NOT EXISTS entries (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id TEXT NOT NULL DEFAULT '',
			text TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT 'user',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_entries_tenant ON entries(tenant_id);
		CREATE INDEX IF NOT EXISTS idx_entries_created ON entries(tenant_id, created_at DESC);

		CREATE TABLE IF NOT EXISTS chat_completions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id TEXT NOT NULL DEFAULT '',
			conversation_id TEXT NOT NULL,
			final_message_id TEXT,
			final_text TEXT,
			model TEXT,
			latency_ms INTEGER,
			finish_reason TEXT,
			received_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			signature_valid BOOLEAN NOT NULL DEFAULT FALSE
		);

		CREATE INDEX IF NOT EXISTS idx_chat_completions_conv ON chat_completions(conversation_id);
	`
	_, err := db.Exec(schema)
	return err
}
