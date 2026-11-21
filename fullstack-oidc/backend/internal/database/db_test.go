package database

import (
	"os"
	"testing"
)

func TestMissingPasswordFailsBeforeConnecting(t *testing.T) {
	for _, unset := range []bool{false, true} {
		name := "empty"
		if unset {
			name = "unset"
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("DB_PASSWORD", "")
			if unset {
				if err := os.Unsetenv("DB_PASSWORD"); err != nil {
					t.Fatal(err)
				}
			}
			cfg := NewConfigFromEnv()
			if cfg.Password != "" {
				t.Fatal("missing DB_PASSWORD must not acquire a default")
			}
			db, err := Connect(cfg)
			if db != nil || err == nil || err.Error() != "DB_PASSWORD is required: supply it from the PostgreSQL namespace Secret" {
				t.Fatalf("expected missing-credential error before connection, got db=%v err=%v", db, err)
			}
		})
	}
}
