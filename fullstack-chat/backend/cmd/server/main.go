// Command server boots the fullstack-chat backend that demonstrates the full
// app↔Moses-Manager chat roundtrip:
//
//   - GET  /api/v1/entries        — list (frontend reads after WS push)
//   - POST /api/v1/entries        — create (Moses Manager calls this via the
//                                     workspace-tools wedge after a
//                                     chat_prompt action fires)
//   - POST /api/v1/webhooks/chat-complete — Moses fires this when the AI
//                                     turn finishes; HMAC-verified
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
	mosesproxy "github.com/moses-platform/moses-templates-shared/mosesproxy-go"
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

	// Mount API routes under MOSES_BASE_PATH for sub-path deploys.
	// /health + /api/openapi.json stay at root for K8s probes and
	// WorkspaceToolProxy auto-discovery.
	basePath := strings.TrimSuffix(os.Getenv("MOSES_BASE_PATH"), "/")
	if basePath == "" {
		alias := strings.TrimSuffix(os.Getenv("BASE_URL"), "/")
		if alias != "" && strings.HasPrefix(alias, "/") {
			basePath = alias
			log.Printf("WARN: BASE_URL is deprecated; please set MOSES_BASE_PATH instead.")
		}
	}

	mux := http.NewServeMux()
	var h http.Handler = mux
	h = middleware.Logging(h)
	h = middleware.MosesHeaders(h)
	h = middleware.CORS(h)

	mux.HandleFunc("/health", handler.Health)
	mux.HandleFunc("/api/openapi.json", handler.OpenAPI)
	mux.HandleFunc("/api/spec", handler.OpenAPI)

	// CHAT-pswm.8/.9 — mosesproxy. The in-iframe SDK at
	// /api/v1/sdk/iframe-sdk.js POSTs to this path (same-origin under
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
	registerAPI := func(prefix string) {
		mux.HandleFunc(prefix+"/api/v1/entries", entries.Entries)
		mux.HandleFunc(prefix+"/api/v1/webhooks/chat-complete", chatWebhook.Handle)
		mux.HandleFunc(prefix+mosesproxy.InvokePath, mosesproxy.NewHandler(proxyCfg))
	}
	registerAPI("")
	if basePath != "" {
		registerAPI(basePath)
	}

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
