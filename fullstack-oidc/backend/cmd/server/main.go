// Command server is the fullstack-oidc template's HTTP backend. It
// demonstrates the vendored oidcauth middleware: the app is an OIDC
// relying party (BFF pattern) that gates its protected routes behind a
// Keycloak login while preserving the X-Moses-* trusted-header path for
// pod-to-pod MCP / workspace-tool calls.
//
// The route surface is the demo: one PUBLIC route (/api/v1/public-info),
// three PROTECTED routes (/api/v1/me, the per-user /api/v1/entries, and the
// tenant-shared /api/v1/shared-notes), and one PROTECTED + ROLE-GATED route
// (/api/v1/admin-area) that enforces an app role projected from the validated
// token. entries (user space) vs shared-notes (tenant space) is the demo's
// data-ownership contrast. /health and the OpenAPI spec are always public.
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

	// CHAT-0lu74: app-owned external secrets (e.g. JWT_SIGNING_KEY for
	// tokens this app mints downstream) belong in `moses-app.config.json`
	// → `secrets.external[]`. The OIDC client secret is platform-managed,
	// NOT declared here. See `skills/secrets-tutorial.md`.

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
			log.Printf("NOTE: BASE_URL is deprecated; set MOSES_BASE_PATH instead")
		}
	}

	entriesHandler := handler.NewEntriesHandler(db)
	sharedNotesHandler := handler.NewSharedNotesHandler(db)
	mux := buildMux(basePath, oidcCfg.Enabled(), entriesHandler, sharedNotesHandler)

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
func buildMux(basePath string, oidcEnabled bool, entriesHandler *handler.EntriesHandler, sharedNotesHandler *handler.SharedNotesHandler) *http.ServeMux {
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
	//   /api/v1/public-info   — PUBLIC  (declared in access.oidc.publicPaths)
	//   /api/v1/me            — PROTECTED (OIDCAuth gates it on a session)
	//   /api/v1/entries       — PROTECTED (USER space: per-user, private)
	//   /api/v1/shared-notes  — PROTECTED (TENANT space: shared by the workspace)
	//   /api/v1/admin-area    — PROTECTED + ROLE-GATED (oidc-admin role)
	//
	// The contrast between public-info and the rest is the whole demo:
	// one route any visitor can read, the others gated by the vendored
	// oidcauth middleware. entries vs shared-notes is the SECOND contrast:
	// entries is owned per OIDC subject (private), shared-notes is owned per
	// tenant (visible to the whole workspace — and to content agents post via
	// workspace tools, which arrive under the agent's X-Moses-User-ID).
	// admin-area goes one step further and enforces an app role projected from
	// the token (see handler.AdminArea).
	mux.HandleFunc(basePath+"/api/v1/public-info", handler.PublicInfo(oidcEnabled))
	mux.HandleFunc(basePath+"/api/v1/me", handler.Me)
	mux.HandleFunc(basePath+"/api/v1/entries", entriesHandler.Entries)
	mux.HandleFunc(basePath+"/api/v1/shared-notes", sharedNotesHandler.SharedNotes)
	mux.HandleFunc(basePath+"/api/v1/admin-area", handler.AdminArea)

	return mux
}
