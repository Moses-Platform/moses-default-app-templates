package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moses-platform/fullstack-showcase/internal/middleware"
)

// TestNotes_ListNotes_TenantMismatch403 covers the 403 cross-check on the
// read handler. Tests run through real middleware so the MosesContext is
// populated identically to production.
//
// CHAT-pxeo.12: body MUST contain the canonical error envelope and MUST
// NOT echo any tenant UUID.
func TestNotes_ListNotes_TenantMismatch403(t *testing.T) {
	t.Setenv("MOSES_TENANT_ID", "self-tenant-uuid-deploy-pinned")
	t.Setenv("MOSES_DEPLOYED", "")
	t.Setenv("MOSES_STRICT_TENANT_CHECK", "true")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notes", nil)
	req.Header.Set("X-Moses-Tenant-ID", "caller-different-uuid")
	rec := httptest.NewRecorder()

	mux := http.NewServeMux()
	h := &NotesHandler{} // db nil — cross-check fires before any DB access
	mux.HandleFunc("/api/v1/notes", h.ListNotes)
	middleware.MosesHeaders(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%q", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	if !strings.Contains(got, `"error":"tenant_mismatch"`) {
		t.Errorf("expected tenant_mismatch in body, got %q", got)
	}
	if !strings.Contains(got, `"code":"E_TENANT_MISMATCH"`) {
		t.Errorf("expected E_TENANT_MISMATCH in body, got %q", got)
	}
	if strings.Contains(got, "caller-different-uuid") {
		t.Errorf("body must NOT echo caller tenant; got %q", got)
	}
}

// TestNotes_CreateNote_TenantMismatch403 mirrors the check on a write
// handler.
func TestNotes_CreateNote_TenantMismatch403(t *testing.T) {
	t.Setenv("MOSES_STRICT_TENANT_CHECK", "true")

	body := []byte(`{"title":"x","content":"y"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Moses-Tenant-ID", "caller-different-uuid")
	rec := httptest.NewRecorder()

	mux := http.NewServeMux()
	h := &NotesHandler{} // db nil
	mux.HandleFunc("/api/v1/notes", h.CreateNote)
	middleware.MosesHeaders(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%q", rec.Code, rec.Body.String())
	}
}

// TestNotes_StrictTenantCheckDisabled — opt-out behaviour. The
// underlying DB is nil so the test recovers from the post-check panic;
// the assertion is that the 403 path is NOT taken.
func TestNotes_StrictTenantCheckDisabled(t *testing.T) {
	t.Setenv("MOSES_STRICT_TENANT_CHECK", "false")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notes", nil)
	req.Header.Set("X-Moses-Tenant-ID", "still-mismatched")
	rec := httptest.NewRecorder()

	mux := http.NewServeMux()
	h := &NotesHandler{}
	mux.HandleFunc("/api/v1/notes", h.ListNotes)
	defer func() { _ = recover() }() // nil DB will panic past the check
	middleware.MosesHeaders(mux).ServeHTTP(rec, req)

	if rec.Code == http.StatusForbidden {
		t.Errorf("expected cross-check to be skipped, got 403")
	}
}
