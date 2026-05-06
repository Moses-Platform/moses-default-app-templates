package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/lib/pq"
)

// Config holds database connection configuration
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// NewConfigFromEnv creates a Config from environment variables
func NewConfigFromEnv() Config {
	return Config{
		Host: getEnv("DB_HOST", "localhost"),
		Port: getEnv("DB_PORT", "5432"),
		// CHAT-59o2 (DEPS-B2): defaults aligned with the platform's
		// agent_helm_provisioner.go convention (username="app",
		// database="appdb"). Local-dev standalone runs override these
		// via env if needed.
		User:     getEnv("DB_USER", "app"),
		Password: getEnv("DB_PASSWORD", "showcase-secret"),
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
		db, err = sql.Open("postgres", dsn)
		if err != nil {
			log.Printf("Database open attempt %d/%d failed: %v", i+1, maxRetries, err)
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

		// CHAT-59o2 (DEPS-B2): fail fast on auth errors — they don't self-heal,
		// so 30 retries are wasted time and noisy logs. The most common cause
		// of this in Moses is a chart values.yaml that pre-creates the postgres
		// dep with a different username than the platform-injected DB_USER.
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "28P01" {
			db.Close()
			return nil, fmt.Errorf(
				"DB password mismatch (pq 28P01 invalid_password) — verify chart's postgresql.auth.username matches DB_USER=%q. error: %w",
				cfg.User, err,
			)
		}

		log.Printf("Database ping attempt %d/%d failed: %v", i+1, maxRetries, err)
		db.Close()
		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
}

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

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
