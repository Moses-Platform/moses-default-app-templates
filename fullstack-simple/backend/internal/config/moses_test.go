package config

import (
	"sync"
	"testing"
)

func resetForTest() {
	selfTenantOnce = sync.Once{}
	selfTenant = ""
}

func TestSelfTenantID_EnvHappyPath(t *testing.T) {
	resetForTest()
	t.Setenv("MOSES_TENANT_ID", "real-tenant-uuid-1234")
	t.Setenv("MOSES_DEPLOYED", "")
	if got := SelfTenantID(); got != "real-tenant-uuid-1234" {
		t.Errorf("got %q, want %q", got, "real-tenant-uuid-1234")
	}
}

func TestSelfTenantID_LocalDevFallback(t *testing.T) {
	resetForTest()
	t.Setenv("MOSES_TENANT_ID", "")
	t.Setenv("MOSES_DEPLOYED", "")
	if got := SelfTenantID(); got != "local-dev" {
		t.Errorf("got %q, want local-dev", got)
	}
}

func TestSelfTenantID_ProductionFailFast_Precondition(t *testing.T) {
	resetForTest()
	t.Setenv("MOSES_TENANT_ID", "")
	t.Setenv("MOSES_DEPLOYED", "1")
	if got := SelfTenantID(); got != "local-dev" {
		t.Fatalf("precondition: got %q want local-dev", got)
	}
}

func TestSelfTenantID_LazyInit(t *testing.T) {
	resetForTest()
	t.Setenv("MOSES_TENANT_ID", "lazy-tenant")
	t.Setenv("MOSES_DEPLOYED", "")
	if got := SelfTenantID(); got != "lazy-tenant" {
		t.Errorf("got %q, want lazy-tenant", got)
	}
}

func TestSelfTenantID_TrimsWhitespace(t *testing.T) {
	resetForTest()
	t.Setenv("MOSES_TENANT_ID", "  pad-me  ")
	t.Setenv("MOSES_DEPLOYED", "")
	if got := SelfTenantID(); got != "pad-me" {
		t.Errorf("got %q, want pad-me", got)
	}
}

func TestSelfTenantID_CachesFirstValue(t *testing.T) {
	resetForTest()
	t.Setenv("MOSES_TENANT_ID", "first")
	first := SelfTenantID()
	t.Setenv("MOSES_TENANT_ID", "second")
	if SelfTenantID() != first {
		t.Errorf("sync.Once contract broken")
	}
}
