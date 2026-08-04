package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func corsProbe(t *testing.T, method, origin string) *httptest.ResponseRecorder {
	t.Helper()
	h := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(method, "/api/v1/anything", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// Deny-by-default: with no allowlist configured, NO CORS headers are
// emitted — not even for a cross-site Origin. This is the regression test
// for the pre-hardening bug where production reflected ANY Origin with
// Allow-Credentials: true.
func TestCORS_NoAllowlist_EmitsNoHeaders(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	rec := corsProbe(t, http.MethodGet, "https://attacker.example.com")
	for _, hdr := range []string{
		"Access-Control-Allow-Origin",
		"Access-Control-Allow-Credentials",
		"Access-Control-Allow-Methods",
		"Access-Control-Allow-Headers",
	} {
		if got := rec.Header().Get(hdr); got != "" {
			t.Errorf("%s = %q, want unset with no allowlist", hdr, got)
		}
	}
	if rec.Code != http.StatusOK {
		t.Errorf("request must pass through, got %d", rec.Code)
	}
}

// A whitespace/comma-only allowlist is treated as unset (CORS disabled).
func TestCORS_WhitespaceOnlyAllowlist_EmitsNoHeaders(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", " , ,")
	rec := corsProbe(t, http.MethodGet, "https://attacker.example.com")
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want unset with whitespace-only allowlist", got)
	}
}

// A non-allowlisted Origin gets no CORS headers even when an allowlist
// exists (no reflection).
func TestCORS_NonAllowlistedOrigin_NotReflected(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://portal.example.com")
	rec := corsProbe(t, http.MethodGet, "https://attacker.example.com")
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want unset for non-allowlisted origin", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("Allow-Credentials = %q, want unset for non-allowlisted origin", got)
	}
}

// An exact allowlist match echoes the origin with credentials + Vary.
func TestCORS_AllowlistedOrigin_EchoedWithCredentials(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://portal.example.com, https://admin.example.com")
	rec := corsProbe(t, http.MethodGet, "https://admin.example.com")
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://admin.example.com" {
		t.Errorf("Allow-Origin = %q, want the allowlisted origin", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Allow-Credentials = %q, want true for an exact allowlisted origin", got)
	}
	if got := rec.Header().Values("Vary"); len(got) == 0 {
		t.Error("expected Vary: Origin so caches key on the Origin header")
	}
}

// A literal "*" entry emits the wildcard but NEVER credentials.
func TestCORS_WildcardEntry_NeverCombinesWithCredentials(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "*")
	rec := corsProbe(t, http.MethodGet, "https://anything.example.com")
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Allow-Origin = %q, want *", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("Allow-Credentials = %q — wildcard + credentials must never be emitted", got)
	}
}

// Preflight for an allowed origin short-circuits with 204.
func TestCORS_Preflight_AllowedOrigin204(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://portal.example.com")
	rec := corsProbe(t, http.MethodOptions, "https://portal.example.com")
	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("expected Allow-Methods on an allowed preflight")
	}
}

// Preflight from a non-allowed origin is NOT answered by the middleware —
// it passes through with no CORS headers.
func TestCORS_Preflight_DeniedOriginPassesThrough(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://portal.example.com")
	rec := corsProbe(t, http.MethodOptions, "https://attacker.example.com")
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want unset on denied preflight", got)
	}
	if rec.Code == http.StatusNoContent {
		t.Error("denied preflight must not be acknowledged with 204")
	}
}

// Same-origin requests (no Origin header) are untouched.
func TestCORS_NoOriginHeader_PassThrough(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://portal.example.com")
	rec := corsProbe(t, http.MethodGet, "")
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want unset without an Origin header", got)
	}
}
