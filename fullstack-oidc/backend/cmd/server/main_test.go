package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// buildMux must mount /health at the canonical root regardless of
// base path (the K8s probe bypasses the ingress).
func TestBuildMux_HealthCanonical(t *testing.T) {
	mux := buildMux("/apps/t/s", false, nil)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/health", nil))
	if rec.Code != 200 {
		t.Errorf("canonical /health status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "fullstack-oidc") {
		t.Errorf("/health body = %q", rec.Body.String())
	}
}

// /health is also reachable under the base-path alias.
func TestBuildMux_HealthBasePathAlias(t *testing.T) {
	mux := buildMux("/apps/t/s", false, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/apps/t/s/health", nil))
	if rec.Code != 200 {
		t.Errorf("base-path /health status = %d, want 200", rec.Code)
	}
}

// The OpenAPI spec is served at the canonical root for platform
// discovery.
func TestBuildMux_OpenAPICanonical(t *testing.T) {
	mux := buildMux("", false, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/openapi.json", nil))
	if rec.Code != 200 {
		t.Errorf("/api/openapi.json status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "openapi") {
		t.Errorf("openapi.json body missing 'openapi' key")
	}
}

// The session-introspection route /api/v1/me is registered under
// MOSES_BASE_PATH. Reaching it with no Identity in context (no OIDCAuth
// in front of the bare mux) yields the zero identity, not an error.
func TestBuildMux_MeRegistered(t *testing.T) {
	mux := buildMux("/apps/t/s", true, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/apps/t/s/api/v1/me", nil))
	if rec.Code != 200 {
		t.Errorf("me status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"authenticated":false`) {
		t.Errorf("me body = %q, want the zero identity (authenticated:false)", rec.Body.String())
	}
}

// public-info is served under MOSES_BASE_PATH and reports the OIDC flag.
func TestBuildMux_PublicInfo(t *testing.T) {
	mux := buildMux("/apps/t/s", true, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/apps/t/s/api/v1/public-info", nil))
	if rec.Code != 200 {
		t.Errorf("public-info status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"oidc_enabled":true`) {
		t.Errorf("public-info body = %q, want oidc_enabled true", rec.Body.String())
	}
}

// The role-gated admin-area route is registered under MOSES_BASE_PATH.
// Reaching it with no Identity in context (as if OIDC were disabled and
// the route un-gated) must 403 — AdminArea independently enforces the
// app role, it does not trust the middleware alone.
func TestBuildMux_AdminAreaRegistered(t *testing.T) {
	mux := buildMux("/apps/t/s", true, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/apps/t/s/api/v1/admin-area", nil))
	if rec.Code != 403 {
		t.Errorf("admin-area (no roles) status = %d, want 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "oidc-admin") {
		t.Errorf("admin-area 403 body should name the required role, got %q", rec.Body.String())
	}
}
