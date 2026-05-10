package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /health: expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "healthy") {
		t.Errorf("GET /health: expected body with 'healthy', got %q", w.Body.String())
	}
}

// TestRenderIndex_SubstitutesMosesConfig verifies the BLF-J server-side
// substitution: chart/deployment/api-base values are interpolated into the
// <meta name="moses-config"> tag the browser-logger snippet reads.
func TestRenderIndex_SubstitutesMosesConfig(t *testing.T) {
	loadIndexTemplate()
	if indexTemplate == nil {
		t.Fatal("indexTemplate is nil after loadIndexTemplate; embedded index.html missing or unparseable")
	}

	w := httptest.NewRecorder()
	if !renderIndex(w, indexContext{
		ChartID:      "chart-123",
		DeploymentID: "dep-abc",
		APIBase:      "http://moses.local",
	}) {
		t.Fatal("renderIndex returned false despite indexTemplate being set")
	}

	body := w.Body.String()
	for _, want := range []string{
		`name="moses-config"`,
		`data-chart-id="chart-123"`,
		`data-deployment-id="dep-abc"`,
		`data-api-base="http://moses.local"`,
		`moses-browser-logger.js`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered index missing %q\n--- body ---\n%s", want, body)
		}
	}
}

// TestRenderIndex_EmptyContextStillRendersTag verifies that when the env
// vars are absent the meta tag is still emitted (with empty data-* values),
// because the snippet's readConfig() short-circuits on missing IDs and that
// branch is the one we rely on for "built outside Moses" no-op behaviour.
func TestRenderIndex_EmptyContextStillRendersTag(t *testing.T) {
	loadIndexTemplate()
	if indexTemplate == nil {
		t.Fatal("indexTemplate is nil after loadIndexTemplate")
	}

	w := httptest.NewRecorder()
	if !renderIndex(w, indexContext{}) {
		t.Fatal("renderIndex returned false despite indexTemplate being set")
	}

	body := w.Body.String()
	if !strings.Contains(body, `name="moses-config"`) {
		t.Errorf("expected <meta name=\"moses-config\"> tag even with empty context\n--- body ---\n%s", body)
	}
	if !strings.Contains(body, `data-chart-id=""`) {
		t.Errorf("expected empty data-chart-id when env var missing\n--- body ---\n%s", body)
	}
}

// CHAT-pxeo.12: TestStatus_TenantMismatch403 verifies the 403 cross-check
// fires on /api/v1/status when the caller-supplied X-Moses-Tenant-ID
// header disagrees with the deploy-pinned MOSES_TENANT_ID env. The
// response body must NOT contain either UUID.
func TestStatus_TenantMismatch403(t *testing.T) {
	t.Setenv("MOSES_TENANT_ID", "self-tenant-uuid-deploy-pinned")
	t.Setenv("MOSES_DEPLOYED", "")
	t.Setenv("MOSES_STRICT_TENANT_CHECK", "true")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.Header.Set("X-Moses-Tenant-ID", "caller-tenant-uuid-different")
	rec := httptest.NewRecorder()
	handleStatus(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%q", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	if !strings.Contains(got, `"error":"tenant_mismatch"`) {
		t.Errorf("expected tenant_mismatch error, got %q", got)
	}
	if !strings.Contains(got, `"code":"E_TENANT_MISMATCH"`) {
		t.Errorf("expected E_TENANT_MISMATCH code, got %q", got)
	}
	if strings.Contains(got, "caller-tenant-uuid-different") || strings.Contains(got, "self-tenant-uuid-deploy-pinned") {
		t.Errorf("body must NOT echo any tenant UUID; got %q", got)
	}
}

// CHAT-pxeo.12: with the strict check disabled the cross-check is skipped.
func TestStatus_StrictTenantCheckDisabled(t *testing.T) {
	t.Setenv("MOSES_STRICT_TENANT_CHECK", "false")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.Header.Set("X-Moses-Tenant-ID", "caller-different")
	rec := httptest.NewRecorder()
	handleStatus(rec, req)

	if rec.Code == http.StatusForbidden {
		t.Errorf("cross-check should be skipped, got 403")
	}
}

// CHAT-pbup: TestEmbeddingHeadersMiddleware verifies that withEmbeddingHeaders
// emits Content-Security-Policy: frame-ancestors per the resolved policy
// AND that X-Frame-Options is only emitted for "denied". The package-level
// resolvedEmbeddingPolicy is set once at init from env vars; we override it
// for each subtest so the assertions are deterministic.
func TestEmbeddingHeadersMiddleware(t *testing.T) {
	saved := resolvedEmbeddingPolicy
	t.Cleanup(func() { resolvedEmbeddingPolicy = saved })

	cases := []struct {
		name           string
		policy         embeddingPolicy
		wantCSP        string
		wantXFOpresent bool
	}{
		{
			name:           "public",
			policy:         embeddingPolicy{cspFrameAncestors: "*"},
			wantCSP:        "frame-ancestors *",
			wantXFOpresent: false,
		},
		{
			name:           "moses-only with multi-origin",
			policy:         embeddingPolicy{cspFrameAncestors: "https://moses.example.com tauri://localhost"},
			wantCSP:        "frame-ancestors https://moses.example.com tauri://localhost",
			wantXFOpresent: false,
		},
		{
			name:           "denied",
			policy:         embeddingPolicy{cspFrameAncestors: "'none'", xFrameOptions: "DENY"},
			wantCSP:        "frame-ancestors 'none'",
			wantXFOpresent: true,
		},
		{
			name:           "with reportUri appended",
			policy:         embeddingPolicy{cspFrameAncestors: "*", reportURI: "/csp-report"},
			wantCSP:        "frame-ancestors *; report-uri /csp-report",
			wantXFOpresent: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolvedEmbeddingPolicy = tc.policy
			h := withEmbeddingHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				_, _ = io.WriteString(w, "<html></html>")
			}))
			r := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			gotCSP := w.Header().Get("Content-Security-Policy")
			if gotCSP != tc.wantCSP {
				t.Errorf("CSP got=%q want=%q", gotCSP, tc.wantCSP)
			}
			gotXFO := w.Header().Get("X-Frame-Options")
			if tc.wantXFOpresent && gotXFO == "" {
				t.Errorf("X-Frame-Options expected non-empty for %s", tc.name)
			}
			if !tc.wantXFOpresent && gotXFO != "" {
				t.Errorf("X-Frame-Options expected empty for %s, got %q", tc.name, gotXFO)
			}
		})
	}
}
