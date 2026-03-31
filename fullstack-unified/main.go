package main

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

//go:embed api/openapi.json
var openapiSpec []byte

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	baseURL := strings.TrimSuffix(os.Getenv("BASE_URL"), "/")

	mux := http.NewServeMux()

	// Health check — always at root (Kubernetes probes bypass ingress)
	mux.HandleFunc("/health", handleHealth)

	// OpenAPI spec — served at root for probe discovery
	mux.HandleFunc("/api/openapi.json", handleOpenAPI)

	// API endpoints — prefixed with BASE_URL for ingress compatibility
	mux.HandleFunc(baseURL+"/api/v1/status", handleStatus)

	// Static files — served at BASE_URL prefix via Go embed
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to create static sub-filesystem: %v", err)
	}
	fileServer := http.FileServer(http.FS(staticFS))
	if baseURL != "" {
		mux.Handle(baseURL+"/", http.StripPrefix(baseURL, fileServer))
	} else {
		mux.Handle("/", fileServer)
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      withMosesHeaders(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.Printf("Server starting on :%s (BASE_URL=%s)", port, baseURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced shutdown: %v", err)
	}
	log.Println("Server stopped")
}

// mosesContext holds Moses platform headers extracted from the request.
type mosesContext struct {
	TenantID  string `json:"tenant_id"`
	UserID    string `json:"user_id"`
	ChartID   string `json:"chart_id"`
	RequestID string `json:"request_id"`
}

func getMosesContext(r *http.Request) mosesContext {
	return mosesContext{
		TenantID:  r.Header.Get("X-Moses-Tenant-ID"),
		UserID:    r.Header.Get("X-Moses-User-ID"),
		ChartID:   r.Header.Get("X-Moses-Chart-ID"),
		RequestID: r.Header.Get("X-Moses-Request-ID"),
	}
}

// withMosesHeaders adds CORS headers for development convenience.
//
// SECURITY WARNING: This template uses permissive CORS (Allow-Origin: "*") for
// development convenience. For production deployments, restrict this to your actual
// domain(s). Example:
//   w.Header().Set("Access-Control-Allow-Origin", "https://yourdomain.com")
// See: https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS
func withMosesHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

var startTime = time.Now()

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "healthy",
		"service": "fullstack-unified",
	})
}

func handleOpenAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(openapiSpec)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	mc := getMosesContext(r)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"app":     "fullstack-unified",
		"version": "1.0.0",
		"uptime":  time.Since(startTime).Round(time.Second).String(),
		"moses":   mc,
		"env": map[string]string{
			"port":     os.Getenv("PORT"),
			"base_url": os.Getenv("BASE_URL"),
		},
	})
}
