package handler

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// Direct unit tests for the tenant-contract helpers (CHAT-pxeo.12). The
// helpers are platform plumbing, so the contract stays covered here
// independently of whatever handlers this app grows.

// CHAT-pxeo.12 acceptance (cross-check): a caller-supplied X-Moses-Tenant-ID
// that disagrees with the deploy-pinned MOSES_TENANT_ID env is rejected with
// 403 + the canonical error envelope. The body MUST NOT echo either UUID.
func TestEnforceTenantMatch_Mismatch403(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/anything", nil)
	req.Header.Set("X-Moses-Tenant-ID", "header-different-tenant-uuid")
	rec := httptest.NewRecorder()

	if !enforceTenantMatch(rec, req) {
		t.Fatal("expected enforceTenantMatch to write the 403 and return true")
	}
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

// No header (browser-driven request through the user-facing app proxy) or a
// matching header (workspace-tool caller) must both pass.
func TestEnforceTenantMatch_AbsentOrMatchingHeaderPasses(t *testing.T) {
	for _, header := range []string{"", "local-dev"} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/anything", nil)
		if header != "" {
			req.Header.Set("X-Moses-Tenant-ID", header)
		}
		rec := httptest.NewRecorder()
		if enforceTenantMatch(rec, req) {
			t.Errorf("header=%q: expected pass-through, got 403 body=%q", header, rec.Body.String())
		}
	}
}

// CHAT-pxeo.12 — the cross-check is opt-out via env knob.
func TestEnforceTenantMatch_StrictCheckDisabled(t *testing.T) {
	t.Setenv("MOSES_STRICT_TENANT_CHECK", "false")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/anything", nil)
	req.Header.Set("X-Moses-Tenant-ID", "still-mismatched")
	rec := httptest.NewRecorder()
	if enforceTenantMatch(rec, req) {
		t.Errorf("expected cross-check skipped with MOSES_STRICT_TENANT_CHECK=false, got 403")
	}
}

func TestStrictTenantCheckEnabled_Values(t *testing.T) {
	cases := []struct {
		env  string
		want bool
	}{
		{"", true},
		{"true", true},
		{"1", true},
		{"anything-else", true},
		{"0", false},
		{"false", false},
		{"FALSE", false},
		{"no", false},
		{"off", false},
	}
	for _, tc := range cases {
		t.Setenv("MOSES_STRICT_TENANT_CHECK", tc.env)
		if got := strictTenantCheckEnabled(); got != tc.want {
			t.Errorf("MOSES_STRICT_TENANT_CHECK=%q: got %v, want %v", tc.env, got, tc.want)
		}
	}
}

func TestGenerateUUID_V4Format(t *testing.T) {
	v4 := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for i := 0; i < 32; i++ {
		id := generateUUID()
		if !v4.MatchString(id) {
			t.Fatalf("generateUUID() = %q, not a v4 UUID", id)
		}
		if seen[id] {
			t.Fatalf("generateUUID() produced a duplicate: %q", id)
		}
		seen[id] = true
	}
}
