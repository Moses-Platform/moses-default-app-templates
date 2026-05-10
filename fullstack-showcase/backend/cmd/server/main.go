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

	"github.com/moses-platform/fullstack-showcase/internal/config"
	"github.com/moses-platform/fullstack-showcase/internal/database"
	"github.com/moses-platform/fullstack-showcase/internal/handler"
	"github.com/moses-platform/fullstack-showcase/internal/middleware"
)

func main() {
	log.Println("Starting Moses Showcase server...")

	// CHAT-pxeo.12: hard fail-fast when MOSES_TENANT_ID is unset on a
	// deployed pod. Storage/lookup keys flow from this env via
	// internal/config.SelfTenantID().
	config.Validate()

	// Connect to database
	dbConfig := database.NewConfigFromEnv()
	log.Printf("Connecting to database at %s:%s...", dbConfig.Host, dbConfig.Port)

	db, err := database.Connect(dbConfig)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()
	log.Println("Connected to database")

	// Run migrations
	if err := database.Migrate(db); err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}
	log.Println("Database migration complete")

	// CHAT-pxeo.12: rewrite legacy 'local-dev'/'default'/'' tenant rows
	// once schema is in place but BEFORE the listener starts. Idempotent.
	if err := database.MigrateTenant(db, config.SelfTenantID()); err != nil {
		log.Fatalf("CHAT-pxeo.12 tenant migration failed: %v", err)
	}

	// Create handlers
	notesHandler := handler.NewNotesHandler(db)
	usersHandler := handler.NewUsersHandler()

	// CHAT-pbup Bug 5: mount API routes under MOSES_BASE_PATH for sub-path
	// deploys. /health + /api/openapi.json stay at root for K8s probes and
	// WorkspaceToolProxy auto-discovery.
	basePath := strings.TrimSuffix(os.Getenv("MOSES_BASE_PATH"), "/")
	if basePath == "" {
		alias := strings.TrimSuffix(os.Getenv("BASE_URL"), "/")
		if alias != "" && strings.HasPrefix(alias, "/") {
			basePath = alias
			log.Printf("WARN: BASE_URL is deprecated; please set MOSES_BASE_PATH instead. See DEPRECATIONS.md")
		}
	}

	// Create router
	mux := http.NewServeMux()

	// Apply middleware
	var h http.Handler = mux
	h = middleware.Logging(h)
	h = middleware.MosesHeaders(h)
	h = middleware.CORS(h)

	// Register routes
	mux.HandleFunc("/health", handler.Health)
	mux.HandleFunc("/api/openapi.json", handler.OpenAPI)
	mux.HandleFunc("/api/spec", handler.OpenAPI)

	registerAPI := func(prefix string) {
		mux.HandleFunc(prefix+"/api/v1/moses-info", handler.MosesInfo)
		mux.HandleFunc(prefix+"/api/v1/capabilities", handler.ListCapabilities)
		mux.HandleFunc(prefix+"/api/v1/capabilities/", handler.GetCapability)
		mux.HandleFunc(prefix+"/api/v1/notes", notesHandler.Notes)
		mux.HandleFunc(prefix+"/api/v1/notes/", notesHandler.Notes)
		mux.HandleFunc(prefix+"/api/v1/users", usersHandler.Users)
	}
	registerAPI("")
	if basePath != "" {
		registerAPI(basePath)
	}

	// Create server
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

	// Start server in goroutine
	go func() {
		log.Printf("Server listening on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Graceful shutdown
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
