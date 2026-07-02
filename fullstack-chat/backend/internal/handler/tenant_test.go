package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moses-platform/fullstack-chat/internal/config"
)

// CHAT-pxeo.12 acceptance tests, exercised directly against the tenant
// helpers in tenant.go (no demo handlers involved, so this file survives
// clean_out_template.sh). The package-level config.SelfTenantID()
// resolves once via sync.Once, so these tests must NOT call it before
// the first t.Setenv they care about. Tests share global state — when
// adding new ones be mindful of ordering.

// TestSelfTenantID_LazyInitWithEnv covers acceptance test #5: importing
// config does not panic, and t.Setenv before the FIRST call returns the
// test value. Because sync.Once locks in the first observed value for
// the rest of the test binary, this test name is alphabetically chosen
// to be the first that reads SelfTenantID. Running `go test ./handler`
// alphabetically orders tests; "TestSelfTenantID_LazyInitWithEnv" is
// before any other config-touching test in this package.
func TestSelfTenantID_LazyInitWithEnv(t *testing.T) {
	t.Setenv("MOSES_TENANT_ID", "tenant-aaaa-bbbb-cccc-dddd")
	got := config.SelfTenantID()
	if got != "tenant-aaaa-bbbb-cccc-dddd" {
		t.Fatalf("expected tenant from env, got %q", got)
	}
}

// TestTenantGate_CachedEnvValue covers acceptance test #1: the cached
// SelfTenantID (locked in by the lazy-init test above) is what every
// write handler keys storage on.
func TestTenantGate_CachedEnvValue(t *testing.T) {
	if got := config.SelfTenantID(); got == "" || got == "local-dev" {
		t.Fatalf("preconditions: expected non-sentinel cached tenant, got %q (alphabetical-test-order invariant broken?)", got)
	}
}

// TestTenantGate_Mismatch403 covers acceptance test #2, straight through
// enforceTenantMatch: env=A (cached), header=B → 403, body has the
// canonical error+code, body does NOT contain the tenant UUIDs.
func TestTenantGate_Mismatch403(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/anything", nil)
	req.Header.Set("X-Moses-Tenant-ID", "header-tenant-uuid-different-1")
	rec := httptest.NewRecorder()

	if !enforceTenantMatch(rec, req) {
		t.Fatal("expected enforceTenantMatch to report a mismatch (true)")
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
	if strings.Contains(got, "header-tenant-uuid-different-1") {
		t.Errorf("body must NOT echo caller tenant UUID; got %q", got)
	}
	if strings.Contains(got, config.SelfTenantID()) {
		t.Errorf("body must NOT echo self tenant UUID; got %q", got)
	}
}

// TestTenantGate_MatchAndNoHeaderPass: a matching header and an absent
// header both pass the gate (false = continue processing).
func TestTenantGate_MatchAndNoHeaderPass(t *testing.T) {
	for name, header := range map[string]string{"matching": config.SelfTenantID(), "absent": ""} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/anything", nil)
			if header != "" {
				req.Header.Set("X-Moses-Tenant-ID", header)
			}
			rec := httptest.NewRecorder()
			if enforceTenantMatch(rec, req) {
				t.Fatalf("expected gate to pass, got 403 body=%q", rec.Body.String())
			}
		})
	}
}

// TestTenantGate_StrictCheckDisabled covers the second half of #2:
// MOSES_STRICT_TENANT_CHECK=false → cross-check is skipped even on a
// mismatching header.
func TestTenantGate_StrictCheckDisabled(t *testing.T) {
	t.Setenv("MOSES_STRICT_TENANT_CHECK", "false")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/anything", nil)
	req.Header.Set("X-Moses-Tenant-ID", "some-mismatched-tenant")
	rec := httptest.NewRecorder()

	if enforceTenantMatch(rec, req) {
		t.Errorf("expected cross-check to be SKIPPED with MOSES_STRICT_TENANT_CHECK=false, got 403")
	}
}

// TestStrictTenantCheckEnabled covers the parsing of the env knob.
func TestStrictTenantCheckEnabled(t *testing.T) {
	cases := []struct {
		val  string
		want bool
	}{
		{"", true},
		{"true", true},
		{"1", true},
		{"yes", true},
		{"false", false},
		{"0", false},
		{"no", false},
		{"off", false},
		{"FALSE", false},
		{"OFF", false},
	}
	for _, c := range cases {
		t.Run(c.val, func(t *testing.T) {
			t.Setenv("MOSES_STRICT_TENANT_CHECK", c.val)
			if got := strictTenantCheckEnabled(); got != c.want {
				t.Errorf("MOSES_STRICT_TENANT_CHECK=%q: got %v want %v", c.val, got, c.want)
			}
		})
	}
}
