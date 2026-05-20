package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moses-platform/fullstack-showcase/internal/handler"
)

func TestHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.Health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /health: expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "healthy") {
		t.Errorf("GET /health: expected body with 'healthy', got %q", w.Body.String())
	}
}

// newTestMux builds the production router (buildMux from main.go). The notes
// handler is constructed with a nil *sql.DB; the route tests below only
// exercise the DB-free capability / moses-info endpoints, so the DB is never
// dereferenced.
func newTestMux(basePath string) *http.ServeMux {
	return buildMux(basePath, handler.NewNotesHandler(nil), handler.NewUsersHandler())
}

// CHAT-8qiu0: the API is registered ONCE, under MOSES_BASE_PATH. When no
// base path is set that collapses to the canonical /api/v1/... path.
func TestRoutes_NoBasePath(t *testing.T) {
	mux := newTestMux("")
	req := httptest.NewRequest("GET", "/api/v1/capabilities", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/v1/capabilities (no base path): expected 200, got %d", rec.Code)
	}
}

// CHAT-8qiu0: with a base path set the API MUST be reachable at the
// base-path-prefixed path — this is where the workspace-tool proxy and the
// browser both call it.
func TestRoutes_SubPathMount(t *testing.T) {
	mux := newTestMux("/apps/t/x")
	req := httptest.NewRequest("GET", "/apps/t/x/api/v1/capabilities", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /apps/t/x/api/v1/capabilities: expected 200, got %d", rec.Code)
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

// CHAT-8qiu0 acceptance check: the OpenAPI spec's servers[] (if present)
// MUST stay base-path-free — the platform's openapi_parser folds
// servers[0].url into endpoint.Path and the proxy prepends MOSES_BASE_PATH;
// a base-path-aware servers[] would double-prefix.
func TestOpenAPISpec_ServersBasePathFree(t *testing.T) {
	specPath := filepath.Join("..", "..", "api", "openapi.json")
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
