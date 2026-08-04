package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moses-platform/backend-template/internal/middleware"
)

// CHAT-pxeo.12: plumbing-level tests for the tenant cross-check helpers.
// These build the MosesContext directly (no env / no store) so they hold
// independently of whatever handlers this app grows.

func TestEnforceTenantMatch_Mismatch403(t *testing.T) {
	t.Setenv("MOSES_STRICT_TENANT_CHECK", "")

	rec := httptest.NewRecorder()
	mc := &middleware.MosesContext{
		SelfTenantID:   "self-tenant-uuid-deploy-pinned",
		CallerTenantID: "caller-different-uuid",
	}
	if !enforceTenantMatch(rec, mc) {
		t.Fatal("expected enforceTenantMatch to write a 403 on mismatch")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	got := rec.Body.String()
	if !strings.Contains(got, `"error":"tenant_mismatch"`) || !strings.Contains(got, `"code":"E_TENANT_MISMATCH"`) {
		t.Errorf("expected canonical tenant_mismatch envelope, got %q", got)
	}
	// Body MUST NOT echo any tenant UUID.
	if strings.Contains(got, "caller-different-uuid") || strings.Contains(got, "self-tenant-uuid-deploy-pinned") {
		t.Errorf("body must NOT echo tenant UUIDs; got %q", got)
	}
}

func TestEnforceTenantMatch_NoCallerHeaderAllowed(t *testing.T) {
	t.Setenv("MOSES_STRICT_TENANT_CHECK", "")

	rec := httptest.NewRecorder()
	mc := &middleware.MosesContext{SelfTenantID: "self-tenant", CallerTenantID: ""}
	if enforceTenantMatch(rec, mc) {
		t.Error("browser-driven requests carry no X-Moses-Tenant-ID; must not 403")
	}

	mc = &middleware.MosesContext{SelfTenantID: "self-tenant", CallerTenantID: "self-tenant"}
	if enforceTenantMatch(rec, mc) {
		t.Error("matching caller tenant must not 403")
	}
}

// CHAT-pxeo.12: opt-out via env knob.
func TestEnforceTenantMatch_Disabled(t *testing.T) {
	t.Setenv("MOSES_STRICT_TENANT_CHECK", "false")

	rec := httptest.NewRecorder()
	mc := &middleware.MosesContext{SelfTenantID: "self-tenant", CallerTenantID: "still-mismatched"}
	if enforceTenantMatch(rec, mc) {
		t.Error("cross-check should be skipped when MOSES_STRICT_TENANT_CHECK=false")
	}
}

func TestStrictTenantCheckEnabled_Values(t *testing.T) {
	cases := map[string]bool{
		"":      true,
		"1":     true,
		"true":  true,
		"yes":   true,
		"0":     false,
		"false": false,
		"no":    false,
		"off":   false,
		"OFF":   false,
	}
	for v, want := range cases {
		t.Setenv("MOSES_STRICT_TENANT_CHECK", v)
		if got := strictTenantCheckEnabled(); got != want {
			t.Errorf("MOSES_STRICT_TENANT_CHECK=%q: got %v, want %v", v, got, want)
		}
	}
}
