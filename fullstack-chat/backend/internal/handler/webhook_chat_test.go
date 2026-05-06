package handler

import (
	"bytes"
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

func TestEntries_RejectsEmptyText(t *testing.T) {
	h := &EntriesHandler{} // DB nil — list will fail; we only test create validation here
	body := []byte(`{"text": "   ", "source": "moses_manager"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entries", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Entries(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestEntries_RejectsInvalidSource(t *testing.T) {
	h := &EntriesHandler{}
	body := []byte(`{"text": "hi", "source": "not-a-known-source"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entries", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Entries(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invalid_source") {
		t.Errorf("expected invalid_source code, got %s", w.Body.String())
	}
}

func TestEntries_RejectsLongText(t *testing.T) {
	h := &EntriesHandler{}
	long := strings.Repeat("a", 281)
	body := []byte(`{"text": "` + long + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entries", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Entries(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}
