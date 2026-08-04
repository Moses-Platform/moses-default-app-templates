// App schema migrations — put YOUR tables here. Connection/retry plumbing
// stays in db.go. Keep both function signatures stable — main.go calls
// them on every boot.
package database

import (
	"database/sql"
)

// MigrateTenant rewrites legacy rows stored under the "local-dev"/"default"
// /empty-string sentinel tenants so they are owned by the deploy-pinned
// tenant id (CHAT-pxeo.12). Runs synchronously in main() BEFORE the HTTP
// listener starts, AFTER Migrate. Idempotent by contract.
//
// TODO: when you add tenant-keyed tables, rewrite their legacy rows here:
//
//	if selfTenantID == "" || selfTenantID == "local-dev" { return nil }
//	_, err := db.Exec(`UPDATE things SET tenant_id = $1
//	                    WHERE tenant_id IN ('local-dev', 'default', '')`, selfTenantID)
func MigrateTenant(db *sql.DB, selfTenantID string) error {
	_, _ = db, selfTenantID
	return nil
}

// Migrate runs this app's schema migrations at boot.
//
// TODO: create your tables here (idempotent DDL — CREATE TABLE IF NOT
// EXISTS). Every tenant-scoped table needs a tenant_id column keyed from
// config.SelfTenantID(); see the tenant gate in internal/handler/tenant.go.
//
// This is the storage layer of the commented `things` vertical slice:
//
//	schema := `
//		CREATE TABLE IF NOT EXISTS things (
//			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
//			tenant_id TEXT NOT NULL DEFAULT '',
//			name TEXT NOT NULL,
//			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
//		);
//		CREATE INDEX IF NOT EXISTS idx_things_tenant ON things(tenant_id);
//	`
//	_, err := db.Exec(schema)
//	return err
func Migrate(db *sql.DB) error {
	_ = db
	return nil
}
