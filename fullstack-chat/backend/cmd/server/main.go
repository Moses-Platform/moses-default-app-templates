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

	"github.com/moses-platform/fullstack-chat/internal/database"
	"github.com/moses-platform/fullstack-chat/internal/handler"
	"github.com/moses-platform/fullstack-chat/internal/middleware"
)

func main() {
	log.Println("Starting fullstack-chat server...")

	// CHAT-0b6g (DEPS-B1): validate the platform-injected env contract
	// before any work begins. In prod mode (MOSES_DEPLOYED=1) missing
	// required vars are fatal; in standalone/dev they only warn.
	validatePlatformEnv(os.Exit)

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

	registerAPI := func(prefix string) {
		mux.HandleFunc(prefix+"/api/v1/entries", entries.Entries)
		mux.HandleFunc(prefix+"/api/v1/webhooks/chat-complete", chatWebhook.Handle)
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
