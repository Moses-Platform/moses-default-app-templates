package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// ChatCompletionPayload mirrors the Moses-side outbound webhook envelope
// (CHAT-rg5t). Field names match the platform's documented contract.
type ChatCompletionPayload struct {
	V              int    `json:"v"`
	ConversationID string `json:"conversationId"`
	ActionID       string `json:"actionId"`
	AppSlug        string `json:"appSlug"`
	FinalMessageID string `json:"finalMessageId"`
	FinalText      string `json:"finalText"`
	Model          string `json:"model"`
	LatencyMs      int    `json:"latencyMs"`
	FinishReason   string `json:"finishReason"`
	Timestamp      string `json:"timestamp"` // RFC3339
	Nonce          string `json:"nonce"`
}

// ChatWebhookHandler receives the chat-completion callback Moses fires after
// a chat_prompt action's AI turn finishes. Verifies the HMAC signature with
// the per-app webhook secret and persists a row for inspection.
type ChatWebhookHandler struct {
	DB     *sql.DB
	Secret []byte // shared signing secret (env: MOSES_CHAT_WEBHOOK_SECRET)
}

// NewChatWebhookHandler reads the shared secret from the environment. In a
// production install Moses provisions this via the app's
// app_integration_grant; for local dev set MOSES_CHAT_WEBHOOK_SECRET.
func NewChatWebhookHandler(db *sql.DB) *ChatWebhookHandler {
	return &ChatWebhookHandler{
		DB:     db,
		Secret: []byte(os.Getenv("MOSES_CHAT_WEBHOOK_SECRET")),
	}
}

// Handle is mounted at POST /api/v1/webhooks/chat-complete.
func (h *ChatWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_failed", err.Error())
		return
	}
	defer r.Body.Close()

	signatureHeader := strings.TrimSpace(r.Header.Get("X-Moses-Signature"))
	signatureValid := h.verifySignature(body, signatureHeader)

	var payload ChatCompletionPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	// Reject stale payloads — 5-minute clock-skew window per the documented
	// recipient contract. Mitigates replay even if the signature leaks.
	if payload.Timestamp != "" {
		if t, perr := time.Parse(time.RFC3339, payload.Timestamp); perr == nil {
			if drift := time.Since(t); drift > 5*time.Minute || drift < -5*time.Minute {
				writeError(w, http.StatusBadRequest, "timestamp_skew",
					"payload timestamp outside 5-minute clock-skew window")
				return
			}
		}
	}

	if !signatureValid {
		// Don't 401-fail the writer — record the attempt (audit trail) but
		// reject. Production deployments may strict-fail here; for the
		// roundtrip-test template we record + return 401 so investigators
		// see the row.
		_ = h.persist(r, payload, false)
		writeError(w, http.StatusUnauthorized, "invalid_signature",
			"X-Moses-Signature did not match expected HMAC-SHA256")
		return
	}

	if err := h.persist(r, payload, true); err != nil {
		writeError(w, http.StatusInternalServerError, "persist_failed", err.Error())
		return
	}

	log.Printf("[chat-webhook] conv=%s reason=%s model=%s latency=%dms",
		payload.ConversationID, payload.FinishReason, payload.Model, payload.LatencyMs)

	writeJSON(w, http.StatusOK, map[string]any{
		"received":       true,
		"conversationId": payload.ConversationID,
	})
}

func (h *ChatWebhookHandler) verifySignature(body []byte, header string) bool {
	if len(h.Secret) == 0 || header == "" {
		return false
	}
	expected, err := hex.DecodeString(header)
	if err != nil || len(expected) != sha256.Size {
		return false
	}
	mac := hmac.New(sha256.New, h.Secret)
	mac.Write(body)
	return hmac.Equal(mac.Sum(nil), expected)
}

func (h *ChatWebhookHandler) persist(r *http.Request, p ChatCompletionPayload, sigValid bool) error {
	if h.DB == nil {
		return nil // best-effort in unit tests with nil DB
	}
	_, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO chat_completions
		   (tenant_id, conversation_id, final_message_id, final_text, model, latency_ms, finish_reason, signature_valid)
		 VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), NULLIF($6,0), NULLIF($7,''), $8)`,
		mosesTenant(r),
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
