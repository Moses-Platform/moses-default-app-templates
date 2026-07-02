// DEMO SCHEMA FILE — clean_out_template.sh replaces this file with an
// empty-schema twin. Connect/retry plumbing lives in db.go and survives
// the clean-out untouched.
package database

import "database/sql"

// Migrate runs schema migrations. The template ships TWO demo resources on
// purpose, because a Moses app almost always needs both:
//
//   - `entries`      — the USER space. Scoped by BOTH the deploy-pinned tenant
//                      id AND the OIDC subject, so a row is private to the
//                      person who created it. Use this shape for genuinely
//                      personal data.
//   - `shared_notes` — the TENANT space. Scoped by tenant id ALONE, so every
//                      member of the workspace sees the same list. `author_sub`
//                      is recorded for attribution only — reads never filter by
//                      it.
//
// Why both matter (the subtle part): when an agent delivers content into this
// app through a Moses workspace-tool call, the request arrives on the trusted
// X-Moses-* header path and `oidcauth` resolves its identity from the
// `X-Moses-User-ID` header — i.e. the AGENT's user id, not the human's. Content
// scoped by user id alone therefore lands under the agent and is invisible to
// the human who owns the app. Default collaborative / agent-fed content to the
// tenant space; reserve the user space for data that is meant to stay private.
func Migrate(db *sql.DB) error {
	schema := `
		CREATE TABLE IF NOT EXISTS entries (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id TEXT NOT NULL DEFAULT '',
			owner_sub TEXT NOT NULL DEFAULT '',
			body TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_entries_tenant ON entries(tenant_id);
		CREATE INDEX IF NOT EXISTS idx_entries_owner ON entries(tenant_id, owner_sub);

		CREATE TABLE IF NOT EXISTS shared_notes (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id TEXT NOT NULL DEFAULT '',
			author_sub TEXT NOT NULL DEFAULT '',
			body TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		-- Tenant-only index: reads filter by tenant_id alone, so the whole
		-- workspace (and content agents post via workspace tools) share one list.
		CREATE INDEX IF NOT EXISTS idx_shared_notes_tenant ON shared_notes(tenant_id, created_at DESC);
	`
	_, err := db.Exec(schema)
	return err
}
