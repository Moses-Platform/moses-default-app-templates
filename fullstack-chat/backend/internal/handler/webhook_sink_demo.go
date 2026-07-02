// DEMO FILE — the Postgres CompletionSink writing to the demo
// chat_completions table (schema in internal/database/migrate_demo.go).
// Removed by clean_out_template.sh; the twin demo_routes.go wires a
// LogCompletionSink instead until the app defines its own storage.
package handler

import (
	"context"
	"database/sql"

	"github.com/moses-platform/fullstack-chat/internal/config"
)

// DBCompletionSink persists webhook deliveries into the demo
// chat_completions table for end-to-end roundtrip inspection.
type DBCompletionSink struct {
	DB *sql.DB
}

// NewDBCompletionSink wires the sink with the platform-injected DB.
func NewDBCompletionSink(db *sql.DB) *DBCompletionSink {
	return &DBCompletionSink{DB: db}
}

// Record implements CompletionSink.
func (s *DBCompletionSink) Record(ctx context.Context, p ChatCompletionPayload, sigValid bool) error {
	if s.DB == nil {
		return nil // best-effort in unit tests with nil DB
	}
	// CHAT-pxeo.12: storage key is deploy-pinned (env). The platform→app
	// webhook is HMAC-signed body only; no X-Moses-Tenant-ID header is
	// sent here, so the cross-check skips cleanly. Spec deliberately does
	// NOT add a 403 path on this route.
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO chat_completions
		   (tenant_id, conversation_id, final_message_id, final_text, model, latency_ms, finish_reason, signature_valid)
		 VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), NULLIF($6,0), NULLIF($7,''), $8)`,
		config.SelfTenantID(),
		p.ConversationID,
		p.FinalMessageID,
		p.FinalText,
		p.Model,
		p.LatencyMs,
		p.FinishReason,
		sigValid,
	)
	return err
}
