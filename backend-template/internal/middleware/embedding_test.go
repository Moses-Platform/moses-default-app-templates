// CHAT-pbup Bug 3: regression tests for the embedding-headers middleware
// added to backend-template. The api appType defaults to "denied" — without
// the middleware, the backend-template emitted no CSP / X-Frame-Options
// headers despite declaring framing in moses-app.config.json.
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithEmbeddingHeaders_AlwaysEmitsCSP(t *testing.T) {
	handler := WithEmbeddingHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/items", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatalf("expected Content-Security-Policy header, got empty")
	}
	// The api default is "denied" → frame-ancestors 'none'.
	want := "frame-ancestors 'none'"
	if csp != want {
		t.Errorf("CSP header = %q, want %q", csp, want)
	}
}

func TestWithEmbeddingHeaders_DeniedSetsXFrameOptions(t *testing.T) {
	handler := WithEmbeddingHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	xfo := rec.Header().Get("X-Frame-Options")
	if xfo != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY for the api default", xfo)
	}
}
