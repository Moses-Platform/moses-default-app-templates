// DEMO TESTS — validation coverage for the entries-feed demo handler.
// Removed together with entries.go by clean_out_template.sh.
//
// ORDERING INVARIANT: this file sorts before tenant_test.go, so nothing in
// here may resolve config.SelfTenantID() — its sync.Once cache must first
// be primed by TestSelfTenantID_LazyInitWithEnv (see tenant_test.go). The
// create handler defers the SelfTenantID read until after validation for
// exactly this reason.
package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
