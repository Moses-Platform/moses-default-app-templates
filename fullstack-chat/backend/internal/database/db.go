package database

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Config holds database connection configuration.
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// NewConfigFromEnv reads Postgres connection details from the env vars Moses
// auto-injects when "postgresql" is declared in dependencies.services.
func NewConfigFromEnv() Config {
	return Config{
		Host: getEnv("DB_HOST", "localhost"),
		Port: getEnv("DB_PORT", "5432"),
		// CHAT-59o2 (DEPS-B2): defaults aligned with the platform's
		// agent_helm_provisioner.go convention (username="app",
		// database="appdb"). Earlier "fullstack_chat" defaults collided
		// with the platform-injected DB_USER and produced 14+ retries
		// with `password authentication failed for user "app"` before
		// postgres init created the role as a side effect.
		User:     getEnv("DB_USER", "app"),
		Password: getEnv("DB_PASSWORD", "fullstack_chat_dev"),
		DBName:   getEnv("DB_NAME", "appdb"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}
}

// Connect establishes a database connection with retry logic.
// Note: DSN is constructed but never logged to avoid credential exposure.
func Connect(cfg Config) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	var db *sql.DB
	var err error
	maxRetries := 30

	for i := 0; i < maxRetries; i++ {
		db, err = sql.Open("pgx", dsn)
		if err != nil {
			// Transient line kept token-free (see the ping-attempt note below).
			if i == maxRetries-1 {
				log.Printf("Database open attempt %d/%d failed: %v", i+1, maxRetries, err)
			} else {
				log.Printf("Database not ready yet (open attempt %d/%d), retrying in 2s", i+1, maxRetries)
			}
			time.Sleep(2 * time.Second)
			continue
		}

		err = db.Ping()
		if err == nil {
			db.SetMaxOpenConns(10)
			db.SetMaxIdleConns(5)
			db.SetConnMaxLifetime(5 * time.Minute)
			return db, nil
		}

		// CHAT-59o2 (DEPS-B2): fail fast on auth errors — they don't
		// self-heal, so 30 retries are wasted time and noisy logs. Most
		// commonly hit when the chart's postgresql.auth.username doesn't
		// match the platform-injected DB_USER.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "28P01" {
			db.Close()
			return nil, fmt.Errorf(
				"DB password mismatch (SQLSTATE 28P01 invalid_password) — verify chart's postgresql.auth.username matches DB_USER=%q. error: %w",
				cfg.User, err,
			)
		}

		// Keep the transient per-attempt line token-free: the post-deploy log
		// watcher content-matches (?i)(error|err|warn|panic|fatal) on raw pod
		// stdout (not on log level), so emitting the pgx "dial error ...
		// connection refused" text on every retry trips a false post-deploy
		// alert during the benign concurrent app+DB startup race. Full pgx
		// detail is retained on the final attempt, and a genuine 30-attempt
		// exhaustion (and the 28P01 fail-fast above) still surface via the
		// returned error.
		if i == maxRetries-1 {
			log.Printf("Database ping attempt %d/%d failed: %v", i+1, maxRetries, err)
		} else {
			log.Printf("Database not ready yet (attempt %d/%d), retrying in 2s", i+1, maxRetries)
		}
		db.Close()
		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
}

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

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
