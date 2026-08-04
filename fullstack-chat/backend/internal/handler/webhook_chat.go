package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
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

// CompletionSink receives verified (and, for the audit trail, rejected)
// webhook deliveries. The receiver below is pure plumbing — signature
// verification, replay protection, claim checks, response shape — and
// delegates persistence to this interface so the storage schema is the
// app's own concern. The template wires the log-only sink below; replace
// it with your own (e.g. a Postgres sink) in newCompletionSink.
type CompletionSink interface {
	// Record persists one delivery. sigValid=false rows are the audit
	// trail for rejected forgery attempts; keep them distinguishable.
	Record(ctx context.Context, p ChatCompletionPayload, sigValid bool) error
}

// LogCompletionSink is the no-schema fallback sink: it logs the delivery
// and stores nothing. Used until the app wires a real sink.
type LogCompletionSink struct{}

// Record implements CompletionSink.
func (LogCompletionSink) Record(_ context.Context, p ChatCompletionPayload, sigValid bool) error {
	log.Printf("[chat-webhook] sink=log conv=%s action=%s sigValid=%t reason=%s",
		p.ConversationID, p.ActionID, sigValid, p.FinishReason)
	return nil
}

// nonceWindow is the replay-rejection window. Matches the 5-minute
// timestamp clock-skew window: a nonce only needs to be remembered for as
// long as its payload would still pass the skew check.
const nonceWindow = 5 * time.Minute

// ChatWebhookHandler receives the chat-completion callback Moses fires after
// a chat_prompt action's AI turn finishes. Verifies the HMAC signature with
// the per-app webhook secret, rejects stale and replayed payloads, checks
// the appSlug claim, then hands the payload to the configured Sink.
//
// Dual-slot rotation (CHAT-v5al): during the 24h overlap window after a
// secret rotation the platform's sender flips immediately to the new active
// secret, but the previous secret stays valid. Configure
// MOSES_CHAT_WEBHOOK_SECRET_PREVIOUS to accept signatures from both during
// that window. Verification tries Active first (the steady-state hot path),
// then falls back to Previous if set.
type ChatWebhookHandler struct {
	Sink           CompletionSink // persistence target; nil → drop (unit tests)
	Secret         []byte         // active signing secret (env: MOSES_CHAT_WEBHOOK_SECRET)
	SecretPrevious []byte         // optional, accepted during rotation overlap (env: MOSES_CHAT_WEBHOOK_SECRET_PREVIOUS)
	AppSlug        string         // expected payload.appSlug claim (env: MOSES_APP_SLUG); empty → check skipped

	// Nonce replay protection: nonce → expiry. Guarded by nonceMu;
	// expired entries are swept on every insert (the rate-limited webhook
	// traffic makes an O(n) sweep per delivery trivially cheap).
	nonceMu   sync.Mutex
	nonceSeen map[string]time.Time
}

// NewChatWebhookHandler reads the shared secret(s) and expected app slug
// from the environment and wires the persistence sink. In a production
// install Moses provisions the active secret via the app's
// app_integration_grant; Previous is set by the operator at rotation time
// and unset once the overlap window closes. For local dev set
// MOSES_CHAT_WEBHOOK_SECRET (Previous is optional).
func NewChatWebhookHandler(sink CompletionSink) *ChatWebhookHandler {
	return &ChatWebhookHandler{
		Sink:           sink,
		Secret:         []byte(os.Getenv("MOSES_CHAT_WEBHOOK_SECRET")),
		SecretPrevious: []byte(os.Getenv("MOSES_CHAT_WEBHOOK_SECRET_PREVIOUS")),
		AppSlug:        strings.TrimSpace(os.Getenv("MOSES_APP_SLUG")),
	}
}

// Handle is mounted at POST /api/v1/webhooks/chat-complete.
//
// Check order is load-bearing: the HMAC signature is verified FIRST so a
// forged payload always lands on the 401 + audit-persist path — never on a
// content-derived 4xx (timestamp skew, slug mismatch) that would leak
// which checks a forgery passed. Only authenticated payloads proceed to
// the freshness / replay / claim checks.
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

	if !signatureValid {
		// Don't 401-fail the writer — record the attempt (audit trail) but
		// reject. Production deployments may strict-fail here; for the
		// roundtrip-test template we record + return 401 so investigators
		// see the row.
		h.record(r.Context(), payload, false)
		writeError(w, http.StatusUnauthorized, "invalid_signature",
			"X-Moses-Signature did not match expected HMAC-SHA256")
		return
	}

	// AppSlug claim check: when the platform pinned our slug into the pod
	// env, a signed payload claiming a different app is a routing error
	// (or a cross-app secret leak) — reject it.
	if h.AppSlug != "" && payload.AppSlug != h.AppSlug {
		writeError(w, http.StatusForbidden, "app_slug_mismatch",
			"payload appSlug does not match this app's MOSES_APP_SLUG")
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

	// Nonce replay protection: within the skew window a captured payload
	// replays byte-identical (same signature), so the timestamp check
	// alone doesn't stop it. Remember each nonce for nonceWindow and
	// reject duplicates.
	if payload.Nonce != "" && !h.storeNonce(payload.Nonce, time.Now()) {
		writeError(w, http.StatusBadRequest, "nonce_replayed",
			"payload nonce was already accepted within the replay window")
		return
	}

	if err := h.recordErr(r.Context(), payload, true); err != nil {
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

// storeNonce records nonce as seen at time now and reports whether it was
// fresh. Expired entries are swept before the lookup so the map stays
// bounded by the traffic of the last nonceWindow.
func (h *ChatWebhookHandler) storeNonce(nonce string, now time.Time) bool {
	h.nonceMu.Lock()
	defer h.nonceMu.Unlock()
	if h.nonceSeen == nil {
		h.nonceSeen = make(map[string]time.Time)
	}
	for n, exp := range h.nonceSeen {
		if now.After(exp) {
			delete(h.nonceSeen, n)
		}
	}
	if _, dup := h.nonceSeen[nonce]; dup {
		return false
	}
	h.nonceSeen[nonce] = now.Add(nonceWindow)
	return true
}

// verifySignature tries Active first, then Previous if it's set (rotation
// overlap). hmac.Equal is constant-time. An empty header or an unparseable
// hex string is rejected outright. If BOTH secrets are empty (misconfigured
// recipient) every payload is rejected — this is intentional fail-closed.
func (h *ChatWebhookHandler) verifySignature(body []byte, header string) bool {
	if header == "" {
		return false
	}
	expected, err := hex.DecodeString(header)
	if err != nil || len(expected) != sha256.Size {
		return false
	}
	for _, key := range [][]byte{h.Secret, h.SecretPrevious} {
		if len(key) == 0 {
			continue
		}
		mac := hmac.New(sha256.New, key)
		mac.Write(body)
		if hmac.Equal(mac.Sum(nil), expected) {
			return true
		}
	}
	return false
}

// record is the best-effort variant (audit rows for rejected payloads).
func (h *ChatWebhookHandler) record(ctx context.Context, p ChatCompletionPayload, sigValid bool) {
	if err := h.recordErr(ctx, p, sigValid); err != nil {
		log.Printf("[chat-webhook] audit persist failed (sigValid=%t): %v", sigValid, err)
	}
}

func (h *ChatWebhookHandler) recordErr(ctx context.Context, p ChatCompletionPayload, sigValid bool) error {
	if h.Sink == nil {
		return nil // best-effort in unit tests without a sink
	}
	return h.Sink.Record(ctx, p, sigValid)
}
