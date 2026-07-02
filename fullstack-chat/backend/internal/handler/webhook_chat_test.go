package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func makePayload(t *testing.T) []byte {
	t.Helper()
	return []byte(`{
		"v": 1,
		"conversationId": "conv-test",
		"actionId": "generate-entry",
		"appSlug": "fullstack-chat",
		"finalMessageId": "msg-1",
		"finalText": "ok",
		"model": "claude",
		"latencyMs": 1234,
		"finishReason": "stop",
		"timestamp": "` + time.Now().UTC().Format(time.RFC3339) + `",
		"nonce": "n-1"
	}`)
}

func sign(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestChatWebhook_AcceptsValidSignature(t *testing.T) {
	secret := []byte("test-secret-32-bytes-test-secret-")
	h := &ChatWebhookHandler{Secret: secret} // DB nil → persist is best-effort

	body := makePayload(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/chat-complete", bytes.NewReader(body))
	req.Header.Set("X-Moses-Signature", sign(secret, body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

// Rotation overlap: signed with the previous secret. The recipient has
// rotated the active secret but kept the previous one configured. The
// dual-slot verifier must accept it.
func TestChatWebhook_AcceptsPreviousSecretDuringRotation(t *testing.T) {
	active := []byte("rotated-active-secret-32-bytes-x")
	previous := []byte("retired-previous-secret-32-bytes")
	h := &ChatWebhookHandler{Secret: active, SecretPrevious: previous}

	body := makePayload(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/chat-complete", bytes.NewReader(body))
	// Sender hasn't rotated yet — still signing with the previous secret.
	req.Header.Set("X-Moses-Signature", sign(previous, body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (previous secret should match during overlap), got %d body=%s", w.Code, w.Body.String())
	}
}

// Negative case: previous secret is configured, but the signature matches
// neither active nor previous. Verifier must still reject.
func TestChatWebhook_RejectsWhenNeitherSecretMatches(t *testing.T) {
	active := []byte("active-secret-32-bytes-padding-x")
	previous := []byte("previous-secret-32-bytes-pad-yyy")
	wrong := []byte("attacker-key-not-known-to-platform")
	h := &ChatWebhookHandler{Secret: active, SecretPrevious: previous}

	body := makePayload(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/chat-complete", bytes.NewReader(body))
	req.Header.Set("X-Moses-Signature", sign(wrong, body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invalid_signature") {
		t.Errorf("expected invalid_signature code in body, got %s", w.Body.String())
	}
}

// Fail-closed: both Active and Previous empty (misconfigured recipient).
// Even a hypothetically "correct" signature must be rejected — there's no
// trusted secret to verify against.
func TestChatWebhook_RejectsWhenBothSecretsEmpty(t *testing.T) {
	h := &ChatWebhookHandler{} // both secrets nil

	body := makePayload(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/chat-complete", bytes.NewReader(body))
	// Any 64-hex header — verifier must reject because no key can verify it.
	req.Header.Set("X-Moses-Signature", "0000000000000000000000000000000000000000000000000000000000000000")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestChatWebhook_RejectsInvalidSignature(t *testing.T) {
	secret := []byte("test-secret-32-bytes-test-secret-")
	h := &ChatWebhookHandler{Secret: secret}

	body := makePayload(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/chat-complete", bytes.NewReader(body))
	req.Header.Set("X-Moses-Signature", "deadbeef00000000000000000000000000000000000000000000000000000000")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invalid_signature") {
		t.Errorf("expected invalid_signature code in body, got %s", w.Body.String())
	}
}

func TestChatWebhook_RejectsStaleTimestamp(t *testing.T) {
	secret := []byte("test-secret-32-bytes-test-secret-")
	h := &ChatWebhookHandler{Secret: secret}

	body := []byte(`{
		"v": 1,
		"conversationId": "conv-test",
		"appSlug": "fullstack-chat",
		"timestamp": "` + time.Now().Add(-10*time.Minute).UTC().Format(time.RFC3339) + `",
		"nonce": "n-stale"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/chat-complete", bytes.NewReader(body))
	req.Header.Set("X-Moses-Signature", sign(secret, body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "timestamp_skew") {
		t.Errorf("expected timestamp_skew code in body, got %s", w.Body.String())
	}
}

func TestChatWebhook_RejectsNonPOST(t *testing.T) {
	h := &ChatWebhookHandler{Secret: []byte("x")}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks/chat-complete", nil)
	w := httptest.NewRecorder()
	h.Handle(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
	if got := w.Header().Get("Allow"); got != "POST" {
		t.Errorf("expected Allow:POST, got %q", got)
	}
}

// recordingSink captures every Record call so tests can assert the audit
// trail (rejected forgeries persist with sigValid=false).
type recordingSink struct {
	calls []struct {
		payload  ChatCompletionPayload
		sigValid bool
	}
}

func (s *recordingSink) Record(_ context.Context, p ChatCompletionPayload, sigValid bool) error {
	s.calls = append(s.calls, struct {
		payload  ChatCompletionPayload
		sigValid bool
	}{p, sigValid})
	return nil
}

// Ordering invariant: a STALE payload with an INVALID signature must land
// on the 401 + audit-persist path, not the 400 timestamp_skew branch —
// signature verification runs before any content-derived check.
func TestChatWebhook_StaleForgeryHits401AuditPath(t *testing.T) {
	secret := []byte("test-secret-32-bytes-test-secret-")
	sink := &recordingSink{}
	h := &ChatWebhookHandler{Secret: secret, Sink: sink}

	body := []byte(`{
		"v": 1,
		"conversationId": "conv-stale-forgery",
		"appSlug": "fullstack-chat",
		"timestamp": "` + time.Now().Add(-10*time.Minute).UTC().Format(time.RFC3339) + `",
		"nonce": "n-stale-forgery"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/chat-complete", bytes.NewReader(body))
	req.Header.Set("X-Moses-Signature", sign([]byte("attacker-key"), body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 (signature checked before skew), got %d body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "timestamp_skew") {
		t.Errorf("stale forgery must NOT surface the skew branch; got %s", w.Body.String())
	}
	if len(sink.calls) != 1 || sink.calls[0].sigValid {
		t.Fatalf("expected exactly one audit persist with sigValid=false, got %+v", sink.calls)
	}
}

// Replay protection: a byte-identical (validly signed) payload re-sent
// within the 5-minute window must be rejected on the nonce.
func TestChatWebhook_RejectsNonceReplay(t *testing.T) {
	secret := []byte("test-secret-32-bytes-test-secret-")
	sink := &recordingSink{}
	h := &ChatWebhookHandler{Secret: secret, Sink: sink}

	body := makePayload(t)
	newReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/chat-complete", bytes.NewReader(body))
		req.Header.Set("X-Moses-Signature", sign(secret, body))
		req.Header.Set("Content-Type", "application/json")
		return req
	}

	w := httptest.NewRecorder()
	h.Handle(w, newReq())
	if w.Code != http.StatusOK {
		t.Fatalf("first delivery: expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	h.Handle(w, newReq())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("replay: expected 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "nonce_replayed") {
		t.Errorf("expected nonce_replayed code, got %s", w.Body.String())
	}
	if len(sink.calls) != 1 {
		t.Errorf("replay must NOT reach the sink; got %d persists", len(sink.calls))
	}
}

// Nonce entries expire: once outside the replay window the same nonce is
// accepted again (the sweep keeps the map bounded).
func TestChatWebhook_NonceExpiresAfterWindow(t *testing.T) {
	h := &ChatWebhookHandler{}
	now := time.Now()
	if !h.storeNonce("n-x", now) {
		t.Fatal("fresh nonce must be accepted")
	}
	if h.storeNonce("n-x", now.Add(time.Minute)) {
		t.Fatal("duplicate inside the window must be rejected")
	}
	if !h.storeNonce("n-x", now.Add(nonceWindow+time.Second)) {
		t.Fatal("nonce past the window must be swept and re-accepted")
	}
}

// AppSlug claim (documented in validate_env.go): when MOSES_APP_SLUG pins
// this app's identity, a signed payload claiming another app is rejected.
func TestChatWebhook_RejectsAppSlugMismatch(t *testing.T) {
	secret := []byte("test-secret-32-bytes-test-secret-")
	sink := &recordingSink{}
	h := &ChatWebhookHandler{Secret: secret, Sink: sink, AppSlug: "some-other-app"}

	body := makePayload(t) // claims appSlug "fullstack-chat"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/chat-complete", bytes.NewReader(body))
	req.Header.Set("X-Moses-Signature", sign(secret, body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "app_slug_mismatch") {
		t.Errorf("expected app_slug_mismatch code, got %s", w.Body.String())
	}
	if len(sink.calls) != 0 {
		t.Errorf("slug-mismatched payload must NOT reach the sink; got %d persists", len(sink.calls))
	}
}

// Matching slug (the steady-state deployed configuration) passes.
func TestChatWebhook_AcceptsMatchingAppSlug(t *testing.T) {
	secret := []byte("test-secret-32-bytes-test-secret-")
	h := &ChatWebhookHandler{Secret: secret, AppSlug: "fullstack-chat"}

	body := makePayload(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/chat-complete", bytes.NewReader(body))
	req.Header.Set("X-Moses-Signature", sign(secret, body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}
