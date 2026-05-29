package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/moses-platform/backend-template/internal/config"
	"github.com/moses-platform/backend-template/internal/handler"
	"github.com/moses-platform/backend-template/internal/middleware"
	"github.com/moses-platform/backend-template/internal/model"
)

func main() {
	// CHAT-pxeo.12: hard fail-fast when MOSES_TENANT_ID is unset on a
	// deployed pod. backend-template has only an in-memory store, so
	// there is no startup data migration to run — restarts wipe state.
	config.Validate()

	// CHAT-0lu74: external runtime secrets (e.g. EXAMPLE_API_KEY) belong in
	// `moses-app.config.json` → `secrets.external[]`. See
	// `skills/secrets-tutorial.md` for the read+validate pattern and
	// `moses-app.config.with-secrets.example.json` for a working config diff.

	// Read configuration from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// CHAT-pbup / CHAT-8qiu0: read MOSES_BASE_PATH (canonical) with BASE_URL as
	// the deprecated alias. API routes are registered ONCE under this prefix
	// (see buildMux) so /apps/<tenant>/<slug>/api/v1/items reaches the handlers
	// for both the browser and the workspace-tool proxy. basePath is normalized
	// to either "" (root) or "/foo/bar" (no trailing slash).
	basePath := strings.TrimSuffix(os.Getenv("MOSES_BASE_PATH"), "/")
	if basePath == "" {
		alias := strings.TrimSuffix(os.Getenv("BASE_URL"), "/")
		// Only honor BASE_URL when it looks like a path prefix (e.g.
		// "/apps/x/y") — earlier versions of this template defaulted
		// BASE_URL to a full URL like "http://localhost:8080" for
		// log output, which must NOT become a route prefix.
		if alias != "" && strings.HasPrefix(alias, "/") {
			basePath = alias
			log.Printf("WARN: BASE_URL is deprecated; please set MOSES_BASE_PATH instead. See DEPRECATIONS.md")
		}
	}
	// observability-only: full URL for log lines below.
	displayURL := os.Getenv("MOSES_BASE_PATH")
	if displayURL == "" {
		displayURL = fmt.Sprintf("http://localhost:%s", port)
	}

	// Initialize in-memory item store
	itemStore := model.NewItemStore()

	// Create router with all routes registered (see buildMux).
	mux := buildMux(basePath, itemStore)

	// Apply middleware chain: logging -> embedding -> moses_headers.
	// withEmbeddingHeaders emits Content-Security-Policy: frame-ancestors
	// per MOSES_EMBEDDING_FRAMING (and X-Frame-Options for "denied"). API
	// templates default to "denied" since they're never iframed. CHAT-pbup
	// Bug 3 — previously this template emitted neither header.
	var h http.Handler = mux
	// RejectCrossSiteCSRF blocks cross-site state-changing requests (the app
	// must not rely on the platform edge alone — the access_token cookie is
	// SameSite=None). Kept inside Logging so blocked requests are still logged.
	h = middleware.RejectCrossSiteCSRF(h)
	h = middleware.MosesHeaders(h)
	h = middleware.WithEmbeddingHeaders(h)
	h = middleware.Logging(h)

	// Create server
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      h,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting Backend Template server on port %s", port)
		log.Printf("Base URL: %s (basePath=%q)", displayURL, basePath)
		log.Printf("Health check: %s/health", displayURL)
		log.Printf("OpenAPI spec: %s/api/openapi.json", displayURL)
		log.Printf("API endpoints: %s%s/api/v1/items", displayURL, basePath)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}

// buildMux wires every route this server serves. Single source of truth
// shared by main() and the cmd/server tests.
//
// MOSES ROUTING. Moses deploys apps at a sub-path; the pod receives that
// path in MOSES_BASE_PATH and the ingress does NOT strip it. The platform's
// workspace-tool proxy calls the app's API *under* MOSES_BASE_PATH, so the
// API is registered ONCE there — basePath is "" for standalone/dev deploys
// and "/apps/<tenant>/<slug>" for Moses sub-path deploys.
//
//   - API routes: single registration under basePath (its browser-facing home).
//   - /health: registered at BOTH /health and {basePath}/health — the kubelet
//     probe hits the canonical /health (it bypasses the ingress), while a
//     browser/proxy reaching the pod via the sub-path still gets a 200.
//   - /api/openapi.json: registered at BOTH the canonical path and
//     {basePath}/api/openapi.json — discovery uses the canonical path; the
//     base-path alias keeps it reachable when the app is mounted under
//     MOSES_BASE_PATH (CHAT-yfmwv).
func buildMux(basePath string, itemStore *model.ItemStore) *http.ServeMux {
	mux := http.NewServeMux()

	// Health check — canonical for the K8s probe, plus the base-path alias
	// so a sub-path caller does not 404.
	mux.HandleFunc("GET /health", handler.HealthCheck)
	if basePath != "" {
		mux.HandleFunc("GET "+basePath+"/health", handler.HealthCheck)
	}

	// OpenAPI spec endpoints — canonical for the WorkspaceToolProxy discovery
	// hook, plus the base-path alias so they stay reachable when the app is
	// mounted under MOSES_BASE_PATH (CHAT-yfmwv).
	mux.HandleFunc("GET /api/openapi.json", handler.ServeOpenAPI)
	mux.HandleFunc("GET /api/spec", handler.ServeOpenAPI)
	if basePath != "" {
		mux.HandleFunc("GET "+basePath+"/api/openapi.json", handler.ServeOpenAPI)
		mux.HandleFunc("GET "+basePath+"/api/spec", handler.ServeOpenAPI)
	}

	// API endpoints — registered ONCE under MOSES_BASE_PATH.
	itemHandler := handler.NewItemHandler(itemStore)
	platformHandler := handler.NewPlatformHandler()
	mux.HandleFunc("GET "+basePath+"/api/v1/items", itemHandler.ListItems)
	mux.HandleFunc("GET "+basePath+"/api/v1/items/{id}", itemHandler.GetItem)
	mux.HandleFunc("POST "+basePath+"/api/v1/items", itemHandler.CreateItem)
	mux.HandleFunc("GET "+basePath+"/api/v1/platform/info", platformHandler.PlatformInfo)

	return mux
}
