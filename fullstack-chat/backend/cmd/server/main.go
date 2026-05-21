// Command server boots the fullstack-chat backend that demonstrates the full
// app↔Moses-Manager chat roundtrip:
//
//   - GET  /api/v1/entries        — list (frontend reads after WS push)
//   - POST /api/v1/entries        — create (Moses Manager calls this via the
//     workspace-tools wedge after a
//     chat_prompt action fires)
//   - POST /api/v1/webhooks/chat-complete — Moses fires this when the AI
//     turn finishes; HMAC-verified
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

	"github.com/moses-platform/fullstack-chat/internal/config"
	"github.com/moses-platform/fullstack-chat/internal/database"
	"github.com/moses-platform/fullstack-chat/internal/handler"
	"github.com/moses-platform/fullstack-chat/internal/middleware"
	"github.com/moses-platform/fullstack-chat/internal/mosesproxy"
)

func main() {
	log.Println("Starting fullstack-chat server...")

	// CHAT-0b6g (DEPS-B1): validate the platform-injected env contract
	// before any work begins. In prod mode (MOSES_DEPLOYED=1) missing
	// required vars are fatal; in standalone/dev they only warn.
	validatePlatformEnv(os.Exit)

	// CHAT-pxeo.12: hard fail-fast when MOSES_TENANT_ID is unset on a
	// deployed pod. Storage/lookup keys flow from this; running with the
	// "local-dev" sentinel in production silently corrupts tenant scope.
	config.Validate()

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

	// CHAT-pxeo.12: rewrite legacy 'local-dev'/'default'/'' tenant rows
	// once the schema is in place but before the listener starts. Runs
	// every boot (idempotent) so data lands under the real tenant on the
	// first deploy after this rollout. Failure is fatal — we'd rather
	// CrashLoopBackOff than serve under inconsistent tenant state.
	if err := database.MigrateTenant(db, config.SelfTenantID()); err != nil {
		log.Fatalf("CHAT-pxeo.12 tenant migration failed: %v", err)
	}

	entries := handler.NewEntriesHandler(db)
	chatWebhook := handler.NewChatWebhookHandler(db)

	// CHAT-pbup / CHAT-8qiu0: read MOSES_BASE_PATH (canonical) with BASE_URL as
	// the deprecated alias. API routes are registered ONCE under this prefix
	// (see buildMux). basePath is "" (root) or "/foo/bar" (no trailing slash).
	basePath := strings.TrimSuffix(os.Getenv("MOSES_BASE_PATH"), "/")
	if basePath == "" {
		alias := strings.TrimSuffix(os.Getenv("BASE_URL"), "/")
		if alias != "" && strings.HasPrefix(alias, "/") {
			basePath = alias
			log.Printf("WARN: BASE_URL is deprecated; please set MOSES_BASE_PATH instead.")
		}
	}

	// CHAT-pswm.8/.9 — mosesproxy. The in-iframe SDK at
	// /api/v1/sdk/iframe-sdk.js POSTs to InvokePath (same-origin under
	// nginx subpath routing) with the user's access_token cookie. The
	// proxy extracts the JWT and forwards pod-to-pod to moses-backend
	// at ${MOSES_INTERNAL_API_BASE}, preserving the user's identity so
	// the resulting chat_conversation lands with user_id populated and
	// shows up in the user's Moses-Manager sidebar.
	//
	// RequireRequestedWith=true closes the same-origin CSRF window on
	// /__moses/invoke: vanilla <form>/<img> CSRF gadgets cannot set the
	// custom X-Requested-With header the SDK emits, so a drive-by
	// attacker can't ride the user's cookie into a free invoke. The SDK
	// adds the header in CHAT-pswm.9.
	proxyCfg := mosesproxy.ConfigFromEnv()
	proxyCfg.RequireRequestedWith = true

	mux := buildMux(basePath, entries, chatWebhook, proxyCfg)
	var h http.Handler = mux
	h = middleware.Logging(h)
	h = middleware.MosesHeaders(h)
	h = middleware.CORS(h)

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
		log.Printf("Server listening on port %s (basePath=%q)", port, basePath)
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

// buildMux wires every route this server serves. Single source of truth
// shared by main() and the cmd/server tests.
//
// MOSES ROUTING. Moses deploys apps at a sub-path; the pod receives that
// path in MOSES_BASE_PATH and the ingress does NOT strip it. The platform's
// workspace-tool proxy calls the app's API *under* MOSES_BASE_PATH, so the
// API (including the mosesproxy InvokePath the in-iframe SDK posts to) is
// registered ONCE there — basePath is "" for standalone/dev deploys and
// "/apps/<tenant>/<slug>" for Moses sub-path deploys.
//
//   - API routes: single registration under basePath (its browser-facing home).
//   - /health: registered at BOTH /health and {basePath}/health — the kubelet
//     probe hits the canonical /health (it bypasses the ingress), while a
//     sub-path caller reaching the pod still gets a 200.
//   - /api/openapi.json: registered at BOTH /api/openapi.json and
//     {basePath}/api/openapi.json — the platform's discovery hook uses the
//     canonical path; the base-path alias keeps it reachable through the
//     frontend nginx, which forwards the prefix unchanged (CHAT-yfmwv).
func buildMux(basePath string, entries *handler.EntriesHandler, chatWebhook *handler.ChatWebhookHandler, proxyCfg mosesproxy.Config) *http.ServeMux {
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
	mux.HandleFunc("/api/spec", handler.OpenAPI)
	if basePath != "" {
		mux.HandleFunc(basePath+"/api/openapi.json", handler.OpenAPI)
		mux.HandleFunc(basePath+"/api/spec", handler.OpenAPI)
	}

	// API endpoints — registered ONCE under MOSES_BASE_PATH.
	mux.HandleFunc(basePath+"/api/v1/entries", entries.Entries)
	mux.HandleFunc(basePath+"/api/v1/webhooks/chat-complete", chatWebhook.Handle)
	mux.HandleFunc(basePath+mosesproxy.InvokePath, mosesproxy.NewHandler(proxyCfg))

	return mux
}
