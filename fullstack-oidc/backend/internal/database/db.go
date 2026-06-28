// Package database provides the PostgreSQL connection + schema for the
// fullstack-oidc template. It mirrors fullstack-showcase's database
// package so the OIDC template's data story matches the rest of the
// fleet — a simple per-tenant table the demo handlers read/write.
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

// NewConfigFromEnv creates a Config from environment variables. Defaults
// align with the platform's agent_helm_provisioner.go convention
// (username="app", database="appdb").
func NewConfigFromEnv() Config {
	return Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "app"),
		Password: getEnv("DB_PASSWORD", "oidc-secret"),
		DBName:   getEnv("DB_NAME", "appdb"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}
}

// Connect establishes a database connection with retry logic.
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

		// Fail fast on auth errors — they don't self-heal.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "28P01" {
			db.Close()
			return nil, fmt.Errorf(
				"DB password mismatch (SQLSTATE 28P01) — verify chart's postgresql.auth.username matches DB_USER=%q: %w",
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

// Migrate runs schema migrations. The `entries` table is owned per
// tenant AND per OIDC subject — the demo shows that an authenticated
// user's data is scoped to their identity.
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
