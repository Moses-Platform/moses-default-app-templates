package main

import (
	"context"
	"embed"
	"encoding/json"
	"html/template"
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

// indexTemplate is parsed once at startup from the embedded static/index.html.
// It renders the Moses browser-logger meta tag using MOSES_CHART_ID,
// MOSES_DEPLOYMENT_ID, and MOSES_API_BASE env vars (BLF-J / CHAT-ry35). When
// the env vars are absent the tag still renders but with empty values, and
// the snippet's readConfig() short-circuits to a no-op.
var indexTemplate *template.Template

// indexContext is the data struct rendered into static/index.html. The
// fields match the data-* attributes on the <meta name="moses-config"> tag.
type indexContext struct {
	ChartID      string
	DeploymentID string
	APIBase      string
}

// loadIndexTemplate parses the embedded static/index.html into indexTemplate.
// Idempotent — safe to call from tests.
func loadIndexTemplate() {
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Printf("WARN: failed to create static sub-filesystem: %v", err)
		return
	}
	data, err := fs.ReadFile(staticFS, "index.html")
	if err != nil {
		log.Printf("WARN: failed to read embedded static/index.html: %v", err)
		return
	}
	tmpl, err := template.New("index.html").Parse(string(data))
	if err != nil {
		log.Printf("WARN: failed to parse static/index.html as template; serving raw: %v", err)
		return
	}
	indexTemplate = tmpl
}

// renderIndex renders indexTemplate into w with the given context. Returns
// whether the template was rendered (false → caller should fall back to the
// file server). Extracted for test coverage of the BLF-J substitution.
func renderIndex(w http.ResponseWriter, ctx indexContext) bool {
	if indexTemplate == nil {
		return false
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := indexTemplate.Execute(w, ctx); err != nil {
		log.Printf("WARN: failed to render index.html template: %v", err)
	}
	return true
}

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

	// Parse index.html as a Go template once at startup so the BLF-J browser
	// logger meta tag can be filled with chart/deployment IDs from the env
	// without re-reading the file per request. If parsing fails the index
	// handler falls back to serving the raw file (the snippet's readConfig()
	// will no-op on the literal {{.…}} placeholders).
	loadIndexTemplate()

	fileServer := http.FileServer(http.FS(staticFS))

	// indexHandler renders index.html through the parsed template (BLF-J),
	// falling back to the file server for every other asset (CSS, JS, fav-
	// icon, etc.). It is mounted AFTER http.StripPrefix(baseURL) so the path
	// here is always relative to the embedded static FS root.
	indexHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" || r.URL.Path == "/" || r.URL.Path == "/index.html" {
			if renderIndex(w, indexContext{
				ChartID:      os.Getenv("MOSES_CHART_ID"),
				DeploymentID: os.Getenv("MOSES_DEPLOYMENT_ID"),
				APIBase:      os.Getenv("MOSES_API_BASE"),
			}) {
				return
			}
		}
		fileServer.ServeHTTP(w, r)
	}

	if baseURL != "" {
		mux.Handle(baseURL+"/", http.StripPrefix(baseURL, http.HandlerFunc(indexHandler)))
	} else {
		mux.HandleFunc("/", indexHandler)
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
