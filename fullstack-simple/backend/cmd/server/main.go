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

	"github.com/moses-platform/fullstack-simple/internal/handler"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// CHAT-pbup Bug 5: mount API routes under MOSES_BASE_PATH so
	// /apps/<tenant>/<slug>/api/... reaches the backend even when the nginx
	// frontend forwards the prefix unchanged. Health + openapi stay at
	// root for K8s probes / WorkspaceToolProxy auto-discovery.
	basePath := strings.TrimSuffix(os.Getenv("MOSES_BASE_PATH"), "/")
	if basePath == "" {
		alias := strings.TrimSuffix(os.Getenv("BASE_URL"), "/")
		if alias != "" && strings.HasPrefix(alias, "/") {
			basePath = alias
			log.Printf("WARN: BASE_URL is deprecated; please set MOSES_BASE_PATH instead. See DEPRECATIONS.md")
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handler.Health)
	mux.HandleFunc("/api/openapi.json", handler.OpenAPI)

	items := handler.NewItemsHandler()
	registerAPI := func(prefix string) {
		mux.HandleFunc(prefix+"/api/v1/status", handler.Status)
		// CRUD example — in-memory items scoped by tenant.
		// For database-backed CRUD patterns, see the fullstack-showcase template.
		mux.HandleFunc(prefix+"/api/v1/items", items.Handle)
		mux.HandleFunc(prefix+"/api/v1/items/", items.HandleWithID)
	}
	registerAPI("")
	if basePath != "" {
		registerAPI(basePath)
	}

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

// SECURITY WARNING: This template uses permissive CORS (Allow-Origin: "*") for
// development convenience. For production deployments, restrict this to your actual
// domain(s). Example:
//   w.Header().Set("Access-Control-Allow-Origin", "https://yourdomain.com")
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
