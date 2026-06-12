package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/moses-platform/backend-template/internal/handler"
	"github.com/moses-platform/backend-template/internal/model"
)

func TestHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.HealthCheck(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /health: expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "healthy") {
		t.Errorf("GET /health: expected body with 'healthy', got %q", w.Body.String())
	}
}

// newTestMux builds the production router (buildMux from main.go) for the
// given basePath.
func newTestMux(basePath string) *http.ServeMux {
	return buildMux(basePath, model.NewItemStore())
}

// CHAT-8qiu0: the API is registered ONCE, under MOSES_BASE_PATH. When no
// base path is set that collapses to the canonical /api/v1/... path.
func TestRoutes_NoBasePath(t *testing.T) {
	mux := newTestMux("")
	req := httptest.NewRequest("GET", "/api/v1/items", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/v1/items (no base path): expected 200, got %d", rec.Code)
	}
}

// CHAT-8qiu0: with a base path set the API MUST be reachable at the
// base-path-prefixed path — this is where the workspace-tool proxy and the
// browser both call it.
func TestRoutes_SubPathMount(t *testing.T) {
	mux := newTestMux("/apps/t/x")

	req := httptest.NewRequest("GET", "/apps/t/x/api/v1/items", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /apps/t/x/api/v1/items: expected 200, got %d", rec.Code)
	}
}

// CHAT-8qiu0: /health answers at BOTH the canonical path (kubelet probe,
// bypasses ingress) AND the base-path-prefixed path (sub-path callers).
func TestRoutes_SubPath_HealthAtBothPaths(t *testing.T) {
	mux := newTestMux("/apps/t/x")

	for _, path := range []string{"/health", "/apps/t/x/health"} {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: expected 200, got %d", path, rec.Code)
		}
	}
}

// CHAT-8qiu0: /api/openapi.json stays canonical (platform discovery hook).
func TestRoutes_OpenAPICanonical(t *testing.T) {
	mux := newTestMux("/apps/t/x")
	req := httptest.NewRequest("GET", "/api/openapi.json", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/openapi.json: expected 200, got %d", rec.Code)
	}
}

// CHAT-yfmwv: /api/openapi.json ALSO answers at the base-path-prefixed path.
func TestRoutes_OpenAPIAtBasePath(t *testing.T) {
	mux := newTestMux("/apps/t/x")
	req := httptest.NewRequest("GET", "/apps/t/x/api/openapi.json", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /apps/t/x/api/openapi.json: expected 200, got %d", rec.Code)
	}
}

// CHAT-8qiu0 acceptance check: the OpenAPI spec's servers[] MUST stay
// base-path-free — the platform's openapi_parser folds servers[0].url into
// endpoint.Path and the proxy prepends MOSES_BASE_PATH; a base-path-aware
// servers[] would double-prefix.
func TestOpenAPISpec_ServersBasePathFree(t *testing.T) {
	assertOpenAPIServersBasePathFree(t, filepath.Join("..", "..", "api", "openapi.json"))
}

// WS-F F1: the served spec is normalized to the canonical pattern —
// servers exactly [{url:"/api/v1"}] with all paths RELATIVE to that base.
// The platform's openapi_parser folds servers[0].url + path into the
// registered endpoint path, so given servers[0].url == "/api/v1" a paths
// key that itself starts with /api/ would double-prefix (the showcase bug
// class), and /health must not be listed (it would register a phantom
// tool at /api/v1/health).
func TestOpenAPISpec_CanonicalServersAndRelativePaths(t *testing.T) {
	spec := loadOpenAPISpec(t, openAPISpecPath())
	if len(spec.Servers) != 1 || spec.Servers[0].URL != "/api/v1" {
		t.Fatalf("servers must be exactly [{url:\"/api/v1\"}], got %+v", spec.Servers)
	}
	for p := range spec.Paths {
		if !strings.HasPrefix(p, "/") {
			t.Errorf("paths key %q must start with /", p)
		}
		if strings.HasPrefix(p, "/api/") {
			t.Errorf("paths key %q must be relative to servers[0].url (/api/v1) — an /api/-rooted key double-prefixes", p)
		}
		if p == "/health" {
			t.Errorf("/health must not be listed in the served spec — it would register a phantom workspace tool")
		}
	}
}

// WS-F F1 spec↔mux consistency lock: every spec path, folded with
// servers[0].url ("/api/v1") and mounted under MOSES_BASE_PATH, must
// resolve to a registered mux route. This is exactly the address the
// platform's workspace-tool proxy requests (openapi_parser.go folds
// servers[0].url + path; apppath prepends MOSES_BASE_PATH). We resolve
// via mux.Handler (pattern non-empty == route exists) instead of
// ServeHTTP status codes because a registered {id} route legitimately
// answers 404 for a missing resource.
func TestOpenAPISpec_MuxConsistency(t *testing.T) {
	spec := loadOpenAPISpec(t, openAPISpecPath())
	mux := newTestMux("/apps/t/x")
	param := regexp.MustCompile(`\{[^}]+\}`)
	for p := range spec.Paths {
		urlPath := "/apps/t/x/api/v1" + param.ReplaceAllString(p, "1")
		req := httptest.NewRequest("GET", urlPath, nil)
		if _, pattern := mux.Handler(req); pattern == "" {
			t.Errorf("spec path %q → GET %s does not resolve to any registered route (spec↔mux drift)", p, urlPath)
		}
	}
}

func openAPISpecPath() string {
	return filepath.Join("..", "..", "api", "openapi.json")
}

type openAPISpec struct {
	Servers []struct {
		URL string `json:"url"`
	} `json:"servers"`
	Paths map[string]json.RawMessage `json:"paths"`
}

func loadOpenAPISpec(t *testing.T, specPath string) openAPISpec {
	t.Helper()
	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read OpenAPI spec %s: %v", specPath, err)
	}
	var spec openAPISpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parse OpenAPI spec %s: %v", specPath, err)
	}
	return spec
}

// assertOpenAPIServersBasePathFree fails if any servers[].url in the given
// OpenAPI spec contains an "/apps/" sub-path prefix.
func assertOpenAPIServersBasePathFree(t *testing.T, specPath string) {
	t.Helper()
	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read OpenAPI spec %s: %v", specPath, err)
	}
	var spec struct {
		Servers []struct {
			URL string `json:"url"`
		} `json:"servers"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parse OpenAPI spec %s: %v", specPath, err)
	}
	for _, s := range spec.Servers {
		if strings.Contains(s.URL, "/apps/") {
			t.Errorf("servers[].url %q must not contain a /apps/ base-path prefix", s.URL)
		}
	}
}
