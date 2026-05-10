package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moses-platform/backend-template/internal/middleware"
	"github.com/moses-platform/backend-template/internal/model"
)

// CHAT-pxeo.12: 403 cross-check on the write handler. Body MUST contain
// the canonical error envelope and MUST NOT echo any tenant UUID.
func TestItems_CreateItem_TenantMismatch403(t *testing.T) {
	t.Setenv("MOSES_TENANT_ID", "self-tenant-uuid-deploy-pinned")
	t.Setenv("MOSES_DEPLOYED", "")
	t.Setenv("MOSES_STRICT_TENANT_CHECK", "true")

	store := model.NewItemStore()
	h := NewItemHandler(store)

	body := []byte(`{"name":"x","description":"y"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/items", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Moses-Tenant-ID", "caller-different-uuid")
	rec := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/items", h.CreateItem)
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
	if strings.Contains(got, "caller-different-uuid") || strings.Contains(got, "self-tenant-uuid-deploy-pinned") {
		t.Errorf("body must NOT echo tenant UUIDs; got %q", got)
	}
}

// CHAT-pxeo.12: opt-out via env knob.
func TestItems_StrictTenantCheckDisabled(t *testing.T) {
	t.Setenv("MOSES_STRICT_TENANT_CHECK", "false")

	store := model.NewItemStore()
	h := NewItemHandler(store)

	body := []byte(`{"name":"x","description":"y"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/items", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Moses-Tenant-ID", "still-mismatched")
	rec := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/items", h.CreateItem)
	middleware.MosesHeaders(mux).ServeHTTP(rec, req)

	if rec.Code == http.StatusForbidden {
		t.Errorf("cross-check should be skipped when MOSES_STRICT_TENANT_CHECK=false, got 403")
	}
}
