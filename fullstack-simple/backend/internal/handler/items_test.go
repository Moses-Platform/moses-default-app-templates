package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// itemsCreate POSTs a fresh item via the handler and returns its ID.
// Mirrors the production wiring (mux dispatches to Handle for the
// collection route). Caller is expected to have set MOSES_TENANT_ID via
// t.Setenv before calling, OR rely on the default 'local-dev' sentinel.
func itemsCreate(t *testing.T, h *ItemsHandler) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"title": "test"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/items", bytes.NewReader(body))
	// CHAT-pxeo.12: header is caller-audit, not authoritative for storage.
	// Setting it to match the env keeps the cross-check happy; storage key
	// comes from config.SelfTenantID().
	rec := httptest.NewRecorder()
	h.Handle(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d body=%q", rec.Code, rec.Body.String())
	}
	var got Item
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("create: decode: %v", err)
	}
	return got.ID
}

// TestItemsDelete_RootMount verifies the delete handler works at the root
// /api/v1/items/<id> path (standalone deploy, no MOSES_BASE_PATH).
func TestItemsDelete_RootMount(t *testing.T) {
	h := NewItemsHandler()
	id := itemsCreate(t, h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/items/"+id, nil)
	rec := httptest.NewRecorder()
	h.HandleWithID(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("DELETE /api/v1/items/%s: expected 204, got %d body=%q", id, rec.Code, rec.Body.String())
	}
}

// TestItemsDelete_SubPath is the CHAT-pbup.17 regression. At the
// MOSES_BASE_PATH-prefixed mount the URL becomes
// /apps/<tenant>/<slug>/api/v1/items/<id>. The previous
// strings.TrimPrefix(r.URL.Path, "/api/v1/items/") returned the literal
// sub-path-prefixed string and the lookup 404'd. FAILS on the old code,
// PASSES on the literal-segment fix.
func TestItemsDelete_SubPath(t *testing.T) {
	h := NewItemsHandler()
	id := itemsCreate(t, h)

	subPath := "/apps/tenant/slug/api/v1/items/" + id
	req := httptest.NewRequest(http.MethodDelete, subPath, nil)
	rec := httptest.NewRecorder()
	h.HandleWithID(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("DELETE %s: expected 204 (CHAT-pbup.17), got %d body=%q", subPath, rec.Code, rec.Body.String())
	}
}

// TestItemsDelete_EmptyID400 verifies the missing-ID guard still triggers
// at both mounts after the literal-segment refactor.
func TestItemsDelete_EmptyID400(t *testing.T) {
	h := NewItemsHandler()
	for _, urlPath := range []string{
		"/api/v1/items/",
		"/apps/t/x/api/v1/items/",
	} {
		req := httptest.NewRequest(http.MethodDelete, urlPath, nil)
		rec := httptest.NewRecorder()
		h.HandleWithID(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("DELETE %s: expected 400 (empty ID), got %d", urlPath, rec.Code)
		}
	}
}

// CHAT-pxeo.12 acceptance test #2 (cross-check): when the caller sends a
// X-Moses-Tenant-ID that disagrees with the deploy-pinned env, the write
// is rejected with 403 + the canonical error envelope. The body MUST NOT
// echo either UUID.
func TestItems_TenantMismatch403(t *testing.T) {
	h := NewItemsHandler()
	body, _ := json.Marshal(map[string]string{"title": "x"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/items", bytes.NewReader(body))
	req.Header.Set("X-Moses-Tenant-ID", "header-different-tenant-uuid")
	rec := httptest.NewRecorder()
	h.Handle(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%q", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	if !strings.Contains(got, `"error":"tenant_mismatch"`) {
		t.Errorf("expected error tenant_mismatch in body, got %q", got)
	}
	if !strings.Contains(got, `"code":"E_TENANT_MISMATCH"`) {
		t.Errorf("expected code E_TENANT_MISMATCH in body, got %q", got)
	}
	if strings.Contains(got, "header-different-tenant-uuid") {
		t.Errorf("body must NOT echo caller tenant; got %q", got)
	}
}

// CHAT-pxeo.12 — the cross-check is opt-out via env knob.
func TestItems_StrictTenantCheckDisabled(t *testing.T) {
	t.Setenv("MOSES_STRICT_TENANT_CHECK", "false")
	h := NewItemsHandler()
	body, _ := json.Marshal(map[string]string{"title": "x"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/items", bytes.NewReader(body))
	req.Header.Set("X-Moses-Tenant-ID", "still-mismatched")
	rec := httptest.NewRecorder()
	h.Handle(rec, req)

	if rec.Code == http.StatusForbidden {
		t.Errorf("expected cross-check skipped with MOSES_STRICT_TENANT_CHECK=false, got 403")
	}
}
