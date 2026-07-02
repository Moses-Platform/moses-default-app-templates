// Copyright (c) 2025-2026 Siemer Industries. All rights reserved.
// Licensed under the Business Source License 1.1. See LICENSE file for details.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moses-platform/fullstack-chat/internal/handler"
	"github.com/moses-platform/fullstack-chat/internal/mosesproxy"
)

// newTestMux builds the production router (buildMux from main.go). The
// webhook handler gets a nil sink and the demo routes a nil *sql.DB — the
// route tests below use a DELETE so the dispatch returns 405 (route
// registered) before any handler dereferences the DB.
func newTestMux(basePath string) *http.ServeMux {
	return buildMux(
		basePath,
		handler.NewChatWebhookHandler(nil),
		mosesproxy.Config{InternalAPIBase: "http://unused.local", AppSlug: "fullstack-chat"},
		nil,
	)
}

// routeRegistered reports whether mux has a route for path: a registered
// route returns anything other than 404 (here a 405 from the method
// dispatch), an unregistered one returns 404.
func routeRegistered(mux *http.ServeMux, method, path string) bool {
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec.Code != http.StatusNotFound
}

func TestHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.Health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /health: expected 200, got %d", w.Code)
	}
}

// CHAT-8qiu0: the API is registered ONCE, under MOSES_BASE_PATH. When no
// base path is set that collapses to the canonical /api/v1/... path.
func TestRoutes_NoBasePath(t *testing.T) {
	mux := newTestMux("")
	if !routeRegistered(mux, http.MethodDelete, "/api/v1/entries") {
		t.Error("/api/v1/entries not registered when no base path is set")
	}
}

// CHAT-8qiu0: with a base path set the API MUST be reachable at the
// base-path-prefixed path — this is where the workspace-tool proxy and the
// browser both call it.
func TestRoutes_SubPathMount(t *testing.T) {
	mux := newTestMux("/apps/t/x")
	if !routeRegistered(mux, http.MethodDelete, "/apps/t/x/api/v1/entries") {
		t.Error("/apps/t/x/api/v1/entries not registered under the base path")
	}
	// The mosesproxy InvokePath must also live under the base path.
	if !routeRegistered(mux, http.MethodGet, "/apps/t/x"+mosesproxy.InvokePath) {
		t.Errorf("%s not registered under the base path", "/apps/t/x"+mosesproxy.InvokePath)
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

// CHAT-yfmwv: /api/openapi.json ALSO answers at the base-path-prefixed path —
// the frontend nginx forwards the prefix unchanged.
func TestRoutes_OpenAPIAtBasePath(t *testing.T) {
	mux := newTestMux("/apps/t/x")
	req := httptest.NewRequest("GET", "/apps/t/x/api/openapi.json", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /apps/t/x/api/openapi.json: expected 200, got %d", rec.Code)
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
