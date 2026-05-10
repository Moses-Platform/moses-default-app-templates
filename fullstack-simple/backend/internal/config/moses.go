// Package config exposes the Moses-deploy-pinned tenant identity for the
// backend. Self-identification (storage/lookup keys) MUST come from the
// MOSES_TENANT_ID env var the platform injects at deploy time, NOT from
// the X-Moses-Tenant-ID request header. The header is caller-context
// (workspace-tool callers, audit) and is absent on browser-driven
// requests through the platform's user-facing app proxy.
//
// See CHAT-pxeo.12 / CHAT-li0b for the cluster-verified failure that
// motivates this helper.
package config

import (
	"log"
	"os"
	"strings"
	"sync"
)

var (
	selfTenant     string
	selfTenantOnce sync.Once
)

// resolveSelfTenant reads MOSES_TENANT_ID from env. Lazy + sync.Once so
// `func init()` doesn't fire fail-fast at package import time, which
// would break every test that imports this package before t.Setenv runs.
func resolveSelfTenant() {
	selfTenantOnce.Do(func() {
		v := strings.TrimSpace(os.Getenv("MOSES_TENANT_ID"))
		if v == "" {
			v = "local-dev"
			if os.Getenv("MOSES_DEPLOYED") == "1" {
				// Production deploy: env MUST be set. Logged but
				// not log.Fatalf'd here — Validate() handles fail-fast
				// when called from main() so test imports stay clean.
				log.Printf("ERROR: MOSES_TENANT_ID env unset on a deployed pod (MOSES_DEPLOYED=1); using sentinel 'local-dev' until Validate() forces shutdown")
			} else {
				log.Printf("warning: MOSES_TENANT_ID not set; using local-dev sentinel for non-deployed run")
			}
		}
		selfTenant = v
	})
}

// SelfTenantID returns the deploy-pinned tenant id from env. First call
// resolves and caches; subsequent calls return the cached value. Safe
// for use from request handlers and tests (override via t.Setenv before
// the first call).
func SelfTenantID() string {
	resolveSelfTenant()
	return selfTenant
}

// Validate is called explicitly from main() AFTER env is set in
// production but BEFORE the HTTP listener starts. It panics on a
// deployed pod (MOSES_DEPLOYED=1) when MOSES_TENANT_ID is unset, so the
// pod fails-fast in Kubernetes (CrashLoopBackOff signals operator).
// Tests that don't import main never trigger this.
func Validate() {
	resolveSelfTenant()
	if selfTenant == "local-dev" && os.Getenv("MOSES_DEPLOYED") == "1" {
		log.Fatalf("MOSES_TENANT_ID env is required when MOSES_DEPLOYED=1 (deployed app pod)")
	}
}
