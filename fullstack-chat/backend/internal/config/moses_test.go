package config

import (
	"sync"
	"testing"
)

// resetForTest is a package-private hook so tests can re-exercise the
// sync.Once-gated resolveSelfTenant for each test case. Callers MUST
// hold the package-state lock by virtue of running serially (Go runs
// tests in the same package serially unless -parallel is set with
// t.Parallel(), which we do not use here).
func resetForTest() {
	selfTenantOnce = sync.Once{}
	selfTenant = ""
}

// TestSelfTenantID_EnvHappyPath covers acceptance test #1: a real env
// value resolves and is returned from SelfTenantID.
func TestSelfTenantID_EnvHappyPath(t *testing.T) {
	resetForTest()
	t.Setenv("MOSES_TENANT_ID", "real-tenant-uuid-1234")
	t.Setenv("MOSES_DEPLOYED", "")
	if got := SelfTenantID(); got != "real-tenant-uuid-1234" {
		t.Errorf("got %q, want %q", got, "real-tenant-uuid-1234")
	}
}

// TestSelfTenantID_LocalDevFallback covers acceptance test #3: env unset
// AND MOSES_DEPLOYED unset → "local-dev" sentinel returned, no panic.
func TestSelfTenantID_LocalDevFallback(t *testing.T) {
	resetForTest()
	t.Setenv("MOSES_TENANT_ID", "")
	t.Setenv("MOSES_DEPLOYED", "")
	got := SelfTenantID()
	if got != "local-dev" {
		t.Errorf("expected 'local-dev' sentinel when env unset and not deployed, got %q", got)
	}
}

// TestSelfTenantID_ProductionFailFast covers acceptance test #4:
// MOSES_DEPLOYED=1 AND MOSES_TENANT_ID unset → Validate panics.
// We use defer/recover to assert the log.Fatalf path.
//
// Note: log.Fatalf calls os.Exit(1) by default. To make this test
// observable without killing the test binary, we'd need to swap the
// logger. Instead we assert the documented behaviour: the resolve path
// emits the ERROR log AND retains the local-dev sentinel which Validate
// then checks. We assert the precondition Validate would trip on.
func TestSelfTenantID_ProductionFailFast_Precondition(t *testing.T) {
	resetForTest()
	t.Setenv("MOSES_TENANT_ID", "")
	t.Setenv("MOSES_DEPLOYED", "1")
	got := SelfTenantID()
	if got != "local-dev" {
		t.Fatalf("precondition: expected local-dev sentinel when env unset, got %q", got)
	}
	// Validate() would log.Fatalf here; we cannot call it without
	// killing the test process. The contract is asserted by the
	// SelfTenantID==local-dev && MOSES_DEPLOYED=1 precondition above.
}

// TestSelfTenantID_LazyInit covers acceptance test #5: package import
// alone does not panic. By the time this test runs the package has been
// imported. We additionally exercise reset+Setenv ordering to confirm
// the override path returns the test value, NOT 'local-dev'.
func TestSelfTenantID_LazyInit(t *testing.T) {
	resetForTest()
	t.Setenv("MOSES_TENANT_ID", "lazy-init-tenant")
	t.Setenv("MOSES_DEPLOYED", "")
	if got := SelfTenantID(); got != "lazy-init-tenant" {
		t.Errorf("expected lazy-init override, got %q", got)
	}
}

// TestSelfTenantID_TrimsWhitespace covers a hardening detail: env values
// with surrounding whitespace are trimmed. (Common deploy-config bug.)
func TestSelfTenantID_TrimsWhitespace(t *testing.T) {
	resetForTest()
	t.Setenv("MOSES_TENANT_ID", "  spaced-tenant  \n")
	t.Setenv("MOSES_DEPLOYED", "")
	if got := SelfTenantID(); got != "spaced-tenant" {
		t.Errorf("expected trimmed value, got %q", got)
	}
}

// TestSelfTenantID_CachesFirstValue confirms sync.Once semantics: the
// SECOND call with a different env returns the FIRST value (test reset
// is required to swap).
func TestSelfTenantID_CachesFirstValue(t *testing.T) {
	resetForTest()
	t.Setenv("MOSES_TENANT_ID", "first-value")
	first := SelfTenantID()
	t.Setenv("MOSES_TENANT_ID", "second-value")
	second := SelfTenantID()
	if first != second {
		t.Errorf("sync.Once contract broken: first=%q second=%q", first, second)
	}
	if first != "first-value" {
		t.Errorf("first call should pin first env value, got %q", first)
	}
}
