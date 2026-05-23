package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/moses-platform/fullstack-simple/internal/config"
	"github.com/moses-platform/fullstack-simple/internal/handler"
)

func main() {
	// CHAT-pxeo.12: hard fail-fast when MOSES_TENANT_ID is unset on a
	// deployed pod. fullstack-simple is in-memory only, so there is no
	// startup data migration to run — restarts wipe state, and tenant
	// scoping starts clean under the deploy-pinned env value.
	config.Validate()

	// CHAT-0lu74: external runtime secrets (e.g. EXAMPLE_API_KEY) belong in
	// `moses-app.config.json` → `secrets.external[]`. See
	// `skills/secrets-tutorial.md` for the read+validate pattern.

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// CHAT-pbup / CHAT-8qiu0: read MOSES_BASE_PATH (canonical) with BASE_URL as
	// the deprecated alias. API routes are registered ONCE under this prefix
	// (see buildMux). basePath is "" (root) or "/foo/bar" (no trailing slash).
	basePath := strings.TrimSuffix(os.Getenv("MOSES_BASE_PATH"), "/")
	if basePath == "" {
		alias := strings.TrimSuffix(os.Getenv("BASE_URL"), "/")
		if alias != "" && strings.HasPrefix(alias, "/") {
			basePath = alias
			log.Printf("WARN: BASE_URL is deprecated; please set MOSES_BASE_PATH instead. See DEPRECATIONS.md")
		}
	}

	mux := buildMux(basePath)

	// CORS middleware
	var h http.Handler = mux
	h = corsMiddleware(h)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      h,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Server listening on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Forced shutdown: %v", err)
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
//     sub-path caller reaching the pod still gets a 200.
//   - /api/openapi.json: registered at BOTH /api/openapi.json and
//     {basePath}/api/openapi.json — the platform's discovery hook uses the
//     canonical path; the base-path alias keeps it reachable through the
//     frontend nginx, which forwards the prefix unchanged (CHAT-yfmwv).
func buildMux(basePath string) *http.ServeMux {
	mux := http.NewServeMux()

	// Health check — canonical for the K8s probe, plus the base-path alias.
	mux.HandleFunc("/health", handler.Health)
	if basePath != "" {
		mux.HandleFunc(basePath+"/health", handler.Health)
	}

	// OpenAPI spec — canonical for the platform's discovery hook, plus the
	// base-path alias so it stays reachable through the frontend nginx, which
	// forwards the prefix unchanged (CHAT-yfmwv).
	mux.HandleFunc("/api/openapi.json", handler.OpenAPI)
	if basePath != "" {
		mux.HandleFunc(basePath+"/api/openapi.json", handler.OpenAPI)
	}

	// API endpoints — registered ONCE under MOSES_BASE_PATH.
	items := handler.NewItemsHandler()
	mux.HandleFunc(basePath+"/api/v1/status", handler.Status)
	// CRUD example — in-memory items scoped by tenant.
	// For database-backed CRUD patterns, see the fullstack-showcase template.
	mux.HandleFunc(basePath+"/api/v1/items", items.Handle)
	mux.HandleFunc(basePath+"/api/v1/items/", items.HandleWithID)

	return mux
}

// SECURITY WARNING: This template uses permissive CORS (Allow-Origin: "*") for
// development convenience. For production deployments, restrict this to your actual
// domain(s). Example:
//
//	w.Header().Set("Access-Control-Allow-Origin", "https://yourdomain.com")
//
// See: https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
