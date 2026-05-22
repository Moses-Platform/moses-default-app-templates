// Command server is the fullstack-oidc template's HTTP backend. It
// demonstrates the vendored oidcauth middleware: the app is an OIDC
// relying party (BFF pattern) that gates its protected routes behind a
// Keycloak login while preserving the X-Moses-* trusted-header path for
// pod-to-pod MCP / workspace-tool calls.
//
// This is the 28.8 skeleton — a working, compiling reference. The full
// demo-page polish lands in ticket 28.14.
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

	"github.com/moses-platform/fullstack-oidc/internal/config"
	"github.com/moses-platform/fullstack-oidc/internal/database"
	"github.com/moses-platform/fullstack-oidc/internal/handler"
	"github.com/moses-platform/fullstack-oidc/internal/middleware"
	"github.com/moses-platform/fullstack-oidc/internal/oidcauth"
)

func main() {
	log.Println("Starting Moses fullstack-oidc server...")

	// Fail-fast when MOSES_TENANT_ID is unset on a deployed pod.
	config.Validate()

	// Connect to the database.
	dbConfig := database.NewConfigFromEnv()
	log.Printf("Connecting to database at %s:%s...", dbConfig.Host, dbConfig.Port)
	db, err := database.Connect(dbConfig)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()
	log.Println("Connected to database")

	if err := database.Migrate(db); err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}
	log.Println("Database migration complete")

	// Build the OIDC middleware from the platform-injected env. When
	// the OIDC vars are absent the middleware degrades to pass-through
	// (public routes + header-trust still work) — Validate() is logged
	// so a misconfigured deploy is visible, not silent.
	oidcCfg := oidcauth.ConfigFromEnv()
	if err := oidcCfg.Validate(); err != nil {
		log.Printf("WARN: %v — running without OIDC enforcement", err)
	} else {
		log.Printf("OIDC enforcement enabled (issuer=%s, client=%s)", oidcCfg.Issuer, oidcCfg.ClientID)
	}
	auth := oidcauth.New(oidcCfg)

	// MOSES_BASE_PATH (canonical) with BASE_URL as the deprecated alias.
	basePath := strings.TrimSuffix(os.Getenv("MOSES_BASE_PATH"), "/")
	if basePath == "" {
		alias := strings.TrimSuffix(os.Getenv("BASE_URL"), "/")
		if alias != "" && strings.HasPrefix(alias, "/") {
			basePath = alias
			log.Printf("WARN: BASE_URL is deprecated; set MOSES_BASE_PATH instead")
		}
	}

	entriesHandler := handler.NewEntriesHandler(db)
	mux := buildMux(basePath, oidcCfg.Enabled(), entriesHandler)

	// Middleware order (outermost first):
	//   Logging -> CORS -> MosesHeaders -> OIDCAuth -> mux
	// OIDCAuth is INSIDE MosesHeaders so the header-trust decision can
	// see the X-Moses-* headers; it owns /auth/* and gates protected
	// routes before the mux runs.
	var h http.Handler = mux
	h = auth.Handler(h)
	h = middleware.MosesHeaders(h)
	h = middleware.CORS(h)
	h = middleware.Logging(h)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
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
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exited")
}

// buildMux wires every route. The OIDCAuth middleware sits in front of
// this mux and serves the /auth/* handshake routes itself — they are
// NOT registered here.
//
// MOSES ROUTING: the API is registered ONCE under MOSES_BASE_PATH (its
// browser-facing home, also where the workspace-tool proxy calls it).
// /health and /api/openapi.json are mounted at BOTH the canonical root
// and the base-path alias. See ../../README.md "deployed-app path
// contract".
func buildMux(basePath string, oidcEnabled bool, entriesHandler *handler.EntriesHandler) *http.ServeMux {
	mux := http.NewServeMux()

	// Health — canonical (K8s probe bypasses the ingress) + alias.
	mux.HandleFunc("/health", handler.Health)
	if basePath != "" {
		mux.HandleFunc(basePath+"/health", handler.Health)
	}

	// OpenAPI spec — canonical (platform discovery) + alias.
	mux.HandleFunc("/api/openapi.json", handler.OpenAPI)
	mux.HandleFunc("/api/spec", handler.OpenAPI)
	if basePath != "" {
		mux.HandleFunc(basePath+"/api/openapi.json", handler.OpenAPI)
		mux.HandleFunc(basePath+"/api/spec", handler.OpenAPI)
	}

	// API endpoints — registered ONCE under MOSES_BASE_PATH.
	//   /api/v1/public-info  — public (declared in access.oidc.publicPaths)
	//   /api/v1/me           — protected (OIDCAuth gates it)
	//   /api/v1/entries      — protected
	mux.HandleFunc(basePath+"/api/v1/public-info", handler.PublicInfo(oidcEnabled))
	mux.HandleFunc(basePath+"/api/v1/me", handler.Me)
	mux.HandleFunc(basePath+"/api/v1/entries", entriesHandler.Entries)

	return mux
}
