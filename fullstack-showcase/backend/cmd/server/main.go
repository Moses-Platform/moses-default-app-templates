package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/moses-platform/fullstack-showcase/internal/database"
	"github.com/moses-platform/fullstack-showcase/internal/handler"
	"github.com/moses-platform/fullstack-showcase/internal/middleware"
)

func main() {
	log.Println("Starting Moses Showcase server...")

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

	// Create handlers
	notesHandler := handler.NewNotesHandler(db)
	usersHandler := handler.NewUsersHandler()

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
	mux.HandleFunc("/api/v1/moses-info", handler.MosesInfo)
	mux.HandleFunc("/api/v1/capabilities", handler.ListCapabilities)
	mux.HandleFunc("/api/v1/capabilities/", handler.GetCapability)
	mux.HandleFunc("/api/v1/notes", notesHandler.Notes)
	mux.HandleFunc("/api/v1/notes/", notesHandler.Notes)
	mux.HandleFunc("/api/v1/users", usersHandler.Users)

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
