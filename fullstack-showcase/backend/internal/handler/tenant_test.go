package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moses-platform/fullstack-showcase/internal/middleware"
)

// Direct unit tests for enforceTenantMatch. Rather than routing through
// middleware.MosesHeaders — which would inherit config.SelfTenantID's
// process-wide sync.Once resolution, where whichever test resolves it
// first pins the value for the whole binary — these construct the
// middleware.MosesContext literal directly. No env-derived global is
// touched, so the assertions hold regardless of test ordering and they
// protect the helper independently of whatever handlers this app grows.

func TestEnforceTenantMatch_Mismatch403(t *testing.T) {
	t.Setenv("MOSES_STRICT_TENANT_CHECK", "true")

	rec := httptest.NewRecorder()
	mc := middleware.MosesContext{
		SelfTenantID:   "self-tenant-uuid",
		CallerTenantID: "caller-different-uuid",
	}

	if !enforceTenantMatch(rec, mc) {
		t.Fatal("expected enforceTenantMatch to fire on mismatch")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	got := rec.Body.String()
	if !strings.Contains(got, `"error":"tenant_mismatch"`) || !strings.Contains(got, `"code":"E_TENANT_MISMATCH"`) {
		t.Errorf("expected canonical envelope, got %q", got)
	}
	// CHAT-pxeo.12: body MUST NOT echo either tenant UUID.
	if strings.Contains(got, "self-tenant-uuid") || strings.Contains(got, "caller-different-uuid") {
		t.Errorf("body must not leak tenant ids, got %q", got)
	}
}

func TestEnforceTenantMatch_NoCallerHeader_Passes(t *testing.T) {
	t.Setenv("MOSES_STRICT_TENANT_CHECK", "true")

	rec := httptest.NewRecorder()
	mc := middleware.MosesContext{SelfTenantID: "self-tenant-uuid", CallerTenantID: ""}
	if enforceTenantMatch(rec, mc) {
		t.Fatal("empty caller header must not trip the cross-check")
	}
	if rec.Code == http.StatusForbidden {
		t.Fatal("no 403 expected for empty caller header")
	}
}

func TestEnforceTenantMatch_MatchingCaller_Passes(t *testing.T) {
	t.Setenv("MOSES_STRICT_TENANT_CHECK", "true")

	rec := httptest.NewRecorder()
	mc := middleware.MosesContext{SelfTenantID: "same-uuid", CallerTenantID: "same-uuid"}
	if enforceTenantMatch(rec, mc) {
		t.Fatal("matching caller must not trip the cross-check")
	}
}

func TestEnforceTenantMatch_StrictDisabled_Passes(t *testing.T) {
	t.Setenv("MOSES_STRICT_TENANT_CHECK", "false")

	rec := httptest.NewRecorder()
	mc := middleware.MosesContext{SelfTenantID: "self", CallerTenantID: "mismatched"}
	if enforceTenantMatch(rec, mc) {
		t.Fatal("cross-check must be skipped when MOSES_STRICT_TENANT_CHECK=false")
	}
}

func TestStrictTenantCheckEnabled_Default(t *testing.T) {
	t.Setenv("MOSES_STRICT_TENANT_CHECK", "")
	if !strictTenantCheckEnabled() {
		t.Fatal("strict tenant check must default to enabled")
	}
	for _, off := range []string{"0", "false", "no", "off", "FALSE", "Off"} {
		t.Setenv("MOSES_STRICT_TENANT_CHECK", off)
		if strictTenantCheckEnabled() {
			t.Errorf("value %q must disable the check", off)
		}
	}
}
